package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewItemFields(t *testing.T) {
	str := func(s string) *string { return &s }
	num := func(n int64) *int64 { return &n }
	for _, c := range []struct {
		name     string
		in       *string
		quantity *int64
		field    string
	}{
		{"valid", str("  Kabel  "), num(0), ""},
		{"120 characters", str(strings.Repeat("é", 120)), num(1), ""},
		{"121 characters", str(strings.Repeat("é", 121)), num(1), "name"},
		{"blank", str("   "), num(1), "name"},
		{"missing name", nil, num(1), "name"},
		{"negative quantity", str("Kabel"), num(-1), "quantity"},
		{"missing quantity", str("Kabel"), nil, "quantity"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := NewItemFields(c.in, c.quantity)
			var bad *InvalidFieldError
			switch {
			case c.field == "" && err != nil:
				t.Fatal(err)
			case c.field != "" && (!errors.As(err, &bad) || bad.Field != c.field):
				t.Fatalf("err = %v, want invalid field %s", err, c.field)
			case c.field == "" && c.in != nil && *c.in == "  Kabel  " && f.Name != "Kabel":
				t.Fatalf("name = %q, want trimmed", f.Name)
			}
		})
	}
}
