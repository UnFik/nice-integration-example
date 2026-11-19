// Package cli is the command line of the client: one handler per command.
package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	deviceusecase "github.com/nice-platform/example/service-account-client/internal/app/device/usecase"
)

// Exit codes of the program.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// Handler serves the commands.
type Handler struct {
	Devices *deviceusecase.Service
	// Interval is how long watch waits between two readings.
	Interval time.Duration
	// Out receives results, Err receives failures: results can be piped.
	Out io.Writer
	Err io.Writer
}

// Command is one command of the program.
type Command struct {
	Name    string
	Args    string
	Summary string
	Run     func(ctx context.Context, args []string) int
}

func (h *Handler) Routes() []Command {
	return []Command{
		{"list", "", "print the devices of the project of the Service Account", h.ListDevices},
		{"create", "-name NAME -serial SERIAL [-status STATUS] [-location LOCATION]", "register one device", h.CreateDevice},
		{"watch", "", "print the devices every WATCH_INTERVAL until interrupted", h.WatchDevices},
	}
}

// Known reports whether args names a command, so the caller can refuse a mistyped one before it reads any setting.
func Known(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, c := range (&Handler{}).Routes() {
		if c.Name == args[0] {
			return true
		}
	}
	return false
}

// Dispatch runs the command args names and returns the exit code.
func (h *Handler) Dispatch(ctx context.Context, args []string) int {
	if len(args) > 0 {
		for _, c := range h.Routes() {
			if c.Name == args[0] {
				return c.Run(ctx, args[1:])
			}
		}
	}
	Usage(h.Err)
	return ExitUsage
}

// Usage prints how to call the program.
func Usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: service-account-client <command> [flags]")
	_, _ = fmt.Fprintln(w)
	for _, c := range (&Handler{}).Routes() {
		line := c.Name
		if c.Args != "" {
			line += " " + c.Args
		}
		_, _ = fmt.Fprintf(w, "  %s\n      %s\n", line, c.Summary)
	}
}
