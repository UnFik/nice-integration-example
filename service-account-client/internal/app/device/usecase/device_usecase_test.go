package deviceusecase

import (
	"context"
	"testing"
	"time"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"
	"github.com/nice-platform/example/service-account-client/internal/domain"
)

var twoDevices = []domain.Device{{ID: "dev_1", Name: "Sensor"}, {ID: "dev_2", Name: "Gateway"}}

func TestListDevices(t *testing.T) {
	s := newService(&fakeGateway{lists: []listResult{{devices: twoDevices}}}, nil)
	devices, failed := s.ListDevices(context.Background())
	if failed != nil || len(devices) != 2 || devices[1].ID != "dev_2" {
		t.Fatalf("devices = %v, failed = %v", devices, failed)
	}
}

func TestListDevicesExplainsTheFailure(t *testing.T) {
	s := newService(&fakeGateway{lists: []listResult{{err: &domain.ServiceError{Status: 403, Code: "insufficient_scope", Description: "permission_denied"}}}}, nil)
	_, failed := s.ListDevices(context.Background())
	if failed == nil || failed.Description != "permission_denied" || failed.Step == "" {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestCreateDevice(t *testing.T) {
	g := &fakeGateway{}
	device, failed := newService(g, nil).CreateDevice(context.Background(),
		devicedto.CreateInput{Name: "  Sensor Suhu ", SerialNumber: "SN-1", Status: "active", Location: "Gudang A"})
	if failed != nil || device.ID != "dev_1" {
		t.Fatalf("device = %+v, failed = %+v", device, failed)
	}
	if len(g.created) != 1 || g.created[0].Name != "Sensor Suhu" || g.created[0].Location != "Gudang A" {
		t.Fatalf("sent = %+v", g.created)
	}
}

func TestCreateDeviceRefusesBadInputBeforeAnyCall(t *testing.T) {
	for field, in := range map[string]devicedto.CreateInput{
		"name":          {SerialNumber: "SN-1", Status: "active"},
		"serial_number": {Name: "Sensor", Status: "active"},
		"status":        {Name: "Sensor", SerialNumber: "SN-1", Status: "broken"},
	} {
		g := &fakeGateway{}
		_, failed := newService(g, nil).CreateDevice(context.Background(), in)
		if failed == nil || failed.Code != "invalid_argument" || failed.Description != field || len(g.created) != 0 {
			t.Errorf("%s: failed = %+v, calls = %d", field, failed, len(g.created))
		}
	}
}

func TestCreateDeviceExplainsTheFailure(t *testing.T) {
	g := &fakeGateway{create: &domain.GrantError{Code: "invalid_client", Permanent: true}}
	_, failed := newService(g, nil).CreateDevice(context.Background(), devicedto.CreateInput{Name: "Sensor", SerialNumber: "SN-1", Status: "active"})
	if failed == nil || failed.Code != "invalid_client" || failed.Temporary {
		t.Fatalf("failed = %+v", failed)
	}
}

func TestWatchDevicesGoesOnAfterATemporaryFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g := &fakeGateway{lists: []listResult{{devices: twoDevices}, {err: domain.ErrUnreachable}, {err: domain.ErrTemporary}, {devices: twoDevices[:1]}}}
	w := &fakeWaiter{stop: 4, after: cancel}
	var readings []devicedto.Reading
	failed := newService(g, w).WatchDevices(ctx, 7*time.Second, func(r devicedto.Reading) { readings = append(readings, r) })
	if failed != nil {
		t.Fatalf("failed = %+v", failed)
	}
	if len(readings) != 4 || len(readings[0].Devices) != 2 || readings[1].Failure == nil || readings[1].Failure.Code != "service_unavailable" ||
		readings[2].Failure == nil || readings[2].Failure.Code != "nice_unavailable" || len(readings[3].Devices) != 1 {
		t.Fatalf("readings = %+v", readings)
	}
	if len(w.waits) != 4 || w.waits[0] != 7*time.Second {
		t.Fatalf("waits = %v", w.waits)
	}
}

func TestWatchDevicesStopsWhenTheKeyIsRefused(t *testing.T) {
	g := &fakeGateway{lists: []listResult{{devices: twoDevices}, {err: &domain.GrantError{Code: "invalid_client", Permanent: true}}}}
	w := &fakeWaiter{}
	var readings int
	failed := newService(g, w).WatchDevices(context.Background(), time.Second, func(devicedto.Reading) { readings++ })
	if failed == nil || failed.Code != "invalid_client" || readings != 1 || g.calls != 2 {
		t.Fatalf("failed = %+v, readings = %d, calls = %d", failed, readings, g.calls)
	}
}

func TestWatchDevicesEndsWithoutFailureWhenInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The interrupted call fails, as a call cut off by the signal does: that is not a failure of the watch.
	g := &fakeGateway{lists: []listResult{{err: domain.ErrUnreachable}}}
	var readings int
	if failed := newService(g, &fakeWaiter{}).WatchDevices(ctx, time.Second, func(devicedto.Reading) { readings++ }); failed != nil || readings != 0 {
		t.Fatalf("failed = %+v, readings = %d", failed, readings)
	}
}
