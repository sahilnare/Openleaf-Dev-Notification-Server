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

	var data models.CarrierAppointmentEmailWorkerData
	err := json.Unmarshal(payload, &data)
	if err != nil {
		return err
	}

	helpers.LogInfo("[worker] carrier appointment email payload", map[string]interface{}{
		"task_type": task.Type(),
		"data": data,
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
			"<tr><td>%d</td><td>%.2f x %.2f x %.2f cm</td><td>%.f</td><td>%.2f KG</td></tr>",
			i+1,
			carton.Length,
			carton.Breadth, 
			carton.Height,
			carton.Quantity,
			carton.Weight,
		))
	}


	body := fmt.Sprintf(
		`<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
				.container { max-width: 600px; margin: 0 auto; }
				.order-details { border: 1px solid #ddd; padding: 15px; margin: 20px 0; }
				.details-grid { display: grid; grid-template-columns: 140px 1fr; gap: 8px; }
				.carton-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
				.carton-table th, .carton-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
				.carton-table th { background: #f5f5f5; font-weight: bold; }
				.location-section { margin: 20px 0; }
				.location-title { font-weight: bold; margin-bottom: 8px; padding-bottom: 4px; border-bottom: 1px solid #ddd; }
				.location-grid { display: grid; grid-template-columns: 60px 1fr; gap: 6px; font-size: 14px; }
				.footer { margin-top: 30px; padding-top: 15px; border-top: 1px solid #ddd; }
			</style>
		</head>
		<body>
			<div class="container">
				<h3 style="margin: 0 0 20px 0;">Delivery Schedule Notification</h3>
				<p>Hello %s Team,</p>
				<p>This is to inform you about the delivery schedule for the following order:</p>
	
				<div class="order-details">
					<div class="details-grid">
						<span>LR Number:</span><span><strong>%s</strong></span>
						<span>PO Number:</span><span><strong>%s</strong></span>
						<span>Master Waybills:</span><span>%s</span>
						<span>Child Waybills:</span><span>%s</span>
						<span>Delivery Schedule:</span><span><strong>%s</strong></span>
						<span>Total Cartons:</span><span>%d</span>
						<span>Total Weight:</span><span>%.2f KG</span>
					</div>
				</div>
	
				<div class="order-details">
					<div class="location-title" style="margin-bottom: 10px;">Carton Details</div>
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
					<div class="location-grid">
						<span>Name:</span><span>%s</span>
						<span>Address:</span><span>%s<br>%s, %s - %s</span>
						<span>Contact:</span><span>%s</span>
						<span>Email:</span><span>%s</span>
					</div>
				</div>
	
				<div class="location-section">
					<div class="location-title">Pickup Address</div>
					<div class="location-grid">
						<span>Name:</span><span>%s</span>
						<span>Address:</span><span>%s<br>%s, %s - %s</span>
						<span>Contact:</span><span>%s</span>
						<span>Email:</span><span>%s</span>
					</div>
				</div>
	
				<div class="footer">
					<p><strong>Best regards,</strong><br>Openleaf Team</p>
				</div>
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
		helpers.DerefFloatPointer(data.Data.TotalDeadWeight),
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

	receiverEmails := strings.Split(data.Settings.ReceiverEmailsForCarrier, ",")
	receiverCC := strings.Split(data.Settings.ReceiverCCEmailsForCarrier + "," + data.Settings.SenderCCEmailsForCarrier, ",")

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
		*data.Data.Files,
	)

	if err != nil {
		helpers.LogException("[worker] failed to send appointment email", map[string]interface{}{
			"error": err.Error(),
		})

		notificationID, err := helpers.UpdateNotification(&models.Notification{
			NotificationID: data.NotificationID,
			OrderID: data.OrderID,
			Sender: data.Settings.SenderEmailsForCarrier,
			CC: fmt.Sprintf("%s,%s", data.Settings.ReceiverCCEmailsForCarrier, data.Settings.SenderCCEmailsForCarrier),
			Receiver: data.Settings.ReceiverEmailsForCarrier,
			Type: "appointment",
			Status: "worker_error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("[worker] failed to update notification", map[string]interface{}{
				"error": err.Error(),
				"notification_id": notificationID,
				"order_id": data.OrderID,
				"sender": data.Settings.SenderEmailsForCarrier,
				"cc": fmt.Sprintf("%s,%s", data.Settings.ReceiverCCEmailsForCarrier, data.Settings.SenderCCEmailsForCarrier),
				"receiver": data.Settings.ReceiverEmailsForCarrier,
				"type": "appointment",
			})
		}

	}

	now := time.Now()

	notificationID, _ := helpers.UpdateNotification(&models.Notification{
		NotificationID: data.NotificationID,
		OrderID: data.OrderID,
		Sender: data.Settings.SenderEmailsForCarrier,
		CC: fmt.Sprintf("%s,%s", data.Settings.ReceiverCCEmailsForCarrier, data.Settings.SenderCCEmailsForCarrier),
		Receiver: data.Settings.ReceiverEmailsForCarrier,
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

	var data models.CarrierAppointmentEmailWorkerData
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

	var cartonTableRows strings.Builder
	for i, carton := range *data.Data.Cartons {
		cartonTableRows.WriteString(fmt.Sprintf(
			"<tr><td>%d</td><td>%.2f x %.2f x %.2f cm</td><td>%.f</td><td>%.2f KG</td></tr>",
			i+1,
			carton.Length,
			carton.Breadth,
			carton.Height,
			carton.Quantity,
			carton.Weight,
		))
	}


	body := fmt.Sprintf(
		`<html>
		<head>
			<style>
				body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
				.container { max-width: 600px; margin: 0 auto; }
				.order-details { border: 1px solid #ddd; padding: 15px; margin: 20px 0; }
				.details-grid { display: grid; grid-template-columns: 140px 1fr; gap: 8px; }
				.carton-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
				.carton-table th, .carton-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
				.carton-table th { background: #f5f5f5; font-weight: bold; }
				.location-section { margin: 20px 0; }
				.location-title { font-weight: bold; margin-bottom: 8px; padding-bottom: 4px; border-bottom: 1px solid #ddd; }
				.location-grid { display: grid; grid-template-columns: 60px 1fr; gap: 6px; font-size: 14px; }
				.footer { margin-top: 30px; padding-top: 15px; border-top: 1px solid #ddd; }
			</style>
		</head>
		<body>
			<div class="container">
				<h3 style="margin: 0 0 20px 0;">Delivery Reminder</h3>
				<p>Hello %s Team,</p>
				<p>This is a gentle reminder about the upcoming delivery for the following order. Kindly ensure all arrangements are made for an on-time and smooth delivery.</p>
	
				<div class="order-details">
					<div class="details-grid">
						<span>LR Number:</span><span><strong>%s</strong></span>
						<span>Master Waybills:</span><span>%s</span>
						<span>Child Waybills:</span><span>%s</span>
						<span>Delivery Schedule:</span><span><strong>%s</strong></span>
						<span>Total Cartons:</span><span>%d</span>
						<span>Total Weight:</span><span>%.2f KG</span>
					</div>
				</div>
	
				<div class="order-details">
					<div class="location-title" style="margin-bottom: 10px;">Carton Details</div>
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
					<div class="location-grid">
						<span>Name:</span><span>%s</span>
						<span>Address:</span><span>%s<br>%s, %s - %s</span>
						<span>Contact:</span><span>%s</span>
						<span>Email:</span><span>%s</span>
					</div>
				</div>
	
				<div class="location-section">
					<div class="location-title">Pickup Address</div>
					<div class="location-grid">
						<span>Name:</span><span>%s</span>
						<span>Address:</span><span>%s<br>%s, %s - %s</span>
						<span>Contact:</span><span>%s</span>
						<span>Email:</span><span>%s</span>
					</div>
				</div>
	
				<div class="footer">
					<p><strong>Best regards,</strong><br>Openleaf Team</p>
				</div>
			</div>
		</body>
		</html>`,
		data.Data.CarrierName,
		helpers.DerefStringPointer(data.Data.LRNumber),
		masterWaybill,
		childWaybill,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentDate),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(data.Data.TotalDeadWeight),
		cartonTableRows.String(), // Add carton table rows here
	
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

	receiverEmails := strings.Split(data.Settings.ReceiverEmailsForCarrier, ",")
	receiverCC := strings.Split(data.Settings.ReceiverCCEmailsForCarrier + "," + data.Settings.SenderCCEmailsForCarrier, ",")

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
		*data.Data.Files,
	)

	if err != nil {
		helpers.LogException("[worker] failed to send reminder email", map[string]interface{}{
			"error": err.Error(),
		})

		notificationID, err := helpers.UpdateNotification(&models.Notification{
			NotificationID: data.NotificationID,
			OrderID: data.OrderID,
			Sender: data.Settings.SenderEmailsForCarrier,
			CC: fmt.Sprintf("%s,%s", data.Settings.ReceiverCCEmailsForCarrier, data.Settings.SenderCCEmailsForCarrier),
			Receiver: data.Settings.ReceiverEmailsForCarrier,
			Type: "appointment_reminder",
			Status: "worker_error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("[worker] failed to update notification", map[string]interface{}{
				"error": err.Error(),
				"notification_id": notificationID,
				"order_id": data.OrderID,
				"sender": data.Settings.SenderEmailsForCarrier,
				"cc": fmt.Sprintf("%s,%s", data.Settings.ReceiverCCEmailsForCarrier, data.Settings.SenderCCEmailsForCarrier),
				"receiver": data.Settings.ReceiverEmailsForCarrier,
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
		NotificationID: data.NotificationID,
		OrderID: data.OrderID,
		Sender: data.Settings.SenderEmailsForCarrier,
		CC: fmt.Sprintf("%s,%s", data.Settings.ReceiverCCEmailsForCarrier, data.Settings.SenderCCEmailsForCarrier),
		Receiver: data.Settings.ReceiverEmailsForCarrier,
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