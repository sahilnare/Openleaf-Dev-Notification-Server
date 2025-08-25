package helpers

import (
	"Notification-Server/db"
	"Notification-Server/models"
)

func InsertNotification(n *models.Notification) (string, error) {

	query := `
		INSERT INTO notifications (
			order_id, sender, cc, receiver, type, status, sent_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		RETURNING notification_id
	`
	var notificationID string
	err := db.GlobalDB.QueryRow(query,
		n.OrderID,
		n.Sender,
		n.CC,
		n.Receiver,
		n.Type,
		n.Status,
		n.SentAt,
	).Scan(&notificationID)
	if err != nil {
		return "", err
	}

	return notificationID, nil

}

func UpdateNotification(n *models.Notification) (string, error) {

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
		return "", err
	}

	return n.NotificationID, nil

}
