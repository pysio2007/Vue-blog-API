package utils

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"

	_ "golang.org/x/image/webp" // 用于解码 webp
)

func ConvertToWebp(buffer []byte) ([]byte, error) {
	// 先解码，确保图片有效
	_, format, err := image.DecodeConfig(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("decode failed: %v", err)
	}

	// 如果已是 webp，直接返回
	if format == "webp" {
		return buffer, nil
	}

	// 创建临时输入文件
	tmpIn, err := os.CreateTemp("", "input-*")
	if err != nil {
		return nil, fmt.Errorf("create temp in file failed: %v", err)
	}
	defer os.Remove(tmpIn.Name())
	defer tmpIn.Close()

	if _, err = tmpIn.Write(buffer); err != nil {
		return nil, fmt.Errorf("write tmpIn failed: %v", err)
	}

	// 创建临时输出文件
	tmpOut, err := os.CreateTemp("", "output-*.webp")
	if err != nil {
		return nil, fmt.Errorf("create temp out file failed: %v", err)
	}
	defer os.Remove(tmpOut.Name())
	defer tmpOut.Close()

	// 调用系统 cwebp 命令，质量默认 80
	cmd := exec.Command("cwebp", "-q", "80", tmpIn.Name(), "-o", tmpOut.Name())
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cwebp failed: %v", err)
	}

	// 读取生成的 WebP
	webpData, err := os.ReadFile(tmpOut.Name())
	if err != nil {
		return nil, fmt.Errorf("read tmpOut failed: %v", err)
	}

	return webpData, nil
}

func ValidateImage(buffer []byte) error {
	r := io.LimitReader(bytes.NewReader(buffer), 20*1024*1024)
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return fmt.Errorf("invalid image: %v", err)
	}
	switch format {
	case "jpeg", "png", "gif", "webp":
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}
