package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	_ = godotenv.Load() // 确保加载 .env 文件
}

// CheckRequiredEnvVars 检查必需的环境变量
func CheckRequiredEnvVars() error {
	required := []string{
		"TOKEN",
		"MONGODB_URI",
		"ADMIN_TOKEN",
	}

	// 如果启用了 Minio，检查 Minio 相关的环境变量
	if os.Getenv("USE_MINIO_STORAGE") == "true" {
		required = append(required,
			"MINIO_ENDPOINT",
			"MINIO_ACCESS_KEY",
			"MINIO_SECRET_KEY",
			"MINIO_BUCKET",
		)
	}

	var missing []string
	for _, env := range required {
		if os.Getenv(env) == "" {
			missing = append(missing, env)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}
