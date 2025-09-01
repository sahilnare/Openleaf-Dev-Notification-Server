package models

import (
	"github.com/google/uuid"
)

type ScheduleAppointmentEmailRequest struct {
	OrderID uuid.UUID `json:"order_id" binding:"required"`
	AdminID uuid.UUID `json:"admin_id" binding:"required"`
	UserID uuid.UUID `json:"user_id" binding:"required"`
}

