package handlers

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"pysio.online/blog_api/models"
	"pysio.online/blog_api/utils"
)

var (
	lastHeartbeat int64
	IPINFO_TOKEN  = os.Getenv("IPINFO_TOKEN")
)

func Home(c *gin.Context) {
	c.String(http.StatusOK, "你来这里干啥 喵?")
}

func Fastfetch(c *gin.Context) {
	cmd := exec.Command("fastfetch", "-c", "all", "--logo", "none")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	output, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	coloredOutput := parseAnsiColors(string(output))
	c.JSON(http.StatusOK, gin.H{"status": "success", "output": coloredOutput})
}

func Heartbeat(c *gin.Context) {
	token := os.Getenv("TOKEN")
	if token == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TOKEN environment variable not set",
			"debug": fmt.Sprintf("Expected: %s, Received: %s",
				c.GetHeader("Authorization"),
				fmt.Sprintf("Bearer %s", token)),
		})
		return
	}

	authHeader := c.GetHeader("Authorization")
	expectedAuth := fmt.Sprintf("Bearer %s", token)

	if authHeader != expectedAuth {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token",
			"debug": fmt.Sprintf("Expected: %s, Received: %s",
				expectedAuth, authHeader),
		})
		return
	}

	lastHeartbeat = time.Now().Unix()
	c.JSON(http.StatusOK, gin.H{"message": "Heartbeat received"})
}

func Check(c *gin.Context) {
	if lastHeartbeat != 0 {
		timeDiff := time.Now().Unix() - lastHeartbeat
		c.JSON(http.StatusOK, gin.H{
			"alive":          timeDiff <= 600,
			"last_heartbeat": lastHeartbeat,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"alive":          false,
			"last_heartbeat": nil,
		})
	}
}

