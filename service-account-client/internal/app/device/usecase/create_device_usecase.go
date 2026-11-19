package deviceusecase

import (
	"context"
	"strings"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/app/device/internal/failure"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// CreateDevice registers one device. Input nadi would refuse anyway is refused here, before a token is asked for.
func (s *Service) CreateDevice(ctx context.Context, in devicedto.CreateInput) (domain.Device, *devicedto.Failure) {
	in.Name, in.SerialNumber, in.Location = strings.TrimSpace(in.Name), strings.TrimSpace(in.SerialNumber), strings.TrimSpace(in.Location)
	switch {
	case in.Name == "":
		return domain.Device{}, failure.Input("name", "a name is required")
	case in.SerialNumber == "":
		return domain.Device{}, failure.Input("serial_number", "a serial number is required")
	case !domain.ValidStatus(in.Status):
		return domain.Device{}, failure.Input("status", "the status must be one of "+strings.Join(domain.DeviceStatuses, ", "))
	}
	device, err := s.Gateway.CreateDevice(ctx, in)
	if err != nil {
		return domain.Device{}, failure.Explain(err)
	}
	return device, nil
}
