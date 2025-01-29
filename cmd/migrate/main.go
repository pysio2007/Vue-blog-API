package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"crypto/tls"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Image struct {
	Hash        string    `bson:"hash"`
	Data        []byte    `bson:"data,omitempty"`
	ContentType string    `bson:"contentType"`
	CreatedAt   time.Time `bson:"createdAt"`
	UseS3       bool      `bson:"useS3"`
}

type Stats struct {
	sync.Mutex
	Total     int64
	Migrated  int64
	Skipped   int64
	Failed    int64
	NoContent int64
	Deleted   int64
	Invalid   int64
}

func (s *Stats) Increment(field *int64) {
	s.Lock()
	*field++
	s.Unlock()
}

const (
	workerCount        = 20         // 增加到20个并发
	minFileSize        = 100 * 1024 // 最小文件大小（100KB）
	migrationBatchSize = 100        // 每批处理的文件数
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	ctx := context.Background()
	mongoClient, minioClient, collection := initClients(ctx)
	defer mongoClient.Disconnect(ctx)

	// 测试 Minio 连接
	testMinioConnection(ctx, minioClient)

	// 执行迁移
	stats := migrateImages(ctx, collection, minioClient)

	// 检查并清理小文件
	cleanupSmallFiles(ctx, collection, minioClient, stats)

	// 清理无效记录
	cleanupInvalidRecords(ctx, collection, minioClient, stats)

	// 打印最终统计信息
	printStats(stats)
}

