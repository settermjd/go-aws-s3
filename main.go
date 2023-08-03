package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

type S3Uploader struct {
	bucketName string
	client     *s3.Client
	session    *session.Session
}

type SimpleS3BucketItem struct {
	Key          *string
	Size         int64
	LastModified *time.Time
}

// ListFiles retrieves a list of files from an S3 bucket and returns a short
// list of them in JSON format
func (r S3Uploader) ListFiles(c *fiber.Ctx) error {
	result, err := r.client.ListObjectsV2(
		context.TODO(),
		&s3.ListObjectsV2Input{
			Bucket: aws.String(r.bucketName),
		},
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": true,
			"msg": fmt.Sprintf(
				"Couldn't list objects in bucket %s. Reason: %v\n",
				r.bucketName,
				err,
			),
		})
	}

	var contents []SimpleS3BucketItem
	for _, item := range result.Contents {
		simpleItem := SimpleS3BucketItem{
			Key:          item.Key,
			Size:         item.Size,
			LastModified: item.LastModified,
		}
		contents = append(contents, simpleItem)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"msg":   "",
		"items": contents,
	})
}

// UploadFile uploads a file received in a POST request to an S3 bucket
func (r S3Uploader) UploadFile(c *fiber.Ctx) error {
	// Attempt to retrieve the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": true,
			"msg":   err.Error(),
		})
	}

	// Attempt to access the contents of the uploaded file
	fileBuffer, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": true,
			"msg":   err.Error(),
		})
	}
	defer fileBuffer.Close()

	// Upload the file to S3.
	_, err = r.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(file.Filename),
		Body:   fileBuffer,
	})
	if err != nil {
		return c.
			Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{
				"error": true,
				"msg":   err.Error(),
			})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"msg":   "File was successfully uploaded",
	})
}

func main() {
	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Could not load environment variables. %v", err)
	}

	// Load the AWS configuration, including the credentials and region
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(os.Getenv("S3_REGION")),
	)
	if err != nil {
		log.Fatalf("Could not load the AWS credentials: %v", err)
	}

	// Create an Amazon S3 service client
	client := s3.NewFromConfig(cfg)
	s3 := S3Uploader{
		bucketName: os.Getenv("S3_BUCKET"),
		client:     client,
	}

	app := fiber.New()

	app.Use(recover.New())

	app.Post("/", s3.UploadFile)
	app.Get("/", s3.ListFiles)

	log.Fatal(app.Listen(":3000"))
}
