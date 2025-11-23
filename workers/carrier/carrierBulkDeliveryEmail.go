package carrierWorker

import (
	"Notification-Server/db"
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/templates"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func SendCarrierBulkDeliverEmail(ctx context.Context, task *asynq.Task) error {
	defer func() {
		if r := recover(); r != nil {
			helpers.LogException("[worker] carrier bulk deliver email worker panic recovered", map[string]interface{}{
				"panic":     r,
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})
		}
	}()

	helpers.LogInfo("[worker] carrier bulk deliver email worker started", map[string]interface{}{
		"task_type":      task.Type(),
		"task_data":      string(task.Payload()),
		"payload_length": len(task.Payload()),
	})

	if task == nil {
		helpers.LogInfo("[worker] carrier bulk deliver email task is nil", map[string]interface{}{
			"task_data": string(task.Payload()),
			"task_type": task.Type(),
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier bulk deliver email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload":   string(payload),
	})

	var data models.CarrierBulkDeliverEmailWorkerData
	if err := json.Unmarshal(task.Payload(), &data); err != nil {
		helpers.LogException("[worker] carrier bulk deliver email failed to unmarshal payload", map[string]interface{}{
			"error":     err.Error(),
			"task_type": task.Type(),
			"payload":   string(task.Payload()),
		})
		return err
	}

	helpers.LogInfo("[worker] carrier bulk deliver email payload unmarshaled", map[string]interface{}{
		"task_type": task.Type(),
		"data":      data,
	})

	// --- Date Logic (matching pickup notification pattern) ---
	day := 0
	if data.Data.Day != nil {
		if parsedDay, err := strconv.Atoi(*data.Data.Day); err == nil {
			day = parsedDay
		} else {
			helpers.LogException("[worker] failed to parse day from payload", map[string]interface{}{
				"error":     err.Error(),
				"day_value": *data.Data.Day,
			})
			return err
		}
	}

	var startOfPeriod, endOfPeriod time.Time
	var targetDateStr string
	now := helpers.GetISTTime()

	if day < 0 {
		// Negative day means past days: -2 means 2 days ago
		targetDate := now.AddDate(0, 0, day)
		year, month, d := targetDate.Date()
		startOfPeriod = time.Date(year, month, d, 0, 0, 0, 0, now.Location())
		endOfPeriod = startOfPeriod.AddDate(0, 0, 1).Add(-time.Nanosecond)
		targetDateStr = targetDate.Format("02 Jan 2006")
	} else if day > 0 {
		// Positive day means future days
		targetDate := now.AddDate(0, 0, day)
		year, month, d := targetDate.Date()
		startOfPeriod = time.Date(year, month, d, 0, 0, 0, 0, now.Location())
		endOfPeriod = startOfPeriod.AddDate(0, 0, 1).Add(-time.Nanosecond)
		targetDateStr = targetDate.Format("02 Jan 2006")
	} else {
		// Day 0 means today
		year, month, d := now.Date()
		startOfDay := time.Date(year, month, d, 0, 0, 0, 0, now.Location())
		startOfPeriod = startOfDay
		endOfPeriod = now
		targetDateStr = now.Format("02 Jan 2006")
	}

	baseQuery := `
    SELECT
        o.order_id, o.carrier_name, o.channel, o.po_number, o.customer_city, o.customer_pincode,
        o.sku_details, o.lr_number, oa.appointment_scheduled_at, "to".expected_delivery_date,
        o.total_cartons, o.total_dead_weight, o.carton_details,
        o.invoice_number, o.total_invoice_value
    FROM
        orders o
    LEFT JOIN
        order_activity oa ON o.order_id = oa.order_id
    LEFT JOIN
        tracking_orders "to" ON o.order_id = "to".order_id
    LEFT JOIN
        notification_logs nl ON o.order_id = nl.order_id 
        AND nl.type = $5 
        AND nl.status = 'sent'
`
	var queryBuilder strings.Builder
	queryBuilder.WriteString(baseQuery)

	queryBuilder.WriteString(`
	WHERE o.carrier_id = $1 AND o.user_id = $2
	AND (
		(oa.appointment_scheduled_at IS NOT NULL AND oa.appointment_scheduled_at >= $3 AND oa.appointment_scheduled_at <= $4)
		OR
		(oa.appointment_scheduled_at IS NULL AND "to".expected_delivery_date >= $3 AND "to".expected_delivery_date <= $4)
	)
	AND nl.notification_id IS NULL`)

	args := []interface{}{
		data.Data.CarrierID,
		data.UserID,
		startOfPeriod,
		endOfPeriod,
		models.EmailCarrierAppointmentBulkReminderQueue,
	}

	finalQuery := queryBuilder.String()

	helpers.LogInfo("[worker] executing database query for bulk delivery", map[string]interface{}{
		"query": finalQuery,
		"args":  args,
	})

	deliveries := []models.CarrierBulkDeliverEmailData{}
	if err := db.GlobalDB.Select(&deliveries, finalQuery, args...); err != nil {
		helpers.LogException("[worker] failed to fetch and scan deliveries", map[string]interface{}{
			"error":      err.Error(),
			"carrier_id": data.Data.CarrierID,
			"query":      finalQuery,
			"args":       args,
		})
		return err
	}

	helpers.LogInfo("[worker] fetched deliveries for carrier bulk deliver email", map[string]interface{}{
		"carrier_id":       data.Data.CarrierID,
		"deliveries_count": len(deliveries),
	})

	if len(deliveries) > 0 {
		var totalCartons int
		var totalWeight float64

		lrSet := make(map[string]bool)
		for _, delivery := range deliveries {
			totalCartons += helpers.DerefIntPointer(delivery.TotalCartons)
			totalWeight += helpers.DerefFloatPointer(delivery.Weight)
			if delivery.LRNumber != nil {
				lrSet[*delivery.LRNumber] = true
			}
		}
		totalLRs := len(lrSet)

		var tableRows strings.Builder
		for i, delivery := range deliveries {
			var poNumberStr string
			var totalSkuQuantity float64

			if delivery.PONumber != nil && len(*delivery.PONumber) > 0 {
				poNumberStr = strings.Join(*delivery.PONumber, ", ")
			} else {
				poNumberStr = "N/A"
			}

			for _, skuItem := range delivery.SKUDetails {
				totalSkuQuantity += skuItem.Quantity
			}

			var dimensionsBuilder strings.Builder
			if delivery.Cartons != nil {
				for _, carton := range *delivery.Cartons {
					dimStr := fmt.Sprintf(
						"%.fx%.fx%.f Inch = %.f<br>",
						helpers.CmToInch(&carton.Length),
						helpers.CmToInch(&carton.Breadth),
						helpers.CmToInch(&carton.Height),
						helpers.DerefFloatPointer(&carton.Quantity),
					)
					dimensionsBuilder.WriteString(dimStr)
				}
			}
			cartonDimensions := dimensionsBuilder.String()
			if cartonDimensions == "" {
				cartonDimensions = "N/A"
			}

			date := ""
			if delivery.AppointmentScheduledAt != nil {
				date = helpers.FormatDateDDMMYYYYHHMM(delivery.AppointmentScheduledAt)
			} else if delivery.EDD != nil {
				date = helpers.FormatDateDDMMYYYYHHMM(delivery.EDD)
			} else {
				date = ""
			}

			rowHTML := fmt.Sprintf(`
            <tr>
                <td>%d</td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
                <td>%s</td>
                <td>%.2f</td>
                <td>%s</td>
                <td>%s</td>
                <td>%d</td>
                <td>%.2f KG</td>
                <td>%s</td>
                <td>%s</td>
                <td>%.f</td>
                <td>%.2f</td>
            </tr>`,
				i+1,
				strings.ToUpper(delivery.Channel),
				poNumberStr,
				helpers.DerefStringPointer(delivery.CustomerWarehouseCity),
				helpers.DerefStringPointer(delivery.CustomerWarehousePin),
				helpers.DerefFloatPointer(delivery.Amount),
				helpers.DerefStringPointer(delivery.LRNumber),
				date,
				helpers.DerefIntPointer(delivery.TotalCartons),
				helpers.DerefFloatPointer(delivery.Weight)/1000,
				cartonDimensions,
				helpers.DerefStringPointer(delivery.InvoiceNumber),
				helpers.DerefFloatPointer(&totalSkuQuantity),
				helpers.DerefFloatPointer(delivery.Amount),
			)
			tableRows.WriteString(rowHTML)
		}

		// var dateStrings []string

		// if day > 0 {
		// 	for i := 1; i <= day; i++ {
		// 		pastDate := now.AddDate(0, 0, i)
		// 		formattedDate := pastDate.Format("2 Jan 2006")
		// 		dateStrings = append(dateStrings, formattedDate)
		// 	}
		// } else if day < 0 {
		// 	for i := 1; i <= (day * -1); i++ {
		// 		pastDate := now.AddDate(0, 0, -i)
		// 		formattedDate := pastDate.Format("2 Jan 2006")
		// 		dateStrings = append(dateStrings, formattedDate)
		// 	}
		// } else {
		// 	dateStrings = append(dateStrings, now.Format("2 Jan 2006"))
		// }

		// finalString := strings.Join(dateStrings, ", ")

		//Final email body
		body := fmt.Sprintf(templates.SendCarrierBulkPickupEmailTemplate,
			deliveries[0].CarrierName,
			targetDateStr,
			totalCartons,
			totalWeight/1000,
			totalLRs,
			tableRows.String(),
		)

		receiverEmails := strings.Split(*data.Settings.ReceiverEmailsForCarrier, ",")
		receiverCC := []string{}
		if data.Settings.ReceiverCCEmailsForCarrier != nil {
			receiverCC = append(receiverCC, strings.Split(*data.Settings.ReceiverCCEmailsForCarrier, ",")...)
		}
		if data.Settings.SenderCCEmailsForCarrier != nil {
			receiverCC = append(receiverCC, strings.Split(*data.Settings.SenderCCEmailsForCarrier, ",")...)
		}

		helpers.LogInfo("[worker] preparing to send bulk pickup email", map[string]interface{}{
			"carrier_id":       data.Data.CarrierID,
			"total_deliveries": len(deliveries),
			"total_cartons":    totalCartons,
			"total_weight":     totalWeight,
			"total_lrs":        totalLRs,
			"date_range_str":   targetDateStr,
			"recipient_count":  len(receiverEmails),
			"cc_count":         len(receiverCC),
		})

		helpers.LogInfo("[worker] attempting to send bulk pickup email", map[string]interface{}{
			"from": helpers.B2B_EMAIL,
			"to":   receiverEmails,
			"cc":   receiverCC,
			"subject": fmt.Sprintf("Pickup Plan for %s",
				targetDateStr,
			),
			"body_length": len(body),
		})

		err := helpers.SendEmail(
			helpers.B2B_EMAIL,
			receiverEmails,
			receiverCC,
			fmt.Sprintf("Pickup Plan for %s",
				targetDateStr,
			),
			body,
			true,
			[]string{},
		)

		if err != nil {
			helpers.LogException("[worker] failed to send reminder email", map[string]interface{}{
				"error":     err.Error(),
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})

			notificationID, err := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: data.NotificationID,
				Sender:         *data.Settings.SenderEmailsForCarrier,
				Receiver:       *data.Settings.ReceiverEmailsForCarrier,
				SenderCC:       data.Settings.SenderCCEmailsForCarrier,
				ReceiverCC:     data.Settings.ReceiverCCEmailsForCarrier,
				Method:         "email",
				Type:           models.EmailCarrierAppointmentBulkReminderQueue,
				Status:         "worker_error",
				SentAt:         nil,
			})

			if err != nil {
				helpers.LogException("[worker] failed to update notification", map[string]interface{}{
					"error":           err.Error(),
					"notification_id": notificationID,
					"sender":          *data.Settings.SenderEmailsForCarrier,
					"receiver":        *data.Settings.ReceiverEmailsForCarrier,
					"sender_cc":       data.Settings.SenderCCEmailsForCarrier,
					"receiver_cc":     data.Settings.ReceiverCCEmailsForCarrier,
					"type":            models.EmailCarrierAppointmentBulkReminderQueue,
				})
			}

			return err
		}

		helpers.LogInfo("[worker] reminder email sent successfully", map[string]interface{}{
			"task_type": task.Type(),
			"task_data": string(task.Payload()),
			"data":      data,
		})

		// Log notification for each order to prevent duplicates
		sentAt := helpers.GetISTTime().Add(time.Second * 5)
		for _, delivery := range deliveries {
			notificationID := uuid.New()
			_, err := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: notificationID,
				OrderID:        delivery.OrderID,
				Sender:         *data.Settings.SenderEmailsForCarrier,
				Receiver:       *data.Settings.ReceiverEmailsForCarrier,
				SenderCC:       data.Settings.SenderCCEmailsForCarrier,
				ReceiverCC:     data.Settings.ReceiverCCEmailsForCarrier,
				Method:         "email",
				Type:           models.EmailCarrierAppointmentBulkReminderQueue,
				Status:         "sent",
				SentAt:         &sentAt,
			})
			if err != nil {
				helpers.LogException("[worker] failed to log notification for order", map[string]interface{}{
					"error":     err.Error(),
					"order_id":  delivery.OrderID,
					"task_type": task.Type(),
				})
			}
		}

		helpers.LogInfo("[worker] carrier bulk deliver email worker completed successfully", map[string]interface{}{
			"task_type":      task.Type(),
			"data":           data,
			"orders_count":   len(deliveries),
			"notifications_logged": len(deliveries),
		})

	}
	return nil
}
