package models

import (
	"time"

	"github.com/google/uuid"
)

type ScheduleAppointmentEmailRequest struct {
	Email   string    `json:"email" binding:"required"`
	Name    string    `json:"name"`
	CC      []string  `json:"cc"`
	SendAt  time.Time `json:"send_at" binding:"required"`
	IsReminder *bool `json:"is_reminder"`
	ReminderSendAt *[]time.Time `json:"reminder_send_at"`
	Data    ScheduleAppointmentEmailData `json:"data" binding:"required"`
}

type ScheduleAppointmentEmailData struct {
	PONumber               *[]string        `json:"po_number"`
	LRNumber               *string          `json:"lr_number"`
	AppointmentDate        *time.Time       `json:"appointment_date"`
	MasterWaybill          *[]string        `json:"master_waybill"`
	ChildWaybill           *[]string        `json:"child_waybill"`
	Cartons                *[]CartonDetails `json:"cartons"`
	TotalCartons           *int             `json:"total_cartons"`
	TotalDeadWeight        *float64         `json:"total_dead_weight"`
	TotalVolumetricWeight  *float64         `json:"total_volumetric_weight"`
	
	WarehouseName          *string          `json:"warehouse_name"`
	WarehouseAddress       *string          `json:"warehouse_address"`
	WarehouseCity          *string          `json:"warehouse_city"`
	WarehouseState         *string          `json:"warehouse_state"`
	WarehousePin           *string          `json:"warehouse_pin"`
	WarehousePhone         *string          `json:"warehouse_phone"`
	WarehouseAlternatePhone *string         `json:"warehouse_alternate_phone"`
	WarehouseEmail         *string          `json:"warehouse_email"`

	CustomerWarehouseName          *string          `json:"customer_warehouse_name"`
	CustomerWarehouseAddress       *string          `json:"customer_warehouse_address"`
	CustomerWarehouseCity          *string          `json:"customer_warehouse_city"`
	CustomerWarehouseState         *string          `json:"customer_warehouse_state"`
	CustomerWarehousePin           *string          `json:"customer_warehouse_pin"`
	CustomerWarehousePhone         *string          `json:"customer_warehouse_phone"`
	CustomerWarehouseAlternatePhone *string         `json:"customer_warehouse_alternate_phone"`
	CustomerWarehouseEmail         *string          `json:"customer_warehouse_email"`
	
	Files                  *[]string        `json:"files"`
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

	AWB                   	*[]string 		`json:"awb" db:"awb"`
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

	AWB                   	*[]string 		`json:"awb" db:"awb"`
	UUIDAWB                 *[]uuid.UUID	`json:"uuid_awb" db:"uuid_awb"`
}
