package main

import (
	"context"
	"fmt"
	"io"
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

// DownloadFile downloads a file from an S3 bucket, then creates a temporary
// file with the downloaded file's contents, before using Fiber's Download()
// method to send the file to the client. For more information, check out
// https://docs.gofiber.io/api/ctx#download
func (r S3Uploader) DownloadFile(c *fiber.Ctx) error {
	// Download the file from the S3 bucket
	filename := c.Params("filename")
	log.Printf("User requested to download file: %s", filename)

	result, err := r.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(filename),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprintf("Couldn't download file %s from bucket %s. Reason: %v.\n", filename, r.bucketName, err),
			})
	}
	defer result.Body.Close()

	// Create a temporary file from the downloaded file's contents
	file, err := os.CreateTemp("/tmp/", "temp-*.txt")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprintf("Couldn't create a temporary file to store the downloaded file. Reason: %v.\n", err),
			})
	}
	defer os.Remove(file.Name())

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprintf("Could not read contents of downloaded file. Reason: %v.\n", err),
			})
	}

	byteCount, err := file.Write(body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprintf("Could not create temporary file with contents of downloaded file. Reason: %v.\n", err),
			})
	}
	log.Printf("Wrote %d bytes to the temporary file: %s.", byteCount, file.Name())

	log.Printf("Ready to download the file: %s from path: %s", filename, file.Name())

	// Send the file to the client
	return c.Download(file.Name(), filename)
}

// DeleteFile deletes a single file/object from an S3 bucket.
func (r S3Uploader) DeleteFile(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if len(filename) == 0 {
		return c.Status(fiber.StatusOK).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprint("The name of the file to be deleted was not provided."),
			})
	}
	log.Printf("User requested to download file: %s", filename)

	_, err := r.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &r.bucketName,
		Key:    &filename,
	})
	if err != nil {
		return c.Status(fiber.StatusOK).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprintf("File %s could not be deleted. Reason: %v", filename, err),
			})
	}

	return c.Status(fiber.StatusOK).
		JSON(fiber.Map{
			"error": false,
			"msg":   fmt.Sprintf("File (%s) has been deleted", filename),
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
	app.Get("/:filename", s3.DownloadFile)
	app.Delete("/:filename", s3.DeleteFile)

	log.Fatal(app.Listen(":3000"))
}
