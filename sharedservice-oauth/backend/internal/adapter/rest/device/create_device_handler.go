package device

import (
	"net/http"

	devicerestdto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/decode"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/device/internal/failure"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/adapter/rest/internal/reply"
)

// @Summary		Create a device
// @Description	The name is 1 to 120 characters and the serial_number 1 to 64 characters after trimming; the status is active, inactive or maintenance; the location is optional, at most 120 characters. An invalid field is named in error_description. created_by is the user of the token; when a backend exchanged the user's token (the token carries act), acted_by is that backend's client.
// @ID				createDevice
// @Tags			devices
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			device	body		devicerestdto.DeviceInput	true	"New device"
// @Success		201		{object}	devicerestdto.Device
// @Failure		400		{object}	restdto.ErrorBody
// @Failure		401		{object}	restdto.ErrorBody
// @Failure		403		{object}	restdto.ErrorBody
// @Failure		503		{object}	restdto.ErrorBody
// @Router			/devices [post]
// @x-nice-permission	"nadi.devices.create"
func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
	sc, ok := reply.ScopeOf(w, r)
	if !ok {
		return
	}
	in, bad := decode.DeviceInput(w, r)
	if bad != "" {
		reply.Error(w, http.StatusBadRequest, "invalid_argument", bad)
		return
	}
	it, err := h.Devices.CreateDevice(r.Context(), sc, in)
	if err != nil {
		failure.Write(w, err)
		return
	}
	w.Header().Set("Location", "/devices/"+it.ID)
	reply.JSON(w, http.StatusCreated, devicerestdto.NewDevice(it))
}
