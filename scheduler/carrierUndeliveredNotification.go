package scheduler

import (
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func InitCarrierUndeliveredNotification() error {

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

			send_undelivered_notification,
			undelivered_notification_time,
			undelivered_notification_days
		FROM
			appointment_notification_settings
		WHERE
			send_undelivered_notification = TRUE
	`)
	if err != nil {
		helpers.LogInfo("InitCarrierUndeliveredNotification failed to fetch carrier undelivered notification settings", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	helpers.LogInfo("InitCarrierUndeliveredNotification fetch carrier undelivered notification settings", map[string]interface{}{
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

			&setting.SendUndeliveredNotification,
			&setting.UndeliveredNotificationTime,
			&setting.UndeliveredNotificationDays,
		)
		if err != nil {
			helpers.LogInfo("InitCarrierUndeliveredNotification failed to scan carrier undelivered notification settings", map[string]interface{}{
				"error": err.Error(),
			})
			return err
		}
		notificationSettings = append(notificationSettings, setting)
	}

	helpers.LogInfo("InitCarrierUndeliveredNotification carrier undelivered notification settings", map[string]interface{}{
		"notification_settings": notificationSettings,
	})

	// Schedule the cron for all carriers
	for _, setting := range notificationSettings {

		helpers.LogInfo("InitCarrierUndeliveredNotification carrier undelivered notification setting", map[string]interface{}{
			"setting": setting,
		})

		if !setting.SendUndeliveredNotification {
			helpers.LogInfo("InitCarrierUndeliveredNotification: skipping setting, notifications disabled", map[string]interface{}{
				"setting_id": setting.AnsID,
			})
			continue
		}

		if setting.UndeliveredNotificationTime == nil || setting.UndeliveredNotificationDays == nil {
			helpers.LogInfo("InitCarrierUndeliveredNotification: skipping setting due to missing time or days", map[string]interface{}{
				"setting_id": setting.AnsID,
			})
			continue
		}

		helpers.LogInfo("InitCarrierUndeliveredNotification: fetching carriers for user", map[string]interface{}{
			"user_id": setting.UserID,
		})

		// Get all the user carriers
		var carrierIDs []string

		carrierRows, err := db.GlobalDB.Query(`
			SELECT carrier_id FROM user_carriers WHERE user_id = $1
		`, setting.UserID)
		if err != nil {
			helpers.LogInfo("InitCarrierUndeliveredNotification failed to fetch carrier IDs", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}
		defer carrierRows.Close()

		for carrierRows.Next() {
			var carrierID uuid.UUID
			err = carrierRows.Scan(&carrierID)
			if err != nil {
				helpers.LogInfo("InitCarrierUndeliveredNotification failed to scan carrier IDs", map[string]interface{}{
					"error": err.Error(),
				})
				continue
			}
			carrierIDs = append(carrierIDs, carrierID.String())
		}

		helpers.LogInfo("InitCarrierUndeliveredNotification: fetched carrier IDs", map[string]interface{}{
			"setting_id":    setting.AnsID,
			"user_id":       setting.UserID,
			"carrier_count": len(carrierIDs),
		})

		// Parse notification times (can be comma-separated like "09:00,15:00")
		times := strings.Split(strings.TrimSpace(*setting.UndeliveredNotificationTime), ",")

		helpers.LogInfo("InitCarrierUndeliveredNotification: parsed schedule from settings", map[string]interface{}{
			"setting_id":   setting.AnsID,
			"parsed_times": times,
		})

		// Schedule for each carrier at each time
		for _, timeStr := range times {
			hours, minutes, _ := strings.Cut(strings.TrimSpace(timeStr), ":")

			if minutes == "00" {
				minutes = "0"
			}

			if hours == "00" {
				hours = "0"
			}

			cronExpr := fmt.Sprintf("%s %s * * *", minutes, hours)

			// If carrier_id is "all", schedule for all carriers
			if setting.CarrierID == "all" {
				for _, carrierID := range carrierIDs {
					payload := models.CarrierBulkDeliverEmailWorkerData{
						NotificationID: setting.AnsID,
						AdminID:        setting.AdminID,
						UserID:         setting.UserID,
						Data: models.CarrierBulkDeliverEmailWorkerDataData{
							CarrierID: carrierID,
							Day:       setting.UndeliveredNotificationDays,
						},
						Settings: setting,
					}

					payloadBytes, err := json.Marshal(payload)
					if err != nil {
						helpers.LogException("failed to marshal payload", map[string]interface{}{
							"error":   err.Error(),
							"payload": payload,
						})
						continue
					}

					task := asynq.NewTask(models.EmailCarrierUndeliveredNotificationQueue, payloadBytes)

					id, err := Scheduler.Register(cronExpr, task)
					if err != nil {
						helpers.LogException("failed to register task with scheduler", map[string]interface{}{
							"error":   err.Error(),
							"payload": payload,
						})
					} else {
						helpers.LogInfo("InitCarrierUndeliveredNotification: scheduled task with scheduler", map[string]interface{}{
							"task_id":    id,
							"cron_expr":  cronExpr,
							"time":       timeStr,
							"carrier_id": carrierID,
						})
					}
				}
			} else {
				// Schedule for specific carrier
				payload := models.CarrierBulkDeliverEmailWorkerData{
					NotificationID: setting.AnsID,
					AdminID:        setting.AdminID,
					UserID:         setting.UserID,
					Data: models.CarrierBulkDeliverEmailWorkerDataData{
						CarrierID: setting.CarrierID,
						Day:       setting.UndeliveredNotificationDays,
					},
					Settings: setting,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					helpers.LogException("failed to marshal payload", map[string]interface{}{
						"error":   err.Error(),
						"payload": payload,
					})
					continue
				}

				task := asynq.NewTask(models.EmailCarrierUndeliveredNotificationQueue, payloadBytes)

				id, err := Scheduler.Register(cronExpr, task)
				if err != nil {
					helpers.LogException("failed to register task with scheduler", map[string]interface{}{
						"error":   err.Error(),
						"payload": payload,
					})
				} else {
					helpers.LogInfo("InitCarrierUndeliveredNotification: scheduled task with scheduler", map[string]interface{}{
						"task_id":    id,
						"cron_expr":  cronExpr,
						"time":       timeStr,
						"carrier_id": setting.CarrierID,
					})
				}
			}
		}
	}

	helpers.LogInfo("InitCarrierUndeliveredNotification: finished scheduling all tasks.", nil)
	return nil
}
