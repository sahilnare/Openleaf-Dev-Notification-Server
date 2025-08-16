package controllers

import (
	"Notification-Server/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ScheduleEmail(c *gin.Context) {

	var request models.ScheduleEmailRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ServerResponse{
			Success: false,
			StatusCode: http.StatusBadRequest,
			Message: "Invalid request",
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.ServerResponse{
		Success: true,
		StatusCode: http.StatusOK,
		Message: "Email scheduled successfully",
		Data: map[string]any{
			"email": request.Email,
			"cc": request.CC,
			"send_at": request.SendAt,
			"data": request.Data,
		},
	})

}