package server

import (
	"Openleaf-Dev-B2B-Appointment-Server/db"
	"Openleaf-Dev-B2B-Appointment-Server/gmail"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Package-level singletons for demo (replace with DI in prod)
var (
	dbReady        = false
	gmailReady     = false
	aiReady        = false
	pushHandler    *gmail.PushHandler
)

func StartWebhookServer() {
	r := gin.Default()

	// Initialize dependencies (in real app, do this in main/init)
	if db.DB != nil {
		dbReady = true
	}
	if gmail.GmailService != nil {
		gmailReady = true
	}
	// For now, AI and PushHandler are always ready as stubs
	aiReady = true
	pushHandler = &gmail.PushHandler{}

	r.POST("/webhook/gmail", func(c *gin.Context) {
		var payload map[string]interface{}
		if err := c.BindJSON(&payload); err != nil {
			log.Printf("Invalid webhook payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid JSON"})
			return
		}
		log.Println("Webhook notification received", payload)
		if pushHandler != nil {
			go pushHandler.HandlePushNotification(payload)
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"components": gin.H{
				"gmail_service": gmailReady,
				"database": dbReady,
				"ai_responder": aiReady,
				"push_handler": pushHandler != nil,
			},
		})
	})

	r.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"database": "postgresql",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	r.Run(":5000")
} 