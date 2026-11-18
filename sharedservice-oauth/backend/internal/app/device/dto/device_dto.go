// Package devicedto holds the data that crosses the boundary of the device use cases.
package devicedto

// DeviceInput is a create or replace request as received: a nil field was left
// out. The use case validates it.
type DeviceInput struct {
	Name         *string
	SerialNumber *string
	Status       *string
	Location     *string
}
