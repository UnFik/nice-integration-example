// Package decode reads the body of an item request.
package decode

import (
	"encoding/json"
	"errors"
	"net/http"

	itemrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/dto"
	itemdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/item/dto"
)

const maxBodyBytes = 64 << 10

// ItemInput reads a single JSON object of known fields. bad names the field
// whose type is wrong, or "body" when the body is not such an object; the
// business rules on the values are the use case's.
func ItemInput(w http.ResponseWriter, r *http.Request) (in itemdto.ItemInput, bad string) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	var body itemrestdto.ItemInput
	if err := dec.Decode(&body); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return itemdto.ItemInput{}, typeErr.Field
		}
		return itemdto.ItemInput{}, "body"
	}
	if dec.More() {
		return itemdto.ItemInput{}, "body"
	}
	return itemdto.ItemInput{Name: body.Name, Quantity: body.Quantity}, ""
}
