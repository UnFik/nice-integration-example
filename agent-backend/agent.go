package main

import (
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nice-platform/nice/pkg/niceclient"
)

// agent holds the MCP server and what its tools need to call other services for the user.
type agent struct {
	server *mcp.Server
	x      *niceclient.ExchangeClient
	log    *slog.Logger
	// bases is where each target is called: inventaris directly, pengiriman (an api_key service) through the proxy
	// of NICE, which checks the exchanged token and adds the service's own key.
	bases map[target]string
}

func newAgent(cfg Config, hc *http.Client, log *slog.Logger) (*agent, error) {
	x, err := niceclient.NewExchange(niceclient.ExchangeConfig{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	a := &agent{x: x, log: log, bases: map[target]string{
		inventaris: cfg.InventarisURL,
		pengiriman: cfg.ProxyURL + "/" + string(pengiriman),
	}}
	a.server = mcp.NewServer(&mcp.Implementation{Name: "agen", Title: "Agen (NICE example)", Version: "1.0.0"}, nil)
	return a, nil
}
