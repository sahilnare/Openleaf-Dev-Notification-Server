package helpers

import (
	"os"
)

func GetRedisConfig() (string, string, string, string) {

	redisHost := os.Getenv("REDIS_HOST")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUsername := os.Getenv("REDIS_USERNAME")
	redisPort := os.Getenv("REDIS_PORT")

	if redisHost == "" {
		panic("Redis configuration is incomplete. REDIS_HOST must be set")
	}
	if redisPort == "" {
		panic("Redis configuration is incomplete. REDIS_PORT must be set")
	}
	if redisUsername == "" {
		panic("Redis configuration is incomplete. REDIS_USERNAME must be set")
	}
	if redisPassword == "" {
		panic("Redis configuration is incomplete. REDIS_PASSWORD must be set")
	}


	return redisHost, redisPassword, redisUsername, redisPort
}