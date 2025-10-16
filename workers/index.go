package workers

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/scheduler"
	carrierWorker "Notification-Server/workers/carrier"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

func InitWorkers() {
	redisHost, redisPassword, redisUsername, redisPort := helpers.GetRedisConfig()

	log.Printf("Initializing workers with Redis at %s:%s", redisHost, redisPort)

	// For Upstash Redis, we need to use TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:         fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password:     redisPassword,
			Username:     redisUsername,
			DB:           0,
			PoolSize:     10,
			DialTimeout:  10 * time.Second,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			TLSConfig:    tlsConfig,
		},
		asynq.Config{
			Concurrency: 0,
		},
	)

	err := scheduler.Scheduler.Ping()
	if err != nil {
		panic("{Worker} Failed to connect to Redis for workers: " + err.Error())
	} else {
		log.Println("Ping successfull")
	}

	mux := asynq.NewServeMux()

	// # Routes
	mux.HandleFunc(models.EmailCarrierAppointmentQueue, carrierWorker.SendAppointmentEmail)
	mux.HandleFunc(models.EmailCarrierAppointmentReminderQueue, carrierWorker.SendAppointmentReminderEmail)
	mux.HandleFunc(models.EmailCarrierBulkPickupNotificationQueue, carrierWorker.SendCarrierBulkPickupEmail)
	mux.HandleFunc(models.EmailCarrierAppointmentBulkReminderQueue, carrierWorker.SendCarrierBulkDeliverEmail)

	log.Println("Starting Asynq worker server...")
	if err := server.Run(mux); err != nil {
		panic("could not run Asynq server: " + err.Error())
	} else {
		log.Println("Asynq worker server.")
	}

}
