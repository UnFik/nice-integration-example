package item

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	itemrestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/failure"
)

// @Summary		List items
// @Description	Items of the organization and project of the token (nice_org_id and prj), oldest first. A token without prj is refused with 403 project_required.
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
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	items, err := h.Items.ListItems(r.Context(), sc)
	if err != nil {
		failure.Write(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, itemrestdto.NewItemList(items))
}
