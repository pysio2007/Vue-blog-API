package middleware

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// GitProxyMiddleware 处理 git clone 请求的中间件
func GitProxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 检查是否是 git clone 请求路径
		if strings.Contains(path, "/github/") || strings.Contains(path, "/gitlab/") {
			// 提取目标URL
			var platform, repoPath string
			if strings.Contains(path, "/github/") {
				parts := strings.SplitN(path, "/github/", 2)
				platform = "github.com"
				repoPath = parts[1]
			} else {
				parts := strings.SplitN(path, "/gitlab/", 2)
				platform = "gitlab.com"
				repoPath = parts[1]
			}

			// 移除URL中的https://部分
			repoPath = strings.TrimPrefix(repoPath, "https://"+platform+"/")

			// 构建目标URL
			targetURL := fmt.Sprintf("https://%s/%s", platform, repoPath)
			target, err := url.Parse(targetURL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse target URL"})
				c.Abort()
				return
			}

			// 设置反向代理
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.Director = func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = target.Path
				// 将原请求中的查询参数传递给目标请求
				req.URL.RawQuery = c.Request.URL.RawQuery
				req.Host = target.Host

				// 保持原始请求头
				if _, ok := req.Header["User-Agent"]; !ok {
					req.Header.Set("User-Agent", "git/2.0")
				}

				// 原有针对/info/refs设置Accept头的代码已被移除
				if strings.Contains(req.URL.Path, "/git-upload-pack") {
					req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
				}
			}

			proxy.ModifyResponse = func(resp *http.Response) error {
				if strings.Contains(resp.Request.URL.Path, "/info/refs") {
					resp.Header.Set("Content-Type", "application/x-git-upload-pack-advertisement")
				} else if strings.Contains(resp.Request.URL.Path, "/git-upload-pack") {
					resp.Header.Set("Content-Type", "application/x-git-upload-pack-result")
				}
				return nil
			}

			proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
				c.JSON(http.StatusBadGateway, gin.H{
					"error": fmt.Sprintf("Git proxy error: %v", err),
				})
			}

			proxy.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		c.Next()
	}
}
