package db

import "time"

// AppointmentScheduling represents a scheduled appointment
// from appointments_scheduling table
 type AppointmentScheduling struct {
	ID          int       `db:"id"`
	UserID      string    `db:"user_id"`
	ScheduleDatetime time.Time `db:"schedule_datetime"`
	IsProcessed string    `db:"is_processed"`
	EmailID     string    `db:"email_id"`
	OrderID     *string   `db:"order_id"`
	PoIDs       *string   `db:"po_ids"`
	CcEmailID   *string   `db:"cc_email_id"`
	Phase       *int      `db:"phase"`
	CarrierEmailID *string `db:"carrier_email_id"`
	CarrierCcEmailID *string `db:"carrier_cc_email_id"`
	IsReminder  *bool     `db:"is_reminder"`
	ConfirmedAppointmentTime *time.Time `db:"confirmed_appointment_time"`
	Address     *string   `db:"address"`
}

// SentEmail tracks sent emails for reply monitoring
 type SentEmail struct {
	ID          int       `db:"id"`
	CampaignID  string    `db:"campaign_id"`
	MessageID   string    `db:"message_id"`
	ThreadID    string    `db:"thread_id"`
	RecipientEmail string `db:"recipient_email"`
	RecipientName string  `db:"recipient_name"`
	Subject     string    `db:"subject"`
	Body        string    `db:"body"`
	SentAt      time.Time `db:"sent_at"`
	Replied     bool      `db:"replied"`
	ResponseSent bool     `db:"response_sent"`
	LastHistoryID *string `db:"last_history_id"`
	ReferencesChain *string `db:"references_chain"`
	RfcMessageID *string  `db:"rfc_message_id"`
	UserID      *string   `db:"user_id"`
	OrderID     *string   `db:"order_id"`
	RecordDate  *time.Time `db:"record_date"`
	CcEmails    *string   `db:"cc_emails"`
}

// EmailReply represents a received reply
 type EmailReply struct {
	MessageID   string    `db:"message_id"`
	ThreadID    string    `db:"thread_id"`
	FromEmail   string    `db:"from_email"`
	Subject     string    `db:"subject"`
	Body        string    `db:"body"`
	ReceivedAt  time.Time `db:"received_at"`
	HistoryID   string    `db:"history_id"`
	OriginalEmail SentEmail
	RfcMessageID *string  `db:"rfc_message_id"`
	ReplyToHeader *string `db:"reply_to_header"`
	ReplyCcHeader *string `db:"reply_cc_header"`
}

// Orders represents the orders table
 type Order struct {
	OrderID         string  `db:"order_id"`
	UserID          string  `db:"user_id"`
	OriginalOrderID string  `db:"original_order_id"`
	PoID            string  `db:"po_id"`
	WarehouseID     string  `db:"warehouse_id"`
	CustomerName    string  `db:"customer_name"`
	CustomerAddress string  `db:"customer_address"`
	CustomerPhone   string  `db:"customer_phone"`
	CustomerPincode string  `db:"customer_pincode"`
	CustomerCity    string  `db:"customer_city"`
	CustomerState   string  `db:"customer_state"`
	CustomerEmail   string  `db:"customer_email"`
	TruckLoadType   string  `db:"truck_load_type"`
	OrderType       string  `db:"order_type"`
	OrderMode       string  `db:"order_mode"`
	LRNumber        string  `db:"lr_number"`
	TotalCartons    int     `db:"total_cartons"`
} 