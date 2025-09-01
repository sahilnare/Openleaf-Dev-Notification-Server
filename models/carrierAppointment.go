package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CarrierAppointmentEmailData struct {
	PONumber              *JSONBArrayString        `json:"po_number"`
	LRNumber              *string          `json:"lr_number"`
	AppointmentDate       *time.Time       `json:"appointment_date"`
	MasterWaybill         *JSONBArrayString        `json:"master_waybill"`
	ChildWaybill          *JSONBArrayString        `json:"child_waybill"`
	Cartons               *CartonDetailsList `json:"cartons"`
	TotalCartons          *int             `json:"total_cartons"`
	TotalDeadWeight       *float64         `json:"total_dead_weight"`
	TotalVolumetricWeight *float64         `json:"total_volumetric_weight"`

	CarrierID string `json:"carrier_id"`
	CarrierName string `json:"carrier_name"`
	Channel string `json:"channel"`

	WarehouseName           *string `json:"warehouse_name"`
	WarehouseAddress        *string `json:"warehouse_address"`
	WarehouseCity           *string `json:"warehouse_city"`
	WarehouseState          *string `json:"warehouse_state"`
	WarehousePin            *string `json:"warehouse_pin"`
	WarehousePhone          *string `json:"warehouse_phone"`
	WarehouseAlternatePhone *string `json:"warehouse_alternate_phone"`
	WarehouseEmail          *string `json:"warehouse_email"`

	CustomerWarehouseName           *string `json:"customer_warehouse_name"`
	CustomerWarehouseAddress        *string `json:"customer_warehouse_address"`
	CustomerWarehouseCity           *string `json:"customer_warehouse_city"`
	CustomerWarehouseState          *string `json:"customer_warehouse_state"`
	CustomerWarehousePin            *string `json:"customer_warehouse_pin"`
	CustomerWarehousePhone          *string `json:"customer_warehouse_phone"`
	CustomerWarehouseAlternatePhone *string `json:"customer_warehouse_alternate_phone"`
	CustomerWarehouseEmail          *string `json:"customer_warehouse_email"`

	OrderPlacedAt          *time.Time `json:"order_placed_at"`
	ExpectedDeliveryDate   *time.Time `json:"expected_delivery_date"`
	AppointmentScheduledAt *time.Time `json:"appointment_scheduled_at"`
	AppointmentTakenAt     *time.Time `json:"appointment_taken_at"`
	IsAppointmentConfirmed *bool      `json:"is_appointment_confirmed"`

	Files *[]string `json:"files"`
}


type CartonDetailsList []CartonDetails

func (c *CartonDetailsList) Scan(value any) error {
	if value == nil {
		*c = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("carton_details: failed to scan value %v of type %T into CartonDetailsList; expected []byte", value, value)
	}

	if err := json.Unmarshal(bytes, c); err != nil {
		return fmt.Errorf("carton_details: failed to unmarshal JSONB: %w", err)
	}

	return nil
}

func (c CartonDetailsList) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

type CartonDetails struct {
	ID               *uuid.UUID `json:"id" db:"id"`
	Type             string     `json:"type" db:"type"`
	Length           float64    `json:"length" db:"length"`
	Breadth          float64    `json:"breadth" db:"breadth"`
	Height           float64    `json:"height" db:"height"`
	Weight           float64    `json:"weight" db:"weight"`
	Quantity         float64    `json:"quantity" db:"quantity"`
	VolumetricWeight float64    `json:"volumetric_weight" db:"volumetric_weight"`

	AWB                   	*JSONBArrayString 		`json:"awb" db:"awb"`
	UUIDAWB                 *[]uuid.UUID	`json:"uuid_awb" db:"uuid_awb"`

	TotalChildCartons *int64 `json:"total_child_cartons" db:"total_child_cartons"`
	ChildCartons *[]ChildCartonDetails `json:"child_cartons" db:"child_cartons"`
}

type ChildCartonDetails struct {
	ID                *uuid.UUID `json:"id" db:"id"`
	Type              string     `json:"type" db:"type"`
	Length            float64    `json:"length" db:"length"`
	Breadth            float64    `json:"breadth" db:"breadth"`
	Height            float64    `json:"height" db:"height"`
	Weight            float64    `json:"weight" db:"weight"`
	Quantity          float64    `json:"quantity" db:"quantity"`
	VolumetricWeight  float64    `json:"volumetric_weight" db:"volumetric_weight"`

	AWB                   	*JSONBArrayString 		`json:"awb" db:"awb"`
	UUIDAWB                 *[]uuid.UUID	`json:"uuid_awb" db:"uuid_awb"`
}

type CarrierAppointmentEmailSettings struct {
	AnsID                    uuid.UUID  `json:"ans_id" db:"ans_id"`
	AdminID                  uuid.UUID  `json:"admin_id" db:"admin_id"`
	UserID                   uuid.UUID  `json:"user_id" db:"user_id"` // warehouse user_id
	Channel                  *string     `json:"channel" db:"channel"`
	CarrierID                string     `json:"carrier_id" db:"carrier_id"`

	SenderEmailsForChannel        *string `json:"sender_emails_for_channel" db:"sender_emails_for_channel"`
	SenderCCEmailsForChannel     *string `json:"sender_cc_emails_for_channel" db:"sender_cc_emails_for_channel"`
	ReceiverEmailsForChannel     *string `json:"receiver_emails_for_channel" db:"receiver_emails_for_channel"`
	ReceiverCCEmailsForChannel   *string `json:"receiver_cc_emails_for_channel" db:"receiver_cc_emails_for_channel"`

	SenderEmailsForCarrier       *string `json:"sender_emails_for_carrier" db:"sender_emails_for_carrier"`
	SenderCCEmailsForCarrier    *string `json:"sender_cc_emails_for_carrier" db:"sender_cc_emails_for_carrier"`
	ReceiverEmailsForCarrier    *string `json:"receiver_emails_for_carrier" db:"receiver_emails_for_carrier"`
	ReceiverCCEmailsForCarrier  *string `json:"receiver_cc_emails_for_carrier" db:"receiver_cc_emails_for_carrier"`

	SendNotification        bool      `json:"send_notification" db:"send_notification"`
	NotificationDays        *string    `json:"notification_days" db:"notification_days"` // 1,2
	NotificationType        *string    `json:"notification_type" db:"notification_type"`

	SendReminder            bool      `json:"send_reminder" db:"send_reminder"`
	ReminderDays            *string    `json:"reminder_days" db:"reminder_days"` // 1,2
	ReminderType            *string    `json:"reminder_type" db:"reminder_type"`

	SendBulkReminder        bool      `json:"send_bulk_reminder" db:"send_bulk_reminder"`
	BulkReminderDays        *string    `json:"bulk_reminder_days" db:"bulk_reminder_days"` // 2,3
	BulkReminderType        *string    `json:"bulk_reminder_type" db:"bulk_reminder_type"`

	CreatedAt               time.Time `json:"created_at" db:"created_at"`
}

type CarrierAppointmentEmailWorkerData struct {
	NotificationID uuid.UUID `json:"notification_id"`

	OrderID uuid.UUID `json:"order_id"`
	AdminID uuid.UUID `json:"admin_id"`
	UserID uuid.UUID `json:"user_id"`
	
	Data     CarrierAppointmentEmailData     `json:"data"`
	Settings CarrierAppointmentEmailSettings `json:"settings"`
}
