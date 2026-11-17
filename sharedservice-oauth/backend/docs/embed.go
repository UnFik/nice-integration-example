// Package docs embeds the OpenAPI document that `make swag` generates.
package docs

import _ "embed"

// OpenAPI is docs/swagger.json, served at /openapi.json and imported by NICE.
//
//go:embed swagger.json
var OpenAPI []byte
