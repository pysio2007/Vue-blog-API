package utils

import (
	"fmt"
	"os"
)

// CheckRequiredEnvVars 检查必需的环境变量
func CheckRequiredEnvVars() error {
	required := []string{
		"STEAM_API_KEY",
		"STEAM_ID",
		"MONGODB_URI",
		"TOKEN",
		"ADMIN_TOKEN",
	}

	missing := []string{}
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
