package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	devicedto "github.com/nice-platform/example/service-account-client/internal/app/device/dto"

	"github.com/nice-platform/example/service-account-client/internal/adapter/cli/internal/mapper"
)

// CreateDevice registers one device and prints its id.
func (h *Handler) CreateDevice(ctx context.Context, args []string) int {
	var in devicedto.CreateInput
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&in.Name, "name", "", "name of the device")
	fs.StringVar(&in.SerialNumber, "serial", "", "serial number of the device")
	fs.StringVar(&in.Status, "status", "active", "active, inactive or maintenance")
	fs.StringVar(&in.Location, "location", "", "where the device is (optional)")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		_, _ = fmt.Fprintln(h.Err, "usage: create -name NAME -serial SERIAL [-status STATUS] [-location LOCATION]")
		return ExitUsage
	}
	device, failed := h.Devices.CreateDevice(ctx, in)
	if failed != nil {
		mapper.WriteFailure(h.Err, failed)
		return ExitFailure
	}
	_, _ = fmt.Fprintln(h.Out, device.ID)
	return ExitOK
}
