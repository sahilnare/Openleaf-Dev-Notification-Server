package scheduler

import (
	"Notification-Server/helpers"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

var Scheduler *asynq.Scheduler

func InitScheduler() {

	redisHost, redisPassword, redisUsername, redisPort := helpers.GetRedisConfig()

	log.Printf("Initializing scheduler with Redis at %s:%s", redisHost, redisPort)

	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic("Failed to load IST timezone: " + err.Error())
	}

	Scheduler = asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password: redisPassword,
			DB: 0,
			Username: redisUsername,
			PoolSize: 10,
			DialTimeout: 10 * time.Second,
			ReadTimeout: 30 * time.Second,
			WriteTimeout: 30 * time.Second,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
		&asynq.SchedulerOpts{
			Location: istLocation,
			PostEnqueueFunc: func(info *asynq.TaskInfo, err error) {
				if err != nil {
					log.Printf("Error enqueuing task: %v", err)
				} else {
					log.Printf("Task enqueued successfully: %v", info)
				}
			},
		},
	)

	log.Println("Scheduler initialized successfully")

}

func StartScheduler() {

	if err := InitCarrierBulkPickupNotification(); err != nil {
		panic("Failed to initialize carrier bulk pickup notification: " + err.Error())
	}

	if err := Scheduler.Run(); err != nil {
		panic("Scheduler failed to run: " + err.Error())
	}

	log.Println("Scheduler started successfully")
}
