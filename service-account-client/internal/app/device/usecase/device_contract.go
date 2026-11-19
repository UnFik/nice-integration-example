package deviceusecase

import (
	"context"
	"time"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// Gateway calls nadi as the Service Account. It is the only way the use cases touch OAuth and HTTP. Its errors are
// *domain.GrantError, *domain.ServiceError, domain.ErrTemporary, domain.ErrUnreachable, or domain.ErrBadAnswer.
type Gateway interface {
	ListDevices(ctx context.Context) ([]domain.Device, error)
	CreateDevice(ctx context.Context, in devicedto.CreateInput) (domain.Device, error)
}

// Waiter pauses watch between two readings. It returns an error when ctx ends first.
type Waiter interface {
	Wait(ctx context.Context, d time.Duration) error
}
