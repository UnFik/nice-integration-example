// Package itemrestdto holds the JSON bodies of the item REST API.
package itemrestdto

import (
	"time"

	"github.com/nice-platform/example/shared-service-oauth/internal/domain"
)

type Item struct {
	ID        string `json:"id" example:"itm_3f2a9c0d1e4b5a6f7081920a3b4c5d6e"`
	Name      string `json:"name" example:"Kabel UTP Cat6"`
	Quantity  int64  `json:"quantity" example:"12"`
	CreatedBy string `json:"created_by" example:"usr_01a1..."`
	// ActedBy is act.sub of the token that created the item: the client of a backend acting for CreatedBy.
	ActedBy    string    `json:"acted_by,omitempty" example:"cli_01a1..."`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
} // @name Item

type ItemInput struct {
	Name     *string `json:"name" binding:"required" minLength:"1" maxLength:"120" example:"Kabel UTP Cat6"`
	Quantity *int64  `json:"quantity" binding:"required" minimum:"0" example:"12"`
} // @name ItemInput

type ItemList struct {
	Items []Item `json:"items"`
} // @name ItemList

func NewItem(it domain.Item) Item {
	return Item{
		ID: it.ID, Name: it.Name, Quantity: it.Quantity, CreatedBy: it.CreatedBy, ActedBy: it.ActedBy,
		CreateTime: it.CreateTime, UpdateTime: it.UpdateTime,
	}
}

func NewItemList(items []domain.Item) ItemList {
	out := ItemList{Items: make([]Item, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, NewItem(it))
	}
	return out
}
