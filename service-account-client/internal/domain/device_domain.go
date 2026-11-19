// Package domain holds the rules of the client that depend on nothing else: what nadi is called, what a device is,
// and the failures a call can end in.
package domain

import (
	"slices"
	"time"
)

// ServiceSlug is the audience of every token this client asks for.
const ServiceSlug = "nadi"

// The permissions of nadi the two commands need.
const (
	PermRead   = "nadi.devices.read"
	PermCreate = "nadi.devices.create"
)

// DeviceStatuses are the values nadi accepts as a device status.
var DeviceStatuses = []string{"active", "inactive", "maintenance"}

// ValidStatus reports whether nadi accepts s as a status.
func ValidStatus(s string) bool { return slices.Contains(DeviceStatuses, s) }

// Device is an IoT device of the project of the Service Account, as nadi returns it.
type Device struct {
	ID           string
	Name         string
	SerialNumber string
	Status       string
	Location     string
	// CreatedBy is the principal that created the device: the id of the Service Account for a device of this client.
	CreatedBy  string
	CreateTime time.Time
	UpdateTime time.Time
}
