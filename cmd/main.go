package main

import (
	"Notification-Server/controllers"
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"Notification-Server/scheduler"
	"Notification-Server/workers"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(".env"); err != nil {
		panic("Error loading .env file: " + err.Error())
	}

	port := os.Getenv("NOTIFICATION_SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	log.Println("server starting")

	// # Setup CORS
	r.Use(corsMiddleware())

	log.Println("server setup cors")

	// # Connect database
	db.Connect()

	log.Println("server connect database")

	// # Setup logger
	helpers.InitLogger()

	log.Println("server setup logger")

	// # Setup S3
	helpers.InitS3()
	log.Println("S3 Successfully Connected")

	// # Setup email configuration
	helpers.InitEmailConfig()

	log.Println("server setup email configuration")

	// # Setup queues
	setupQueues()

	log.Println("server setup queues")

	// # Setup scheduler
	setupScheduler()

	time.Sleep(2 * time.Second)

	log.Println("server setup scheduler")

	// # Setup workers
	setupWorkers()

	time.Sleep(2 * time.Second)

	log.Println("server setup workers")

	// Maintenance route (must be before auth middleware)
	r.GET("/api/admin/maintenance", func(c *gin.Context) {

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.ServerResponse{
				Success:    false,
				StatusCode: http.StatusUnauthorized,
				Message:    "Unauthorized",
			})
			return
		} else if authHeader != "f50e16a16a69968bd1da889be47b6bc9" {
			c.JSON(http.StatusUnauthorized, models.ServerResponse{
				Success:    false,
				StatusCode: http.StatusUnauthorized,
				Message:    "Unauthorized",
			})
			return
		}

		rotateLogsResult, rotateErr := rotateLogs()

		if rotateErr != nil {
			rotateLogsResult["error"] = rotateErr.Error()
		}

		helpers.LogInfo("Rotated logs", rotateLogsResult)

		success := rotateErr == nil
		if !success {
			// Check individual log success
			if combinedLog, ok := rotateLogsResult["combined_log"].(map[string]any); ok {
				if combinedSuccess, _ := combinedLog["success"].(bool); combinedSuccess {
					success = true
				}
			}
			if exceptionsLog, ok := rotateLogsResult["exceptions_log"].(map[string]any); ok {
				if exceptionsSuccess, _ := exceptionsLog["success"].(bool); exceptionsSuccess {
					success = true
				}
			}
		}

		c.JSON(http.StatusOK, models.ServerResponse{
			Success:    success,
			StatusCode: http.StatusOK,
			Message:    "Maintenance completed",
			Data:       rotateLogsResult,
		})
	})

	r.Use(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]interface{}{
				"success":    false,
				"statusCode": http.StatusUnauthorized,
				"message":    "Missing or invalid Authorization header",
			})
			return
		}
		token := authHeader[len("Bearer "):]
		expectedToken := os.Getenv("NOTIFICATION_SERVER_TOKEN")
		if expectedToken == "" || token != expectedToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]interface{}{
				"success":    false,
				"statusCode": http.StatusUnauthorized,
				"message":    "Invalid or expired token",
			})
			return
		}
		c.Next()
	})

	// # Routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, models.ServerResponse{
			Success:    true,
			StatusCode: http.StatusOK,
			Message:    "Server is running properly",
			Data: map[string]interface{}{
				"server_timestamp": helpers.GetISTTime().Format(time.RFC3339),
				"environment":      os.Getenv("APP_ENV"),
				"database": map[string]interface{}{
					"initialized": db.GetDB() != nil,
				},
				"queue_status": map[string]interface{}{
					"initialized": queues.EmailQueueClient != nil,
					"count":       2,
					"names": []string{
						models.EmailCarrierAppointmentQueue,
						models.EmailCarrierAppointmentReminderQueue,
					},
				},
				"scheduler_status": map[string]interface{}{
					"initialized": scheduler.Scheduler != nil,
					"count":       2,
					"names": []string{
						models.EmailCarrierBulkPickupNotificationQueue,
						models.EmailCarrierAppointmentBulkReminderQueue,
					},
				},
				"worker_status": map[string]interface{}{
					"initialized": true,
					"count":       4,
					"names": []string{
						models.EmailCarrierBulkPickupNotificationQueue,
						models.EmailCarrierAppointmentBulkReminderQueue,
						models.EmailCarrierAppointmentQueue,
						models.EmailCarrierAppointmentReminderQueue,
					},
				},
			},
		})
	})

	r.POST("/api/scheduleCarrierAppointmentEmail", controllers.ScheduleCarrierAppointmentEmail)

	log.Println("server setup routes")

	// # Start the server
	log.Printf("Server started on port %s\n", port)
	err := r.Run(fmt.Sprintf(":%s", port))

	if err != nil {
		log.Println("failed to start server", err.Error())
		os.Exit(1)
	}

}

