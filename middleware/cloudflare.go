package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// GraphQL 查询结构
const graphqlQuery = `
query ZoneAnalytics($zoneTag: string, $since: string, $until: string) {
  viewer {
    zones(filter: { zoneTag: $zoneTag }) {
      httpRequests1dGroups(
        limit: 1
        filter: { date_geq: $since, date_leq: $until }
      ) {
        sum {
          requests
          bytes
        }
      }
    }
  }
}
`

// 修改 GraphQL 查询，使用动态时间和正确的类型
const zoneDetailsQuery = `
query ZoneDetails($zoneTag: String!, $since: Date!, $until: Date!) {
  viewer {
    zones(filter: { zoneTag: $zoneTag }) {
      httpRequests1dGroups(
        limit: 1
        filter: { date_geq: $since, date_leq: $until }
      ) {
        sum {
          requests
          bytes
        }
        dimensions {
          userAgent {
            clientRequestUserAgent
            requests
            bytes
          }
        }
      }
    }
  }
}
`

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GraphQLResponse struct {
	Data struct {
		Viewer struct {
			Zones []struct {
				HTTPRequests1DGroups []struct {
					Sum struct {
						Requests int64 `json:"requests"`
						Bytes    int64 `json:"bytes"`
					} `json:"sum"`
				} `json:"httpRequests1dGroups"`
			} `json:"zones"`
		} `json:"viewer"`
	} `json:"data"`
}

type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ZonesResponse struct {
	Result  []Zone `json:"result"`
	Success bool   `json:"success"`
	Errors  []any  `json:"errors"`
}

// 添加新的类型定义
type UAStats struct {
	UA        string `json:"ua"`
	Requests  int64  `json:"requests"`
	Bandwidth int64  `json:"bytes"`
}

// 添加全局变量用于保存 fakeID 与真实 zoneID 的映射（简单示例，非线程安全）
var fakeIDMapping = make(map[string]string)

