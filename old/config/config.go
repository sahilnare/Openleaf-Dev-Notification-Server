package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds global configuration
var ConfigData *Config

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	GmailTokenFile string
	SenderEmail string
	GeminiAPIKey string
	BusinessName string
	BusinessHoursStart string
	BusinessHoursEnd string
	GoogleCloudProjectID string
	PubSubTopicName string
}

// LoadEnv loads environment variables from .env and populates ConfigData
func LoadEnv() {
	_ = godotenv.Load()
	ConfigData = &Config{
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		GmailTokenFile: os.Getenv("GMAIL_TOKEN_FILE"),
		SenderEmail: os.Getenv("SENDER_EMAIL"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		BusinessName: os.Getenv("BUSINESS_NAME"),
		BusinessHoursStart: os.Getenv("BUSINESS_HOURS_START"),
		BusinessHoursEnd: os.Getenv("BUSINESS_HOURS_END"),
		GoogleCloudProjectID: os.Getenv("GOOGLE_CLOUD_PROJECT_ID"),
		PubSubTopicName: os.Getenv("PUBSUB_TOPIC_NAME"),
	}
	log.Println("[OK] Environment variables loaded")
} 