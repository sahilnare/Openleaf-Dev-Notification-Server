package scheduler

import (
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
)

func InitCarrierBulkDeliverNotification() error {
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
			bulk_reminder_type
		FROM
			appointment_notification_settings
	`)
	if err != nil {
		helpers.LogInfo("Failed to fetch notification settings", map[string]interface{}{"error": err})
		return err
	}
	defer rows.Close()
	helpers.LogInfo("Successfully fetched notification settings", map[string]interface{}{"error": err})
	helpers.LogInfo("Data fetched", map[string]interface{}{"data": rows})

	var allSettings []models.CarrierAppointmentEmailSettings
	for rows.Next() {

		var setting models.CarrierAppointmentEmailSettings
		if err := rows.Scan(
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
		); err != nil {
			helpers.LogInfo("Failed to scan reminder setting row", map[string]interface{}{"error": err})
			continue // Skip this row and move to the next
		}
		allSettings = append(allSettings, setting)
	}

	helpers.LogInfo("Successfully loaded all reminder settings. Starting scheduler loop.", map[string]interface{}{
		"settings_count": len(allSettings),
		"settings":       allSettings,
	})

	for _, setting := range allSettings {
		if setting.BulkReminderDaysRange == nil || setting.BulkReminderTime == nil {
			helpers.LogInfo("Skipping setting due to missing time or days range", map[string]interface{}{"setting_id": setting.AnsID})
			continue
		}
		helpers.LogInfo("setting", setting)
		days := strings.Split(strings.TrimSpace(*setting.BulkReminderDaysRange), ",")
		times := strings.Split(strings.TrimSpace(*setting.BulkReminderTime), ",")

		// var days = []string{"2"}
		// var times = []string{"15:39"}

		for _, day := range days {
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

				payloadBytes, _ := json.Marshal(payload)
				task := asynq.NewTask(models.EmailCarrierAppointmentBulkReminderQueue, payloadBytes)

				taskID, err := Scheduler.Register(cronExpr, task)
				if err != nil {
					helpers.LogInfo("Failed to register scheduled task", map[string]interface{}{"cron": cronExpr, "payload": payload, "error": err})
				} else {
					helpers.LogInfo("Successfully scheduled reminder task", map[string]interface{}{
						"task_id":    taskID,
						"cron_expr":  cronExpr,
						"setting_id": setting.AnsID,
						"carrier_id": setting.CarrierID,
					})
				}
			}
		}
	}

	helpers.LogInfo("Finished scheduling all reminder tasks for bulk deliver.", nil)
	return nil
}
