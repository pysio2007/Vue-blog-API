package middleware

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"pysio.online/blog_api/models"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}

		c.Next()
	}
}

func VerifyAdminToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+os.Getenv("ADMIN_TOKEN") {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func CountAPICall() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		update := bson.M{
			"$inc": bson.M{"count": 1},
			"$set": bson.M{"lastUpdated": time.Now()},
		}
		opts := options.Update().SetUpsert(true)

		_, err := models.CountsCollection.UpdateOne(
			context.Background(),
			bson.M{"key": path},
			update,
			opts,
		)

		if err != nil {
			// 只记录错误，不中断请求
			c.Error(err)
		}

		c.Next()
	}
}
