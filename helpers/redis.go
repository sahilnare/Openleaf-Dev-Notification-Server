package helpers

import (
	"log"
	"os"
)

func GetRedisConfig() (string, string, string, string) {

	redisHost := os.Getenv("REDIS_HOST")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUsername := os.Getenv("REDIS_USERNAME")
	redisPort := os.Getenv("REDIS_PORT")

	if redisHost == "" {
		log.Fatal("Redis configuration is incomplete. REDIS_HOST must be set")
	}
	if redisPort == "" {
		log.Fatal("Redis configuration is incomplete. REDIS_PORT must be set")
	}
	if redisUsername == "" {
		log.Fatal("Redis configuration is incomplete. REDIS_USERNAME must be set")
	}
	if redisPassword == "" {
		log.Fatal("Redis configuration is incomplete. REDIS_PASSWORD must be set")
	}


	return redisHost, redisPassword, redisUsername, redisPort
}