func SteamStatus(c *gin.Context) {
	// 在函数内部获取环境变量，而不是使用包级变量
	steamAPIKey := os.Getenv("STEAM_API_KEY")
	steamID := os.Getenv("STEAM_ID")

	// 检查环境变量
	if steamAPIKey == "" || steamID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Missing environment variables. STEAM_API_KEY: %v, STEAM_ID: %v",
				steamAPIKey != "", steamID != ""),
		})
		return
	}

	// 获取用户信息
	userDetailsUrl := fmt.Sprintf("https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=%s&steamids=%s",
		steamAPIKey, steamID)
	resp, err := http.Get(userDetailsUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Steam API request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Steam API returned status code: %d", resp.StatusCode)})
		return
	}

	// 检查 Content-Type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":        "Steam API returned non-JSON response",
			"content_type": contentType,
			"response":     string(body),
		})
		return
	}

	// 解析用户信息
	var userResult struct {
		Response struct {
			Players []struct {
				PersonaState  int    `json:"personastate"`
				GameExtraInfo string `json:"gameextrainfo,omitempty"`
				GameID        string `json:"gameid,omitempty"`
			} `json:"players"`
		} `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userResult); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to decode Steam API response: %v", err)})
		return
	}

	if len(userResult.Response.Players) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Player not found"})
		return
	}

	player := userResult.Response.Players[0]

	if player.GameExtraInfo != "" {
		// 获取游戏详细信息
		gameDetailsUrl := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&l=schinese&cc=CN", player.GameID)
		gameResp, err := http.Get(gameDetailsUrl)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer gameResp.Body.Close()

		var gameResult map[string]struct {
			Success bool `json:"success"`
			Data    struct {
				Name             string `json:"name"`
				ShortDescription string `json:"short_description"`
				HeaderImage      string `json:"header_image"`
				PriceOverview    struct {
					Final           int `json:"final"`
					Initial         int `json:"initial"`
					DiscountPercent int `json:"discount_percent"`
				} `json:"price_overview"`
			} `json:"data"`
		}

		if err := json.NewDecoder(gameResp.Body).Decode(&gameResult); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 获取游戏时长
		ownedGamesUrl := fmt.Sprintf("https://api.steampowered.com/IPlayerService/GetOwnedGames/v1/?key=%s&steamid=%s&include_appinfo=1", steamAPIKey, steamID)
		ownedResp, err := http.Get(ownedGamesUrl)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer ownedResp.Body.Close()

		var ownedResult struct {
			Response struct {
				Games []struct {
					AppID           int `json:"appid"`
					PlaytimeForever int `json:"playtime_forever"`
				} `json:"games"`
			} `json:"response"`
		}

		if err := json.NewDecoder(ownedResp.Body).Decode(&ownedResult); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 获取成就完成度
		achievementUrl := fmt.Sprintf("https://api.steampowered.com/ISteamUserStats/GetPlayerAchievements/v1/?appid=%s&key=%s&steamid=%s",
			player.GameID, steamAPIKey, steamID)
		achieveResp, err := http.Get(achievementUrl)

		var achievementPercent float64
		if err == nil && achieveResp.StatusCode == http.StatusOK {
			defer achieveResp.Body.Close()

			var achieveResult struct {
				PlayerStats struct {
					Achievements []struct {
						Achieved int `json:"achieved"`
					} `json:"achievements"`
				} `json:"playerstats"`
			}

			if err := json.NewDecoder(achieveResp.Body).Decode(&achieveResult); err == nil {
				total := len(achieveResult.PlayerStats.Achievements)
				completed := 0
				for _, ach := range achieveResult.PlayerStats.Achievements {
					if ach.Achieved == 1 {
						completed++
					}
				}
				if total > 0 {
					achievementPercent = float64(completed) * 100 / float64(total)
				}
			}
		}

		gameData := gameResult[player.GameID].Data
		var playtime int
		for _, game := range ownedResult.Response.Games {
			if strconv.Itoa(game.AppID) == player.GameID {
				playtime = game.PlaytimeForever
				break
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":                 "在游戏中",
			"game":                   player.GameExtraInfo,
			"game_id":                player.GameID,
			"description":            gameData.ShortDescription,
			"price":                  fmt.Sprintf("%.2f", float64(gameData.PriceOverview.Final)/100),
			"playtime":               fmt.Sprintf("%d小时%d分钟", playtime/60, playtime%60),
			"achievement_percentage": fmt.Sprintf("%.1f%%", achievementPercent),
		})
	} else {
		status := "离线"
		if player.PersonaState == 1 {
			status = "在线"
		}
		c.JSON(http.StatusOK, gin.H{"status": status})
	}
}

func IPCheck(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "IP 参数是必须的"})
		return
	}

	resp, err := http.Get(fmt.Sprintf("https://ipinfo.io/widget/demo/%s", ip))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// 使用备用 API
		resp, err = http.Get(fmt.Sprintf("https://ipinfo.io/%s?token=%s", ip, IPINFO_TOKEN))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
			return
		}
		defer resp.Body.Close()
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func RandomImage(c *gin.Context) {
	count, err := models.ImagesCollection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if count == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No images available"})
		return
	}

	var result models.Image
	err = models.ImagesCollection.FindOne(context.Background(), bson.M{}, options.FindOne().SetSkip(int64(time.Now().UnixNano()%count))).Decode(&result)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "image/webp")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.webp"`, result.Hash))
	c.Data(http.StatusOK, "image/webp", result.Data)
}

type lowerCount struct {
	Key         string    `json:"key"`
	Count       int64     `json:"count"`
	LastUpdated time.Time `json:"lastUpdated"`
}

