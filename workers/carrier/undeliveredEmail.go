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

func SendCarrierUndeliveredEmail(ctx context.Context, task *asynq.Task) error {
	defer func() {
		if r := recover(); r != nil {
			helpers.LogException("[worker] carrier undelivered email worker panic recovered", map[string]interface{}{
				"panic":     r,
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})
		}
	}()

	helpers.LogInfo("[worker] carrier undelivered email worker started", map[string]interface{}{
		"task_type":      task.Type(),
		"task_data":      string(task.Payload()),
		"payload_length": len(task.Payload()),
	})

	if task == nil {
		helpers.LogInfo("[worker] carrier undelivered email task is nil", map[string]interface{}{
			"task_data": string(task.Payload()),
			"task_type": task.Type(),
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier undelivered email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload":   string(payload),
	})

	var data models.CarrierBulkDeliverEmailWorkerData
	if err := json.Unmarshal(task.Payload(), &data); err != nil {
		helpers.LogException("[worker] carrier undelivered email failed to unmarshal payload", map[string]interface{}{
			"error":     err.Error(),
			"task_type": task.Type(),
			"payload":   string(task.Payload()),
		})
		return err
	}

	helpers.LogInfo("[worker] carrier undelivered email payload unmarshaled", map[string]interface{}{
		"task_type": task.Type(),
		"data":      data,
	})

	// --- Date Logic: Find orders marked as UNDELIVERED ---
	days := 1
	if data.Data.Day != nil {
		if parsedDay, err := strconv.Atoi(*data.Data.Day); err == nil {
			days = parsedDay
		} else {
			helpers.LogException("[worker] failed to parse day from payload", map[string]interface{}{
				"error":     err.Error(),
				"day_value": *data.Data.Day,
			})
			return err
		}
	}

	now := helpers.GetISTTime()
	// Calculate the date range: orders that were undelivered X days ago
	targetDate := now.AddDate(0, 0, -days)
	year, month, d := targetDate.Date()
	startOfPeriod := time.Date(year, month, d, 0, 0, 0, 0, now.Location())
	endOfPeriod := startOfPeriod.AddDate(0, 0, 1).Add(-time.Nanosecond)

	helpers.LogInfo("[worker] searching for undelivered orders", map[string]interface{}{
		"days_ago":        days,
		"start_of_period": startOfPeriod,
		"end_of_period":   endOfPeriod,
	})

	// Query to find undelivered orders
	query := `
    SELECT
        o.order_id, o.carrier_name, o.channel, o.po_number, o.customer_city, o.customer_pincode,
        o.sku_details, o.lr_number, oa.appointment_scheduled_at, "to".expected_delivery_date,
        o.total_cartons, o.total_dead_weight, o.carton_details,
        o.invoice_number, o.total_invoice_value, od.asn_number,
		"to".last_updated_at
    FROM
        orders o
    LEFT JOIN
        order_activity oa ON o.order_id = oa.order_id
    LEFT JOIN
        tracking_orders "to" ON o.order_id = "to".order_id
    LEFT JOIN
        order_documents od ON o.order_id = od.order_id
    LEFT JOIN
        notification_logs nl ON o.order_id = nl.order_id AND nl.type = $5 AND nl.status = 'sent'
	WHERE 
		o.carrier_id = $1 
		AND o.user_id = $2 
		AND "to".current_status = 'UNDELIVERED'
		AND "to".last_updated_at >= $3 
		AND "to".last_updated_at <= $4
		AND nl.notification_id IS NULL
	`

	args := []interface{}{
		data.Data.CarrierID,
		data.AdminID,
		startOfPeriod,
		endOfPeriod,
		models.EmailCarrierUndeliveredNotificationQueue,
	}

	helpers.LogInfo("[worker] executing database query for undelivered orders", map[string]interface{}{
		"query": query,
		"args":  args,
	})

	undeliveredOrders := []models.CarrierBulkDeliverEmailData{}
	if err := db.GlobalDB.Select(&undeliveredOrders, query, args...); err != nil {
		helpers.LogException("[worker] failed to fetch undelivered orders", map[string]interface{}{
			"error":      err.Error(),
			"carrier_id": data.Data.CarrierID,
			"query":      query,
			"args":       args,
		})
		return err
	}

	helpers.LogInfo("[worker] fetched undelivered orders for carrier", map[string]interface{}{
		"carrier_id":     data.Data.CarrierID,
		"orders_count":   len(undeliveredOrders),
		"start_of_period": startOfPeriod,
		"end_of_period":   endOfPeriod,
	})

	if len(undeliveredOrders) > 0 {
		var totalCartons int
		var totalWeight float64

		lrSet := make(map[string]bool)
		for _, order := range undeliveredOrders {
			totalCartons += helpers.DerefIntPointer(order.TotalCartons)
			totalWeight += helpers.DerefFloatPointer(order.Weight)
			if order.LRNumber != nil {
				lrSet[*order.LRNumber] = true
			}
		}
		totalLRs := len(lrSet)

		var tableRows strings.Builder
		for i, order := range undeliveredOrders {
			var poNumberStr string
			var totalSkuQuantity float64

			if order.PONumber != nil && len(*order.PONumber) > 0 {
				poNumberStr = strings.Join(*order.PONumber, ", ")
			} else {
				poNumberStr = "N/A"
			}

			for _, skuItem := range order.SKUDetails {
				totalSkuQuantity += skuItem.Quantity
			}

			var dimensionsBuilder strings.Builder
			if order.Cartons != nil {
				for _, carton := range *order.Cartons {
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

			scheduledDate := ""
			if order.AppointmentScheduledAt != nil {
				scheduledDate = helpers.FormatDateDDMMYYYYHHMM(order.AppointmentScheduledAt)
			} else if order.EDD != nil {
				scheduledDate = helpers.FormatDateDDMMYYYYHHMM(order.EDD)
			} else {
				scheduledDate = "N/A"
			}

			asnNumber := "N/A"
			if order.ASNNumber != nil && *order.ASNNumber != "" {
				asnNumber = *order.ASNNumber
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
                <td>%s</td>
                <td>%d</td>
                <td>%.2f KG</td>
                <td>%s</td>
                <td>%s</td>
                <td>%.f</td>
                <td>%.2f</td>
            </tr>`,
				i+1,
				strings.ToUpper(order.Channel),
				poNumberStr,
				helpers.DerefStringPointer(order.CustomerWarehouseCity),
				helpers.DerefStringPointer(order.CustomerWarehousePin),
				helpers.DerefFloatPointer(order.Amount),
				helpers.DerefStringPointer(order.LRNumber),
				scheduledDate,
				asnNumber,
				helpers.DerefIntPointer(order.TotalCartons),
				helpers.DerefFloatPointer(order.Weight)/1000,
				cartonDimensions,
				helpers.DerefStringPointer(order.InvoiceNumber),
				helpers.DerefFloatPointer(&totalSkuQuantity),
				helpers.DerefFloatPointer(order.Amount),
			)
			tableRows.WriteString(rowHTML)
		}

		dateStr := targetDate.Format("02 Jan 2006")

		// Final email body
		body := fmt.Sprintf(templates.SendCarrierUndeliveredEmailTemplate,
			undeliveredOrders[0].CarrierName,
			dateStr,
			totalCartons,
			helpers.RoundFloat(totalWeight/1000, 2),
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

		helpers.LogInfo("[worker] preparing to send undelivered email", map[string]interface{}{
			"carrier_id":      data.Data.CarrierID,
			"total_orders":    len(undeliveredOrders),
			"total_cartons":   totalCartons,
			"total_weight":    totalWeight,
			"total_lrs":       totalLRs,
			"date_str":        dateStr,
			"recipient_count": len(receiverEmails),
			"cc_count":        len(receiverCC),
		})

		subject := fmt.Sprintf("UNDELIVERED Orders - Action Required for %s", dateStr)

		// Append display_company_name to subject if it exists
		displayCompanyName := helpers.GetDisplayCompanyName(data.UserID)
		if displayCompanyName != nil && *displayCompanyName != "" {
			subject = fmt.Sprintf("%s <> %s", subject, *displayCompanyName)
		}

		helpers.LogInfo("[worker] attempting to send undelivered email", map[string]interface{}{
			"from":        helpers.B2B_EMAIL,
			"to":          receiverEmails,
			"cc":          receiverCC,
			"subject":     subject,
			"body_length": len(body),
		})

		err := helpers.SendEmail(
			helpers.B2B_EMAIL,
			receiverEmails,
			receiverCC,
			subject,
			body,
			true,
			[]string{},
		)

		if err != nil {
			helpers.LogException("[worker] failed to send undelivered email", map[string]interface{}{
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
				Type:           models.EmailCarrierUndeliveredNotificationQueue,
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
					"type":            models.EmailCarrierUndeliveredNotificationQueue,
				})
			}

			return err
		}

		helpers.LogInfo("[worker] undelivered email sent successfully", map[string]interface{}{
			"task_type": task.Type(),
			"task_data": string(task.Payload()),
			"data":      data,
		})

		// Log notification for each order to prevent duplicates
		sentAt := helpers.GetISTTime()
		for _, order := range undeliveredOrders {
			notificationID := uuid.New()
			_, err := helpers.InsertNotificationLog(&models.Notification{
				NotificationID: notificationID,
				OrderID:        order.OrderID,
				Sender:         *data.Settings.SenderEmailsForCarrier,
				Receiver:       *data.Settings.ReceiverEmailsForCarrier,
				SenderCC:       data.Settings.SenderCCEmailsForCarrier,
				ReceiverCC:     data.Settings.ReceiverCCEmailsForCarrier,
				Method:         "email",
				Type:           models.EmailCarrierUndeliveredNotificationQueue,
				Status:         "sent",
				SentAt:         &sentAt,
			})
			if err != nil {
				helpers.LogException("[worker] failed to log notification for order", map[string]interface{}{
					"error":     err.Error(),
					"order_id":  order.OrderID,
					"task_type": task.Type(),
				})
			}
		}

		helpers.LogInfo("[worker] carrier undelivered email worker completed successfully", map[string]interface{}{
			"task_type":            task.Type(),
			"data":                 data,
			"orders_count":         len(undeliveredOrders),
			"notifications_logged": len(undeliveredOrders),
		})

	} else {
		helpers.LogInfo("[worker] no undelivered orders found for the specified period", map[string]interface{}{
			"carrier_id":      data.Data.CarrierID,
			"start_of_period": startOfPeriod,
			"end_of_period":   endOfPeriod,
		})
	}

	return nil
}
