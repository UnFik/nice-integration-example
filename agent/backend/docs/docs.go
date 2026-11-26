// Package docs holds the catalog document of agen. It is written by hand: it has one operation.
package docs

import _ "embed"

// OpenAPI is the document NICE imports into its catalog and the agent serves at /openapi.json.
//
//go:embed openapi.json
var OpenAPI []byte
