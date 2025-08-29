package controllers

import (
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func ScheduleCarrierAppointmentEmail(c *gin.Context) {

	var request models.ScheduleAppointmentEmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		helpers.LogException("invalid request", map[string]interface{}{
			"request": request,
			"request_headers": c.Request.Header,
			"request_url": c.Request.URL,
			"request_method": c.Request.Method,	
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, models.ServerResponse{
			Success: false,
			StatusCode: http.StatusBadRequest,
			Message: "Invalid request",
			Error: err.Error(),
		})
		return
	}

	var data models.CarrierAppointmentEmailData
	
	query := `
		SELECT 
			o.po_number, 
			o.lr_number, 
			o.order_created_at, 
			o.channel, 
			o.warehouse_id, 
			o.customer_warehouse_id, 
			o.carton_details, 
			o.total_cartons, 
			o.total_dead_weight, 
			o.total_volumetric_weight,
			o.carrier_name,

			od.master_waybill,
			od.child_waybills,
			od.uploaded_documents,

			to.expected_delivery_date,

			oa.order_placed_at,
			oa.is_appointment_confirmed,
			oa.appointment_scheduled_at

			-- Pickup Location (Warehouse)
			pl.warehouse_name,
			pl.address_line_1,
			pl.city,
			pl.state,
			pl.pincode,
			pl.phone,
			pl.email,

			-- Customer Location (Customer Warehouse)
			cl.warehouse_name,
			cl.address_line_1,
			cl.city,
			cl.state,
			cl.pincode,
			cl.phone,
			cl.email

		FROM orders o
		LEFT JOIN order_documents od ON o.order_id = od.order_id
		LEFT JOIN tracking_orders to ON o.order_id = to.order_id
		LEFT JOIN order_activity oa ON o.order_id = oa.order_id
		LEFT JOIN pickup_locations pl ON o.warehouse_id = pl.warehouse_id
		LEFT JOIN customer_locations cl ON o.customer_warehouse_id = cl.warehouse_id
		WHERE o.order_id = $1
	`

	var (
		poNumberJSON        []byte
		lrNumber            *string
		appointmentDate     *time.Time
		channel             string
		warehouseID         *string
		customerWarehouseID *string
		cartonDetailsJSON   []byte
		totalCartons        *int
		totalDeadWeight     *float64
		totalVolumetricWeight *float64
		carrierName *string
		masterWaybill       *string
		childWaybillsJSON   []byte
		uploadedDocumentsJSON []byte

		expectedDeliveryDate *time.Time

		// Pickup Location
		warehouseName, warehouseAddress, warehouseCity, warehouseState, warehousePin, warehousePhone, warehouseEmail *string

		// Customer Location
		customerWarehouseName, customerWarehouseAddress, customerWarehouseCity, customerWarehouseState, customerWarehousePin, customerWarehousePhone, customerWarehouseEmail *string

		orderPlacedAt *time.Time
		isAppointmentConfirmed *bool
		appointmentScheduledAt *time.Time
	)

	err := db.GlobalDB.QueryRow(query, request.OrderID).Scan(
		&poNumberJSON,
		&lrNumber,
		&appointmentDate,
		&channel,
		&warehouseID,
		&customerWarehouseID,
		&cartonDetailsJSON,
		&totalCartons,
		&totalDeadWeight,
		&totalVolumetricWeight,
		&carrierName,
		&masterWaybill,
		&childWaybillsJSON,
		&uploadedDocumentsJSON,

		&expectedDeliveryDate,

		// Pickup Location
		&warehouseName,
		&warehouseAddress,
		&warehouseCity,
		&warehouseState,
		&warehousePin,
		&warehousePhone,
		&warehouseEmail,

		// Customer Location
		&customerWarehouseName,
		&customerWarehouseAddress,
		&customerWarehouseCity,
		&customerWarehouseState,
		&customerWarehousePin,
		&customerWarehousePhone,
		&customerWarehouseEmail,

		&orderPlacedAt,
		&isAppointmentConfirmed,
		&appointmentScheduledAt,
	)

	if isAppointmentConfirmed != nil && !*isAppointmentConfirmed {
		helpers.LogInfo("appointment not confirmed for order", map[string]interface{}{
			"order_id": request.OrderID,
			"is_appointment_confirmed": isAppointmentConfirmed,
		})
		c.JSON(http.StatusOK, models.ServerResponse{
			Success: true,
			StatusCode: http.StatusOK,
			Message: "Appointment not confirmed for this order",
		})
		return
	}

	if err == nil {
		// Use the ScheduleCarrierAppointmentEmailData struct as defined in models

		// Unmarshal po_number
		var poNumbers []string
		if poNumberJSON != nil {
			_ = json.Unmarshal(poNumberJSON, &poNumbers)
			data.PONumber = &poNumbers
		}
		data.LRNumber = lrNumber
		data.AppointmentDate = appointmentDate
		data.Channel = channel
		data.TotalCartons = totalCartons
		data.TotalDeadWeight = totalDeadWeight
		data.TotalVolumetricWeight = totalVolumetricWeight
		data.CarrierName = *carrierName
		// Unmarshal carton_details
		var cartons []models.CartonDetails
		if cartonDetailsJSON != nil {
			_ = json.Unmarshal(cartonDetailsJSON, &cartons)
			data.Cartons = &cartons
		}

		// MasterWaybill
		if masterWaybill != nil {
			mw := []string{*masterWaybill}
			data.MasterWaybill = &mw
		}

		// ChildWaybill
		if childWaybillsJSON != nil {
			var cw []string
			_ = json.Unmarshal(childWaybillsJSON, &cw)
			data.ChildWaybill = &cw
		}

		// Files (from uploaded_documents)
		if uploadedDocumentsJSON != nil {
			var docsMap map[string][]string
			_ = json.Unmarshal(uploadedDocumentsJSON, &docsMap)
			var files []string
			for _, arr := range docsMap {
				files = append(files, arr...)
			}
			data.Files = &files
		}

		// Set Pickup Location (Warehouse) fields
		data.WarehouseName = warehouseName
		data.WarehouseAddress = warehouseAddress
		data.WarehouseCity = warehouseCity
		data.WarehouseState = warehouseState
		data.WarehousePin = warehousePin
		data.WarehousePhone = warehousePhone
		data.WarehouseEmail = warehouseEmail
		data.WarehouseAlternatePhone = nil

		// Set Customer Location (Customer Warehouse) fields
		data.CustomerWarehouseName = customerWarehouseName
		data.CustomerWarehouseAddress = customerWarehouseAddress
		data.CustomerWarehouseCity = customerWarehouseCity
		data.CustomerWarehouseState = customerWarehouseState
		data.CustomerWarehousePin = customerWarehousePin
		data.CustomerWarehousePhone = customerWarehousePhone
		data.CustomerWarehouseEmail = customerWarehouseEmail
		data.CustomerWarehouseAlternatePhone = nil

		data.ExpectedDeliveryDate = expectedDeliveryDate
		data.OrderPlacedAt = orderPlacedAt
		data.IsAppointmentConfirmed = isAppointmentConfirmed
		data.AppointmentScheduledAt = appointmentScheduledAt
	}

	// Fetch notification settings
	var notificationSettings models.CarrierAppointmentEmailSettings

	err = db.GlobalDB.QueryRow(`
		SELECT 
			admin_id,
			user_id,
			channel,
			carrier_id,
			sender_emails_for_channel,
			sender_cc_emails_for_channel,
			receiver_emails_for_channel,
			receiver_cc_emails_for_channel,
			sender_emails_for_carrier,
			sender_cc_emails_for_carrier,
			receiver_emails_for_carrier,
			receiver_cc_emails_for_carrier,
			
			send_notification,
			notification_days,
			notification_type,
			
			send_reminder,
			reminder_days,
			reminder_type,
			
			send_bulk_reminder,
			bulk_reminder_days,
			bulk_reminder_type,
			created_at
		FROM appointment_notification_settings
		WHERE user_id = $1 AND admin_id = $2
	`, request.UserID, request.AdminID).Scan(
		&notificationSettings.AnsID,
		&notificationSettings.AdminID,
		&notificationSettings.UserID,
		&notificationSettings.Channel,
		&notificationSettings.CarrierID,
		&notificationSettings.SenderEmailsForChannel,
		&notificationSettings.SenderCCEmailsForChannel,
		&notificationSettings.ReceiverEmailsForChannel,
		&notificationSettings.ReceiverCCEmailsForChannel,
		&notificationSettings.SenderEmailsForCarrier,
		&notificationSettings.SenderCCEmailsForCarrier,
		&notificationSettings.ReceiverEmailsForCarrier,
		&notificationSettings.ReceiverCCEmailsForCarrier,
		&notificationSettings.SendNotification,
		&notificationSettings.NotificationDays,
		&notificationSettings.NotificationType,
		&notificationSettings.SendReminder,
		&notificationSettings.ReminderDays,
		&notificationSettings.ReminderType,
		&notificationSettings.SendBulkReminder,
		&notificationSettings.BulkReminderDays,
		&notificationSettings.BulkReminderType,
		&notificationSettings.CreatedAt,
	)

	if err != nil {
		helpers.LogException("failed to fetch notification settings", map[string]interface{}{
			"order_id": request.OrderID,
			"user_id": request.UserID,
			"admin_id": request.AdminID,
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, models.ServerResponse{
			Success: false,
			StatusCode: http.StatusInternalServerError,
			Message: "Failed to fetch notification settings",
			Error: err.Error(),
		})
		return
	}

	queueData := models.CarrierAppointmentEmailWorkerData{
		OrderID: request.OrderID,
		AdminID: request.AdminID,
		UserID: request.UserID,
		Data: data,
		Settings: notificationSettings,
	}


	// # Schedule appointment email
	var appointmentSendAt []time.Time

	if notificationSettings.SendNotification {
		
		sendAt := time.Now().Add(time.Second * 5)
		
		switch notificationSettings.NotificationType {
		case "after_appointment_taken":

			days := strings.Split(strings.TrimSpace(notificationSettings.NotificationDays), ",")
			for _, day := range days {
				daysInt, _ := strconv.Atoi(strings.TrimSpace(day))
				sendAt = appointmentScheduledAt.Add(time.Duration(daysInt) * time.Hour * 24)

				if sendAt.Before(time.Now()) {
					sendAt = time.Now().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to marshal request payload",
						Error: err.Error(),
					})
					return
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(sendAt))
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request": request,
						"settings": notificationSettings,
						"payload": payload,
						"error": err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID: request.OrderID,
						Sender: notificationSettings.SenderEmailsForCarrier,
						CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
						Receiver: notificationSettings.ReceiverEmailsForCarrier,
						Type: models.EmailCarrierAppointmentQueue,
						Status: "error",
						SentAt: nil,
					})
			
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to enqueue email task",
						Error: err.Error(),
					})
			
					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID: request.OrderID,
					Sender: notificationSettings.SenderEmailsForCarrier,
					CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
					Receiver: notificationSettings.ReceiverEmailsForCarrier,
					Type: models.EmailCarrierAppointmentQueue,
					Status: "scheduled",
					SentAt: nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":  info.ID,
					"queue":    info.Queue,
					"task_type": task.Type(),
					"send_at":  sendAt,
					"type": "after_appointment_taken",
					"notification_id": notificationID,
				})

				appointmentSendAt = append(appointmentSendAt, sendAt)
			}

		case "before_delivery":
			
			days := strings.Split(strings.TrimSpace(notificationSettings.NotificationDays), ",")
			for _, day := range days {
				daysInt, _ := strconv.Atoi(strings.TrimSpace(day))
				sendAt = expectedDeliveryDate.Add((-time.Duration(daysInt)) * time.Hour * 24)

				if sendAt.Before(time.Now()) {
					sendAt = time.Now().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to marshal request payload",
						Error: err.Error(),
					})
					return
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(sendAt))
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request": request,
						"settings": notificationSettings,
						"error": err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID: request.OrderID,
						Sender: notificationSettings.SenderEmailsForCarrier,
						CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
						Receiver: notificationSettings.ReceiverEmailsForCarrier,
						Type: models.EmailCarrierAppointmentQueue,
						Status: "error",
						SentAt: nil,
					})
			
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to enqueue email task",
						Error: err.Error(),
					})
			
					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID: request.OrderID,
					Sender: notificationSettings.SenderEmailsForCarrier,
					CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
					Receiver: notificationSettings.ReceiverEmailsForCarrier,
					Type: models.EmailCarrierAppointmentQueue,
					Status: "scheduled",
					SentAt: nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":  info.ID,
					"queue":    info.Queue,
					"task_type": task.Type(),
					"send_at":  sendAt,
					"type": "before_delivery",
					"notification_id": notificationID,
				})

				appointmentSendAt = append(appointmentSendAt, sendAt)

			}

		default:
			helpers.LogInfo("invalid notification type", map[string]interface{}{
				"order_id": request.OrderID,
				"notification_type": notificationSettings.NotificationType,
			})
			
			queueData.NotificationID = uuid.New()

			payload, err := json.Marshal(queueData)
			if err != nil {
				helpers.LogException("failed to marshal request payload", map[string]interface{}{
					"request": request,
					"request_headers": c.Request.Header,
					"request_url": c.Request.URL,
					"request_method": c.Request.Method,	
					"error": err.Error(),
				})
				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success: false,
					StatusCode: http.StatusInternalServerError,
					Message: "Failed to marshal request payload",
					Error: err.Error(),
				})
				return
			}

			task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

			info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(sendAt))
			if err != nil {
				helpers.LogException("failed to enqueue email task", map[string]interface{}{
					"request": request,
					"settings": notificationSettings,
					"payload": payload,
					"error": err.Error(),
				})

				helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID: request.OrderID,
					Sender: notificationSettings.SenderEmailsForCarrier,
					CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
					Receiver: notificationSettings.ReceiverEmailsForCarrier,
					Type: models.EmailCarrierAppointmentQueue,
					Status: "error",
					SentAt: nil,
				})

				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success: false,
					StatusCode: http.StatusInternalServerError,
					Message: "Failed to enqueue email task",
					Error: err.Error(),
				})
		
				return
			}

			notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: queueData.NotificationID,
				OrderID: request.OrderID,
				Sender: notificationSettings.SenderEmailsForCarrier,
				CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
				Receiver: notificationSettings.ReceiverEmailsForCarrier,
				Type: models.EmailCarrierAppointmentQueue,
				Status: "scheduled",
				SentAt: nil,
			})

			helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
				"task_id":  info.ID,
				"queue":    info.Queue,
				"task_type": task.Type(),
				"send_at":  sendAt,
				"type": "after_appointment_taken",
				"notification_id": notificationID,
			})

			appointmentSendAt = append(appointmentSendAt, sendAt)
			
		}
	}

	// # Check for reminders
	var reminderSendAt []time.Time

	if notificationSettings.SendReminder {

		sendAt := time.Now().Add(time.Second * 5)
		
		switch notificationSettings.ReminderType {
		case "after_appointment_taken":

			days := strings.Split(strings.TrimSpace(notificationSettings.ReminderDays), ",")
			for _, day := range days {
				daysInt, _ := strconv.Atoi(strings.TrimSpace(day))
				sendAt = appointmentScheduledAt.Add(time.Duration(daysInt) * time.Hour * 24)

				if sendAt.Before(time.Now()) {
					sendAt = time.Now().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(sendAt))
				if err != nil {
					helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
						"request": request,
						"settings": notificationSettings,
						"payload": payload,
						"error": err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID: request.OrderID,
						Sender: notificationSettings.SenderEmailsForCarrier,
						CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
						Receiver: notificationSettings.ReceiverEmailsForCarrier,
						Type: models.EmailCarrierAppointmentReminderQueue,
						Status: "error",
						SentAt: nil,
					})
			
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to enqueue reminder email task",
						Error: err.Error(),
					})
			
					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID: request.OrderID,
					Sender: notificationSettings.SenderEmailsForCarrier,
					CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
					Receiver: notificationSettings.ReceiverEmailsForCarrier,
					Type: models.EmailCarrierAppointmentReminderQueue,
					Status: "scheduled",
					SentAt: nil,
				})

				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id":  info.ID,
					"queue":    info.Queue,
					"task_type": task.Type(),
					"send_at":  sendAt,
					"type": "after_appointment_taken",
					"notification_id": notificationID,
				})

				reminderSendAt = append(reminderSendAt, sendAt)
			}

		case "before_delivery":
			
			days := strings.Split(strings.TrimSpace(notificationSettings.ReminderDays), ",")
			for _, day := range days {
				daysInt, _ := strconv.Atoi(strings.TrimSpace(day))
				sendAt = expectedDeliveryDate.Add((-time.Duration(daysInt)) * time.Hour * 24)

				if sendAt.Before(time.Now()) {
					sendAt = time.Now().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(sendAt))
				if err != nil {
					helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
						"request": request,
						"settings": notificationSettings,
						"payload": payload,
						"error": err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID: request.OrderID,
						Sender: notificationSettings.SenderEmailsForCarrier,
						CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
						Receiver: notificationSettings.ReceiverEmailsForCarrier,
						Type: models.EmailCarrierAppointmentReminderQueue,
						Status: "error",
						SentAt: nil,
					})
			
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to enqueue reminder email task",
						Error: err.Error(),
					})
			
					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID: request.OrderID,
					Sender: notificationSettings.SenderEmailsForCarrier,
					CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
					Receiver: notificationSettings.ReceiverEmailsForCarrier,
					Type: models.EmailCarrierAppointmentReminderQueue,
					Status: "scheduled",
					SentAt: nil,
				})

				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id":  info.ID,
					"queue":    info.Queue,
					"task_type": task.Type(),
					"send_at":  sendAt,
					"type": "before_delivery",
					"notification_id": notificationID,
				})

				reminderSendAt = append(reminderSendAt, sendAt)

			}

		default:
			helpers.LogInfo("invalid reminder notification type", map[string]interface{}{
				"order_id": request.OrderID,
				"notification_type": notificationSettings.ReminderType,
			})
			
			queueData.NotificationID = uuid.New()

			payload, err := json.Marshal(queueData)
			if err != nil {
				helpers.LogException("failed to marshal request payload", map[string]interface{}{
					"request": request,
					"request_headers": c.Request.Header,
					"request_url": c.Request.URL,
					"request_method": c.Request.Method,	
					"error": err.Error(),
				})
			}

			task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

			info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(sendAt))
			if err != nil {
				helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
					"request": request,
					"settings": notificationSettings,
					"payload": payload,
					"error": err.Error(),
				})

				helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID: request.OrderID,
					Sender: notificationSettings.SenderEmailsForCarrier,
					CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
					Receiver: notificationSettings.ReceiverEmailsForCarrier,
					Type: models.EmailCarrierAppointmentReminderQueue,
					Status: "error",
					SentAt: nil,
				})
		
				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success: false,
					StatusCode: http.StatusInternalServerError,
					Message: "Failed to enqueue reminder email task",
					Error: err.Error(),
				})
		
				return
			}

			notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: queueData.NotificationID,
				OrderID: request.OrderID,
				Sender: notificationSettings.SenderEmailsForCarrier,
				CC: fmt.Sprintf("%s,%s", notificationSettings.SenderCCEmailsForCarrier, notificationSettings.ReceiverCCEmailsForCarrier),
				Receiver: notificationSettings.ReceiverEmailsForCarrier,
				Type: models.EmailCarrierAppointmentReminderQueue,
				Status: "scheduled",
				SentAt: nil,
			})

			helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
				"task_id":  info.ID,
				"queue":    info.Queue,
				"task_type": task.Type(),
				"send_at":  sendAt,
				"type": "default",
				"notification_id": notificationID,
			})

			reminderSendAt = append(reminderSendAt, sendAt)
			
		}
	}

	// # Check for bulk reminders
	var bulkReminderSendAt []time.Time

	c.JSON(http.StatusOK, models.ServerResponse{
		Success: true,
		StatusCode: http.StatusOK,
		Message: "Appointment email scheduled successfully",
		Data: map[string]any{
			"order_id": request.OrderID,
			"user_id": request.UserID,
			"admin_id": request.AdminID,
			"carrier_appointment_email": map[string]any{
				"queue": models.EmailCarrierAppointmentQueue,
				"send_at": appointmentSendAt,
			},
			"carrier_appointment_reminder_email": map[string]any{
				"queue": models.EmailCarrierAppointmentReminderQueue,
				"send_at": reminderSendAt,
			},
			"carrier_appointment_bulk_reminder_email": map[string]any{
				"queue": models.EmailCarrierAppointmentBulkReminderQueue,
				"send_at": bulkReminderSendAt,
			},
		},
	})

}