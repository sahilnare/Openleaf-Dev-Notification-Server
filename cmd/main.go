package main

import (
	"Notification-Server/controllers"
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"Notification-Server/workers"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file", err.Error())
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()

	log.Println("server starting")

	// # Setup CORS
	r.Use(corsMiddleware())

	log.Println("server setup cors")

	// # Setup queues
	setupQueues()

	log.Println("server setup queues")

	// # Setup workers
	setupWorkers()

	time.Sleep(2 * time.Second)

	log.Println("server setup workers")

	// # Connect database
	db.Connect()

	log.Println("server connect database")

	// # Setup logger
	helpers.InitLogger()

	log.Println("server setup logger")

	// # Setup email configuration
	helpers.InitEmailConfig()

	log.Println("server setup email configuration")


	// # Routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, models.ServerResponse{
			Success:    true,
			StatusCode: http.StatusOK,
			Message:    "Server is running",
			Data: map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"env":       os.Getenv("ENV"),
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