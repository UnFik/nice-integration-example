// Package decode reads the body of a device request.
package decode

import (
	"encoding/json"
	"errors"
	"net/http"

	devicerestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/dto"
	devicedto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/dto"
)

const maxBodyBytes = 64 << 10

// DeviceInput reads a single JSON object of known fields. bad names the field
// whose type is wrong, or "body" when the body is not such an object; the
// business rules on the values are the use case's.
func DeviceInput(w http.ResponseWriter, r *http.Request) (in devicedto.DeviceInput, bad string) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	var body devicerestdto.DeviceInput
	if err := dec.Decode(&body); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return devicedto.DeviceInput{}, typeErr.Field
		}
		return devicedto.DeviceInput{}, "body"
	}
	if dec.More() {
		return devicedto.DeviceInput{}, "body"
	}
	return devicedto.DeviceInput{Name: body.Name, SerialNumber: body.SerialNumber, Status: body.Status, Location: body.Location}, ""
}
