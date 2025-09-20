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

	"github.com/hibiken/asynq"
)

func SendCarrierBulkPickupEmail(ctx context.Context, task *asynq.Task) error {
	defer func() {
		if r := recover(); r != nil {
			helpers.LogException("[worker] carrier bulk pickup email worker panic recovered", map[string]interface{}{
				"panic":     r,
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})
		}
	}()

	helpers.LogInfo("[worker] carrier bulk pickup email worker started", map[string]interface{}{
		"task_type":      task.Type(),
		"task_data":      string(task.Payload()),
		"payload_length": len(task.Payload()),
	})

	if task == nil {
		helpers.LogInfo("[worker] carrier bulk pickup email task is nil", map[string]interface{}{
			"task_data": string(task.Payload()),
			"task_type": task.Type(),
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier bulk pickup email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload":   string(payload),
	})

	var data models.CarrierBulkPickupEmailWorkerData
	err := json.Unmarshal(payload, &data)
	if err != nil {
		helpers.LogException("[worker] carrier bulk pickup email failed to unmarshal payload", map[string]interface{}{
			"error":     err.Error(),
			"task_type": task.Type(),
			"payload":   string(payload),
		})
		return err
	}

	helpers.LogInfo("[worker] carrier bulk pickup email payload unmarshaled", map[string]interface{}{
		"task_type": task.Type(),
		"data":      data,
	})

	// todo fetch data

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
	now := time.Now()

	baseQuery := `
    SELECT
        o.channel, o.po_number, o.customer_city, o.customer_pincode,
        o.sku_details, o.lr_number, oa.appointment_scheduled_at,
        o.total_cartons, o.total_dead_weight, o.carton_details,
        o.invoice_number, o.total_invoice_value
    FROM
        orders o
    LEFT JOIN
        order_activity oa ON o.order_id = oa.order_id
`
	var queryBuilder strings.Builder
	queryBuilder.WriteString(baseQuery)

	queryBuilder.WriteString(" WHERE o.carrier_id = $1")
	args := []interface{}{data.Data.CarrierID}

	if day < 0 {
		fromDate := now.AddDate(0, 0, day)
		queryBuilder.WriteString(" AND oa.order_placed_at >= $2 AND oa.order_placed_at < $3")
		args = append(args, fromDate, now)

	} else if day > 0 {
		toDate := now.AddDate(0, 0, day)
		queryBuilder.WriteString(" AND oa.order_placed_at > $2 AND oa.order_placed_at <= $3")
		args = append(args, now, toDate)

	} else {
		year, month, d := now.Date()
		startOfDay := time.Date(year, month, d, 0, 0, 0, 0, now.Location())

		queryBuilder.WriteString(" AND oa.order_placed_at >= $2 AND oa.order_placed_at <= $3")
		args = append(args, startOfDay, now)
	}

	finalQuery := queryBuilder.String()

	helpers.LogInfo("[worker] executing database query", map[string]interface{}{
		"query": finalQuery,
		"args":  args,
	})

	orders := []models.CarrierBulkPickupEmailData{}
	if err := db.GlobalDB.Select(&orders, finalQuery, args...); err != nil {
		helpers.LogException("[worker] failed to fetch and scan orders", map[string]interface{}{
			"error":      err.Error(),
			"carrier_id": data.Data.CarrierID,
			"query":      finalQuery,
			"args":       args,
		})
		return err

	}

	helpers.LogInfo("[worker] fetched orders for carrier bulk pickup", map[string]interface{}{
		"carrier_id":   data.Data.CarrierID,
		"orders_count": len(orders),
	})
	// todo send email

	if len(orders) > 0 {
		var totalCartons int
		var totalWeight float64

		lrSet := make(map[string]bool)
		for _, order := range orders {
			totalCartons += helpers.DerefIntPointer(order.TotalCartons)
			totalWeight += helpers.DerefFloatPointer(order.Weight)
			if order.LRNumber != nil {
				lrSet[*order.LRNumber] = true
			}
		}
		totalLRs := len(lrSet)

		var tableRows strings.Builder

		for i, order := range orders {
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
						"%.2f x %.2f x %.2f Inch = %.f<br>",
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

			// Creating rows for the table
			rowHTML := fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%s</td>
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
				<td>%.2f</td>
				<td>%.2f</td>
			</tr>`,
				i+1,           // Sr No
				order.Channel, // PO Details: Channel
				poNumberStr,   // PO Details: Number
				helpers.FormatDateDDMMYYYY(order.AppointmentScheduledAt), // PO Details: Pickup Date
				helpers.DerefStringPointer(order.CustomerWarehouseCity),  // PO Details: City
				helpers.DerefStringPointer(order.WarehousePin),           // PO Details: Pincode
				helpers.DerefFloatPointer(order.Amount),                  // PO Details: Amount
				helpers.DerefStringPointer(order.LRNumber),               // LR Number
				helpers.FormatDateDDMMYYYY(order.AppointmentScheduledAt), // Appointment Date
				helpers.DerefIntPointer(order.TotalCartons),              // Carton Details: Quantity
				helpers.DerefFloatPointer(order.Weight)/1000,             // Carton Details: Weight
				cartonDimensions, // Carton Details: Dimensions
				helpers.DerefStringPointer(order.InvoiceNumber), // Invoice Details: Number
				helpers.DerefFloatPointer(totalSkuQuantity),     // SKU Quantity
				helpers.DerefFloatPointer(order.Amount),         // Invoice Details: Amount
			)
			tableRows.WriteString(rowHTML)
		}

		//Final email body
		body := fmt.Sprintf(templates.SendCarrierBulkPickupEmailTemplate,
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

		var dateStrings []string

		if day > 0 {
			for i := 1; i <= day; i++ {
				pastDate := now.AddDate(0, 0, i)
				formattedDate := pastDate.Format("2 Jan 2006")
				dateStrings = append(dateStrings, formattedDate)
			}
		} else if day < 0 {
			for i := 1; i <= (day * -1); i++ {
				pastDate := now.AddDate(0, 0, -i)
				formattedDate := pastDate.Format("2 Jan 2006")
				dateStrings = append(dateStrings, formattedDate)
			}
		} else {
			dateStrings = append(dateStrings, now.Format("2 Jan 2006"))
		}

		finalString := strings.Join(dateStrings, ", ")

		helpers.LogInfo("[worker] preparing to send bulk pickup email", map[string]interface{}{
			"carrier_id":      data.Data.CarrierID,
			"total_orders":    len(orders),
			"total_cartons":   totalCartons,
			"total_weight":    totalWeight,
			"total_lrs":       totalLRs,
			"date_range_str":  finalString,
			"recipient_count": len(receiverEmails),
			"cc_count":        len(receiverCC),
		})

		helpers.LogInfo("[worker] attempting to send bulk pickup email", map[string]interface{}{
			"from": helpers.B2B_EMAIL,
			"to":   receiverEmails,
			"cc":   receiverCC,
			"subject": fmt.Sprintf("Pickup Plan for %s",
				finalString,
			),
			"body_length": len(body),
		})

		err = helpers.SendEmail(
			helpers.B2B_EMAIL,
			receiverEmails,
			receiverCC,
			fmt.Sprintf("Pickup Plan for %s",
				finalString,
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
				Type:           models.EmailCarrierAppointmentReminderQueue,
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
					"type":            models.EmailCarrierAppointmentReminderQueue,
				})
			}

			return err
		}

		helpers.LogInfo("[worker] reminder email sent successfully", map[string]interface{}{
			"task_type": task.Type(),
			"task_data": string(task.Payload()),
			"data":      data,
		})

		helpers.LogInfo("[worker] carrier bulk pickup email worker completed successfully", map[string]interface{}{
			"task_type": task.Type(),
			"data":      data,
		})

	}
	return nil
}
