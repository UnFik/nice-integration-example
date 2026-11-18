package deviceusecase

import (
	"context"

	devicedto "github.com/nice-platform/example/sharedservice-oauth/backend/internal/app/device/dto"
	"github.com/nice-platform/example/sharedservice-oauth/backend/internal/domain"
)

// CreateDevice validates in and stores the device in s. The user (s.Subject) stays
// the creator; an agent that exchanged the user's token is recorded beside it
// as s.Actor.
func (s *Service) CreateDevice(ctx context.Context, sc domain.Scope, in devicedto.DeviceInput) (domain.Device, error) {
	if err := sc.Validate(); err != nil {
		return domain.Device{}, err
	}
	f, err := domain.NewDeviceFields(in.Name, in.SerialNumber, in.Status, in.Location)
	if err != nil {
		return domain.Device{}, err
	}
	return s.Repo.Create(ctx, sc, f)
}
