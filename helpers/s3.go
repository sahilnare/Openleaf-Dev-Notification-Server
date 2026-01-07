package helpers

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	S3Client    *s3.Client
	region      = "ap-south-1"
	accessToken = "AKIAUE4EUPTG6UPYCL5I"
	secretToken = "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"
	bucketName  = "b2b-openleaf-bucket-1"
)

// InitS3 initializes the S3 client
func InitS3() {

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessToken,
			secretToken,
			"",
		)),
	)
	if err != nil {
		log.Panic("InitS3 error", err)
		if CombinedLogger != nil {
			LogException("S3 initialization failed", map[string]any{
				"error":  err.Error(),
				"region": region,
			})
		}
		panic(err)
	}
	S3Client = s3.NewFromConfig(cfg)
	if CombinedLogger != nil {
		LogInfo("S3 client initialized successfully", map[string]any{
			"region": region,
			"bucket": bucketName,
		})
	}
}

// UploadBytesToS3 uploads byte data to S3 and returns the HTTPS URL
func UploadBytesToS3(data []byte, fileName, contentType string) (string, error) {
	if S3Client == nil {
		InitS3()
	}

	_, err := S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		if CombinedLogger != nil {
			LogException("S3 bytes upload failed", map[string]any{
				"error":       err.Error(),
				"fileName":    fileName,
				"contentType": contentType,
			})
		}
		return "", fmt.Errorf("S3 upload error: %w", err)
	}

	fileUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucketName, region, fileName)

	if CombinedLogger != nil {
		LogInfo("S3 bytes upload successful", map[string]any{
			"fileName":    fileName,
			"fileUrl":     fileUrl,
			"contentType": contentType,
		})
	}

	return fileUrl, nil
}
