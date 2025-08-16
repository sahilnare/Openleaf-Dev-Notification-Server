package queues

import (
	"log"
	"os"

	"github.com/hibiken/asynq"
)

var redisAddr string
var redisPassword string
var redisUsername string

var EmailQueueClient *asynq.Client

func init() {
	redisAddr = os.Getenv("REDIS_ADDR")
	redisPassword = os.Getenv("REDIS_PASSWORD")
	redisUsername = os.Getenv("REDIS_USERNAME")
}


func InitEmailQueueClient() {

	EmailQueueClient = asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisAddr,
		Password: redisPassword,
		DB: 0,
		Username: redisUsername,
	})

	if EmailQueueClient == nil {
		log.Fatal("Failed to create email queue client")
	}

	defer EmailQueueClient.Close()

}

