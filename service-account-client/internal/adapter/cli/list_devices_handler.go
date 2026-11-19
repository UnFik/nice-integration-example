package cli

import (
	"context"
	"fmt"

	"github.com/nice-platform/example/service-account-client/internal/adapter/cli/internal/mapper"
)

// ListDevices prints the devices as a table.
func (h *Handler) ListDevices(ctx context.Context, args []string) int {
	if len(args) > 0 {
		_, _ = fmt.Fprintln(h.Err, "list takes no arguments")
		return ExitUsage
	}
	devices, failed := h.Devices.ListDevices(ctx)
	if failed != nil {
		mapper.WriteFailure(h.Err, failed)
		return ExitFailure
	}
	mapper.WriteDevices(h.Out, devices)
	return ExitOK
}
