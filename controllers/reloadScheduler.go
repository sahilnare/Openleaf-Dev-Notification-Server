package controllers

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"Notification-Server/scheduler"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ReloadScheduler re-reads appointment_notification_settings and re-registers
// the bulk reminder / pickup crons at runtime, with no server restart.
func ReloadScheduler(c *gin.Context) {

	count, err := scheduler.ReloadAppointmentSchedules()
	if err != nil {
		helpers.LogException("ReloadScheduler: failed to reload appointment schedules", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, models.ServerResponse{
			Success:    false,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to reload appointment schedules",
			Error:      err.Error(),
		})
		return
	}

	helpers.LogInfo("ReloadScheduler: appointment schedules reloaded", map[string]interface{}{
		"registered": count,
	})
	c.JSON(http.StatusOK, models.ServerResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Appointment schedules reloaded",
		Data: map[string]any{
			"registered": count,
		},
	})
}
