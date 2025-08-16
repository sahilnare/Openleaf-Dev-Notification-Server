package workers

import (
	"log"
	"os"

	"github.com/hibiken/asynq"
)

var redisAddr string
var redisPassword string
var redisUsername string

func init() {
	redisAddr = os.Getenv("REDIS_ADDR")
	redisPassword = os.Getenv("REDIS_PASSWORD")
	redisUsername = os.Getenv("REDIS_USERNAME")
}


func InitWorkers() {

	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr: redisAddr,
			Password: redisPassword,
			DB: 0,
			Username: redisUsername,
		},
		asynq.Config{
			Concurrency: 0,
		},
	)

	mux := asynq.NewServeMux()

	// # Routes
	// mux.HandleFunc("email:send", handleEmailTask)

	if err := server.Run(mux); err != nil {
		log.Fatalf("could not run Asynq server: %v", err)
	}

}