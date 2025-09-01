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
	defer func() {
		if r := recover(); r != nil {
			helpers.LogException("[worker] carrier appointment email worker panic recovered", map[string]interface{}{
				"panic": r,
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})
		}
	}()

	helpers.LogInfo("[worker] carrier appointment email worker started", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload_length": len(task.Payload()),
	})

	if task == nil {
		helpers.LogInfo("[worker] carrier appointment email task is nil", map[string]interface{}{
			"task_data": string(task.Payload()),
			"task_type": task.Type(),
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier appointment email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload": string(payload),
	})

	var data models.CarrierAppointmentEmailWorkerData
	err := json.Unmarshal(payload, &data)
	if err != nil {
		helpers.LogException("[worker] failed to unmarshal carrier appointment email payload", map[string]interface{}{
			"error": err.Error(),
			"payload": string(payload),
			"task_type": task.Type(),
			"task_data": string(task.Payload()),
		})
		return err
	}

	helpers.LogInfo("[worker] carrier appointment email payload", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"data": data,
		"notification_id": data.NotificationID,
		"order_id": data.OrderID,
	})

	masterWaybill := ""
	if data.Data.MasterWaybill != nil && len(*data.Data.MasterWaybill) > 0 {
		masterWaybill = strings.Join(*data.Data.MasterWaybill, ", ")
	}

	childWaybill := ""
	if data.Data.ChildWaybill != nil && len(*data.Data.ChildWaybill) > 0 {
		childWaybill = strings.Join(*data.Data.ChildWaybill, ", ")
	}

	poNumbers := ""
	if data.Data.PONumber != nil && len(*data.Data.PONumber) > 0 {
		poNumbers = strings.Join(*data.Data.PONumber, ", ")
	}

	var cartonDetails strings.Builder
	for i, carton := range *data.Data.Cartons {
		cartonDetails.WriteString(fmt.Sprintf(
			"<tr><td>%d</td><td>%.2f x %.2f x %.2f Inch</td><td>%.f</td><td>%.2f KG</td></tr>",
			i+1,
			helpers.CmToInch(&carton.Length),
			helpers.CmToInch(&carton.Breadth), 
			helpers.CmToInch(&carton.Height),
			carton.Quantity,
			helpers.DerefFloatPointer(helpers.RoundFloat(carton.Weight / 1000.0)),
		))
	}


	body := fmt.Sprintf(
		`<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
				.order-details { margin: 20px 0; }
				.details-row { margin-bottom: 8px; }
				.label { font-weight: bold; display: inline; }
				.value { display: inline; margin-left: 5px; }
				.carton-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
				.carton-table th, .carton-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
				.carton-table th { background: #f5f5f5; font-weight: bold; }
				.location-section { margin: 20px 0; }
				.location-title { font-weight: bold; margin-bottom: 8px; }
				.location-row { margin-bottom: 6px; font-size: 14px; }
				.footer { margin-top: 30px; }
			</style>
		</head>
		<body>
			<p>Hello %s Team,</p>
			<p>This is to inform you about the delivery schedule for the following order:</p>

			<div class="order-details">
				<div class="details-row">
					<span class="label">LR Number:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">PO Number:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Master Waybills:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Child Waybills:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Delivery Schedule:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Total Cartons:</span>
					<span class="value">%d</span>
				</div>
				<div class="details-row">
					<span class="label">Total Weight:</span>
					<span class="value">%.2f KG</span>
				</div>
			</div>
	
			<div class="order-details">
				<div class="location-title">Carton Details</div>
				<table class="carton-table">
					<tr>
						<th>Carton</th>
						<th>Dimensions (L x B x H)</th>
						<th>Quantity</th>
						<th>Weight</th>
					</tr>
					%s
				</table>
			</div>
	
			<div class="location-section">
				<div class="location-title">Delivery Address</div>
				<div class="location-row">
					<span class="label">Name:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Address:</span>
					<span class="value">%s<br>%s, %s - %s</span>
				</div>
				<div class="location-row">
					<span class="label">Contact:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Email:</span>
					<span class="value">%s</span>
				</div>
			</div>
	
			<div class="location-section">
				<div class="location-title">Pickup Address</div>
				<div class="location-row">
					<span class="label">Name:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Address:</span>
					<span class="value">%s<br>%s, %s - %s</span>
				</div>
				<div class="location-row">
					<span class="label">Contact:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Email:</span>
					<span class="value">%s</span>
				</div>
			</div>
	
			<div class="footer">
				<p><strong>Best regards,</strong><br>Openleaf Team</p>
			</div>
		</body>
		</html>`,
		data.Data.CarrierName,
		helpers.DerefStringPointer(data.Data.LRNumber),
		poNumbers,
		masterWaybill,
		childWaybill,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentDate),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(helpers.RoundFloat(helpers.RoundFloat(data.Data.TotalDeadWeight) / 1000.0)),
		cartonDetails.String(),
	
		// Delivery warehouse
		helpers.DerefStringPointer(data.Data.WarehouseName),
		helpers.DerefStringPointer(data.Data.WarehouseAddress),
		helpers.DerefStringPointer(data.Data.WarehouseCity),
		helpers.DerefStringPointer(data.Data.WarehouseState),
		helpers.DerefStringPointer(data.Data.WarehousePin),
		helpers.DerefStringPointer(data.Data.WarehousePhone),
		helpers.DerefStringPointer(data.Data.WarehouseEmail),
	
		// Pickup warehouse
		helpers.DerefStringPointer(data.Data.CustomerWarehouseName),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAddress),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseCity),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseState),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePin),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseEmail),
	)

	receiverEmails := strings.Split(*data.Settings.ReceiverEmailsForCarrier, ",")
	
	receiverCC := []string{}
	if data.Settings.ReceiverCCEmailsForCarrier != nil {
		receiverCC = append(receiverCC, strings.Split(*data.Settings.ReceiverCCEmailsForCarrier, ",")...)
	}
	if data.Settings.SenderCCEmailsForCarrier != nil {
		receiverCC = append(receiverCC, strings.Split(*data.Settings.SenderCCEmailsForCarrier, ",")...)
	}

	// Prepare file URLs for attachment
	var fileURLs []string
	if data.Data.Files != nil {
		fileURLs = *data.Data.Files
	}

	helpers.LogInfo("[worker] attempting to send email", map[string]interface{}{
		"from": helpers.B2B_EMAIL,
		"to": receiverEmails,
		"cc": receiverCC,
		"subject": fmt.Sprintf("Delivery Scheduled for LR %s on %s",
			helpers.DerefStringPointer(data.Data.LRNumber),
			helpers.FormatDateDDMMYYYY(data.Data.AppointmentDate),
		),
		"body_length": len(body),
		"files_count": len(fileURLs),
	})

	err = helpers.SendEmail(
		helpers.B2B_EMAIL, 
		receiverEmails, 
		receiverCC, 
		fmt.Sprintf("Delivery Scheduled for LR %s on %s",
			helpers.DerefStringPointer(data.Data.LRNumber),
			helpers.FormatDateDDMMYYYY(data.Data.AppointmentDate),
		),
		body, 
		true, 
		fileURLs,
	)

	if err != nil {
		helpers.LogException("[worker] failed to send appointment email", map[string]interface{}{
			"error": err.Error(),
		})

		notificationID, err := helpers.UpdateNotification(&models.Notification{
			NotificationID: data.NotificationID,
			OrderID: data.OrderID,
			Sender: *data.Settings.SenderEmailsForCarrier,
			Receiver: *data.Settings.ReceiverEmailsForCarrier,
			SenderCC: data.Settings.SenderCCEmailsForCarrier,
			ReceiverCC: data.Settings.ReceiverCCEmailsForCarrier,
			Method: "email",
			Type: models.EmailCarrierAppointmentQueue,
			Status: "worker_error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("[worker] failed to update notification", map[string]interface{}{
				"error": err.Error(),
				"notification_id": notificationID,
				"order_id": data.OrderID,
				"sender": *data.Settings.SenderEmailsForCarrier,
				"sender_cc": *data.Settings.SenderCCEmailsForCarrier,
				"receiver_cc": *data.Settings.ReceiverCCEmailsForCarrier,
				"receiver": *data.Settings.ReceiverEmailsForCarrier,
				"type": models.EmailCarrierAppointmentQueue,
			})
		}

		return err
	}

	now := time.Now()

	notificationID, err := helpers.UpdateNotification(&models.Notification{
		NotificationID: data.NotificationID,
		OrderID: data.OrderID,
		Sender: *data.Settings.SenderEmailsForCarrier,
		Receiver: *data.Settings.ReceiverEmailsForCarrier,
		SenderCC: data.Settings.SenderCCEmailsForCarrier,
		ReceiverCC: data.Settings.ReceiverCCEmailsForCarrier,
		Method: "email",
		Type: models.EmailCarrierAppointmentQueue,
		Status: "sent",
		SentAt: &now,
	})

	if err != nil {
		helpers.LogException("[worker] failed to update notification status to sent", map[string]interface{}{
			"error": err.Error(),
			"notification_id": data.NotificationID,
			"order_id": data.OrderID,
		})
	}

	helpers.LogInfo("[worker] appointment email sent successfully", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
		"notification_id": notificationID,
	})

	return nil

}