func setupQueues() {
	queues.InitEmailQueueClient()
}

func setupWorkers() {
	go workers.InitWorkers()
}

func setupScheduler() {
	scheduler.InitScheduler()
	go scheduler.StartScheduler()
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func rotateLogs() (result map[string]any, err error) {
	start := time.Now()
	result = make(map[string]any)

	// Always use yesterday's date for the log file name (since we're rotating at midnight)
	today := time.Now().AddDate(0, 0, -1).Format("02-01-2006-15-04")

	// Rotate combined.log
	combinedLogPath := filepath.Join("logs", "combined.log")
	combinedLogData, readErr := os.ReadFile(combinedLogPath)
	if readErr != nil {
		result["combined_log"] = map[string]any{
			"success": false,
			"error":   readErr.Error(),
		}
	} else {
		combinedFileName := today + "-notification-server-combined.log"
		combinedUrl, uploadErr := helpers.UploadBytesToS3(combinedLogData, "notification-server-logs/"+combinedFileName, "text/plain")
		if uploadErr != nil {
			result["combined_log"] = map[string]any{
				"success":   false,
				"file_name": combinedFileName,
				"error":     uploadErr.Error(),
			}
		} else {
			// After successful upload, truncate the log file to make it blank
			truncateErr := os.WriteFile(combinedLogPath, []byte{}, 0644)
			if truncateErr != nil {
				result["combined_log"] = map[string]any{
					"success":   false,
					"file_name": combinedFileName,
					"url":       combinedUrl,
					"error":     truncateErr.Error(),
				}
			} else {
				result["combined_log"] = map[string]any{
					"success":   true,
					"file_name": combinedFileName,
					"url":       combinedUrl,
				}
			}
		}
	}

	// Rotate exceptions.log
	exceptionsLogPath := filepath.Join("logs", "exceptions.log")
	exceptionsLogData, readErr := os.ReadFile(exceptionsLogPath)
	if readErr != nil {
		result["exceptions_log"] = map[string]any{
			"success": false,
			"error":   readErr.Error(),
		}
	} else {
		exceptionsFileName := today + "-notification-server-exceptions.log"
		exceptionsUrl, uploadErr := helpers.UploadBytesToS3(exceptionsLogData, "notification-server-logs/"+exceptionsFileName, "text/plain")
		if uploadErr != nil {
			result["exceptions_log"] = map[string]any{
				"success":   false,
				"file_name": exceptionsFileName,
				"error":     uploadErr.Error(),
			}
		} else {
			// After successful upload, truncate the log file to make it blank
			truncateErr := os.WriteFile(exceptionsLogPath, []byte{}, 0644)
			if truncateErr != nil {
				result["exceptions_log"] = map[string]any{
					"success":   false,
					"file_name": exceptionsFileName,
					"url":       exceptionsUrl,
					"error":     truncateErr.Error(),
				}
			} else {
				result["exceptions_log"] = map[string]any{
					"success":   true,
					"file_name": exceptionsFileName,
					"url":       exceptionsUrl,
				}
			}
		}
	}

	elapsed := time.Since(start)
	result["elapsed"] = elapsed.String()

	// Check if both operations succeeded
	combinedSuccess, _ := result["combined_log"].(map[string]any)["success"].(bool)
	exceptionsSuccess, _ := result["exceptions_log"].(map[string]any)["success"].(bool)
	if !combinedSuccess || !exceptionsSuccess {
		err = fmt.Errorf("one or more log rotations failed")
	}

	return result, err
}
