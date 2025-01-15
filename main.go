package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"pysio.online/blog_api/handlers"
	"pysio.online/blog_api/middleware"
	"pysio.online/blog_api/models"
	"pysio.online/blog_api/utils"
)

func loadEnv() {
	// 尝试多个位置的 .env 文件
	locations := []string{
		".env",                                   // 当前目录
		"../.env",                                // 上级目录
		filepath.Join(os.Getenv("HOME"), ".env"), // 用户主目录
		"/etc/blog-api/.env",                     // 系统配置目录
	}

	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		locations = append([]string{
			filepath.Join(execDir, ".env"),
			filepath.Join(execDir, "config", ".env"),
		}, locations...)
	}

	var loadError error
	for _, location := range locations {
		if err := godotenv.Load(location); err == nil {
			log.Printf("Loaded env from: %s", location)
			return
		} else {
			loadError = err
		}
	}

	log.Printf("Warning: No .env file found in any location. Last error: %v", loadError)
	log.Printf("Current working directory: %s", getCurrentDir())
	logEnvironmentStatus()
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

func logEnvironmentStatus() {
	vars := []string{"TOKEN", "STEAM_API_KEY", "STEAM_ID", "MONGODB_URI", "ADMIN_TOKEN"}
	log.Println("Environment variables status:")
	for _, v := range vars {
		value := os.Getenv(v)
		if value != "" {
			// 对于 TOKEN，显示具体值以便调试
			if v == "TOKEN" {
				log.Printf("%s: [Set: %s]", v, value)
			} else {
				log.Printf("%s: [Set]", v)
			}
		} else {
			log.Printf("%s: [Not Set]", v)
		}
	}
}

func main() {
	// 初始化缓存目录
	if err := utils.InitCache(); err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}

	// 加载环境变量
	loadEnv()

	// 检查必需的环境变量
	if err := utils.CheckRequiredEnvVars(); err != nil {
		log.Printf("Environment variables warning: %v", err)
	}

	// 初始化数据库连接
	if err := models.InitDB(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 创建 Gin 实例
	r := gin.Default()

	// 配置中间件
	r.Use(middleware.CORS())
	r.Use(middleware.CountAPICall())

	// 配置路由
	r.GET("/", handlers.Home)
	r.GET("/fastfetch", handlers.Fastfetch)
	r.POST("/heartbeat", handlers.Heartbeat)
	r.GET("/check", handlers.Check)
	r.GET("/steam_status", handlers.SteamStatus)
	r.GET("/ipcheck", handlers.IPCheck)
	r.GET("/random_image", handlers.RandomImage)
	r.GET("/api_stats", handlers.GetAPIStats)
	r.GET("/api_stats/:key", handlers.GetAPIStatsByKey)
	r.GET("/images/count", handlers.GetImageCount)
	r.GET("/images/list", handlers.GetImageList)
	r.POST("/images/add", middleware.VerifyAdminToken(), handlers.AddImage)
	r.DELETE("/images/:hash", middleware.VerifyAdminToken(), handlers.DeleteImage)
	r.GET("/images/:hash", handlers.GetImage)
	r.GET("/i/:hash", handlers.GetImageByHash)
	r.GET("/egg", handlers.Egg)
	r.GET("/404", handlers.NotFound)

	r.GET("/50x", handlers.ServerError)

	// 添加新的路由
	adminGroup := r.Group("/admin")
	adminGroup.Use(middleware.VerifyAdminToken())
	{
		adminGroup.POST("/refcache", handlers.RefreshCache)
	}

	// 启动服务器
	r.Run(":5000")
}
