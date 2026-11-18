package device

import (
	"net/http"

	devicerestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/failure"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// @Summary		List devices
// @Description	Devices of the organization and project of the token (nice_org_id and prj), oldest first. A token without prj is refused with 403 project_required.
// @ID				listDevices
// @Tags			devices
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	devicerestdto.DeviceList
// @Failure		401	{object}	restdto.ErrorBody
// @Failure		403	{object}	restdto.ErrorBody
// @Failure		503	{object}	restdto.ErrorBody
// @Router			/devices [get]
// @x-nice-permission	"nadi.devices.read"
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	devices, err := h.Devices.ListDevices(r.Context(), sc)
	if err != nil {
		failure.Write(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, devicerestdto.NewDeviceList(devices))
}
