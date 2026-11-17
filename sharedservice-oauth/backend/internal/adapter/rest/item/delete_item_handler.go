package item

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/item/internal/failure"
)

// @Summary		Delete an item
// @Description	Sensitive: besides the permission in the token, every call is checked online with NICE. A revoked access is refused with 403 access_denied, and 503 is answered when the check cannot be made.
// @ID				deleteItem
// @Tags			items
// @Security		BearerAuth
// @Param			id	path	string	true	"Item ID"
// @Success		204
// @Failure		401	{object}	restdto.ErrorBody
// @Failure		403	{object}	restdto.ErrorBody
// @Failure		404	{object}	restdto.ErrorBody
// @Failure		503	{object}	restdto.ErrorBody
// @Router			/items/{id} [delete]
// @x-nice-permission	"inventaris.items.delete"
// @x-nice-sensitive	true
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	if err := h.Items.DeleteItem(r.Context(), sc, r.PathValue("id")); err != nil {
		failure.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
