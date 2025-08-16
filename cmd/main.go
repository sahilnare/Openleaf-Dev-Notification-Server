package main

import (
	"Notification-Server/controllers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"Notification-Server/workers"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.Default()
	r.Use(corsMiddleware())

	// # Setup queues
	setupQueues()	
	setupWorkers()

	// # Routes
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, models.ServerResponse{
			Success:    true,
			StatusCode: http.StatusOK,
			Message:    "Server is running",
		})
	})

	r.POST("/scheduleEmail", controllers.ScheduleEmail)

	// # Start the server
	err := r.Run(fmt.Sprintf(":%s", port))

	if err != nil {
		log.Printf("Failed to start server : %v", err)
	}

}


func setupQueues() {
	queues.InitEmailQueueClient()
}

func setupWorkers() {
	workers.InitWorkers()
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}