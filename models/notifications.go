package models

import (
	"time"

	"github.com/google/uuid"
)

type Notification struct {
	NotificationID string `json:"notification_id"`
	
	OrderID        uuid.UUID  `json:"order_id"`
	Sender         string     `json:"sender"`
	CC             string     `json:"cc"`
	Receiver       string     `json:"receiver"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	SentAt         *time.Time `json:"sent_at"`
}