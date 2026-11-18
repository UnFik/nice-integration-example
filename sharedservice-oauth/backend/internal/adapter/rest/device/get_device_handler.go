package device

import (
	"net/http"

	devicerestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/failure"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// @Summary		Get a device
// @ID				getDevice
// @Tags			devices
// @Produce		json
// @Security		BearerAuth
// @Param			id	path		string	true	"Device ID"
// @Success		200	{object}	devicerestdto.Device
// @Failure		401	{object}	restdto.ErrorBody
// @Failure		403	{object}	restdto.ErrorBody
// @Failure		404	{object}	restdto.ErrorBody
// @Failure		503	{object}	restdto.ErrorBody
// @Router			/devices/{id} [get]
// @x-nice-permission	"nadi.devices.read"
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	it, err := h.Devices.GetDevice(r.Context(), sc, r.PathValue("id"))
	if err != nil {
		failure.Write(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, devicerestdto.NewDevice(it))
}
