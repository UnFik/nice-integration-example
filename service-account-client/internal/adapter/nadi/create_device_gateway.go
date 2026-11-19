package nadi

import (
	"context"
	"net/http"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// CreateDevice registers one device in the project of the Service Account.
func (g *Gateway) CreateDevice(ctx context.Context, in devicedto.CreateInput) (domain.Device, error) {
	body := map[string]string{"name": in.Name, "serial_number": in.SerialNumber, "status": in.Status}
	if in.Location != "" {
		body["location"] = in.Location
	}
	var out device
	if err := g.send(ctx, http.MethodPost, "/devices", body, &out); err != nil {
		return domain.Device{}, err
	}
	if out.ID == "" {
		return domain.Device{}, domain.ErrBadAnswer
	}
	return out.domain(), nil
}
