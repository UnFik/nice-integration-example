package device

import (
	"net/http"

	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/failure"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// @Summary		Delete a device
// @Description	Sensitive: besides the permission in the token, every call is checked online with NICE. A revoked access is refused with 403 access_denied, and 503 is answered when the check cannot be made.
// @ID				deleteDevice
// @Tags			devices
// @Security		BearerAuth
// @Param			id	path	string	true	"Device ID"
// @Success		204
// @Failure		401	{object}	restdto.ErrorBody
// @Failure		403	{object}	restdto.ErrorBody
// @Failure		404	{object}	restdto.ErrorBody
// @Failure		503	{object}	restdto.ErrorBody
// @Router			/devices/{id} [delete]
// @x-nice-permission	"nadi.devices.delete"
// @x-nice-sensitive	true
func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	if err := h.Devices.DeleteDevice(r.Context(), sc, r.PathValue("id")); err != nil {
		failure.Write(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
