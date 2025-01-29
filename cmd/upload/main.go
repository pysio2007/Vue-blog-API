package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

var (
	folderPath  string
	apiEndpoint string
	adminToken  string
	concurrent  int
)

func init() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// 命令行参数
	flag.StringVar(&folderPath, "folder", "", "图片文件夹路径")
	flag.StringVar(&apiEndpoint, "api", "https://blogapi.pysio.online", "API 端点")
	flag.StringVar(&adminToken, "token", os.Getenv("ADMIN_TOKEN"), "管理员令牌")
	flag.IntVar(&concurrent, "concurrent", 5, "并发上传数量")
	flag.Parse()

	if folderPath == "" {
		log.Fatal("请指定图片文件夹路径 (-folder)")
	}

	if adminToken == "" {
		log.Fatal("请设置管理员令牌 (ADMIN_TOKEN 环境变量或 -token 参数)")
	}
}

func main() {
	// 获取所有图片文件
	files, err := getImageFiles(folderPath)
	if err != nil {
		log.Fatalf("读取文件夹失败: %v", err)
	}

	if len(files) == 0 {
		log.Fatal("未找到图片文件")
	}

	log.Printf("找到 %d 个图片文件", len(files))

	// 创建工作通道和等待组
	fileChan := make(chan string, concurrent)
	var wg sync.WaitGroup

	// 启动上传工作线程
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				uploadFile(file)
			}
		}()
	}

	// 发送文件到工作通道
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	// 等待所有上传完成
	wg.Wait()
	log.Println("所有文件上传完成")
}

func getImageFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			// 支持的图片格式
			switch ext {
			case ".jpg", ".jpeg", ".png", ".webp", ".gif":
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

func uploadFile(filePath string) {
	log.Printf("正在上传: %s", filePath)

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("打开文件失败 %s: %v", filePath, err)
		return
	}
	defer file.Close()

	// 创建 multipart 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", filepath.Base(filePath))
	if err != nil {
		log.Printf("创建表单失败 %s: %v", filePath, err)
		return
	}

	// 复制文件内容
	if _, err = io.Copy(part, file); err != nil {
		log.Printf("写入文件内容失败 %s: %v", filePath, err)
		return
	}
	writer.Close()

	// 创建请求
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/images/add", apiEndpoint), body)
	if err != nil {
		log.Printf("创建请求失败 %s: %v", filePath, err)
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adminToken))

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("上传失败 %s: %v", filePath, err)
		return
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("上传失败 %s: 状态码 %d, 响应: %s", filePath, resp.StatusCode, string(respBody))
		return
	}

	log.Printf("上传成功: %s", filePath)
}
