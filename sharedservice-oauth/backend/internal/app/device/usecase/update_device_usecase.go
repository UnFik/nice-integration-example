package deviceusecase

import (
	"context"

	devicedto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// UpdateDevice replaces the fields of a device with the same rules as creating one.
func (s *Service) UpdateDevice(ctx context.Context, sc domain.Scope, id string, in devicedto.DeviceInput) (domain.Device, error) {
	if err := sc.Validate(); err != nil {
		return domain.Device{}, err
	}
	f, err := domain.NewDeviceFields(in.Name, in.SerialNumber, in.Status, in.Location)
	if err != nil {
		return domain.Device{}, err
	}
	return s.Repo.Update(ctx, sc, id, f)
}
