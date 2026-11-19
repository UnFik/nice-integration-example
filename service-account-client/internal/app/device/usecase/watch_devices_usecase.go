package deviceusecase

import (
	"context"
	"log/slog"
	"time"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/app/device/internal/failure"
)

// WatchDevices reads the devices every interval and hands each reading to emit, until ctx ends. A temporary failure
// is handed to emit and the watch goes on. A failure that waiting cannot cure, such as a refused API key, ends the
// watch and is returned. It returns nil when ctx ended.
func (s *Service) WatchDevices(ctx context.Context, every time.Duration, emit func(devicedto.Reading)) *devicedto.Failure {
	for {
		devices, err := s.Gateway.ListDevices(ctx)
		switch {
		case ctx.Err() != nil:
			// The operator stopped the watch; a call it interrupted is not a failure.
			return nil
		case err != nil:
			f := failure.Explain(err)
			if !f.Temporary {
				return f
			}
			s.Log.Warn("reading failed, trying again", slog.String("code", f.Code), slog.Duration("in", every))
			emit(devicedto.Reading{Failure: f})
		default:
			emit(devicedto.Reading{Devices: devices})
		}
		if s.Waiter.Wait(ctx, every) != nil {
			return nil
		}
	}
}
