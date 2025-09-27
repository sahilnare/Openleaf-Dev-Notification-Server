package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	EmailCarrierAppointmentQueue = "email:carrier-appointment-notification"
	EmailCarrierAppointmentReminderQueue = "email:carrier-appointment-reminder"
	EmailCarrierAppointmentBulkReminderQueue = "email:carrier-appointment-bulk-reminder"
	
	EmailCarrierBulkPickupNotificationQueue = "email:carrier-bulk-pickup-notification"
)

type Notification struct {
	NotificationID uuid.UUID `json:"notification_id"`
	
	OrderID        uuid.UUID  `json:"order_id"`

	Sender         string     `json:"sender"`
	SenderCC       *string     `json:"sender_cc"`
	Receiver       string     `json:"receiver"`
	ReceiverCC     *string     `json:"receiver_cc"`

	Method string `json:"method"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	SentAt         *time.Time `json:"sent_at"`
}
