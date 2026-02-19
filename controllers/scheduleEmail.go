package controllers

import (
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func getSkipSundayByUserID(userID uuid.UUID) bool {
	var skipSunday bool
	// table name to add
	err := db.GlobalDB.QueryRow(`
		SELECT skip_sunday
		FROM users  
		WHERE user_id = $1
	`, userID).Scan(&skipSunday)

	if err != nil {
		if err == sql.ErrNoRows {
			// User not found → default behaviour
			return false
		}

		// Log unexpected DB error
		helpers.LogException("failed to fetch skip_sunday by user_id", map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		})

		return false
	}

	return skipSunday
}

func ScheduleCarrierAppointmentEmail(c *gin.Context) {

	var request models.ScheduleAppointmentEmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		helpers.LogException("invalid request", map[string]interface{}{
			"request":         request,
			"request_headers": c.Request.Header,
			"request_url":     c.Request.URL,
			"request_method":  c.Request.Method,
			"error":           err.Error(),
		})
		c.JSON(http.StatusBadRequest, models.ServerResponse{
			Success:    false,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid request",
			Error:      err.Error(),
		})
		return
	}

	istLocation, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		helpers.LogException("failed to load IST timezone", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, models.ServerResponse{
			Success:    false,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to load timezone",
			Error:      err.Error(),
		})
		return
	}

	skipSunday := getSkipSundayByUserID(request.UserID)

	var data models.CarrierAppointmentEmailData

	// "to" is a reserved keyword in PostgreSQL, so it must be quoted.
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
			o.carrier_id,

			od.master_waybill,
			od.child_waybills,
			od.uploaded_documents,

			"to".expected_delivery_date,

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
			cl.email,

			oa.order_placed_at,
			oa.is_appointment_confirmed,
			oa.appointment_taken_at,
			oa.appointment_scheduled_at

		FROM orders o
		LEFT JOIN order_documents od ON o.order_id = od.order_id
		LEFT JOIN tracking_orders "to" ON o.order_id = "to".order_id
		LEFT JOIN order_activity oa ON o.order_id = oa.order_id
		LEFT JOIN pickup_locations pl ON o.warehouse_id = pl.warehouse_id
		LEFT JOIN customer_locations cl ON o.customer_warehouse_id = cl.warehouse_id
		WHERE o.order_id = $1
	`

	var (
		lrNumber              *string
		orderCreatedAt        *time.Time
		channel               string
		warehouseID           *string
		customerWarehouseID   *string
		cartonDetails         models.CartonDetailsList
		totalCartons          *int
		totalDeadWeight       *float64
		totalVolumetricWeight *float64
		carrierName           *string
		carrierID             *string

		masterWaybill     *string
		uploadedDocuments *[]byte

		expectedDeliveryDate *time.Time

		// Pickup Location
		warehouseName, warehouseAddress, warehouseCity, warehouseState, warehousePin, warehousePhone, warehouseEmail *string

		// Customer Location
		customerWarehouseName, customerWarehouseAddress, customerWarehouseCity, customerWarehouseState, customerWarehousePin, customerWarehousePhone, customerWarehouseEmail *string

		orderPlacedAt          *time.Time
		isAppointmentConfirmed *bool
		appointmentTakenAt     *time.Time
		appointmentScheduledAt *time.Time
	)

	err = db.GlobalDB.QueryRow(query, request.OrderID).Scan(
		&data.PONumber,         // o.po_number
		&lrNumber,              // o.lr_number
		&orderCreatedAt,        // o.order_created_at
		&channel,               // o.channel
		&warehouseID,           // o.warehouse_id
		&customerWarehouseID,   // o.customer_warehouse_id
		&cartonDetails,         // o.carton_details
		&totalCartons,          // o.total_cartons
		&totalDeadWeight,       // o.total_dead_weight
		&totalVolumetricWeight, // o.total_volumetric_weight
		&carrierName,           // o.carrier_name
		&carrierID,             // o.carrier_id

		&masterWaybill,     // od.master_waybill
		&data.ChildWaybill, // od.child_waybills
		&uploadedDocuments, // od.uploaded_documents

		&expectedDeliveryDate, // "to".expected_delivery_date

		// Pickup Location (Warehouse)
		&warehouseName,    // pl.warehouse_name
		&warehouseAddress, // pl.address_line_1
		&warehouseCity,    // pl.city
		&warehouseState,   // pl.state
		&warehousePin,     // pl.pincode
		&warehousePhone,   // pl.phone
		&warehouseEmail,   // pl.email

		// Customer Location (Customer Warehouse)
		&customerWarehouseName,    // cl.warehouse_name
		&customerWarehouseAddress, // cl.address_line_1
		&customerWarehouseCity,    // cl.city
		&customerWarehouseState,   // cl.state
		&customerWarehousePin,     // cl.pincode
		&customerWarehousePhone,   // cl.phone
		&customerWarehouseEmail,   // cl.email

		&orderPlacedAt,          // oa.order_placed_at
		&isAppointmentConfirmed, // oa.is_appointment_confirmed
		&appointmentTakenAt,     // oa.appointment_taken_at
		&appointmentScheduledAt, // oa.appointment_scheduled_at
	)

	if err != nil {
		helpers.LogException("failed to fetch order data", map[string]interface{}{
			"order_id": request.OrderID,
			"error":    err.Error(),
		})
		c.JSON(http.StatusInternalServerError, models.ServerResponse{
			Success:    false,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch order data",
			Error:      err.Error(),
		})
		return
	}

	data.LRNumber = lrNumber
	data.AppointmentTakenAt = appointmentTakenAt
	data.AppointmentScheduledAt = appointmentScheduledAt
	data.Channel = channel
	data.TotalCartons = totalCartons
	data.TotalDeadWeight = totalDeadWeight
	data.TotalVolumetricWeight = totalVolumetricWeight
	data.CarrierName = *carrierName
	data.CarrierID = *carrierID
	data.Cartons = &cartonDetails
	// Unmarshal carton_details

	// MasterWaybill
	if masterWaybill != nil {
		mw := models.JSONBArrayString{*masterWaybill}
		data.MasterWaybill = &mw
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
	data.AppointmentTakenAt = appointmentTakenAt

	if uploadedDocuments != nil {
		var docsMap map[string][]string
		if err := json.Unmarshal(*uploadedDocuments, &docsMap); err == nil {
			if invoiceFiles, ok := docsMap["invoice"]; ok {
				data.Files = &invoiceFiles
			}
		}
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
		bulk_reminder_time,
		bulk_reminder_days_range,
		bulk_reminder_type,
		created_at
	FROM appointment_notification_settings
	WHERE user_id = $1 AND admin_id = $2 AND carrier_id = $3
`, request.UserID, request.AdminID, carrierID).Scan(
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
		&notificationSettings.BulkReminderTime,
		&notificationSettings.BulkReminderDaysRange,
		&notificationSettings.BulkReminderType,
		&notificationSettings.CreatedAt,
	)

	if err != nil && err != sql.ErrNoRows {
		helpers.LogException("failed to fetch notification settings", map[string]interface{}{
			"order_id": request.OrderID,
			"user_id":  request.UserID,
			"admin_id": request.AdminID,
			"error":    err.Error(),
		})
		c.JSON(http.StatusInternalServerError, models.ServerResponse{
			Success:    false,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch notification settings",
			Error:      err.Error(),
		})
		return
	} else if err == sql.ErrNoRows {
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

				skip_sunday,
				
				send_reminder,
				reminder_days,
				reminder_type,
				
			send_bulk_reminder,
			bulk_reminder_time,
			bulk_reminder_days_range,
			bulk_reminder_type,
			created_at
		FROM appointment_notification_settings
		WHERE user_id = $1 AND admin_id = $2 AND carrier_id = $3
	`, request.UserID, request.AdminID, "all").Scan(
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
			&notificationSettings.BulkReminderTime,
			&notificationSettings.BulkReminderDaysRange,
			&notificationSettings.BulkReminderType,
			&notificationSettings.CreatedAt,
		)

		if err != nil && err != sql.ErrNoRows {
			helpers.LogException("failed to fetch notification settings", map[string]interface{}{
				"order_id": request.OrderID,
				"user_id":  request.UserID,
				"admin_id": request.AdminID,
				"error":    err.Error(),
			})
			c.JSON(http.StatusInternalServerError, models.ServerResponse{
				Success:    false,
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to fetch notification settings",
				Error:      err.Error(),
			})
			return
		}

		if err == sql.ErrNoRows {
			helpers.LogInfo("no notification settings found", map[string]interface{}{
				"order_id":   request.OrderID,
				"user_id":    request.UserID,
				"admin_id":   request.AdminID,
				"carrier_id": carrierID,
			})
			c.JSON(http.StatusBadRequest, models.ServerResponse{
				Success:    false,
				StatusCode: http.StatusBadRequest,
				Message:    "Notification settings not found",
				Data: map[string]any{
					"order_id":   request.OrderID,
					"user_id":    request.UserID,
					"admin_id":   request.AdminID,
					"carrier_id": carrierID,
				},
				Error: "Notification settings not found",
			})
			return
		}

	} else {
		helpers.LogInfo("notification settings found", map[string]interface{}{
			"order_id":   request.OrderID,
			"user_id":    request.UserID,
			"admin_id":   request.AdminID,
			"carrier_id": carrierID,
		})
	}

	queueData := models.CarrierAppointmentEmailWorkerData{
		OrderID:  request.OrderID,
		AdminID:  request.AdminID,
		UserID:   request.UserID,
		Data:     data,
		Settings: notificationSettings,
	}

	// # Schedule appointment email
	appointmentSendAt := make(map[uuid.UUID]time.Time)

	if notificationSettings.SendNotification {

		sendAt := helpers.GetISTTime().Add(time.Second * 5)

		if notificationSettings.NotificationType == nil {
			helpers.LogInfo("notification type is nil", map[string]interface{}{
				"order_id":          request.OrderID,
				"notification_type": notificationSettings.NotificationType,
			})
			c.JSON(http.StatusBadRequest, models.ServerResponse{
				Success:    false,
				StatusCode: http.StatusBadRequest,
				Message:    "Notification type is nil",
				Data: map[string]any{
					"order_id":   request.OrderID,
					"user_id":    request.UserID,
					"admin_id":   request.AdminID,
					"carrier_id": carrierID,
				},
				Error: "Notification type is nil",
			})
			return
		}

		// Check if notification already sent for this order and type
		var existingNotificationCount int
		checkQuery := `
			SELECT COUNT(*) 
			FROM notification_logs 
			WHERE order_id = $1 
			AND type = $2 
			AND status = 'sent'
		`
		err = db.GlobalDB.QueryRow(checkQuery, request.OrderID, models.EmailCarrierAppointmentQueue).Scan(&existingNotificationCount)
		if err != nil {
			helpers.LogException("failed to check existing notifications", map[string]interface{}{
				"order_id": request.OrderID,
				"type":     models.EmailCarrierAppointmentQueue,
				"error":    err.Error(),
			})
		}

		if existingNotificationCount > 0 {
			helpers.LogInfo("notification already sent for this order", map[string]interface{}{
				"order_id":       request.OrderID,
				"type":           models.EmailCarrierAppointmentQueue,
				"existing_count": existingNotificationCount,
			})
			c.JSON(http.StatusOK, models.ServerResponse{
				Success:    true,
				StatusCode: http.StatusOK,
				Message:    "Notification already sent for this order",
				Data: map[string]any{
					"order_id": request.OrderID,
					"type":     models.EmailCarrierAppointmentQueue,
					"status":   "skipped",
				},
			})
			return
		}

		switch *notificationSettings.NotificationType {

		case "after_appointment_taken":

			// # Check if order placed or not
			if orderPlacedAt == nil || orderPlacedAt.IsZero() {

				helpers.LogInfo("order placed at is nil or zero", map[string]interface{}{
					"order_id":        request.OrderID,
					"order_placed_at": orderPlacedAt,
				})

				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Order placed at is nil or zero",
					Data: map[string]any{
						"order_id":   request.OrderID,
						"user_id":    request.UserID,
						"admin_id":   request.AdminID,
						"carrier_id": carrierID,
					},
					Error: "Order placed at is nil or zero",
				})

				return
			}

			if appointmentTakenAt == nil || appointmentTakenAt.IsZero() {
				helpers.LogInfo("appointment taken at is nil or zero", map[string]interface{}{
					"order_id":             request.OrderID,
					"appointment_taken_at": appointmentTakenAt,
				})
				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Appointment taken at is nil or zero",
					Data: map[string]any{
						"order_id":   request.OrderID,
						"user_id":    request.UserID,
						"admin_id":   request.AdminID,
						"carrier_id": carrierID,
					},
					Error: "Appointment taken at is nil or zero",
				})
				return
			}

			// # Schedule email
			notificationDays := "1"
			if notificationSettings.NotificationDays != nil {
				notificationDays = *notificationSettings.NotificationDays
			}

			days := strings.Split(strings.TrimSpace(notificationDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				// Convert UTC time to IST timezone before adding days
				istAppointmentTakenAt := appointmentTakenAt.In(istLocation)
				sendAt = istAppointmentTakenAt.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				// fmt.Println("sendAt", sendAt)
				// fmt.Println("queueData", queueData.NotificationID)

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to marshal request payload",
						Error:      err.Error(),
						Data: map[string]any{
							"order_id":   request.OrderID,
							"user_id":    request.UserID,
							"admin_id":   request.AdminID,
							"carrier_id": carrierID,
						},
					})
					return
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"payload":  payload,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentQueue,
						Method:         "email",
						Status:         "error",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "after_appointment_date",
					"notification_id": notificationID,
				})

				appointmentSendAt[queueData.NotificationID] = sendAt
			}

		case "after_appointment_date":

			notificationDays := "1"
			if notificationSettings.NotificationDays != nil {
				notificationDays = *notificationSettings.NotificationDays
			}

			days := strings.Split(strings.TrimSpace(notificationDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				// Convert UTC time to IST timezone before adding days
				istAppointmentScheduledAt := appointmentScheduledAt.In(istLocation)
				sendAt = istAppointmentScheduledAt.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to marshal request payload",
						Error:      err.Error(),
					})
					return
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"payload":  payload,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentQueue,
						Method:         "email",
						Status:         "error",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "after_appointment_date",
					"notification_id": notificationID,
				})

				appointmentSendAt[queueData.NotificationID] = sendAt
			}

		case "before_delivery":

			if expectedDeliveryDate == nil || expectedDeliveryDate.IsZero() {
				if appointmentScheduledAt != nil && !appointmentScheduledAt.IsZero() {
					expectedDeliveryDate = appointmentScheduledAt
				} else {
					helpers.LogInfo("expected delivery date is nil or zero and appointment scheduled at is nil or zero", map[string]interface{}{
						"order_id":                 request.OrderID,
						"expected_delivery_date":   expectedDeliveryDate,
						"appointment_scheduled_at": appointmentScheduledAt,
					})
				}
				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Expected delivery date is nil or zero and appointment scheduled at is nil or zero",
				})
				return
			} else if expectedDeliveryDate != nil && !expectedDeliveryDate.IsZero() && appointmentScheduledAt != nil && !appointmentScheduledAt.IsZero() {
				if appointmentScheduledAt.After(*expectedDeliveryDate) || appointmentScheduledAt.Equal(*expectedDeliveryDate) {
					expectedDeliveryDate = appointmentScheduledAt
				}
			}

			notificationDays := "-1"
			if notificationSettings.NotificationDays != nil {
				notificationDays = *notificationSettings.NotificationDays
			}

			days := strings.Split(strings.TrimSpace(notificationDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				// Convert UTC time to IST timezone before adding days
				istExpectedDeliveryDate := expectedDeliveryDate.In(istLocation)
				sendAt = istExpectedDeliveryDate.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to marshal request payload",
						Error:      err.Error(),
					})
					return
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentQueue,
						Method:         "email",
						Status:         "error",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "before_delivery",
					"notification_id": notificationID,
				})

				appointmentSendAt[queueData.NotificationID] = sendAt

			}

		case "before_appointment_date":

			if appointmentScheduledAt == nil || appointmentScheduledAt.IsZero() {
				helpers.LogInfo("appointment scheduled at is nil or zero", map[string]interface{}{
					"order_id":                 request.OrderID,
					"appointment_scheduled_at": appointmentScheduledAt,
				})
				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Appointment scheduled at is nil or zero",
				})
				return
			}

			notificationDays := "-1"
			if notificationSettings.NotificationDays != nil {
				notificationDays = *notificationSettings.NotificationDays
			}

			days := strings.Split(strings.TrimSpace(notificationDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				// Convert UTC time to IST timezone before adding days
				istAppointmentScheduledAt := appointmentScheduledAt.In(istLocation)
				sendAt = istAppointmentScheduledAt.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to marshal request payload",
						Error:      err.Error(),
					})
					return
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentQueue,
						Method:         "email",
						Status:         "error",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "before_delivery",
					"notification_id": notificationID,
				})

				appointmentSendAt[queueData.NotificationID] = sendAt

			}

		default:
			helpers.LogInfo("invalid notification type", map[string]interface{}{
				"order_id":          request.OrderID,
				"notification_type": notificationSettings.NotificationType,
			})

			queueData.NotificationID = uuid.New()

			payload, err := json.Marshal(queueData)
			if err != nil {
				helpers.LogException("failed to marshal request payload", map[string]interface{}{
					"request":         request,
					"request_headers": c.Request.Header,
					"request_url":     c.Request.URL,
					"request_method":  c.Request.Method,
					"error":           err.Error(),
				})
				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusInternalServerError,
					Message:    "Failed to marshal request payload",
					Error:      err.Error(),
				})
				return
			}

			options := []asynq.Option{
				asynq.MaxRetry(3),
				asynq.ProcessAt(sendAt),
			}

			task := asynq.NewTask(models.EmailCarrierAppointmentQueue, payload)

			info, err := queues.EmailQueueClient.Enqueue(task, options...)
			if err != nil {
				helpers.LogException("failed to enqueue email task", map[string]interface{}{
					"request":  request,
					"settings": notificationSettings,
					"payload":  payload,
					"error":    err.Error(),
				})

				helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentQueue,
					Method:         "email",
					Status:         "error",
					SentAt:         nil,
				})

				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusInternalServerError,
					Message:    "Failed to enqueue email task",
					Error:      err.Error(),
				})

				return
			}

			notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: queueData.NotificationID,
				OrderID:        request.OrderID,
				Sender:         *notificationSettings.SenderEmailsForCarrier,
				SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
				ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
				Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
				Type:           models.EmailCarrierAppointmentQueue,
				Method:         "email",
				Status:         "scheduled",
				SentAt:         nil,
			})

			helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
				"task_id":         info.ID,
				"queue":           info.Queue,
				"task_type":       task.Type(),
				"send_at":         sendAt,
				"type":            "after_appointment_date",
				"notification_id": notificationID,
			})

			appointmentSendAt[queueData.NotificationID] = sendAt

		}
	}

	// # Check for reminders
	reminderSendAt := make(map[uuid.UUID]time.Time)

	if notificationSettings.SendReminder {

		if notificationSettings.ReminderType == nil {
			helpers.LogInfo("reminder type is nil", map[string]interface{}{
				"order_id":      request.OrderID,
				"reminder_type": notificationSettings.ReminderType,
			})
			c.JSON(http.StatusBadRequest, models.ServerResponse{
				Success:    false,
				StatusCode: http.StatusBadRequest,
				Message:    "Reminder type is nil",
			})
			return
		}

		// Check if reminder notification already sent for this order and type
		var existingReminderCount int
		checkReminderQuery := `
		SELECT COUNT(*) 
		FROM notification_logs 
		WHERE order_id = $1 
		AND type = $2 
		AND status = 'sent'
	`
		err = db.GlobalDB.QueryRow(checkReminderQuery, request.OrderID, models.EmailCarrierAppointmentReminderQueue).Scan(&existingReminderCount)
		if err != nil {
			helpers.LogException("failed to check existing reminder notifications", map[string]interface{}{
				"order_id": request.OrderID,
				"type":     models.EmailCarrierAppointmentReminderQueue,
				"error":    err.Error(),
			})
		}

		if existingReminderCount > 0 {
			helpers.LogInfo("reminder notification already sent for this order", map[string]interface{}{
				"order_id":       request.OrderID,
				"type":           models.EmailCarrierAppointmentReminderQueue,
				"existing_count": existingReminderCount,
			})
			c.JSON(http.StatusOK, models.ServerResponse{
				Success:    true,
				StatusCode: http.StatusOK,
				Message:    "Reminder notification already sent for this order",
				Data: map[string]any{
					"order_id": request.OrderID,
					"type":     models.EmailCarrierAppointmentReminderQueue,
					"status":   "skipped",
				},
			})
			return
		}

		sendAt := helpers.GetISTTime().Add(time.Second * 5)

		switch *notificationSettings.ReminderType {
		case "after_appointment_taken":

			// # Check if order placed or not
			if orderPlacedAt == nil || orderPlacedAt.IsZero() {

				helpers.LogInfo("order placed at is nil or zero", map[string]interface{}{
					"order_id":        request.OrderID,
					"order_placed_at": orderPlacedAt,
				})

				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Order placed at is nil or zero",
				})

				return
			}

			if appointmentTakenAt == nil || appointmentTakenAt.IsZero() {
				helpers.LogInfo("appointment taken at is nil or zero", map[string]interface{}{
					"order_id":             request.OrderID,
					"appointment_taken_at": appointmentTakenAt,
				})
				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Appointment taken at is nil or zero",
				})
				return
			}

			// # Schedule reminder email
			days := strings.Split(strings.TrimSpace(*notificationSettings.ReminderDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				// Convert UTC time to IST timezone before adding days
				istAppointmentTakenAt := appointmentTakenAt.In(istLocation)
				sendAt = istAppointmentTakenAt.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to marshal request payload",
						Error:      err.Error(),
					})
					return
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"payload":  payload,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentReminderQueue,
						Method:         "email",
						Status:         "error",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentReminderQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "after_appointment_taken",
					"notification_id": notificationID,
				})

				appointmentSendAt[queueData.NotificationID] = sendAt
			}

		case "after_appointment_date":

			days := strings.Split(strings.TrimSpace(*notificationSettings.ReminderDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				sendAt = appointmentScheduledAt.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"payload":  payload,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentReminderQueue,
						Status:         "error",
						Method:         "email",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue reminder email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentReminderQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "after_appointment_date",
					"notification_id": notificationID,
				})

				reminderSendAt[queueData.NotificationID] = sendAt
			}

		case "before_delivery":

			if expectedDeliveryDate == nil || expectedDeliveryDate.IsZero() {
				if appointmentScheduledAt != nil && !appointmentScheduledAt.IsZero() {
					expectedDeliveryDate = appointmentScheduledAt
				} else {
					helpers.LogInfo("expected delivery date is nil or zero and appointment scheduled at is nil or zero", map[string]interface{}{
						"order_id":                 request.OrderID,
						"expected_delivery_date":   expectedDeliveryDate,
						"appointment_scheduled_at": appointmentScheduledAt,
					})
					c.JSON(http.StatusBadRequest, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusBadRequest,
						Message:    "Expected delivery date is nil or zero and appointment scheduled at is nil or zero",
					})
					return
				}
			} else if expectedDeliveryDate != nil && !expectedDeliveryDate.IsZero() && appointmentScheduledAt != nil && !appointmentScheduledAt.IsZero() {
				if appointmentScheduledAt.After(*expectedDeliveryDate) || appointmentScheduledAt.Equal(*expectedDeliveryDate) {
					expectedDeliveryDate = appointmentScheduledAt
				}
			}

			days := strings.Split(strings.TrimSpace(*notificationSettings.ReminderDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				sendAt = expectedDeliveryDate.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"payload":  payload,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentReminderQueue,
						Status:         "error",
						Method:         "email",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue reminder email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentReminderQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "before_delivery",
					"notification_id": notificationID,
				})

				reminderSendAt[queueData.NotificationID] = sendAt

			}

		case "before_appointment_date":

			if appointmentScheduledAt == nil || appointmentScheduledAt.IsZero() {
				helpers.LogInfo("expected delivery date is nil or zero and appointment scheduled at is nil or zero", map[string]interface{}{
					"order_id":                 request.OrderID,
					"expected_delivery_date":   expectedDeliveryDate,
					"appointment_scheduled_at": appointmentScheduledAt,
				})
				c.JSON(http.StatusBadRequest, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusBadRequest,
					Message:    "Appointment scheduled at is nil or zero",
				})
				return
			}

			days := strings.Split(strings.TrimSpace(*notificationSettings.ReminderDays), ",")
			for _, day := range days {
				daysFloat, _ := strconv.ParseFloat(strings.TrimSpace(day), 64)
				sendAt = appointmentScheduledAt.Add(time.Duration(daysFloat) * time.Hour * 24)

				// Adjust to skip Sunday - move to Saturday if notification falls on Sunday
				sendAt = helpers.AdjustNotificationTimeToSkipSunday(sendAt, skipSunday)

				if sendAt.Before(helpers.GetISTTime()) {
					sendAt = helpers.GetISTTime().Add(time.Second * 5)
				}

				queueData.NotificationID = uuid.New()

				payload, err := json.Marshal(queueData)
				if err != nil {
					helpers.LogException("failed to marshal request payload", map[string]interface{}{
						"request":         request,
						"request_headers": c.Request.Header,
						"request_url":     c.Request.URL,
						"request_method":  c.Request.Method,
						"error":           err.Error(),
					})
				}

				options := []asynq.Option{
					asynq.MaxRetry(3),
					asynq.ProcessAt(sendAt),
				}

				task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

				info, err := queues.EmailQueueClient.Enqueue(task, options...)
				if err != nil {
					helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
						"request":  request,
						"settings": notificationSettings,
						"payload":  payload,
						"error":    err.Error(),
					})

					helpers.InsertNotificationLog(&models.Notification{
						NotificationID: queueData.NotificationID,
						OrderID:        request.OrderID,
						Sender:         *notificationSettings.SenderEmailsForCarrier,
						SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
						ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
						Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
						Type:           models.EmailCarrierAppointmentReminderQueue,
						Status:         "error",
						Method:         "email",
						SentAt:         nil,
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success:    false,
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to enqueue reminder email task",
						Error:      err.Error(),
					})

					return
				}

				notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentReminderQueue,
					Method:         "email",
					Status:         "scheduled",
					SentAt:         nil,
				})

				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id":         info.ID,
					"queue":           info.Queue,
					"task_type":       task.Type(),
					"send_at":         sendAt,
					"type":            "before_delivery",
					"notification_id": notificationID,
				})

				reminderSendAt[queueData.NotificationID] = sendAt

			}

		default:
			helpers.LogInfo("invalid reminder notification type", map[string]interface{}{
				"order_id":          request.OrderID,
				"notification_type": notificationSettings.ReminderType,
			})

			queueData.NotificationID = uuid.New()

			payload, err := json.Marshal(queueData)
			if err != nil {
				helpers.LogException("failed to marshal request payload", map[string]interface{}{
					"request":         request,
					"request_headers": c.Request.Header,
					"request_url":     c.Request.URL,
					"request_method":  c.Request.Method,
					"error":           err.Error(),
				})
			}

			options := []asynq.Option{
				asynq.MaxRetry(3),
				asynq.ProcessAt(sendAt),
			}

			task := asynq.NewTask(models.EmailCarrierAppointmentReminderQueue, payload)

			info, err := queues.EmailQueueClient.Enqueue(task, options...)
			if err != nil {
				helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
					"request":  request,
					"settings": notificationSettings,
					"payload":  payload,
					"error":    err.Error(),
				})

				helpers.InsertNotificationLog(&models.Notification{
					NotificationID: queueData.NotificationID,
					OrderID:        request.OrderID,
					Sender:         *notificationSettings.SenderEmailsForCarrier,
					SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
					ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
					Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
					Type:           models.EmailCarrierAppointmentReminderQueue,
					Status:         "error",
					Method:         "email",
					SentAt:         nil,
				})

				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success:    false,
					StatusCode: http.StatusInternalServerError,
					Message:    "Failed to enqueue reminder email task",
					Error:      err.Error(),
				})

				return
			}

			notificationID, _ := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: queueData.NotificationID,
				OrderID:        request.OrderID,
				Sender:         *notificationSettings.SenderEmailsForCarrier,
				Receiver:       *notificationSettings.ReceiverEmailsForCarrier,
				SenderCC:       notificationSettings.SenderCCEmailsForCarrier,
				ReceiverCC:     notificationSettings.ReceiverCCEmailsForCarrier,
				Type:           models.EmailCarrierAppointmentReminderQueue,
				Method:         "email",
				Status:         "scheduled",
				SentAt:         nil,
			})

			helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
				"task_id":         info.ID,
				"queue":           info.Queue,
				"task_type":       task.Type(),
				"send_at":         sendAt,
				"type":            "default",
				"notification_id": notificationID,
			})

			reminderSendAt[queueData.NotificationID] = sendAt

		}
	}

	// # Check for bulk reminders
	// var bulkReminderSendAt []time.Time

	c.JSON(http.StatusOK, models.ServerResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Appointment email scheduled successfully",
		Data: map[string]any{
			"order_id":   request.OrderID,
			"user_id":    request.UserID,
			"admin_id":   request.AdminID,
			"carrier_id": carrierID,
			"carrier_appointment_email": map[string]any{
				"queue":   models.EmailCarrierAppointmentQueue,
				"send_at": appointmentSendAt,
			},
			"carrier_appointment_reminder_email": map[string]any{
				"queue":   models.EmailCarrierAppointmentReminderQueue,
				"send_at": reminderSendAt,
			},
			"carrier_appointment_bulk_reminder_email": map[string]any{
				"queue":   models.EmailCarrierAppointmentBulkReminderQueue,
				"send_at": nil,
			},
		},
	})

}
