package controllers

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/queues"
	"encoding/json"
	"net/http"
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
		request.SendAt = time.Now().Add(time.Second * 10)
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
		return
	}
	
	helpers.LogInfo("email task enqueued successfully", map[string]interface{}{
		"task_id": info.ID,
		"queue": info.Queue,
		"task_type": task.Type(),
		"send_at": request.SendAt,
	})

	if request.IsReminder != nil && *request.IsReminder {
		
		for _, reminderSendAt := range *request.ReminderSendAt {
			request.SendAt = reminderSendAt
			if reminderSendAt.Before(time.Now()) {
				request.SendAt = time.Now().Add(time.Second * 30)
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
			} else {
				helpers.LogInfo("reminder email task enqueued successfully", map[string]interface{}{
					"task_id": reminderInfo.ID,
					"queue": reminderInfo.Queue,
					"task_type": reminderTask.Type(),
					"send_at": reminderSendAt,
				})
			}
		}

	}

	c.JSON(http.StatusOK, models.ServerResponse{
		Success: true,
		StatusCode: http.StatusOK,
		Message: "Email scheduled successfully",
		Data: map[string]any{
			"email": request.Email,
			"cc": request.CC,
			"send_at": request.SendAt,
			"is_reminder": request.IsReminder,
			"reminder_send_at": request.ReminderSendAt,
			// "data": request.Data,
		},
	})

}