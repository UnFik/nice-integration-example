package item

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	itemrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/failure"
)

// @Summary		Get an item
// @ID				getItem
// @Tags			items
// @Produce		json
// @Security		BearerAuth
// @Param			id	path		string	true	"Item ID"
// @Success		200	{object}	itemrestdto.Item
// @Failure		401	{object}	restdto.ErrorBody
// @Failure		403	{object}	restdto.ErrorBody
// @Failure		404	{object}	restdto.ErrorBody
// @Failure		503	{object}	restdto.ErrorBody
// @Router			/items/{id} [get]
// @x-nice-permission	"inventaris.items.read"
func (h *Handler) GetItem(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	it, err := h.Items.GetItem(r.Context(), sc, r.PathValue("id"))
	if err != nil {
		failure.Write(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, itemrestdto.NewItem(it))
}
