package helpers

import (
	"os"
)

func GetRedisConfig() (string, string, string, string) {

	redisHost := os.Getenv("NOTIFICAION_SERVER_REDIS_HOST")
	redisPassword := os.Getenv("NOTIFICAION_SERVER_REDIS_PASSWORD")
	redisUsername := os.Getenv("NOTIFICAION_SERVER_REDIS_USERNAME")
	redisPort := os.Getenv("NOTIFICAION_SERVER_REDIS_PORT")

	if redisHost == "" {
		panic("Redis configuration is incomplete. NOTIFICAION_SERVER_REDIS_HOST must be set")
	}
	if redisPort == "" {
		panic("Redis configuration is incomplete. NOTIFICAION_SERVER_REDIS_PORT must be set")
	}
	if redisUsername == "" {
		panic("Redis configuration is incomplete. NOTIFICAION_SERVER_REDIS_USERNAME must be set")
	}
	// if redisPassword == "" {
	// 	panic("Redis configuration is incomplete. NOTIFICAION_SERVER_REDIS_PASSWORD must be set")
	// }

	return redisHost, redisPassword, redisUsername, redisPort
}
