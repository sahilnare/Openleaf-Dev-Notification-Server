package queues

import (
	"Notification-Server/helpers"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

var EmailQueueClient *asynq.Client

func InitEmailQueueClient() {
	redisHost, _, redisUsername, redisPort := helpers.GetRedisConfig()

	log.Printf("Connecting to Redis at %s:%s", redisHost, redisPort)

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	EmailQueueClient = asynq.NewClient(asynq.RedisClusterClientOpt{
		Addrs: []string{fmt.Sprintf("%s:%s", redisHost, redisPort)},
		// Password: redisPassword,
		// DB: 0,
		Username: redisUsername,
		// PoolSize: 10,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		TLSConfig:    tlsConfig,
	})

	if EmailQueueClient == nil {
		panic("Failed to create email queue client")
	}

	// Test the connection by trying to ping Redis
	log.Println("Testing Redis connection with ping...")

	time.Sleep(1 * time.Second)

	log.Println("Email queue client initialized successfully")

	// defer EmailQueueClient.Close()
}
