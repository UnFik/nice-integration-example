package item

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	itemrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/decode"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/failure"
)

// @Summary		Replace an item
// @Description	Same rules as creating an item.
// @ID				updateItem
// @Tags			items
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			id		path		string					true	"Item ID"
// @Param			item	body		itemrestdto.ItemInput	true	"New name and quantity"
// @Success		200		{object}	itemrestdto.Item
// @Failure		400		{object}	restdto.ErrorBody
// @Failure		401		{object}	restdto.ErrorBody
// @Failure		403		{object}	restdto.ErrorBody
// @Failure		404		{object}	restdto.ErrorBody
// @Failure		503		{object}	restdto.ErrorBody
// @Router			/items/{id} [put]
// @x-nice-permission	"inventaris.items.edit"
func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	in, bad := decode.ItemInput(w, r)
	if bad != "" {
		reply.Error(w, http.StatusBadRequest, "invalid_argument", bad)
		return
	}
	it, err := h.Items.UpdateItem(r.Context(), sc, r.PathValue("id"), in)
	if err != nil {
		failure.Write(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, itemrestdto.NewItem(it))
}
