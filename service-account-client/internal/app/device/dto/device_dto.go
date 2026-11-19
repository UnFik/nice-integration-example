// Package devicedto holds the data that crosses the boundary of the device use cases.
package devicedto

import "github.com/nice-platform/example/service-account-client/internal/domain"

// CreateInput are the fields of a device to register.
type CreateInput struct {
	Name         string
	SerialNumber string
	Status       string
	Location     string
}

// Reading is one round of watch: the devices, or the temporary failure that kept them from being read.
type Reading struct {
	Devices []domain.Device
	Failure *Failure
}
