package item

import (
	"net/http"

	"github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest/internal/reply"
	itemrestdto "github.com/nice-platform/example/shared-service-oauth/internal/adapter/rest/item/dto"
)

// @Summary		List items
// @Description	Items of the organization of the token, oldest first.
// @ID				listItems
// @Tags			items
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	itemrestdto.ItemList
// @Failure		401	{object}	restdto.ErrorBody
// @Failure		403	{object}	restdto.ErrorBody
// @Failure		503	{object}	restdto.ErrorBody
// @Router			/items [get]
// @x-nice-permission	"inventaris.items.read"
func (h *Handler) ListItems(w http.ResponseWriter, r *http.Request) {
	org, ok := reply.TenantOf(w, r)
	if !ok {
		return
	}
	reply.JSON(w, http.StatusOK, itemrestdto.NewItemList(h.Items.ListItems(org)))
}
