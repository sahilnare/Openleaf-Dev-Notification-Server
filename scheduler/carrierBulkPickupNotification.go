package scheduler

import (
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func InitCarrierBulkPickupNotification() error {

	var notificationSettings []models.CarrierAppointmentEmailSettings

	rows, err := db.GlobalDB.Query(`
		SELECT
			ans_id,
			admin_id,
			user_id,
			carrier_id,
			sender_emails_for_carrier,
			sender_cc_emails_for_carrier,
			receiver_emails_for_carrier,
			receiver_cc_emails_for_carrier,

			send_pickup_weight_notification,
			pickup_weight_notification_time,
			pickup_weight_notification_days_range
		FROM
			appointment_notification_settings
		WHERE
			send_pickup_weight_notification = TRUE
	`)
	if err != nil {
		helpers.LogInfo("InitCarrierBulkPickupNotification failed to fetch carrier bulk pickup notification settings", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var setting models.CarrierAppointmentEmailSettings
		err = rows.Scan(
			&setting.AnsID,
			&setting.AdminID,
			&setting.UserID,
			&setting.CarrierID,
			&setting.SenderEmailsForCarrier,
			&setting.SenderCCEmailsForCarrier,
			&setting.ReceiverEmailsForCarrier,
			&setting.ReceiverCCEmailsForCarrier,

			&setting.SendPickupWeightNotification,
			&setting.PickupWeightNotificationTime,
			&setting.PickupWeightNotificationDaysRange,
		)
		if err != nil {
			helpers.LogInfo("InitCarrierBulkPickupNotification failed to scan carrier bulk pickup notification settings", map[string]interface{}{
				"error": err.Error(),
			})
			return err
		}
		notificationSettings = append(notificationSettings, setting)
	}

	helpers.LogInfo("InitCarrierBulkPickupNotification carrier bulk pickup notification settings", map[string]interface{}{
		"notification_settings": notificationSettings,
	})

	// # Schedule the cron for separate carriers
	for _, setting := range notificationSettings {
		
		helpers.LogInfo("InitCarrierBulkPickupNotification carrier bulk pickup notification setting", map[string]interface{}{
			"setting": setting,
		})

		if !setting.SendPickupWeightNotification {
			continue
		}

		// # Get all the user carriers
		var carrierIDs []string

		rows, err := db.GlobalDB.Query(`
			SELECT carrier_id FROM user_carriers WHERE user_id = $1
		`, setting.UserID)
		if err != nil {
			helpers.LogInfo("InitCarrierBulkPickupNotification failed to fetch carrier IDs", map[string]interface{}{
				"error": err.Error(),
			})
		}

		for rows.Next() {
			var carrierID uuid.UUID
			err = rows.Scan(&carrierID)
			if err != nil {
				helpers.LogInfo("InitCarrierBulkPickupNotification failed to scan carrier IDs", map[string]interface{}{
					"error": err.Error(),
				})
			}
			carrierIDs = append(carrierIDs, carrierID.String())
			defer rows.Close()
		}

		isCarrierID := slices.Contains(carrierIDs, setting.CarrierID)

		if !isCarrierID {
			continue
		}

		days := strings.SplitSeq(strings.TrimSpace(*setting.PickupWeightNotificationDaysRange), ",")

		for day := range days {

			times := strings.SplitSeq(strings.TrimSpace(*setting.PickupWeightNotificationTime), ",")
			
			for time := range times {

				hours, minutes, _ := strings.Cut(strings.TrimSpace(time), ":")

				cronExpr := fmt.Sprintf("%s %s * * *", minutes, hours)

				payload := models.CarrierBulkPickupEmailWorkerData{
					NotificationID: setting.AnsID,
					AdminID: setting.AdminID,
					UserID: setting.UserID,
					Data: models.CarrierBulkPickupEmailWorkerDataData{
						CarrierID: setting.CarrierID,
						Day: &day,
					},
					Settings: setting,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					helpers.LogException("failed to marshal payload", map[string]interface{}{
						"error": err.Error(),
						"payload": payload,
					})
				}

				task := asynq.NewTask(models.EmailCarrierBulkPickupNotificationQueue, payloadBytes)
		
				Scheduler.Register(cronExpr, task)


				//#  Remove carrierID from carrierIDs after scheduling the task
				for i, id := range carrierIDs {
					if id == setting.CarrierID {
						carrierIDs = append(carrierIDs[:i], carrierIDs[i+1:]...)
						break
					}
				}
		
			}

		}
		
	}

	// # Schedule the cron for all carriers
	for _, setting := range notificationSettings {
		
		helpers.LogInfo("InitCarrierBulkPickupNotification carrier bulk pickup notification setting", map[string]interface{}{
			"setting": setting,
		})

		if !setting.SendPickupWeightNotification {
			continue
		}

		// # Get all the user carriers
		var carrierIDs []string

		rows, err := db.GlobalDB.Query(`
			SELECT carrier_id FROM user_carriers WHERE user_id = $1
		`, setting.UserID)
		if err != nil {
			helpers.LogInfo("InitCarrierBulkPickupNotification failed to fetch carrier IDs", map[string]interface{}{
				"error": err.Error(),
			})
		}

		for rows.Next() {
			var carrierID uuid.UUID
			err = rows.Scan(&carrierID)
			if err != nil {
				helpers.LogInfo("InitCarrierBulkPickupNotification failed to scan carrier IDs", map[string]interface{}{
					"error": err.Error(),
				})
			}
			carrierIDs = append(carrierIDs, carrierID.String())
			defer rows.Close()
		}

		days := strings.SplitSeq(strings.TrimSpace(*setting.PickupWeightNotificationDaysRange), ",")

		for day := range days {

			times := strings.SplitSeq(strings.TrimSpace(*setting.PickupWeightNotificationTime), ",")
			
			for time := range times {

				hours, minutes, _ := strings.Cut(strings.TrimSpace(time), ":")

				cronExpr := fmt.Sprintf("%s %s * * *", minutes, hours)

				for _, carrierID := range carrierIDs {

					payload := models.CarrierBulkPickupEmailWorkerData{
						NotificationID: setting.AnsID,
						AdminID: setting.AdminID,
						UserID: setting.UserID,
						Data: models.CarrierBulkPickupEmailWorkerDataData{
							CarrierID: carrierID,
							Day: &day,
						},
						Settings: setting,
					}
	
					payloadBytes, err := json.Marshal(payload)
					if err != nil {
						helpers.LogException("failed to marshal payload", map[string]interface{}{
							"error": err.Error(),
							"payload": payload,
						})
					}
	
					task := asynq.NewTask(models.EmailCarrierBulkPickupNotificationQueue, payloadBytes)
			
					Scheduler.Register(cronExpr, task)

				}
		
			}

		}
		
	}

	return nil
}
