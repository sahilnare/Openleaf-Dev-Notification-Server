package models

import (
	"github.com/google/uuid"
)

type ScheduleAppointmentEmailRequest struct {
	OrderID uuid.UUID `json:"order_id" binding:"required"`
	AdminID uuid.UUID `json:"admin_id" binding:"required"`
	UserID uuid.UUID `json:"user_id" binding:"required"`
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
