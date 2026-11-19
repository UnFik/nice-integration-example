package deviceusecase

import (
	"context"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/app/device/internal/failure"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// ListDevices returns the devices of the project of the Service Account, or a Failure that says why not.
func (s *Service) ListDevices(ctx context.Context) ([]domain.Device, *devicedto.Failure) {
	devices, err := s.Gateway.ListDevices(ctx)
	if err != nil {
		return nil, failure.Explain(err)
	}
	return devices, nil
}
