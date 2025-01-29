package models

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	DB               *mongo.Database
	ImagesCollection *mongo.Collection
	CountsCollection *mongo.Collection
)

type Image struct {
	Hash        string    `bson:"hash"`
	Data        []byte    `bson:"data,omitempty"`
	ContentType string    `bson:"contentType"`
	CreatedAt   time.Time `bson:"createdAt"`
	UseS3       bool      `bson:"useS3"`
}

type Count struct {
	Key         string    `bson:"key"`
	Count       int64     `bson:"count"`
	LastUpdated time.Time `bson:"lastUpdated"`
}

func InitDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取环境变量
	mongoURI := os.Getenv("MONGODB_URI")
	dbName := os.Getenv("MONGODB_DB_NAME")

	// 设置默认值
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	if dbName == "" {
		dbName = "image-store"
	}

	// 连接MongoDB
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// 测试连接
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	DB = client.Database(dbName)
	ImagesCollection = DB.Collection("images")
	CountsCollection = DB.Collection("counts")

	return nil
}
