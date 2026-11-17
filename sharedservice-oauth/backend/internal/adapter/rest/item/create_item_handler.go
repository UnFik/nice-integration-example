package item

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	itemrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/decode"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/failure"
)

// @Summary		Create an item
// @Description	The name is 1 to 120 characters after trimming; the quantity is an integer of at least 0. An invalid field is named in error_description. created_by is the user of the token; when a backend exchanged the user's token (the token carries act), acted_by is that backend's client.
// @ID				createItem
// @Tags			items
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			item	body		itemrestdto.ItemInput	true	"New item"
// @Success		201		{object}	itemrestdto.Item
// @Failure		400		{object}	restdto.ErrorBody
// @Failure		401		{object}	restdto.ErrorBody
// @Failure		403		{object}	restdto.ErrorBody
// @Failure		503		{object}	restdto.ErrorBody
// @Router			/items [post]
// @x-nice-permission	"inventaris.items.create"
func (h *Handler) CreateItem(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	in, bad := decode.ItemInput(w, r)
	if bad != "" {
		reply.Error(w, http.StatusBadRequest, "invalid_argument", bad)
		return
	}
	it, err := h.Items.CreateItem(r.Context(), sc, in)
	if err != nil {
		failure.Write(w, err)
		return
	}
	w.Header().Set("Location", "/items/"+it.ID)
	reply.JSON(w, http.StatusCreated, itemrestdto.NewItem(it))
}
