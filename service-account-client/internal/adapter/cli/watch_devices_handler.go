package cli

import (
	"context"
	"fmt"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"

	"github.com/nice-platform/example/service-account-client/internal/adapter/cli/internal/mapper"
)

// WatchDevices prints the devices at every interval. It ends with ExitOK when the operator interrupts it and with
// ExitFailure when a failure that waiting cannot cure, such as a refused API key, stopped it.
func (h *Handler) WatchDevices(ctx context.Context, args []string) int {
	if len(args) > 0 {
		_, _ = fmt.Fprintln(h.Err, "watch takes no arguments")
		return ExitUsage
	}
	failed := h.Devices.WatchDevices(ctx, h.Interval, func(r devicedto.Reading) {
		if r.Failure != nil {
			mapper.WriteFailure(h.Err, r.Failure)
			return
		}
		_, _ = fmt.Fprintf(h.Out, "# %d device(s)\n", len(r.Devices))
		mapper.WriteDevices(h.Out, r.Devices)
	})
	if failed != nil {
		mapper.WriteFailure(h.Err, failed)
		return ExitFailure
	}
	return ExitOK
}
