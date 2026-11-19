package nadi

import (
	"context"
	"net/http"

	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// ListDevices reads the devices of the project of the Service Account.
func (g *Gateway) ListDevices(ctx context.Context) ([]domain.Device, error) {
	var out struct {
		Devices []device `json:"devices"`
	}
	if err := g.send(ctx, http.MethodGet, "/devices", nil, &out); err != nil {
		return nil, err
	}
	devices := make([]domain.Device, 0, len(out.Devices))
	for _, d := range out.Devices {
		devices = append(devices, d.domain())
	}
	return devices, nil
}
