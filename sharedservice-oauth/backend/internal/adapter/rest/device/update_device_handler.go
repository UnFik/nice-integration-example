package device

import (
	"net/http"

	devicerestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/decode"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/failure"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// @Summary		Replace a device
// @Description	Same rules as creating a device.
// @ID				updateDevice
// @Tags			devices
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			id		path		string					true	"Device ID"
// @Param			device	body		devicerestdto.DeviceInput	true	"New device fields"
// @Success		200		{object}	devicerestdto.Device
// @Failure		400		{object}	restdto.ErrorBody
// @Failure		401		{object}	restdto.ErrorBody
// @Failure		403		{object}	restdto.ErrorBody
// @Failure		404		{object}	restdto.ErrorBody
// @Failure		503		{object}	restdto.ErrorBody
// @Router			/devices/{id} [put]
// @x-nice-permission	"nadi.devices.edit"
func (h *Handler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	in, bad := decode.DeviceInput(w, r)
	if bad != "" {
		reply.Error(w, http.StatusBadRequest, "invalid_argument", bad)
		return
	}
	it, err := h.Devices.UpdateDevice(r.Context(), sc, r.PathValue("id"), in)
	if err != nil {
		failure.Write(w, err)
		return
	}
	reply.JSON(w, http.StatusOK, devicerestdto.NewDevice(it))
}
