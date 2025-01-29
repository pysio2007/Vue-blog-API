package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	cacheDir = "./cache"
	// 当使用 Minio 时禁用缓存
	cacheEnabled = os.Getenv("USE_MINIO_STORAGE") != "true"
)

// init 函数会在包被导入时自动执行
func init() {
	if !cacheEnabled {
		return
	}
	// 确保缓存目录存在
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("Failed to create cache directory: %v\n", err)
	}
}

// InitCache 显式初始化缓存目录
func InitCache() error {
	if !cacheEnabled {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %v", err)
	}
	return nil
}

func ImageExistsInCache(hash string) bool {
	if !cacheEnabled {
		return false
	}
	_, err := os.Stat(filepath.Join(cacheDir, hash+".webp"))
	return err == nil
}

func SaveImageToCache(hash string, data []byte) error {
	if !cacheEnabled {
		return nil
	}
	return ioutil.WriteFile(filepath.Join(cacheDir, hash+".webp"), data, 0644)
}

func LoadImageFromCache(hash string) ([]byte, error) {
	if !cacheEnabled {
		return nil, fmt.Errorf("cache is disabled")
	}
	return ioutil.ReadFile(filepath.Join(cacheDir, hash+".webp"))
}

func DeleteImageFromCache(hash string) error {
	if !cacheEnabled {
		return nil
	}
	return os.Remove(filepath.Join(cacheDir, hash+".webp"))
}
