package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	EmailCarrierAppointmentQueue = "email:carrier-appointment"
	EmailCarrierAppointmentReminderQueue = "email:carrier-appointment-reminder"
	EmailCarrierAppointmentBulkReminderQueue = "email:carrier-appointment-bulk-reminder"
)

type Notification struct {
	NotificationID uuid.UUID `json:"notification_id"`
	
	OrderID        uuid.UUID  `json:"order_id"`
	Sender         string     `json:"sender"`
	CC             string     `json:"cc"`
	Receiver       string     `json:"receiver"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	SentAt         *time.Time `json:"sent_at"`
}
