package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewDeviceFields(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, c := range []struct {
		name         string
		in           *string
		serialNumber *string
		status       *string
		location     *string
		field        string
	}{
		{"valid", str("  Sensor  "), str(" SN-1 "), str("active"), str("Gudang A"), ""},
		{"location left out", str("Sensor"), str("SN-1"), str("maintenance"), nil, ""},
		{"blank location", str("Sensor"), str("SN-1"), str("inactive"), str("  "), ""},
		{"120 characters", str(strings.Repeat("é", 120)), str("SN-1"), str("active"), nil, ""},
		{"121 characters", str(strings.Repeat("é", 121)), str("SN-1"), str("active"), nil, "name"},
		{"blank", str("   "), str("SN-1"), str("active"), nil, "name"},
		{"missing name", nil, str("SN-1"), str("active"), nil, "name"},
		{"missing serial number", str("Sensor"), nil, str("active"), nil, "serial_number"},
		{"blank serial number", str("Sensor"), str(" "), str("active"), nil, "serial_number"},
		{"65 character serial number", str("Sensor"), str(strings.Repeat("9", 65)), str("active"), nil, "serial_number"},
		{"missing status", str("Sensor"), str("SN-1"), nil, nil, "status"},
		{"unknown status", str("Sensor"), str("SN-1"), str("broken"), nil, "status"},
		{"121 character location", str("Sensor"), str("SN-1"), str("active"), str(strings.Repeat("é", 121)), "location"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := NewDeviceFields(c.in, c.serialNumber, c.status, c.location)
			var bad *InvalidFieldError
			switch {
			case c.field == "" && err != nil:
				t.Fatal(err)
			case c.field != "" && (!errors.As(err, &bad) || bad.Field != c.field):
				t.Fatalf("err = %v, want invalid field %s", err, c.field)
			case c.name == "valid" && (f.Name != "Sensor" || f.SerialNumber != "SN-1" || f.Location != "Gudang A"):
				t.Fatalf("fields = %+v, want trimmed", f)
			}
		})
	}
}
