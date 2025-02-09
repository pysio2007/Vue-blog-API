package middleware

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// GithubAPIProxyMiddleware Github API 代理中间件
func GithubAPIProxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 检查是否是 Github API 请求路径
		if !strings.HasPrefix(path, "/githubapi/") {
			c.Next()
			return
		}

		// 从路径中提取实际的 Github API URL
		fullURL := strings.TrimPrefix(path, "/githubapi/")
		targetURL, err := url.Parse(fullURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
			c.Abort()
			return
		}

		// 检查是否在白名单中
		allowed := false
		allowedPaths := viper.GetStringSlice("github.allowed_paths")
		for _, allowedPath := range allowedPaths {
			if strings.Contains(targetURL.Host+targetURL.Path, allowedPath) {
				allowed = true
				break
			}
		}

		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "This API endpoint is not allowed",
			})
			c.Abort()
			return
		}

		// 创建反向代理
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.Director = func(req *http.Request) {
			req.URL = targetURL
			req.Host = targetURL.Host

			// 设置必要的请求头
			req.Header.Set("Accept", "application/vnd.github.v3+json")
			// 从配置文件获取 token
			if token := viper.GetString("github.token"); token != "" {
				req.Header.Set("Authorization", "token "+token)
			}
		}

		// 错误处理
		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": "Github API proxy error: " + err.Error(),
			})
		}

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
