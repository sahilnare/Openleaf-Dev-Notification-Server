package carrierWorker

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

// # Send Email
func SendAppointmentEmail(ctx context.Context, task *asynq.Task) error {

	if task == nil {
		helpers.LogInfo("[worker] carrier appointment email task is nil", map[string]interface{}{
			"task": task,
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier appointment email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"payload": string(payload),
	})

	var data models.ScheduleAppointmentEmailRequest
	err := json.Unmarshal(payload, &data)
	if err != nil {
		return err
	}

	helpers.LogInfo("[worker] carrier appointment email payload", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
	})

	masterWaybill := ""
	childWaybill := ""
	if data.Data.MasterWaybill != nil && len(*data.Data.MasterWaybill) > 0 {
		masterWaybill = strings.Join(*data.Data.MasterWaybill, ", ")
	}
	if data.Data.ChildWaybill != nil && len(*data.Data.ChildWaybill) > 0 {
		childWaybill = strings.Join(*data.Data.ChildWaybill, ", ")
	}

	body := fmt.Sprintf(
		`<html>
		<body style="font-family: Arial, sans-serif; color: #222;">
			<p>Hello %s Team,</p>
			<p>
				This is to inform you about the delivery schedule for the following order:
			</p>
			<table cellpadding="6" style="border-collapse: collapse; margin-bottom: 30px;">
				<tr>
					<td><b>LR Number:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Master Waybills:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Child Waybills:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Scheduled Delivery Date &amp; Time:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Total Cartons:</b></td>
					<td><b>%d</b></td>
				</tr>
				<tr>
					<td><b>Total Weight:</b></td>
					<td><b>%.2f KG</b></td>
				</tr>
			</table>

			<div style="margin-bottom: 25px;">
				<div style="margin-bottom: 2px;"><u>Delivery Warehouse:</u></div>
				<table cellpadding="2" style="border-collapse: collapse;">
					<tr>
						<td>Name:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td>Address:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td></td>
						<td>%s, %s - %s</td>
					</tr>
					<tr>
						<td>Phone:</td>
						<td>%s / %s</td>
					</tr>
					<tr>
						<td>Email:</td>
						<td>%s</td>
					</tr>
				</table>
			</div>

			<div style="margin-bottom: 25px;">
				<div style="margin-bottom: 2px;"><u>Pickup Warehouse:</u></div>
				<table cellpadding="2" style="border-collapse: collapse;">
					<tr>
						<td>Name:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td>Address:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td></td>
						<td>%s, %s - %s</td>
					</tr>
					<tr>
						<td>Phone:</td>
						<td>%s / %s</td>
					</tr>
					<tr>
						<td>Email:</td>
						<td>%s</td>
					</tr>
				</table>
			</div>

			<p>Best regards,<br>
			Openleaf Team</p>
		</body>
		</html>`,
		data.Name,
		helpers.DerefStringPointer(data.Data.LRNumber),
		masterWaybill,
		childWaybill,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentDate),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(data.Data.TotalDeadWeight),

		// Delivery warehouse
		helpers.DerefStringPointer(data.Data.WarehouseName),
		helpers.DerefStringPointer(data.Data.WarehouseAddress),
		helpers.DerefStringPointer(data.Data.WarehouseCity),
		helpers.DerefStringPointer(data.Data.WarehouseState),
		helpers.DerefStringPointer(data.Data.WarehousePin),
		helpers.DerefStringPointer(data.Data.WarehousePhone),
		helpers.DerefStringPointer(data.Data.WarehouseAlternatePhone),
		helpers.DerefStringPointer(data.Data.WarehouseEmail),

		// Pickup warehouse
		helpers.DerefStringPointer(data.Data.CustomerWarehouseName),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAddress),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseCity),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseState),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePin),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAlternatePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseEmail),
	)

	err = helpers.SendEmail(
		helpers.B2B_EMAIL, 
		[]string{data.Email}, 
		data.CC, 
		fmt.Sprintf("Delivery Appointment Scheduled for LR%s on %s",
			helpers.DerefStringPointer(data.Data.LRNumber),
			helpers.FormatDateDDMMYYYY(data.Data.AppointmentDate),
		),
		body, 
		true, 
		*data.Data.Files,
	)

	if err != nil {
		helpers.LogException("[worker] failed to send appointment email", map[string]interface{}{
			"error": err.Error(),
		})

		notificationID, err := helpers.UpdateNotification(&models.Notification{
			OrderID: data.OrderID,
			Sender: data.Email,
			CC: strings.Join(data.CC, ","),
			Receiver: data.Email,
			Type: "appointment",
			Status: "worker_error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("[worker] failed to update notification", map[string]interface{}{
				"error": err.Error(),
				"notification_id": notificationID,
				"order_id": data.OrderID,
				"sender": data.Email,
				"cc": strings.Join(data.CC, ","),
				"receiver": data.Email,
				"type": "appointment",
			})
		}

	}

	now := time.Now()

	notificationID, _ := helpers.UpdateNotification(&models.Notification{
		OrderID: data.OrderID,
		Sender: data.Email,
		CC: strings.Join(data.CC, ","),
		Receiver: data.Email,
		Type: "appointment",
		Status: "sent",
		SentAt: &now,
	})

	helpers.LogInfo("[worker] appointment email sent successfully", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
		"notification_id": notificationID,
	})

	return nil

}

