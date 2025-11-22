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
					"count":  2,
					"names": []string{
						models.EmailCarrierAppointmentQueue,
						models.EmailCarrierAppointmentReminderQueue,
					},
				},
				"scheduler_status": map[string]interface{}{
					"initialized":    scheduler.Scheduler != nil,
					"count": 2,
					"names": []string{
						models.EmailCarrierBulkPickupNotificationQueue,
						models.EmailCarrierAppointmentBulkReminderQueue,
					},
				},
				"worker_status": map[string]interface{}{
					"initialized":      true,
					"count": 4,
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