// # Send Reminder Email
func SendAppointmentReminderEmail(ctx context.Context, task *asynq.Task) error {
	defer func() {
		if r := recover(); r != nil {
			helpers.LogException("[worker] carrier appointment reminder email worker panic recovered", map[string]interface{}{
				"panic": r,
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})
		}
	}()

	helpers.LogInfo("[worker] carrier appointment reminder email worker started", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload_length": len(task.Payload()),
	})

	if task == nil {
		helpers.LogInfo("[worker] carrier appointment reminder email task is nil", map[string]interface{}{
			"task_data": string(task.Payload()),
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier appointment reminder email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload": string(payload),
	})

	var data models.CarrierAppointmentEmailWorkerData
	err := json.Unmarshal(payload, &data)
	if err != nil {
		helpers.LogException("[worker] failed to unmarshal carrier appointment reminder email payload", map[string]interface{}{
			"error": err.Error(),
			"payload": string(payload),
			"task_type": task.Type(),
			"task_data": string(task.Payload()),
		})
		return err
	}

	helpers.LogInfo("[worker] carrier appointment reminder email payload", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"data": data,
		"notification_id": data.NotificationID,
		"order_id": data.OrderID,
	})

	masterWaybill := ""
	if data.Data.MasterWaybill != nil && len(*data.Data.MasterWaybill) > 0 {
		masterWaybill = strings.Join(*data.Data.MasterWaybill, ", ")
	}

	childWaybill := ""
	if data.Data.ChildWaybill != nil && len(*data.Data.ChildWaybill) > 0 {
		childWaybill = strings.Join(*data.Data.ChildWaybill, ", ")
	}

	poNumbers := ""
	if data.Data.PONumber != nil && len(*data.Data.PONumber) > 0 {
		poNumbers = strings.Join(*data.Data.PONumber, ", ")
	}

	var cartonDetails strings.Builder
	for i, carton := range *data.Data.Cartons {
		cartonDetails.WriteString(fmt.Sprintf(
			"<tr><td>%d</td><td>%.2f x %.2f x %.2f Inch</td><td>%.f</td><td>%.2f KG</td></tr>",
			i+1,
			helpers.CmToInch(&carton.Length),
			helpers.CmToInch(&carton.Breadth), 
			helpers.CmToInch(&carton.Height),
			carton.Quantity,
			helpers.DerefFloatPointer(helpers.RoundFloat(carton.Weight / 1000.0)),
		))
	}

	body := fmt.Sprintf(
		`<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
				.order-details { margin: 20px 0; }
				.details-row { margin-bottom: 8px; }
				.label { font-weight: bold; display: inline; }
				.value { display: inline; margin-left: 5px; }
				.carton-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
				.carton-table th, .carton-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
				.carton-table th { background: #f5f5f5; font-weight: bold; }
				.location-section { margin: 20px 0; }
				.location-title { font-weight: bold; margin-bottom: 8px; }
				.location-row { margin-bottom: 6px; font-size: 14px; }
				.footer { margin-top: 30px; }
			</style>
		</head>
		<body>
			<p>Hello %s Team,</p>
			<p>This is a gentle reminder about the upcoming delivery for the following order. Kindly ensure all arrangements are made for an on-time and smooth delivery.</p>

			<div class="order-details">
				<div class="details-row">
					<span class="label">LR Number:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">PO Number:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Master Waybills:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Child Waybills:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Delivery Schedule:</span>
					<span class="value">%s</span>
				</div>
				<div class="details-row">
					<span class="label">Total Cartons:</span>
					<span class="value">%d</span>
				</div>
				<div class="details-row">
					<span class="label">Total Weight:</span>
					<span class="value">%.2f KG</span>
				</div>
			</div>
	
			<div class="order-details">
				<div class="location-title">Carton Details</div>
				<table class="carton-table">
					<tr>
						<th>Carton</th>
						<th>Dimensions (L x B x H)</th>
						<th>Quantity</th>
						<th>Weight</th>
					</tr>
					%s
				</table>
			</div>
	
			<div class="location-section">
				<div class="location-title">Delivery Address</div>
				<div class="location-row">
					<span class="label">Name:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Address:</span>
					<span class="value">%s<br>%s, %s - %s</span>
				</div>
				<div class="location-row">
					<span class="label">Contact:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Email:</span>
					<span class="value">%s</span>
				</div>
			</div>
	
			<div class="location-section">
				<div class="location-title">Pickup Address</div>
				<div class="location-row">
					<span class="label">Name:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Address:</span>
					<span class="value">%s<br>%s, %s - %s</span>
				</div>
				<div class="location-row">
					<span class="label">Contact:</span>
					<span class="value">%s</span>
				</div>
				<div class="location-row">
					<span class="label">Email:</span>
					<span class="value">%s</span>
				</div>
			</div>
	
			<div class="footer">
				<p><strong>Best regards,</strong><br>Openleaf Team</p>
			</div>
		</body>
		</html>`,
		data.Data.CarrierName,
		helpers.DerefStringPointer(data.Data.LRNumber),
		poNumbers,
		masterWaybill,
		childWaybill,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentDate),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(helpers.RoundFloat(helpers.RoundFloat(data.Data.TotalDeadWeight) / 1000.0)),
		cartonDetails.String(),

		// Delivery warehouse
		helpers.DerefStringPointer(data.Data.WarehouseName),
		helpers.DerefStringPointer(data.Data.WarehouseAddress),
		helpers.DerefStringPointer(data.Data.WarehouseCity),
		helpers.DerefStringPointer(data.Data.WarehouseState),
		helpers.DerefStringPointer(data.Data.WarehousePin),
		helpers.DerefStringPointer(data.Data.WarehousePhone),
		helpers.DerefStringPointer(data.Data.WarehouseEmail),

		// Pickup warehouse
		helpers.DerefStringPointer(data.Data.CustomerWarehouseName),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAddress),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseCity),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseState),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePin),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseEmail),
	)

	receiverEmails := strings.Split(*data.Settings.ReceiverEmailsForCarrier, ",")
	receiverCC := []string{}
	if data.Settings.ReceiverCCEmailsForCarrier != nil {
		receiverCC = append(receiverCC, strings.Split(*data.Settings.ReceiverCCEmailsForCarrier, ",")...)
	}
	if data.Settings.SenderCCEmailsForCarrier != nil {
		receiverCC = append(receiverCC, strings.Split(*data.Settings.SenderCCEmailsForCarrier, ",")...)
	}

	// Prepare file URLs for attachment  
	var fileURLs []string
	if data.Data.Files != nil {
		fileURLs = *data.Data.Files
	}

	helpers.LogInfo("[worker] attempting to send reminder email", map[string]interface{}{
		"from": helpers.B2B_EMAIL,
		"to": receiverEmails,
		"cc": receiverCC,
		"subject": fmt.Sprintf("Reminder: Delivery for LR %s on %s",
			helpers.DerefStringPointer(data.Data.LRNumber),
			helpers.FormatDateDDMMYYYY(data.Data.AppointmentDate),
		),
		"body_length": len(body),
		"files_count": len(fileURLs),
	})

	err = helpers.SendEmail(
		helpers.B2B_EMAIL, 
		receiverEmails, 
		receiverCC, 
		fmt.Sprintf("Reminder: Delivery for LR %s on %s",
			helpers.DerefStringPointer(data.Data.LRNumber),
			helpers.FormatDateDDMMYYYY(data.Data.AppointmentDate),
		), 
		body,
		true,
		fileURLs,
	)

	if err != nil {
		helpers.LogException("[worker] failed to send reminder email", map[string]interface{}{
			"error": err.Error(),
			"task_type": task.Type(),
			"task_data": string(task.Payload()),
		})

		notificationID, err := helpers.UpdateNotification(&models.Notification{
			NotificationID: data.NotificationID,
			OrderID: data.OrderID,
			Sender: *data.Settings.SenderEmailsForCarrier,
			Receiver: *data.Settings.ReceiverEmailsForCarrier,
			SenderCC: data.Settings.SenderCCEmailsForCarrier,
			ReceiverCC: data.Settings.ReceiverCCEmailsForCarrier,
			Method: "email",
			Type: models.EmailCarrierAppointmentReminderQueue,
			Status: "worker_error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("[worker] failed to update notification", map[string]interface{}{
				"error": err.Error(),
				"notification_id": notificationID,
				"order_id": data.OrderID,
				"sender": *data.Settings.SenderEmailsForCarrier,
				"receiver": *data.Settings.ReceiverEmailsForCarrier,
				"sender_cc": data.Settings.SenderCCEmailsForCarrier,
				"receiver_cc": data.Settings.ReceiverCCEmailsForCarrier,
				"type": models.EmailCarrierAppointmentReminderQueue,
			})
		}
		
		return err
	}

	helpers.LogInfo("[worker] reminder email sent successfully", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"data": data,
	})

	now := time.Now()

	notificationID, err := helpers.UpdateNotification(&models.Notification{
		NotificationID: data.NotificationID,
		OrderID: data.OrderID,
		Sender: *data.Settings.SenderEmailsForCarrier,
		Receiver: *data.Settings.ReceiverEmailsForCarrier,
		SenderCC: data.Settings.SenderCCEmailsForCarrier,
		ReceiverCC: data.Settings.ReceiverCCEmailsForCarrier,
		Method: "email",
		Type: models.EmailCarrierAppointmentReminderQueue,
		Status: "sent",
		SentAt: &now,
	})

	if err != nil {
		helpers.LogException("[worker] failed to update notification status to sent", map[string]interface{}{
			"error": err.Error(),
			"notification_id": data.NotificationID,
			"order_id": data.OrderID,
		})
	}

	helpers.LogInfo("[worker] reminder email sent successfully", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"data": data,
		"notification_id": notificationID,
	})

	return nil

}