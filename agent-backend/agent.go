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
}

func newAgent(cfg Config, hc *http.Client, log *slog.Logger) (*agent, error) {
	x, err := niceclient.NewExchange(niceclient.ExchangeConfig{
		Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, HTTP: hc, Log: log,
	})
	if err != nil {
		return nil, err
	}
	a := &agent{x: x, log: log}
	a.server = mcp.NewServer(&mcp.Implementation{Name: "agen", Title: "Agen (NICE example)", Version: "1.0.0"}, nil)
	return a, nil
}
