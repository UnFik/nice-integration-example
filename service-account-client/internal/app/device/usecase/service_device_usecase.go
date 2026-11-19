// Package deviceusecase reads and registers devices of nadi as a Service Account.
package deviceusecase

import "log/slog"

// Service runs the device use cases.
type Service struct {
	Gateway Gateway
	Waiter  Waiter
	Log     *slog.Logger
}
