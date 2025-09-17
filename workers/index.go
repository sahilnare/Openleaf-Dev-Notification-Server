package workers

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
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
			Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password: redisPassword,
			DB: 0,
			Username: redisUsername,
			PoolSize: 10,
			DialTimeout: 10 * time.Second,
			ReadTimeout: 30 * time.Second,
			WriteTimeout: 30 * time.Second,
			TLSConfig: tlsConfig,
		},
		asynq.Config{
			Concurrency: 0,
		},
	)

	mux := asynq.NewServeMux()

	// # Routes
	mux.HandleFunc(models.EmailCarrierAppointmentQueue, carrierWorker.SendAppointmentEmail)
	mux.HandleFunc(models.EmailCarrierAppointmentReminderQueue, carrierWorker.SendAppointmentReminderEmail)
	mux.HandleFunc(models.EmailCarrierBulkPickupNotificationQueue, carrierWorker.SendCarrierBulkPickupEmail)

	log.Println("Starting Asynq worker server...")
	if err := server.Run(mux); err != nil {
		panic("could not run Asynq server: " + err.Error())
	}

}