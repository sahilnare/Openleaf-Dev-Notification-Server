package helpers

import (
	"Notification-Server/db"
	"Notification-Server/models"

	"github.com/google/uuid"
)

func InsertNotificationLog(n *models.Notification) (uuid.UUID, error) {

	query := `
		INSERT INTO notifications (
			notification_id, order_id, sender, cc, receiver, type, status, sent_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		RETURNING notification_id
	`
	err := db.GlobalDB.QueryRow(query,
		n.NotificationID,
		n.OrderID,
		n.Sender,
		n.CC,
		n.Receiver,
		n.Type,
		n.Status,
		n.SentAt,
	).Scan(&n.NotificationID)
	if err != nil {
		return uuid.Nil, err
	}

	return n.NotificationID, nil

}

func UpdateNotification(n *models.Notification) (uuid.UUID, error) {

	query := `
		UPDATE notifications SET
			sender = $1,
			cc = $2,
			receiver = $3,
			type = $4,
			status = $5,
			sent_at = $6
		WHERE notification_id = $7 AND order_id = $8
	`
	_, err := db.GlobalDB.Exec(query,
		n.Sender,
		n.CC,
		n.Receiver,
		n.Type,
		n.Status,
		n.SentAt,
		n.NotificationID,
		n.OrderID,
	)

	if err != nil {
		return uuid.Nil, err
	}

	return n.NotificationID, nil

}
