package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxItemNameLength = 120

// Item is a stock item of one organization.
type Item struct {
	ID        string
	Name      string
	Quantity  int64
	CreatedBy string
	// ActedBy is act.sub of the token that created the item: the client of a backend acting for CreatedBy.
	ActedBy    string
	CreateTime time.Time
	UpdateTime time.Time
}

// ItemFields are the fields a client may set on an item.
type ItemFields struct {
	Name     string
	Quantity int64
}

var ErrItemNotFound = errors.New("item not found")

// InvalidFieldError names the first field that broke a rule.
type InvalidFieldError struct{ Field string }

func (e *InvalidFieldError) Error() string { return "invalid field: " + e.Field }

// NewItemFields validates the fields of a create or replace request. A nil
// pointer is a field the client left out. The name is trimmed and counts 1 to
// MaxItemNameLength characters (not bytes); the quantity is at least 0.
func NewItemFields(name *string, quantity *int64) (ItemFields, error) {
	if name == nil {
		return ItemFields{}, &InvalidFieldError{Field: "name"}
	}
	n := strings.TrimSpace(*name)
	if l := utf8.RuneCountInString(n); l < 1 || l > MaxItemNameLength {
		return ItemFields{}, &InvalidFieldError{Field: "name"}
	}
	if quantity == nil || *quantity < 0 {
		return ItemFields{}, &InvalidFieldError{Field: "quantity"}
	}
	return ItemFields{Name: n, Quantity: *quantity}, nil
}