// 添加辅助函数生成随机 fakeID（8位数字字母组合）
func generateFakeID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// 更新 CloudflareStats 函数，移除调试日志
func CloudflareStats(c *gin.Context) {
	updateFunc := func() (interface{}, error) {
		apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
		if apiToken == "" {
			return nil, fmt.Errorf("Cloudflare 鉴权信息未配置")
		}

		// 获取所有zones列表
		zonesURL := "https://api.cloudflare.com/client/v4/zones"
		req, err := http.NewRequest("GET", zonesURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var zonesRes ZonesResponse
		if err := json.NewDecoder(resp.Body).Decode(&zonesRes); err != nil {
			return nil, fmt.Errorf("获取 zones 失败: %v", err)
		}

		var totalRequests, totalBandwidth int64

		// 计算时间范围：过去24小时
		now := time.Now().UTC()
		until := now.Format("2006-01-02")
		since := now.Add(-24 * time.Hour).Format("2006-01-02")

		// 遍历每个zone获取统计数据
		for _, zone := range zonesRes.Result {
			variables := map[string]interface{}{
				"zoneTag": zone.ID,
				"since":   since,
				"until":   until,
			}

			graphqlReq := GraphQLRequest{
				Query:     graphqlQuery,
				Variables: variables,
			}

			jsonData, err := json.Marshal(graphqlReq)
			if err != nil {
				continue
			}

			req, err := http.NewRequest("POST", "https://api.cloudflare.com/client/v4/graphql", bytes.NewBuffer(jsonData))
			if err != nil {
				continue
			}

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			var graphqlRes GraphQLResponse
			if err := json.NewDecoder(resp.Body).Decode(&graphqlRes); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			// 累加统计数据
			if len(graphqlRes.Data.Viewer.Zones) > 0 && len(graphqlRes.Data.Viewer.Zones[0].HTTPRequests1DGroups) > 0 {
				sum := graphqlRes.Data.Viewer.Zones[0].HTTPRequests1DGroups[0].Sum
				totalRequests += sum.Requests
				totalBandwidth += sum.Bytes
			}
		}

		return gin.H{
			"total_requests":  totalRequests,
			"total_bandwidth": totalBandwidth,
			"timestamp":       time.Now().Format(time.RFC3339),
		}, nil
	}

	data, err := getOrSetCache("cloudflare_stats", updateFunc)
	if err != nil {
		if data != nil {
			// 如果有缓存数据，返回缓存
			c.JSON(http.StatusOK, data)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// 添加到全局类型定义部分
type DomainObj struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

// 添加缓存结构
type CloudflareCache struct {
	LastUpdate   time.Time
	LastRefresh  time.Time
	Data         interface{}
	IsRefreshing bool
	RefreshLock  sync.Mutex
}

var (
	statsCache = make(map[string]*CloudflareCache)
	cacheMutex sync.RWMutex
	cacheTTL   = 1 * time.Hour
	refreshTTL = 5 * time.Minute
	cacheDir   = "cache/cloudflare" // 添加缓存目录
)

// 检查缓存是否需要在后台更新
func needsBackgroundRefresh(cache *CloudflareCache) bool {
	return time.Since(cache.LastRefresh) > refreshTTL
}

// 异步更新缓存
func asyncRefreshCache(key string, updateFunc func() (interface{}, error)) {
	cacheMutex.RLock()
	cache, exists := statsCache[key]
	cacheMutex.RUnlock()

	if !exists || cache == nil {
		return
	}

	cache.RefreshLock.Lock()
	if cache.IsRefreshing {
		cache.RefreshLock.Unlock()
		return
	}
	cache.IsRefreshing = true
	cache.RefreshLock.Unlock()

	go func() {
		data, err := updateFunc()
		cache.RefreshLock.Lock()
		defer cache.RefreshLock.Unlock()

		if err == nil && data != nil {
			cacheMutex.Lock()
			cache.Data = data
			cache.LastRefresh = time.Now()
			cacheMutex.Unlock()
		}
		cache.IsRefreshing = false
	}()
}

// 获取或设置缓存
func getOrSetCache(key string, updateFunc func() (interface{}, error)) (interface{}, error) {
	cacheMutex.RLock()
	cache, exists := statsCache[key]
	cacheMutex.RUnlock()

	if exists && time.Since(cache.LastUpdate) < cacheTTL {
		if needsBackgroundRefresh(cache) {
			asyncRefreshCache(key, updateFunc)
		}
		return cache.Data, nil
	}

	// 缓存不存在或已过期，同步更新
	data, err := updateFunc()
	if err != nil {
		if exists {
			// 如果更新失败但有旧数据，返回旧数据
			return cache.Data, nil
		}
		return nil, err
	}

	cacheMutex.Lock()
	statsCache[key] = &CloudflareCache{
		LastUpdate:  time.Now(),
		LastRefresh: time.Now(),
		Data:        data,
	}
	cacheMutex.Unlock()

	return data, nil
}

// 持久化存储 fakeID 映射
type IDMapping struct {
	FakeID string `json:"fake_id"`
	RealID string `json:"real_id"`
	Domain string `json:"domain"`
}

var (
	idMappings    = make(map[string]IDMapping)
	idMappingFile = "id_mappings.json"
	idMutex       sync.RWMutex
)

// 加载 ID 映射
func loadIDMappings() error {
	idMutex.Lock()
	defer idMutex.Unlock()

	data, err := os.ReadFile(idMappingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var mappings []IDMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return err
	}

	for _, mapping := range mappings {
		idMappings[mapping.FakeID] = mapping
	}
	return nil
}

// 保存 ID 映射
func saveIDMappings() error {
	idMutex.RLock()
	mappings := make([]IDMapping, 0, len(idMappings))
	for _, mapping := range idMappings {
		mappings = append(mappings, mapping)
	}
	idMutex.RUnlock()

	data, err := json.Marshal(mappings)
	if err != nil {
		return err
	}

	return os.WriteFile(idMappingFile, data, 0644)
}

// 修改 ListDomains 函数
func ListDomains(c *gin.Context) {
	updateFunc := func() (interface{}, error) {
		apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
		if apiToken == "" {
			return nil, fmt.Errorf("Cloudflare 鉴权信息未配置")
		}

		// 获取所有zones列表
		zonesURL := "https://api.cloudflare.com/client/v4/zones"
		req, err := http.NewRequest("GET", zonesURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var zonesRes ZonesResponse
		if err := json.NewDecoder(resp.Body).Decode(&zonesRes); err != nil {
			return nil, fmt.Errorf("获取 zones 失败: %v", err)
		}

		domainsList := make([]DomainObj, 0, len(zonesRes.Result))
		idMutex.Lock()
		defer idMutex.Unlock()

		for _, zone := range zonesRes.Result {
			var fakeID string
			// 查找现有映射
			for id, mapping := range idMappings {
				if mapping.RealID == zone.ID {
					fakeID = id
					break
				}
			}

			// 如果没有找到映射，创建新的
			if fakeID == "" {
				fakeID = generateFakeID()
				idMappings[fakeID] = IDMapping{
					FakeID: fakeID,
					RealID: zone.ID,
					Domain: zone.Name,
				}
			}

			domainsList = append(domainsList, DomainObj{
				ID:     fakeID,
				Domain: zone.Name,
			})
		}

		// 保存映射到文件
		go saveIDMappings()

		return gin.H{"domains": domainsList}, nil
	}

	data, err := getOrSetCache("domains_list", updateFunc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// 修改 GetDomainDetails 函数
func GetDomainDetails(c *gin.Context) {
	fakeID := strings.TrimPrefix(c.Param("domain"), "/")
	if fakeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少域名 ID"})
		return
	}

	idMutex.RLock()
	mapping, exists := idMappings[fakeID]
	idMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的域名 ID"})
		return
	}

	updateFunc := func() (interface{}, error) {
		apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
		if apiToken == "" {
			return nil, fmt.Errorf("Cloudflare 鉴权信息未配置")
		}

		// 获取详细统计信息
		now := time.Now().UTC()
		until := now.Format("2006-01-02")
		since := now.Add(-24 * time.Hour).Format("2006-01-02")

		variables := map[string]interface{}{
			"zoneTag": mapping.RealID,
			"since":   since,
			"until":   until,
		}

		graphqlReq := GraphQLRequest{
			Query:     zoneDetailsQuery,
			Variables: variables,
		}

		jsonData, err := json.Marshal(graphqlReq)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", "https://api.cloudflare.com/client/v4/graphql", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result struct {
			Data struct {
				Viewer struct {
					Zones []struct {
						HTTPRequests1DGroups []struct {
							Sum struct {
								Requests int64 `json:"requests"`
								Bytes    int64 `json:"bytes"`
							} `json:"sum"`
							Dimensions struct {
								UserAgent []struct {
									UA       string `json:"clientRequestUserAgent"`
									Requests int64  `json:"requests"`
									Bytes    int64  `json:"bytes"`
								} `json:"userAgent"`
							} `json:"dimensions"`
						} `json:"httpRequests1dGroups"`
					} `json:"zones"`
				} `json:"viewer"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		var uaStats []UAStats
		if len(result.Data.Viewer.Zones) > 0 && len(result.Data.Viewer.Zones[0].HTTPRequests1DGroups) > 0 {
			group := result.Data.Viewer.Zones[0].HTTPRequests1DGroups[0]
			for _, ua := range group.Dimensions.UserAgent {
				uaStats = append(uaStats, UAStats{
					UA:        ua.UA,
					Requests:  ua.Requests,
					Bandwidth: ua.Bytes,
				})
			}

			sort.Slice(uaStats, func(i, j int) bool {
				return uaStats[i].Bandwidth > uaStats[j].Bandwidth
			})

			if len(uaStats) > 15 {
				uaStats = uaStats[:15]
			}

			response := gin.H{
				"id":              fakeID,
				"total_requests":  group.Sum.Requests,
				"total_bandwidth": group.Sum.Bytes,
				"top_ua":          uaStats,
			}

			// 设置缓存
			setCache("domain_"+fakeID, response)
			return response, nil
		} else {
			return nil, fmt.Errorf("未找到统计数据")
		}
	}

	data, err := getOrSetCache("domain_"+fakeID, updateFunc)
	if err != nil {
		if data != nil {
			// 如果有缓存数据，即使更新失败也返回缓存
			c.JSON(http.StatusOK, data)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

type ZoneInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type ZoneStats struct {
	Requests  int64 `json:"requests"`
	Bandwidth int64 `json:"bandwidth"`
}

// 获取域名基本信息
func getZoneInfo(client *http.Client, apiToken, zoneID string) (*ZoneInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", zoneID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result ZoneInfo `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Result, nil
}

// 获取域名统计信息
func getZoneStats(client *http.Client, apiToken, zoneID, since, until string) (*ZoneStats, error) {
	variables := map[string]interface{}{
		"zoneTag": zoneID,
		"since":   since,
		"until":   until,
	}

	graphqlReq := GraphQLRequest{
		Query:     graphqlQuery,
		Variables: variables,
	}

	jsonData, err := json.Marshal(graphqlReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.cloudflare.com/client/v4/graphql", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data.Viewer.Zones) == 0 || len(result.Data.Viewer.Zones[0].HTTPRequests1DGroups) == 0 {
		return &ZoneStats{}, nil
	}

	stats := result.Data.Viewer.Zones[0].HTTPRequests1DGroups[0].Sum
	return &ZoneStats{
		Requests:  stats.Requests,
		Bandwidth: stats.Bytes,
	}, nil
}

// 初始化函数
func init() {
	// 创建缓存目录
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create cache directory: %v\n", err)
	}

	// 更新 idMappingFile 路径
	idMappingFile = filepath.Join(cacheDir, "id_mappings.json")

	// 加载持久化的 ID 映射
	if err := loadIDMappings(); err != nil {
		fmt.Printf("Warning: Failed to load ID mappings: %v\n", err)
	}

	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())
}

// 修改缓存相关函数，使用 CloudflareCache 的 Data 字段
func setCache(key string, data interface{}) {
	cacheMutex.Lock()
	statsCache[key] = &CloudflareCache{
		LastUpdate:  time.Now(),
		LastRefresh: time.Now(),
		Data:        data,
	}
	cacheMutex.Unlock()
}
