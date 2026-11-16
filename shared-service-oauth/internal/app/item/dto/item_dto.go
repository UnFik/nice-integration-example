// Package itemdto holds the data that crosses the boundary of the item use cases.
package itemdto

// ItemInput is a create or replace request as received: a nil field was left
// out. The use case validates it.
type ItemInput struct {
	Name     *string
	Quantity *int64
}
