package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/nice-platform/nice/pkg/nicetoken"
)

const (
	maxNameLength = 120
	maxBodyBytes  = 64 << 10
)

type itemInput struct {
	Name     *string `json:"name" binding:"required" minLength:"1" maxLength:"120" example:"Kabel UTP Cat6"`
	Quantity *int64  `json:"quantity" binding:"required" minimum:"0" example:"12"`
} // @name ItemInput

type itemList struct {
	Items []Item `json:"items"`
} // @name ItemList

// errorBody has the shape nicetoken uses for its own refusals, so a client
// reads every error of the service the same way.
type errorBody struct {
	Error            string `json:"error" example:"invalid_argument"`
	ErrorDescription string `json:"error_description,omitempty" example:"name"`
} // @name Error

type api struct {
	store *Store
}

// @Summary		List items
// @Description	Items of the organization of the token, oldest first.
// @ID				listItems
// @Tags			items
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	itemList
// @Failure		401	{object}	errorBody
// @Failure		403	{object}	errorBody
// @Failure		503	{object}	errorBody
// @Router			/items [get]
// @x-nice-permission	"inventaris.items.read"
func (a *api) listItems(w http.ResponseWriter, r *http.Request) {
	org, ok := tenantOf(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, itemList{Items: a.store.List(org)})
}

// @Summary		Get an item
// @ID				getItem
// @Tags			items
// @Produce		json
// @Security		BearerAuth
// @Param			id	path		string	true	"Item ID"
// @Success		200	{object}	Item
// @Failure		401	{object}	errorBody
// @Failure		403	{object}	errorBody
// @Failure		404	{object}	errorBody
// @Failure		503	{object}	errorBody
// @Router			/items/{id} [get]
// @x-nice-permission	"inventaris.items.read"
func (a *api) getItem(w http.ResponseWriter, r *http.Request) {
	org, ok := tenantOf(w, r)
	if !ok {
		return
	}
	it, found := a.store.Get(org, r.PathValue("id"))
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeJSON(w, http.StatusOK, it)
}

// @Summary		Create an item
// @Description	The name is 1 to 120 characters after trimming; the quantity is an integer of at least 0. An invalid field is named in error_description. created_by is the user of the token; when a backend exchanged the user's token (the token carries act), acted_by is that backend's client.
// @ID				createItem
// @Tags			items
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			item	body		itemInput	true	"New item"
// @Success		201		{object}	Item
// @Failure		400		{object}	errorBody
// @Failure		401		{object}	errorBody
// @Failure		403		{object}	errorBody
// @Failure		503		{object}	errorBody
// @Router			/items [post]
// @x-nice-permission	"inventaris.items.create"
func (a *api) createItem(w http.ResponseWriter, r *http.Request) {
	org, ok := tenantOf(w, r)
	if !ok {
		return
	}
	name, quantity, bad := decodeItem(w, r)
	if bad != "" {
		writeError(w, http.StatusBadRequest, "invalid_argument", bad)
		return
	}
	claims, _ := nicetoken.FromContext(r.Context())
	// The user stays the creator; an agent that exchanged the user's token is recorded beside it.
	it := a.store.Create(org, name, quantity, claims.Subject, claims.Actor)
	w.Header().Set("Location", "/items/"+it.ID)
	writeJSON(w, http.StatusCreated, it)
}

// @Summary		Replace an item
// @Description	Same rules as creating an item.
// @ID				updateItem
// @Tags			items
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			id		path		string		true	"Item ID"
// @Param			item	body		itemInput	true	"New name and quantity"
// @Success		200		{object}	Item
// @Failure		400		{object}	errorBody
// @Failure		401		{object}	errorBody
// @Failure		403		{object}	errorBody
// @Failure		404		{object}	errorBody
// @Failure		503		{object}	errorBody
// @Router			/items/{id} [put]
// @x-nice-permission	"inventaris.items.edit"
func (a *api) updateItem(w http.ResponseWriter, r *http.Request) {
	org, ok := tenantOf(w, r)
	if !ok {
		return
	}
	name, quantity, bad := decodeItem(w, r)
	if bad != "" {
		writeError(w, http.StatusBadRequest, "invalid_argument", bad)
		return
	}
	it, found := a.store.Update(org, r.PathValue("id"), name, quantity)
	if !found {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	writeJSON(w, http.StatusOK, it)
}

// @Summary		Delete an item
// @Description	Sensitive: besides the permission in the token, every call is checked online with NICE. A revoked access is refused with 403 access_denied, and 503 is answered when the check cannot be made.
// @ID				deleteItem
// @Tags			items
// @Security		BearerAuth
// @Param			id	path	string	true	"Item ID"
// @Success		204
// @Failure		401	{object}	errorBody
// @Failure		403	{object}	errorBody
// @Failure		404	{object}	errorBody
// @Failure		503	{object}	errorBody
// @Router			/items/{id} [delete]
// @x-nice-permission	"inventaris.items.delete"
// @x-nice-sensitive	true
func (a *api) deleteItem(w http.ResponseWriter, r *http.Request) {
	org, ok := tenantOf(w, r)
	if !ok {
		return
	}
	if !a.store.Delete(org, r.PathValue("id")) {
		writeError(w, http.StatusNotFound, "not_found", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tenantOf returns the tenant nicetoken resolved from nice_org_id. The tenant
// never comes from a header, parameter or body of the client.
func tenantOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	org, ok := nicetoken.TenantFromContext(r.Context())
	if !ok || org == "" {
		// Fail closed: without a tenant every organization would share one set of items.
		writeError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "tenant_unavailable")
		return "", false
	}
	return org, true
}

// decodeItem reads and validates an itemInput. bad names the first invalid
// field, or "body" when the body is not a single JSON object of known fields.
func decodeItem(w http.ResponseWriter, r *http.Request) (name string, quantity int64, bad string) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	var in itemInput
	if err := dec.Decode(&in); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return "", 0, typeErr.Field
		}
		return "", 0, "body"
	}
	if dec.More() {
		return "", 0, "body"
	}
	if in.Name == nil {
		return "", 0, "name"
	}
	name = strings.TrimSpace(*in.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxNameLength {
		return "", 0, "name"
	}
	if in.Quantity == nil || *in.Quantity < 0 {
		return "", 0, "quantity"
	}
	return name, *in.Quantity, ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, errorBody{Error: code, ErrorDescription: description})
}
