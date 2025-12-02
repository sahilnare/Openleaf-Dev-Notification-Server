package carrierWorker

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/templates"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	masterWaybillHTML := ""
	if data.Data.MasterWaybill != nil && len(*data.Data.MasterWaybill) > 0 {
		masterWaybill := strings.Join(*data.Data.MasterWaybill, ", ")
		if masterWaybill != "" {
			masterWaybillHTML = fmt.Sprintf(`<div class="details-row"><span class="label">Master Waybills:</span><span class="value">%s</span></div>`, masterWaybill)
		}
	}

	childWaybillHTML := ""
	if data.Data.ChildWaybill != nil && len(*data.Data.ChildWaybill) > 0 {
		childWaybill := strings.Join(*data.Data.ChildWaybill, ", ")
		if childWaybill != "" {
			childWaybillHTML = fmt.Sprintf(`<div class="details-row"><span class="label">Child Waybills:</span><span class="value">%s</span></div>`, childWaybill)
		}
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


	body := fmt.Sprintf(templates.SendAppointmentEmailTemplate,
		data.Data.CarrierName,
		helpers.DerefStringPointer(data.Data.LRNumber),
		poNumbers,
		masterWaybillHTML,
		childWaybillHTML,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentScheduledAt),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(helpers.RoundFloat(helpers.RoundFloat(data.Data.TotalDeadWeight) / 1000.0)),
		cartonDetails.String(),
	
		// Delivery warehouse - SWAPPED: CustomerWarehouse is now delivery
		helpers.DerefStringPointer(data.Data.CustomerWarehouseName),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAddress),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseCity),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseState),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePin),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseEmail),
	
		// Pickup warehouse - SWAPPED: Warehouse is now pickup
		helpers.DerefStringPointer(data.Data.WarehouseName),
		helpers.DerefStringPointer(data.Data.WarehouseAddress),
		helpers.DerefStringPointer(data.Data.WarehouseCity),
		helpers.DerefStringPointer(data.Data.WarehouseState),
		helpers.DerefStringPointer(data.Data.WarehousePin),
		helpers.DerefStringPointer(data.Data.WarehousePhone),
		helpers.DerefStringPointer(data.Data.WarehouseEmail),
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

	subject := fmt.Sprintf("Delivery Scheduled for LR %s on %s",
		helpers.DerefStringPointer(data.Data.LRNumber),
		helpers.FormatDateDDMMYYYY(data.Data.AppointmentScheduledAt),
	)
	
	// Append display_company_name to subject if it exists
	displayCompanyName := helpers.GetDisplayCompanyName(data.UserID)
	if displayCompanyName != nil && *displayCompanyName != "" {
		subject = fmt.Sprintf("%s <> %s", subject, *displayCompanyName)
	}

	helpers.LogInfo("[worker] attempting to send email", map[string]interface{}{
		"from": helpers.B2B_EMAIL,
		"to": receiverEmails,
		"cc": receiverCC,
		"subject": subject,
		"body_length": len(body),
		"files_count": len(fileURLs),
	})

	err = helpers.SendEmail(
		helpers.B2B_EMAIL, 
		receiverEmails, 
		receiverCC, 
		subject,
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

	now := helpers.GetISTTime()

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

	// Only show Master Waybills row if data exists
	masterWaybillHTML := ""
	if data.Data.MasterWaybill != nil && len(*data.Data.MasterWaybill) > 0 {
		masterWaybill := strings.Join(*data.Data.MasterWaybill, ", ")
		if masterWaybill != "" {
			masterWaybillHTML = fmt.Sprintf(`<div class="details-row"><span class="label">Master Waybills:</span><span class="value">%s</span></div>`, masterWaybill)
		}
	}

	// Only show Child Waybills row if data exists
	childWaybillHTML := ""
	if data.Data.ChildWaybill != nil && len(*data.Data.ChildWaybill) > 0 {
		childWaybill := strings.Join(*data.Data.ChildWaybill, ", ")
		if childWaybill != "" {
			childWaybillHTML = fmt.Sprintf(`<div class="details-row"><span class="label">Child Waybills:</span><span class="value">%s</span></div>`, childWaybill)
		}
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

	body := fmt.Sprintf(templates.SendAppointmentReminderEmailTemplate,
		data.Data.CarrierName,
		helpers.DerefStringPointer(data.Data.LRNumber),
		poNumbers,
		masterWaybillHTML,
		childWaybillHTML,
		helpers.FormatDateDDMMYYYYHHMM(data.Data.AppointmentScheduledAt),
		helpers.DerefIntPointer(data.Data.TotalCartons),
		helpers.DerefFloatPointer(helpers.RoundFloat(helpers.RoundFloat(data.Data.TotalDeadWeight) / 1000.0)),
		cartonDetails.String(),

		// Delivery warehouse - SWAPPED: CustomerWarehouse is now delivery
		helpers.DerefStringPointer(data.Data.CustomerWarehouseName),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseAddress),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseCity),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseState),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePin),
		helpers.DerefStringPointer(data.Data.CustomerWarehousePhone),
		helpers.DerefStringPointer(data.Data.CustomerWarehouseEmail),

		// Pickup warehouse - SWAPPED: Warehouse is now pickup
		helpers.DerefStringPointer(data.Data.WarehouseName),
		helpers.DerefStringPointer(data.Data.WarehouseAddress),
		helpers.DerefStringPointer(data.Data.WarehouseCity),
		helpers.DerefStringPointer(data.Data.WarehouseState),
		helpers.DerefStringPointer(data.Data.WarehousePin),
		helpers.DerefStringPointer(data.Data.WarehousePhone),
		helpers.DerefStringPointer(data.Data.WarehouseEmail),
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

	subject := fmt.Sprintf("Reminder: Delivery for LR %s on %s",
		helpers.DerefStringPointer(data.Data.LRNumber),
		helpers.FormatDateDDMMYYYY(data.Data.AppointmentScheduledAt),
	)
	
	// Append display_company_name to subject if it exists
	displayCompanyName := helpers.GetDisplayCompanyName(data.UserID)
	if displayCompanyName != nil && *displayCompanyName != "" {
		subject = fmt.Sprintf("%s <> %s", subject, *displayCompanyName)
	}

	helpers.LogInfo("[worker] attempting to send reminder email", map[string]interface{}{
		"from": helpers.B2B_EMAIL,
		"to": receiverEmails,
		"cc": receiverCC,
		"subject": subject,
		"body_length": len(body),
		"files_count": len(fileURLs),
	})

	err = helpers.SendEmail(
		helpers.B2B_EMAIL, 
		receiverEmails, 
		receiverCC, 
		subject, 
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

	now := helpers.GetISTTime()

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