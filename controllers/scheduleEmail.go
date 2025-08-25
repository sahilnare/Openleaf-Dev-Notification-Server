package controllers

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

func ScheduleAppointmentEmail(c *gin.Context) {

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

	payload, err := json.Marshal(request)
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

	task := asynq.NewTask("email:carrier-appointment-notification", payload)

	if request.SendAt.Before(time.Now()) {
		request.SendAt = time.Now().Add(time.Second * 5)
	}

	info, err := queues.EmailQueueClient.Enqueue(task, asynq.ProcessAt(request.SendAt))

	if err != nil {

		helpers.LogException("failed to enqueue email task", map[string]interface{}{
			"request": request,
			"request_headers": c.Request.Header,
			"request_url": c.Request.URL,
			"request_method": c.Request.Method,	
			"error": err.Error(),
		})

		c.JSON(http.StatusInternalServerError, models.ServerResponse{
			Success: false,
			StatusCode: http.StatusInternalServerError,
			Message: "Failed to enqueue email task",
			Error: err.Error(),
		})

		_, err = helpers.InsertNotification(&models.Notification{
			OrderID: request.OrderID,
			Sender: request.Email,
			CC: strings.Join(request.CC, ","),
			Receiver: request.Email,
			Type: "appointment",
			Status: "error",
			SentAt: nil,
		})

		if err != nil {
			helpers.LogException("failed to insert notification", map[string]interface{}{
				"request": request,
				"request_headers": c.Request.Header,
				"request_url": c.Request.URL,
				"request_method": c.Request.Method,	
				"error": err.Error(),
			})

			c.JSON(http.StatusInternalServerError, models.ServerResponse{
				Success: false,
				StatusCode: http.StatusInternalServerError,
				Message: "Failed to insert notification",
				Error: err.Error(),
			})
			return
		}

		return
	}

	notificationID, err := helpers.InsertNotification(&models.Notification{
		OrderID: request.OrderID,
		Sender: request.Email,
		CC: strings.Join(request.CC, ","),
		Receiver: request.Email,
		Type: "appointment",
		Status: "scheduled",
		SentAt: nil,
	})

	if err != nil {
		helpers.LogException("failed to insert notification", map[string]interface{}{
			"request": request,
			"request_headers": c.Request.Header,
			"request_url": c.Request.URL,
			"request_method": c.Request.Method,	
			"error": err.Error(),
		})
	}

	helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
		"task_id": info.ID,
		"queue": info.Queue,
		"task_type": task.Type(),
		"send_at": request.SendAt,
		"notification_id": notificationID,
	})

	// # Check for reminders
	if request.IsReminder != nil && *request.IsReminder {
		
		for _, reminderSendAt := range *request.ReminderSendAt {
			request.SendAt = reminderSendAt
			if reminderSendAt.Before(time.Now()) {
				request.SendAt = time.Now().Add(time.Second * 5)
			}
			payload, err = json.Marshal(request)
			if err != nil {

				helpers.LogException("failed to marshal reminder request payload", map[string]interface{}{
					"request": request,
					"request_headers": c.Request.Header,
					"request_url": c.Request.URL,
					"request_method": c.Request.Method,	
					"error": err.Error(),
				})

				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success: false,
					StatusCode: http.StatusInternalServerError,
					Message: "Failed to marshal reminder request payload",
					Error: err.Error(),
				})

				_, err = helpers.InsertNotification(&models.Notification{
					OrderID: request.OrderID,
					Sender: request.Email,
					CC: strings.Join(request.CC, ","),
					Receiver: request.Email,
					Type: "appointment_reminder",
					Status: "error",
					SentAt: nil,
				})

				if err != nil {
					helpers.LogException("failed to insert reminder notification", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to insert reminder notification",
						Error: err.Error(),
					})
					return
				}

				return
			}

			reminderTask := asynq.NewTask("email:carrier-appointment-reminder", payload)
			reminderInfo, err := queues.EmailQueueClient.Enqueue(reminderTask, asynq.ProcessAt(reminderSendAt))
			if err != nil {
				helpers.LogException("failed to enqueue reminder email task", map[string]interface{}{
					"request": request,
					"request_headers": c.Request.Header,
					"request_url": c.Request.URL,
					"request_method": c.Request.Method,	
					"error": err.Error(),
				})

				c.JSON(http.StatusInternalServerError, models.ServerResponse{
					Success: false,
					StatusCode: http.StatusInternalServerError,
					Message: "Failed to enqueue reminder email task",
					Error: err.Error(),
				})

				_, err = helpers.InsertNotification(&models.Notification{
					OrderID: request.OrderID,
					Sender: request.Email,
					CC: strings.Join(request.CC, ","),
					Receiver: request.Email,
					Type: "appointment_reminder",
					Status: "error",
					SentAt: nil,
				})

				if err != nil {

					helpers.LogException("failed to insert reminder notification", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to insert reminder notification",
						Error: err.Error(),
					})
					return
				}

				return

			} else {

				_, err = helpers.InsertNotification(&models.Notification{
					OrderID: request.OrderID,
					Sender: request.Email,
					CC: strings.Join(request.CC, ","),
					Receiver: request.Email,
					Type: "appointment_reminder",
					Status: "scheduled",
					SentAt: nil,
				})

				if err != nil {
					helpers.LogException("failed to insert reminder notification", map[string]interface{}{
						"request": request,
						"request_headers": c.Request.Header,
						"request_url": c.Request.URL,
						"request_method": c.Request.Method,	
						"error": err.Error(),
					})

					c.JSON(http.StatusInternalServerError, models.ServerResponse{
						Success: false,
						StatusCode: http.StatusInternalServerError,
						Message: "Failed to insert reminder notification",
						Error: err.Error(),
					})
					return
				}

				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id": reminderInfo.ID,
					"queue": reminderInfo.Queue,
					"task_type": reminderTask.Type(),
					"send_at": reminderSendAt,
					"notification_id": notificationID,
				})
			}
		}

	}

	c.JSON(http.StatusOK, models.ServerResponse{
		Success: true,
		StatusCode: http.StatusOK,
		Message: "Appointment email scheduled successfully",
		Data: map[string]any{
			"order_id": request.OrderID,
			"email": request.Email,
			"cc": request.CC,
			"send_at": request.SendAt,
			"is_reminder": request.IsReminder,
			"reminder_send_at": request.ReminderSendAt,
		},
	})

}