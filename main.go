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
	"github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

type S3BucketManager struct {
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
func (manager S3BucketManager) ListFiles(c *fiber.Ctx) error {
	result, err := manager.client.ListObjectsV2(
		context.TODO(),
		&s3.ListObjectsV2Input{
			Bucket: aws.String(manager.bucketName),
		},
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": true,
			"msg": fmt.Sprintf(
				"Couldn't list objects in bucket %s. Reason: %v\n",
				manager.bucketName,
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
func (manager S3BucketManager) UploadFile(c *fiber.Ctx) error {
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
	_, err = manager.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(manager.bucketName),
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

	SendSMS(file.Filename, manager.bucketName)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"msg":   "File was successfully uploaded",
	})
}

// DownloadFile downloads a file from an S3 bucket, then creates a temporary
// file with the downloaded file's contents, before using Fiber's Download()
// method to send the file to the client. For more information, check out
// https://docs.gofiber.io/api/ctx#download
func (manager S3BucketManager) DownloadFile(c *fiber.Ctx) error {
	// Download the file from the S3 bucket
	filename := c.Params("filename")
	log.Printf("User requested to download file: %s", filename)

	result, err := manager.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(manager.bucketName),
		Key:    aws.String(filename),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprintf("Couldn't download file %s from bucket %s. Reason: %v.\n", filename, manager.bucketName, err),
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
func (manager S3BucketManager) DeleteFile(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if len(filename) == 0 {
		return c.Status(fiber.StatusOK).
			JSON(fiber.Map{
				"error": true,
				"msg":   fmt.Sprint("The name of the file to be deleted was not provided."),
			})
	}
	log.Printf("User requested to download file: %s", filename)

	_, err := manager.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &manager.bucketName,
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

	client := s3.NewFromConfig(cfg)

	manager := S3BucketManager{
		bucketName: os.Getenv("S3_BUCKET"),
		client:     client,
	}

	app := fiber.New()

	app.Use(recover.New())

	app.Post("/", manager.UploadFile)
	app.Get("/", manager.ListFiles)
	app.Get("/:filename", manager.DownloadFile)
	app.Delete("/:filename", manager.DeleteFile)

	log.Fatal(app.Listen(":3000"))
}

func SendSMS(fileName, bucketName string) error {
	params := &api.CreateMessageParams{}
	params.SetBody(fmt.Sprintf(
		"File %s has been successfully uploaded to your S3 bucket: %s",
		fileName,
		bucketName,
	))
	params.SetFrom(os.Getenv("TWILIO_PHONE_NUMBER"))
	params.SetTo(os.Getenv("RECIPIENT_PHONE_NUMBER"))

	client := twilio.NewRestClient()
	_, err := client.Api.CreateMessage(params)

	return err
}
