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
	helpers.LogInfo("InitCarrierBulkPickupNotification fetch carrier bulk pickup notification settings", map[string]interface{}{
		"data": rows,
	})

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

		days := strings.Split(strings.TrimSpace(*setting.PickupWeightNotificationDaysRange), ",")

		for _, day := range days {

			times := strings.Split(strings.TrimSpace(*setting.PickupWeightNotificationTime), ",")

			for _, time := range times {

				hours, minutes, _ := strings.Cut(strings.TrimSpace(time), ":")

				cronExpr := fmt.Sprintf("%s %s * * *", minutes, hours)

				payload := models.CarrierBulkPickupEmailWorkerData{
					NotificationID: setting.AnsID,
					AdminID:        setting.AdminID,
					UserID:         setting.UserID,
					Data: models.CarrierBulkPickupEmailWorkerDataData{
						CarrierID: setting.CarrierID,
						Day:       &day,
					},
					Settings: setting,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					helpers.LogException("failed to marshal payload", map[string]interface{}{
						"error":   err.Error(),
						"payload": payload,
					})
				}

				helpers.LogException("successfully marshal payload", map[string]interface{}{
					"payload": payload,
				})

				task := asynq.NewTask(models.EmailCarrierBulkPickupNotificationQueue, payloadBytes)

				id, err := Scheduler.Register(cronExpr, task)
				if err != nil {
					helpers.LogException("failed to register task with scheduler", map[string]interface{}{
						"error":   err.Error(),
						"payload": payload,
					})
				} else {
					helpers.LogInfo("InitCarrierBulkPickupNotification: scheduled task with scheduler", map[string]interface{}{
						"task_id":   id,
						"cron_expr": cronExpr,
						"payload":   payload,
					})
				}

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
			helpers.LogInfo("InitCarrierBulkPickupNotification: skipping setting, notifications disabled", map[string]interface{}{
				"setting_id": setting.AnsID,
			})

			continue
		}

		helpers.LogInfo("InitCarrierBulkPickupNotification: fetching carriers for user", map[string]interface{}{
			"user_id": setting.UserID,
		})

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

		helpers.LogInfo("InitCarrierBulkPickupNotification: fetched carrier IDs", map[string]interface{}{
			"setting_id":    setting.AnsID,
			"user_id":       setting.UserID,
			"carrier_count": len(carrierIDs),
		})

		days := strings.Split(strings.TrimSpace(*setting.PickupWeightNotificationDaysRange), ",")

		helpers.LogInfo("InitCarrierBulkPickupNotification: parsed schedule from settings", map[string]interface{}{
			"setting_id":  setting.AnsID,
			"parsed_days": days,
		})

		for _, day := range days {

			times := strings.Split(strings.TrimSpace(*setting.PickupWeightNotificationTime), ",")

			for _, time := range times {

				hours, minutes, _ := strings.Cut(strings.TrimSpace(time), ":")

				cronExpr := fmt.Sprintf("%s %s * * *", minutes, hours)

				for _, carrierID := range carrierIDs {

					payload := models.CarrierBulkPickupEmailWorkerData{
						NotificationID: setting.AnsID,
						AdminID:        setting.AdminID,
						UserID:         setting.UserID,
						Data: models.CarrierBulkPickupEmailWorkerDataData{
							CarrierID: carrierID,
							Day:       &day,
						},
						Settings: setting,
					}

					payloadBytes, err := json.Marshal(payload)
					if err != nil {
						helpers.LogException("failed to marshal payload", map[string]interface{}{
							"error":   err.Error(),
							"payload": payload,
						})
					}

					task := asynq.NewTask(models.EmailCarrierBulkPickupNotificationQueue, payloadBytes)

					id, err := Scheduler.Register(cronExpr, task)
					if err != nil {
						helpers.LogException("failed to register task with scheduler", map[string]interface{}{
							"error":   err.Error(),
							"payload": payload,
						})
					} else {
						helpers.LogInfo("InitCarrierBulkPickupNotification: scheduled task with scheduler", map[string]interface{}{
							"task_id":   id,
							"cron_expr": cronExpr,
							"payload":   payload,
						})
					}
				}

			}

		}

	}
	helpers.LogInfo("InitCarrierBulkPickupNotification: finished scheduling all tasks.", nil)
	return nil
}
