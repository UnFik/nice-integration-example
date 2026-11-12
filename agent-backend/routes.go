package main

import (
	_ "embed"
	"net/http"
)

// serviceSlug is the slug registered in the NICE catalog. Tokens for the agent carry it as aud.
const serviceSlug = "agen"

const (
	permUse = serviceSlug + ".mcp.use"
	mcpPath = "/mcp"
)

// openAPIDoc is the catalog document of agen, written by hand: it has one operation.
//
//go:embed openapi.json
var openAPIDoc []byte

func serveOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(openAPIDoc)
}