func initClients(ctx context.Context) (*mongo.Client, *minio.Client, *mongo.Collection) {
	// 设置 MongoDB 连接选项
	mongoOpts := options.Client().
		ApplyURI(os.Getenv("MONGODB_URI")).
		SetServerSelectionTimeout(10 * time.Second). // 服务器选择超时
		SetConnectTimeout(10 * time.Second).         // 连接超时
		SetSocketTimeout(30 * time.Second).          // Socket 超时
		SetMaxConnIdleTime(30 * time.Second).        // 最大空闲时间
		SetRetryWrites(true).                        // 启用重试写入
		SetRetryReads(true).                         // 启用重试读取
		SetMaxPoolSize(100)                          // 连接池大小

	// 连接 MongoDB
	mongoClient, err := mongo.Connect(ctx, mongoOpts)
	if err != nil {
		log.Fatalf("Failed to create MongoDB client: %v", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 尝试 ping 数据库
	if err = mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	log.Println("Successfully connected to MongoDB")

	// 初始化 Minio 客户端
	minioClient, err := minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), ""),
		Secure: os.Getenv("MINIO_USE_SSL") == "true",
		Region: os.Getenv("MINIO_REGION"),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
		BucketLookup: minio.BucketLookupAuto,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Minio client: %v", err)
	}

	// 测试 Minio 连接并确保存储桶存在
	bucketName := os.Getenv("MINIO_BUCKET")
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		log.Printf("Failed to check bucket existence: %v", err)
		log.Printf("Minio endpoint: %s", os.Getenv("MINIO_ENDPOINT"))
		log.Printf("Bucket name: %s", bucketName)
		log.Printf("SSL enabled: %v", os.Getenv("MINIO_USE_SSL") == "true")
		log.Printf("Access key: %s", os.Getenv("MINIO_ACCESS_KEY"))
		log.Printf("Region: %s", os.Getenv("MINIO_REGION"))
		log.Fatalf("Please check your Minio configuration")
	}

	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
			Region: os.Getenv("MINIO_REGION"),
		})
		if err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
		log.Printf("Created new bucket: %s", bucketName)
	}

	// 设置存储桶策略
	policy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"s3:GetBucketLocation",
					"s3:ListBucket",
					"s3:ListBucketMultipartUploads"
				],
				"Resource": [
					"arn:aws:s3:::randomimg"
				]
			},
			{
				"Effect": "Allow",
				"Action": [
					"s3:ListMultipartUploadParts",
					"s3:PutObject",
					"s3:DeleteObject",
					"s3:GetObject"
				],
				"Resource": [
					"arn:aws:s3:::randomimg/*"
				]
			}
		]
	}`

	// 尝试设置存储桶策略
	err = minioClient.SetBucketPolicy(ctx, bucketName, policy)
	if err != nil {
		// 只记录警告，不要中断程序
		log.Printf("Warning: Failed to set bucket policy: %v", err)
		log.Printf("Continuing with existing bucket policy...")
	} else {
		log.Printf("Successfully set bucket policy")
	}

	// 测试存储桶权限
	testData := []byte("test")
	reader := bytes.NewReader(testData)
	_, err = minioClient.PutObject(ctx, bucketName, "test-permissions.txt", reader, int64(len(testData)),
		minio.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		log.Printf("Warning: Failed to write test file: %v", err)
		log.Printf("Please verify your Minio credentials and bucket permissions")
	} else {
		// 清理测试文件
		err = minioClient.RemoveObject(ctx, bucketName, "test-permissions.txt", minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("Warning: Failed to cleanup test file: %v", err)
		}
		log.Printf("Successfully tested bucket write permissions")
	}

	return mongoClient, minioClient, mongoClient.Database("image-store").Collection("images")
}

func testMinioConnection(ctx context.Context, minioClient *minio.Client) {
	bucketName := os.Getenv("MINIO_BUCKET")

	// 测试上传小文件
	testData := []byte("test")
	reader := bytes.NewReader(testData)
	_, err := minioClient.PutObject(ctx, bucketName, "test.txt", reader, int64(len(testData)),
		minio.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		log.Fatalf("Failed to upload test file: %v", err)
	}

	// 测试删除文件
	err = minioClient.RemoveObject(ctx, bucketName, "test.txt", minio.RemoveObjectOptions{})
	if err != nil {
		log.Fatalf("Failed to delete test file: %v", err)
	}

	log.Println("Minio connection test passed successfully")
}

func migrateImages(ctx context.Context, collection *mongo.Collection, minioClient *minio.Client) *Stats {
	stats := &Stats{}

	// 只获取需要迁移的文件
	filter := bson.M{
		"$or": []bson.M{
			{"useS3": false},
			{"useS3": bson.M{"$exists": false}},
		},
		"data": bson.M{"$ne": nil},
	}

	// 使用投影只获取必要的字段
	opts := options.Find().SetProjection(bson.M{
		"hash":        1,
		"data":        1,
		"contentType": 1,
		"useS3":       1,
	})

	// 获取总数
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("Failed to get total documents: %v", err)
	} else {
		log.Printf("Total documents to process: %d", total)
	}

	// 如果没有需要迁移的文件，直接返回
	if total == 0 {
		log.Println("No files need to be migrated")
		return stats
	}

	// 创建工作通道，增加缓冲区大小
	imageChan := make(chan Image, workerCount*4)
	var wg sync.WaitGroup

	// 启动工作线程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker(ctx, collection, minioClient, imageChan, stats, &wg)
	}

	// 启动进度报告协程
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				processed := stats.Migrated + stats.Skipped + stats.Failed + stats.NoContent
				percentage := float64(processed) / float64(total) * 100
				log.Printf("Progress: %.2f%% (Processed=%d/%d, Migrated=%d, Skipped=%d, Failed=%d, NoContent=%d)",
					percentage, processed, total, stats.Migrated, stats.Skipped, stats.Failed, stats.NoContent)
			}
		}
	}()

	// 获取所有图片
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Failed to query images: %v", err)
	}
	defer cursor.Close(ctx)

	// 发送图片到工作通道
	for cursor.Next(ctx) {
		stats.Increment(&stats.Total)
		var image Image
		if err := cursor.Decode(&image); err != nil {
			log.Printf("Failed to decode image: %v", err)
			stats.Increment(&stats.Failed)
			continue
		}
		imageChan <- image
	}

	// 关闭通道并等待所有工作线程完成
	close(imageChan)
	wg.Wait()
	close(done) // 停止进度报告

	return stats
}

func worker(ctx context.Context, collection *mongo.Collection, minioClient *minio.Client, imageChan <-chan Image, stats *Stats, wg *sync.WaitGroup) {
	defer wg.Done()
	bucketName := os.Getenv("MINIO_BUCKET")

	// 每个 worker 独立计数
	var localTotal, localMigrated int64

	// 批量处理的缓冲区
	var batch []Image
	batchSize := 0

	processBatch := func(images []Image) {
		if len(images) == 0 {
			return
		}

		for _, image := range images {
			// 上传到 Minio
			reader := bytes.NewReader(image.Data)
			info, err := minioClient.PutObject(ctx, bucketName, image.Hash+".webp", reader, int64(len(image.Data)),
				minio.PutObjectOptions{ContentType: "image/webp"})
			if err != nil {
				log.Printf("Worker: Failed to upload %s to Minio: %v", image.Hash, err)
				log.Printf("Details: size=%d, bucket=%s, content-type=%s", len(image.Data), bucketName, "image/webp")
				if minioErr, ok := err.(minio.ErrorResponse); ok {
					log.Printf("Minio error details: code=%s, message=%s, requestid=%s",
						minioErr.Code, minioErr.Message, minioErr.RequestID)
				}
				stats.Increment(&stats.Failed)
				continue
			}
			log.Printf("Successfully uploaded %s (size: %d)", image.Hash, info.Size)

			// 更新数据库记录
			_, err = collection.UpdateOne(ctx, bson.M{"hash": image.Hash}, bson.M{
				"$set": bson.M{
					"useS3": true,
					"data":  nil,
				},
			})
			if err != nil {
				log.Printf("Worker: Failed to update MongoDB record for %s: %v", image.Hash, err)
				_ = minioClient.RemoveObject(ctx, bucketName, image.Hash+".webp", minio.RemoveObjectOptions{})
				stats.Increment(&stats.Failed)
				continue
			}

			localMigrated++
			stats.Increment(&stats.Migrated)
		}
	}

	for image := range imageChan {
		localTotal++

		if image.UseS3 {
			stats.Increment(&stats.Skipped)
			continue
		}

		if len(image.Data) == 0 {
			stats.Increment(&stats.NoContent)
			continue
		}

		batch = append(batch, image)
		batchSize++

		if batchSize >= migrationBatchSize {
			processBatch(batch)
			batch = batch[:0]
			batchSize = 0
			log.Printf("Worker progress: Processed %d, Migrated %d", localTotal, localMigrated)
		}
	}

	// 处理剩余的批次
	processBatch(batch)
}

func cleanupSmallFiles(ctx context.Context, collection *mongo.Collection, minioClient *minio.Client, stats *Stats) {
	log.Println("\nStarting small files cleanup...")
	bucketName := os.Getenv("MINIO_BUCKET")

	// 获取所有使用 S3 的图片
	cursor, err := collection.Find(ctx, bson.M{"useS3": true})
	if err != nil {
		log.Printf("Failed to query S3 images: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var wg sync.WaitGroup
	fileChan := make(chan string, workerCount)

	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	// 启动清理工作线程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for hash := range fileChan {
				// 构建公共 URL
				url := fmt.Sprintf("https://%s/%s/%s.webp",
					os.Getenv("MINIO_ENDPOINT"),
					bucketName,
					hash)

				// 发送 HEAD 请求获取文件大小
				resp, err := httpClient.Head(url)
				if err != nil {
					log.Printf("Failed to get file info for %s: %v", hash, err)
					continue
				}
				resp.Body.Close()

				// 获取文件大小
				size := resp.ContentLength
				if size < minFileSize {
					log.Printf("Small file found: %s (size: %d bytes)", hash, size)

					// 删除 Minio 中的文件
					err = minioClient.RemoveObject(ctx, bucketName, hash+".webp", minio.RemoveObjectOptions{})
					if err != nil {
						log.Printf("Failed to delete small file %s from Minio: %v", hash, err)
						continue
					}

					// 更新数据库记录
					_, err = collection.UpdateOne(ctx, bson.M{"hash": hash}, bson.M{
						"$set": bson.M{"useS3": false},
					})
					if err != nil {
						log.Printf("Failed to update MongoDB record for deleted file %s: %v", hash, err)
						continue
					}

					stats.Increment(&stats.Deleted)
					log.Printf("Deleted small file: %s", hash)
				}
			}
		}()
	}

	// 发送文件到清理通道
	for cursor.Next(ctx) {
		var image Image
		if err := cursor.Decode(&image); err != nil {
			log.Printf("Failed to decode image during cleanup: %v", err)
			continue
		}
		fileChan <- image.Hash
	}

	close(fileChan)
	wg.Wait()
}

func cleanupInvalidRecords(ctx context.Context, collection *mongo.Collection, minioClient *minio.Client, stats *Stats) {
	log.Println("\nStarting invalid records cleanup...")
	bucketName := os.Getenv("MINIO_BUCKET")

	// 获取所有标记为 useS3=true 的记录
	cursor, err := collection.Find(ctx, bson.M{"useS3": true})
	if err != nil {
		log.Printf("Failed to query S3 images: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var wg sync.WaitGroup
	type checkItem struct {
		hash string
		id   interface{}
	}
	itemChan := make(chan checkItem, workerCount)

	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	// 启动检查线程
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemChan {
				// 构建公共 URL
				url := fmt.Sprintf("https://%s/%s/%s.webp",
					os.Getenv("MINIO_ENDPOINT"),
					bucketName,
					item.hash)

				// 发送 HEAD 请求检查文件是否存在
				resp, err := httpClient.Head(url)
				if err != nil || resp.StatusCode == http.StatusNotFound {
					if resp != nil {
						resp.Body.Close()
					}
					// 文件不存在，删除数据库记录
					log.Printf("File not found in Minio: %s, removing database record", item.hash)
					_, err := collection.DeleteOne(ctx, bson.M{"_id": item.id})
					if err != nil {
						log.Printf("Failed to delete invalid record for %s: %v", item.hash, err)
					} else {
						stats.Increment(&stats.Deleted)
						log.Printf("Deleted invalid record: %s", item.hash)
					}
					continue
				}
				resp.Body.Close()
			}
		}()
	}

	// 发送记录到检查通道
	var invalidCount int64
	for cursor.Next(ctx) {
		var image struct {
			ID   interface{} `bson:"_id"`
			Hash string      `bson:"hash"`
		}
		if err := cursor.Decode(&image); err != nil {
			log.Printf("Failed to decode image during cleanup: %v", err)
			continue
		}
		itemChan <- checkItem{hash: image.Hash, id: image.ID}
		invalidCount++
	}

	close(itemChan)
	wg.Wait()

	// 删除所有失败的记录
	result, err := collection.DeleteMany(ctx, bson.M{
		"useS3": false,
		"data":  nil,
	})
	if err != nil {
		log.Printf("Failed to delete failed records: %v", err)
	} else {
		log.Printf("Deleted %d failed records", result.DeletedCount)
		stats.Increment(&stats.Deleted)
	}

	log.Printf("Invalid records cleanup completed. Processed %d records", invalidCount)
}

func printStats(stats *Stats) {
	fmt.Printf("\nMigration and cleanup completed:\n")
	fmt.Printf("Total processed: %d\n", stats.Total)
	fmt.Printf("Successfully migrated: %d\n", stats.Migrated)
	fmt.Printf("Skipped (already in S3): %d\n", stats.Skipped)
	fmt.Printf("No content: %d\n", stats.NoContent)
	fmt.Printf("Failed: %d\n", stats.Failed)
	fmt.Printf("Small files deleted: %d\n", stats.Deleted)
	fmt.Printf("Invalid records removed: %d\n", stats.Invalid)
}
