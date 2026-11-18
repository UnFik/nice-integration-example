package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxDeviceNameLength   = 120
	MaxSerialNumberLength = 64
	MaxLocationLength     = 120
)

const (
	StatusActive      = "active"
	StatusInactive    = "inactive"
	StatusMaintenance = "maintenance"
)

// DeviceStatuses are the values a device status may take.
var DeviceStatuses = []string{StatusActive, StatusInactive, StatusMaintenance}

// Device is an IoT device of one project of one organization.
type Device struct {
	ID           string
	OrgID        string
	ProjectID    string
	Name         string
	SerialNumber string
	Status       string
	Location     string
	CreatedBy    string
	// ActedBy is act.sub of the token that created the device: the client of a backend acting for CreatedBy.
	ActedBy    string
	CreateTime time.Time
	UpdateTime time.Time
}

// DeviceFields are the fields a client may set on a device.
type DeviceFields struct {
	Name         string
	SerialNumber string
	Status       string
	Location     string
}

var ErrDeviceNotFound = errors.New("device not found")

// InvalidFieldError names the first field that broke a rule.
type InvalidFieldError struct{ Field string }

func (e *InvalidFieldError) Error() string { return "invalid field: " + e.Field }

// NewDeviceFields validates the fields of a create or replace request. A nil
// pointer is a field the client left out. Name and serial number are trimmed and
// count 1 to MaxDeviceNameLength and MaxSerialNumberLength characters (not
// bytes); the status is one of DeviceStatuses; the location is optional and
// counts at most MaxLocationLength characters.
func NewDeviceFields(name, serialNumber, status, location *string) (DeviceFields, error) {
	n, ok := trimmed(name, 1, MaxDeviceNameLength)
	if !ok {
		return DeviceFields{}, &InvalidFieldError{Field: "name"}
	}
	sn, ok := trimmed(serialNumber, 1, MaxSerialNumberLength)
	if !ok {
		return DeviceFields{}, &InvalidFieldError{Field: "serial_number"}
	}
	if status == nil || !slices.Contains(DeviceStatuses, *status) {
		return DeviceFields{}, &InvalidFieldError{Field: "status"}
	}
	var loc string
	if location != nil {
		if loc, ok = trimmed(location, 0, MaxLocationLength); !ok {
			return DeviceFields{}, &InvalidFieldError{Field: "location"}
		}
	}
	return DeviceFields{Name: n, SerialNumber: sn, Status: *status, Location: loc}, nil
}

func trimmed(s *string, min, max int) (string, bool) {
	if s == nil {
		return "", false
	}
	v := strings.TrimSpace(*s)
	l := utf8.RuneCountInString(v)
	return v, l >= min && l <= max
}
