// Package devicerestdto holds the JSON bodies of the device REST API.
package devicerestdto

import (
	"time"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

type Device struct {
	ID           string `json:"id" example:"dev_3f2a9c0d1e4b5a6f7081920a3b4c5d6e"`
	Name         string `json:"name" example:"Sensor Suhu Gudang"`
	SerialNumber string `json:"serial_number" example:"SN-2026-0001"`
	Status       string `json:"status" enums:"active,inactive,maintenance" example:"active"`
	Location     string `json:"location" example:"Gudang A, rak 3"`
	CreatedBy    string `json:"created_by" example:"usr_01a1..."`
	// ActedBy is act.sub of the token that created the device: the client of a backend acting for CreatedBy.
	ActedBy    string    `json:"acted_by,omitempty" example:"cli_01a1..."`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
} // @name Device

type DeviceInput struct {
	Name         *string `json:"name" binding:"required" minLength:"1" maxLength:"120" example:"Sensor Suhu Gudang"`
	SerialNumber *string `json:"serial_number" binding:"required" minLength:"1" maxLength:"64" example:"SN-2026-0001"`
	Status       *string `json:"status" binding:"required" enums:"active,inactive,maintenance" example:"active"`
	Location     *string `json:"location" maxLength:"120" example:"Gudang A, rak 3"`
} // @name DeviceInput

type DeviceList struct {
	Devices []Device `json:"devices"`
} // @name DeviceList

func NewDevice(it domain.Device) Device {
	return Device{
		ID: it.ID, Name: it.Name, SerialNumber: it.SerialNumber, Status: it.Status, Location: it.Location, CreatedBy: it.CreatedBy, ActedBy: it.ActedBy,
		CreateTime: it.CreateTime, UpdateTime: it.UpdateTime,
	}
}

func NewDeviceList(devices []domain.Device) DeviceList {
	out := DeviceList{Devices: make([]Device, 0, len(devices))}
	for _, it := range devices {
		out.Devices = append(out.Devices, NewDevice(it))
	}
	return out
}