func GetAPIStats(c *gin.Context) {
	stats, err := models.CountsCollection.Find(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var results []models.Count
	if err = stats.All(context.Background(), &results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	lowerResults := make([]lowerCount, len(results))
	for i, r := range results {
		lowerResults[i] = lowerCount{
			Key:         r.Key,
			Count:       r.Count,
			LastUpdated: r.LastUpdated,
		}
	}

	c.JSON(http.StatusOK, lowerResults)
}

func GetAPIStatsByKey(c *gin.Context) {
	key := c.Param("key")
	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}
	var result models.Count
	err := models.CountsCollection.FindOne(context.Background(), bson.M{"key": key}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "No stats found for this endpoint"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	lc := lowerCount{
		Key:         result.Key,
		Count:       result.Count,
		LastUpdated: result.LastUpdated,
	}
	c.JSON(http.StatusOK, lc)
}

func GetImageCount(c *gin.Context) {
	count, err := models.ImagesCollection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

type lowerImage struct {
	Hash        string    `json:"hash"`
	Data        []byte    `json:"data"`
	ContentType string    `json:"contentType"`
	CreatedAt   time.Time `json:"createdAt"`
}

func GetImageList(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 64)
	skip := (page - 1) * limit

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit).
		SetProjection(bson.M{"data": 0}) // 不返回图片数据

	cursor, err := models.ImagesCollection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var images []models.Image
	if err = cursor.All(context.Background(), &images); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	total, err := models.ImagesCollection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 将 images 转为小写 JSON 字段
	lowerImages := make([]lowerImage, len(images))
	for i, img := range images {
		lowerImages[i] = lowerImage{
			Hash:        img.Hash,
			Data:        img.Data,
			ContentType: img.ContentType,
			CreatedAt:   img.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"images": lowerImages,
		"pagination": gin.H{
			"current": page,
			"size":    limit,
			"total":   total,
		},
	})
}

func AddImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image file is required"})
		return
	}

	// 读取文件内容
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	buffer := make([]byte, file.Size)
	if _, err := src.Read(buffer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 验证图片
	if err := utils.ValidateImage(buffer); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid image: %v", err)})
		return
	}

	// 转换为WebP并计算哈希
	webpBuffer, err := utils.ConvertToWebp(buffer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to convert image to WebP"})
		return
	}

	hash := fmt.Sprintf("%x", md5.Sum(webpBuffer))

	// 检查是否已存在
	var existingImage models.Image
	err = models.ImagesCollection.FindOne(context.Background(), bson.M{"hash": hash}).Decode(&existingImage)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Image already exists",
			"hash":  hash,
		})
		return
	}

	// 保存图片
	image := models.Image{
		Hash:        hash,
		Data:        webpBuffer,
		ContentType: "image/webp",
		CreatedAt:   time.Now(),
	}

	_, err = models.ImagesCollection.InsertOne(context.Background(), image)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"hash": hash,
		"size": len(webpBuffer),
	})
}

func DeleteImage(c *gin.Context) {
	hash := c.Param("hash")
	result, err := models.ImagesCollection.DeleteOne(context.Background(), bson.M{"hash": hash})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Image deleted successfully"})
}

func GetImage(c *gin.Context) {
	hash := c.Param("hash")
	var image models.Image
	err := models.ImagesCollection.FindOne(context.Background(), bson.M{"hash": hash}).Decode(&image)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", image.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.webp"`, image.Hash))
	c.Data(http.StatusOK, image.ContentType, image.Data)
}

func GetImageByHash(c *gin.Context) {
	hash := c.Param("hash")
	var image models.Image
	err := models.ImagesCollection.FindOne(context.Background(), bson.M{"hash": hash}).Decode(&image)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", image.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.webp"`, image.Hash))
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("ETag", fmt.Sprintf(`"%s"`, image.Hash))
	c.Header("CDN-Cache-Control", "max-age=31536000")
	c.Header("Surrogate-Control", "max-age=31536000")

	if c.GetHeader("If-None-Match") == fmt.Sprintf(`"%s"`, image.Hash) {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, image.ContentType, image.Data)
}

func Egg(c *gin.Context) {
	c.String(http.StatusOK, "Oops!")
}

func NotFound(c *gin.Context) {
	c.String(http.StatusNotFound, "404 Not Found")
}

func ServerError(c *gin.Context) {
	c.String(http.StatusInternalServerError, "Server Down")
}

func parseAnsiColors(text string) string {
	colorMap := map[string]string{
		"30": "black", "31": "red", "32": "green", "33": "yellow",
		"34": "blue", "35": "magenta", "36": "cyan", "37": "white",
		"90": "bright-black", "91": "bright-red", "92": "bright-green",
		"93": "bright-yellow", "94": "bright-blue", "95": "bright-magenta",
		"96": "bright-cyan", "97": "bright-white",
	}

	result := text
	for code, color := range colorMap {
		// 替换ANSI颜色代码为HTML标签
		result = strings.ReplaceAll(result, fmt.Sprintf("\x1b[%sm", code), fmt.Sprintf(`<span style="color:%s">`, color))
	}
	return result
}
