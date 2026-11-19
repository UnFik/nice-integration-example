// Package bootstrap assembles the client: configuration, the NICE SDK, the gateway to nadi, the device use cases,
// and the command line. It is the only place that knows every layer.
package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/nice-platform/nice/pkg/niceauth/credentials"

	"github.com/nice-platform/example/service-account-client/internal/adapter/cli"
	"github.com/nice-platform/example/service-account-client/internal/adapter/nadi"
	deviceusecase "github.com/nice-platform/example/service-account-client/internal/app/device/usecase"
	"github.com/nice-platform/example/service-account-client/internal/domain"
	"github.com/nice-platform/example/service-account-client/internal/platform/config"
)

// Options replace parts of the assembly. The zero value is what the program runs with; tests set a clock and
// short intervals so that a renewal does not take ten minutes.
type Options struct {
	HTTP   *http.Client
	Waiter deviceusecase.Waiter
	// Now, Tick, RefreshAfter and Jitter go to credentials.Config as they are.
	Now          func() time.Time
	Tick         time.Duration
	RefreshAfter time.Duration
	Jitter       time.Duration
}

// Run executes the command args names and returns the exit code.
func Run(ctx context.Context, args []string, lookup config.LookupFunc, dotenv string, stdout, stderr io.Writer, log *slog.Logger) int {
	return RunWith(ctx, args, lookup, dotenv, stdout, stderr, log, Options{})
}

// RunWith is Run with parts of the assembly replaced.
func RunWith(ctx context.Context, args []string, lookup config.LookupFunc, dotenv string, stdout, stderr io.Writer, log *slog.Logger, o Options) int {
	// A mistyped command is refused before any setting is read, so the usage shows without a .env.
	if !cli.Known(args) {
		cli.Usage(stderr)
		return cli.ExitUsage
	}
	cfg, err := config.Load(lookup, dotenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "FAILED:", err)
		return cli.ExitFailure
	}
	client, err := NewClient(cfg, log, o)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "FAILED:", err)
		return cli.ExitFailure
	}
	// Run renews the token in the background while a command is in progress. It matters for watch only; list and
	// create end long before the first renewal.
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	go client.Run(ctx)

	log.Debug("client ready", slog.Any("config", cfg))
	h := &cli.Handler{Devices: NewDeviceService(cfg, client, log, o), Interval: cfg.WatchInterval, Out: stdout, Err: stderr}
	return h.Dispatch(ctx, args)
}

// NewClient makes the SDK client of the Service Account. KindServiceAccount makes it prove the API key with
// client_credentials; the key is given to the SDK here and to nothing else.
func NewClient(cfg config.Config, log *slog.Logger, o Options) (*credentials.Client, error) {
	hc := o.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return credentials.New(credentials.Config{
		Issuer:       cfg.Issuer,
		ClientID:     cfg.AccountID,
		ClientSecret: cfg.APIKey,
		Kind:         credentials.KindServiceAccount,
		HTTP:         hc,
		Log:          log,
		Now:          o.Now,
		Tick:         o.Tick,
		RefreshAfter: o.RefreshAfter,
		Jitter:       o.Jitter,
		// NICE refused the key for good. The SDK already stopped asking; the next call returns the refusal, which
		// ends the command. A long-running integration would alert its operator here.
		OnSessionEnded: func(subject string, _ error) {
			log.Error("NICE refused the API key for good; the client stops asking for tokens", slog.String("service_account", subject))
		},
	})
}

// NewDeviceService wires the device use cases to nadi through the SDK client. The key of the token names only the
// service: the SDK fills the id of the Service Account, and NICE issues the token for the project the account belongs
// to. The *http.Client of the key goes to the nadi gateway alone, since it sends the token to any host it calls.
func NewDeviceService(cfg config.Config, client *credentials.Client, log *slog.Logger, o Options) *deviceusecase.Service {
	w := o.Waiter
	if w == nil {
		w = timerWaiter{}
	}
	hc := client.HTTPClient(credentials.Key{Audience: domain.ServiceSlug})
	return &deviceusecase.Service{Gateway: nadi.New(cfg.NadiURL, hc), Waiter: w, Log: log}
}

type timerWaiter struct{}

func (timerWaiter) Wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
