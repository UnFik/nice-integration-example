package deviceusecase

import (
	"context"
	"log/slog"
	"time"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

// fakeGateway answers each call with the next scripted result; the last one repeats.
type fakeGateway struct {
	lists   []listResult
	calls   int
	created []devicedto.CreateInput
	create  error
}

type listResult struct {
	devices []domain.Device
	err     error
}

func (g *fakeGateway) ListDevices(context.Context) ([]domain.Device, error) {
	r := g.lists[min(g.calls, len(g.lists)-1)]
	g.calls++
	return r.devices, r.err
}

func (g *fakeGateway) CreateDevice(_ context.Context, in devicedto.CreateInput) (domain.Device, error) {
	g.created = append(g.created, in)
	if g.create != nil {
		return domain.Device{}, g.create
	}
	return domain.Device{ID: "dev_1", Name: in.Name, SerialNumber: in.SerialNumber, Status: in.Status, Location: in.Location}, nil
}

// fakeWaiter does not wait. It runs after once it was called that many times, which is how a test stops a watch.
type fakeWaiter struct {
	waits []time.Duration
	stop  int
	after func()
}

func (w *fakeWaiter) Wait(ctx context.Context, d time.Duration) error {
	w.waits = append(w.waits, d)
	if len(w.waits) == w.stop {
		w.after()
	}
	return ctx.Err()
}

func newService(g Gateway, w Waiter) *Service {
	return &Service{Gateway: g, Waiter: w, Log: slog.New(slog.DiscardHandler)}
}
