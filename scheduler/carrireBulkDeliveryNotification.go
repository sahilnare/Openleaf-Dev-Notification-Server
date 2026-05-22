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

func InitCarrierBulkDeliverNotification() error {

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
			send_bulk_reminder,
			bulk_reminder_time,
			bulk_reminder_days_range,
			bulk_reminder_type,
			carrier_name
		FROM
			appointment_notification_settings
		WHERE
			send_bulk_reminder = TRUE
	`)
	if err != nil {
		helpers.LogInfo("InitCarrierBulkDeliverNotification failed to fetch carrier bulk delivery notification settings", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	helpers.LogInfo("InitCarrierBulkDeliverNotification fetch carrier bulk delivery notification settings", map[string]interface{}{
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
			&setting.SendBulkReminder,
			&setting.BulkReminderTime,
			&setting.BulkReminderDaysRange,
			&setting.BulkReminderType,
			&setting.CarrierName,
		)
		if err != nil {
			helpers.LogInfo("InitCarrierBulkDeliverNotification failed to scan carrier bulk delivery notification settings", map[string]interface{}{
				"error": err.Error(),
			})
			return err
		}
		notificationSettings = append(notificationSettings, setting)
	}

	helpers.LogInfo("InitCarrierBulkDeliverNotification carrier bulk delivery notification settings", map[string]interface{}{
		"notification_settings": notificationSettings,
	})

	// # Schedule the cron for separate carriers
	for _, setting := range notificationSettings {

		helpers.LogInfo("InitCarrierBulkDeliverNotification carrier bulk delivery notification setting", map[string]interface{}{
			"setting": setting,
		})

		if !setting.SendBulkReminder {
			continue
		}

		if setting.BulkReminderDaysRange == nil || setting.BulkReminderTime == nil {
			helpers.LogInfo("InitCarrierBulkDeliverNotification: skipping setting due to missing time or days range", map[string]interface{}{
				"setting_id": setting.AnsID,
			})
			continue
		}

		// # Get all the user carriers
		var carrierIDs []string

		rows, err := db.GlobalDB.Query(`
			SELECT carrier_id FROM user_carriers WHERE user_id = $1
		`, setting.UserID)
		if err != nil {
			helpers.LogInfo("InitCarrierBulkDeliverNotification failed to fetch carrier IDs", map[string]interface{}{
				"error": err.Error(),
			})
		}

		for rows.Next() {
			var carrierID uuid.UUID
			err = rows.Scan(&carrierID)
			if err != nil {
				helpers.LogInfo("InitCarrierBulkDeliverNotification failed to scan carrier IDs", map[string]interface{}{
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

		days := strings.Split(strings.TrimSpace(*setting.BulkReminderDaysRange), ",")

		for _, day := range days {

			times := strings.Split(strings.TrimSpace(*setting.BulkReminderTime), ",")

			for _, time := range times {

				// ! TESTING: Uncomment this to test the cron expression for a specific carrier
				// if setting.CarrierID == "5c54da80-3160-486e-a0f9-151ceedfc41a" {
				// 	time = "00:40"
				// }

				hours, minutes, _ := strings.Cut(strings.TrimSpace(time), ":")

				if minutes == "00" {
					minutes = "0"
				}

				if hours == "00" {
					hours = "0"
				}

				cronExpr := fmt.Sprintf("%s %s * * *", minutes, hours)

				dayTrimmed := strings.TrimSpace(day)
				payload := models.CarrierBulkDeliverEmailWorkerData{
					NotificationID: setting.AnsID,
					AdminID:        setting.AdminID,
					UserID:         setting.UserID,
					Data: models.CarrierBulkDeliverEmailWorkerDataData{
						CarrierID: setting.CarrierID,
						Day:       &dayTrimmed,
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

				task := asynq.NewTask(models.EmailCarrierAppointmentBulkReminderQueue, payloadBytes)

				id, err := Scheduler.Register(cronExpr, task)
				if err != nil {
					helpers.LogException("failed to register task with scheduler", map[string]interface{}{
						"error":   err.Error(),
						"payload": payload,
					})
				} else {
					trackAppointmentEntry(id)
					helpers.LogInfo("InitCarrierBulkDeliverNotification: scheduled task with scheduler", map[string]interface{}{
						"task_id":    id,
						"cron_expr":  cronExpr,
						"day":        dayTrimmed,
						"time":       time,
						"carrier_id": setting.CarrierID,
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

	helpers.LogInfo("InitCarrierBulkDeliverNotification: finished scheduling all tasks.", nil)
	return nil
}
