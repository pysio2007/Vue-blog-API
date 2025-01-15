package utils

import (
	"os"
	"path/filepath"
)

const CacheDir = "./cache/images"

func InitCache() error {
	// 确保缓存目录存在
	if err := os.MkdirAll(CacheDir, 0755); err != nil {
		return err
	}
	return nil
}

func GetImagePath(hash string) string {
	return filepath.Join(CacheDir, hash+".webp")
}

func SaveImageToCache(hash string, data []byte) error {
	return os.WriteFile(GetImagePath(hash), data, 0644)
}

func LoadImageFromCache(hash string) ([]byte, error) {
	return os.ReadFile(GetImagePath(hash))
}

func ImageExistsInCache(hash string) bool {
	_, err := os.Stat(GetImagePath(hash))
	return err == nil
}