// # Send Reminder Email
func SendAppointmentReminderEmail(ctx context.Context, task *asynq.Task) error {

	if task == nil {
		helpers.LogInfo("[worker] carrier appointment reminder email task is nil", map[string]interface{}{
			"task": task,
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier appointment reminder email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"payload": string(payload),
	})

	var data models.ScheduleAppointmentEmailRequest
	err := json.Unmarshal(payload, &data)
	if err != nil {
		return err
	}

	helpers.LogInfo("[worker] carrier appointment reminder email payload", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
	})

	masterWaybill := ""
	childWaybill := ""
	if data.Data.MasterWaybill != nil && len(*data.Data.MasterWaybill) > 0 {
		masterWaybill = strings.Join(*data.Data.MasterWaybill, ", ")
	}
	if data.Data.ChildWaybill != nil && len(*data.Data.ChildWaybill) > 0 {
		childWaybill = strings.Join(*data.Data.ChildWaybill, ", ")
	}

	body := fmt.Sprintf(
		`<html>
		<body style="font-family: Arial, sans-serif; color: #222;">
			<p>Hello %s Team,</p>
			<p>
				This is a gentle reminder about the upcoming delivery for the following order, Kindly ensure all arrangements are made for an on-time and smooth delivery.
			</p>
			<table cellpadding="6" style="border-collapse: collapse; margin-bottom: 30px;">
				<tr>
					<td><b>LR Number:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Master Waybills:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Child Waybills:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Scheduled Delivery Date &amp; Time:</b></td>
					<td><b>%s</b></td>
				</tr>
				<tr>
					<td><b>Total Cartons:</b></td>
					<td><b>%d</b></td>
				</tr>
				<tr>
					<td><b>Total Weight:</b></td>
					<td><b>%.2f KG</b></td>
				</tr>
			</table>

			<div style="margin-bottom: 25px;">
				<div style="margin-bottom: 2px;"><u>Delivery Warehouse:</u></div>
				<table cellpadding="2" style="border-collapse: collapse;">
					<tr>
						<td>Name:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td>Address:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td></td>
						<td>%s, %s - %s</td>
					</tr>
					<tr>
						<td>Phone:</td>
						<td>%s / %s</td>
					</tr>
					<tr>
						<td>Email:</td>
						<td>%s</td>
					</tr>
				</table>
			</div>

			<div style="margin-bottom: 25px;">
				<div style="margin-bottom: 2px;"><u>Pickup Warehouse:</u></div>
				<table cellpadding="2" style="border-collapse: collapse;">
					<tr>
						<td>Name:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td>Address:</td>
						<td>%s</td>
					</tr>
					<tr>
						<td></td>
						<td>%s, %s - %s</td>
					</tr>
					<tr>
						<td>Phone:</td>
						<td>%s / %s</td>
					</tr>
					<tr>
						<td>Email:</td>
						<td>%s</td>
					</tr>
				</table>
			</div>

			<p>Best regards,<br>
			Openleaf Team</p>
		</body>
		</html>`,
		data.Name,
		helpers.DerefStringPointer(data.Data.LRNumber),
		masterWaybill,
		childWaybill,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentDate),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(data.Data.TotalDeadWeight),

		// Delivery warehouse
		helpers.DerefStringPointer(data.Data.WarehouseName),
		helpers.DerefStringPointer(data.Data.WarehouseAddress),
		helpers.DerefStringPointer(data.Data.WarehouseCity),
		helpers.DerefStringPointer(data.Data.WarehouseState),
		helpers.DerefStringPointer(data.Data.WarehousePin),
		helpers.DerefStringPointer(data.Data.WarehousePhone),
		helpers.DerefStringPointer(data.Data.WarehouseAlternatePhone),
		helpers.DerefStringPointer(data.Data.WarehouseEmail),

		// Pickup warehouse
		helpers.DerefStringPointer(data.Data.CustomerWarehouseName),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAddress),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseCity),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseState),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePin),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAlternatePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseEmail),
	)

	err = helpers.SendEmail(
		helpers.B2B_EMAIL, 
		[]string{data.Email}, 
		data.CC, 
		fmt.Sprintf("Reminder: Delivery for LR%s on %s",
			helpers.DerefStringPointer(data.Data.LRNumber),
			helpers.FormatDateDDMMYYYY(data.Data.AppointmentDate),
		), 
		body, 
		true, 
		*data.Data.Files,
	)

	if err != nil {
		helpers.LogException("[worker] failed to send reminder email", map[string]interface{}{
			"error": err.Error(),
		})

		notificationID, err := helpers.UpdateNotification(&models.Notification{
			OrderID: data.OrderID,
			Sender: data.Email,
			CC: strings.Join(data.CC, ","),
			Receiver: data.Email,
			Type: "appointment_reminder",
			Status: "worker_error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("[worker] failed to update notification", map[string]interface{}{
				"error": err.Error(),
				"notification_id": notificationID,
				"order_id": data.OrderID,
				"sender": data.Email,
				"cc": strings.Join(data.CC, ","),
				"receiver": data.Email,
				"type": "appointment_reminder",
			})
		}
	}

	helpers.LogInfo("[worker] reminder email sent successfully", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
	})

	now := time.Now()

	notificationID, _ := helpers.UpdateNotification(&models.Notification{
		OrderID: data.OrderID,
		Sender: data.Email,
		CC: strings.Join(data.CC, ","),
		Receiver: data.Email,
		Type: "appointment_reminder",
		Status: "sent",
		SentAt: &now,
	})

	helpers.LogInfo("[worker] reminder email sent successfully", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
		"notification_id": notificationID,
	})

	return nil

}