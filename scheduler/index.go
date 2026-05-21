package scheduler

import (
	"Notification-Server/helpers"
	"crypto/tls"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hibiken/asynq"
)

var Scheduler *asynq.Scheduler

// appointmentEntryIDs tracks every cron entry registered from appointment_notification_settings
// so a runtime reload can unregister them before re-reading the table.
var (
	appointmentEntryIDs []string
	appointmentEntryMu  sync.Mutex
)

// trackAppointmentEntry records a registered cron entry ID for later reload.
func trackAppointmentEntry(id string) {
	appointmentEntryMu.Lock()
	appointmentEntryIDs = append(appointmentEntryIDs, id)
	appointmentEntryMu.Unlock()
}

// ReloadAppointmentSchedules unregisters all appointment crons and re-registers
// them from the current appointment_notification_settings rows, with no restart.
func ReloadAppointmentSchedules() (int, error) {
	appointmentEntryMu.Lock()
	old := appointmentEntryIDs
	appointmentEntryIDs = nil
	appointmentEntryMu.Unlock()

	for _, id := range old {
		if err := Scheduler.Unregister(id); err != nil {
			helpers.LogException("ReloadAppointmentSchedules: failed to unregister entry", map[string]interface{}{
				"entry_id": id,
				"error":    err.Error(),
			})
		}
	}

	if err := InitCarrierBulkPickupNotification(); err != nil {
		return 0, err
	}
	if err := InitCarrierBulkDeliverNotification(); err != nil {
		return 0, err
	}

	appointmentEntryMu.Lock()
	count := len(appointmentEntryIDs)
	appointmentEntryMu.Unlock()

	helpers.LogInfo("ReloadAppointmentSchedules: reload complete", map[string]interface{}{
		"unregistered": len(old),
		"registered":   count,
	})
	return count, nil
}

func InitScheduler() {

	redisHost, redisPassword, redisUsername, redisPort := helpers.GetRedisConfig()

	log.Printf("Initializing scheduler with Redis at %s:%s", redisHost, redisPort)

	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic("Failed to load IST timezone: " + err.Error())
	}

	Scheduler = asynq.NewScheduler(
		asynq.RedisClientOpt{
			Addr:         fmt.Sprintf("%s:%s", redisHost, redisPort),
			Password:     redisPassword,
			Username:     redisUsername,
			DB:           0,
			PoolSize:     10,
			DialTimeout:  10 * time.Second,
			ReadTimeout:  30 * time.Second,
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
					helpers.LogInfo("Task enqueued successfully: %v", string(info.Payload))
				}
			},
		},
	)

	err = Scheduler.Ping()
	if err != nil {
		panic("Failed to connect to Redis for scheduler: " + err.Error())
	} else {
		log.Println("Ping successfull")
	}
	log.Println("Scheduler initialized successfully")
}

func StartScheduler() {

	if err := InitCarrierBulkPickupNotification(); err != nil {
		panic("Failed to initialize carrier bulk pickup notification: " + err.Error())
	}
	if err := InitCarrierBulkDeliverNotification(); err != nil {
		panic("Failed to initialize carrier bulk deliver notification: " + err.Error())
	}

	if err := Scheduler.Run(); err != nil {
		panic("Scheduler failed to run: " + err.Error())
	}

	log.Println("Scheduler started successfully")
}
