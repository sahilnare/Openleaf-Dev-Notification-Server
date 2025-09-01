package helpers

import (
	"Notification-Server/db"
	"Notification-Server/models"
	"fmt"

	"github.com/google/uuid"
)

func InsertNotificationLog(n *models.Notification) (uuid.UUID, error) {
	query := `
		INSERT INTO notification_logs (
			notification_id,
			order_id,
			sender_email,
			sender_cc_emails,
			receiver_emails,
			receiver_cc_emails,
			method,
			type,
			status,
			sent_at,
			custom_data
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		RETURNING notification_id
	`
	err := db.GlobalDB.QueryRow(query,
		n.NotificationID,
		n.OrderID,
		n.Sender,
		n.SenderCC,
		n.Receiver,
		n.ReceiverCC,
		n.Method,
		n.Type,
		n.Status,
		n.SentAt,
		nil,
	).Scan(&n.NotificationID)
	if err != nil {
		fmt.Println("err", err.Error())
		return uuid.Nil, err
	}

	return n.NotificationID, nil
}

func UpdateNotification(n *models.Notification) (uuid.UUID, error) {
	query := `
		UPDATE notification_logs SET
			sender_email = $1,
			sender_cc_emails = $2,
			receiver_emails = $3,
			receiver_cc_emails = $4,
			method = $5,
			type = $6,
			status = $7,
			sent_at = $8,
			custom_data = $9
		WHERE notification_id = $10 AND order_id = $11
	`
	_, err := db.GlobalDB.Exec(query,
		n.Sender,
		n.SenderCC,
		n.Receiver,
		n.ReceiverCC,
		n.Method,
		n.Type,
		n.Status,
		n.SentAt,
		nil,
		n.NotificationID,
		n.OrderID,
	)

	if err != nil {
		fmt.Println("err", err.Error())
		return uuid.Nil, err
	}

	return n.NotificationID, nil
}
