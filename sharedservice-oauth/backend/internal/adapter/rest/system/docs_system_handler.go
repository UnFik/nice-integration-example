package system

import "net/http"

// swaggerPage loads Swagger UI from a CDN pinned to one version, so the service needs no extra Go dependency.
// The UI reads the same /openapi.json NICE imports, from the same origin, so "Try it out" needs no CORS.
const swaggerPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Inventaris API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/openapi.json", dom_id: "#swagger-ui", persistAuthorization: false });
  </script>
</body>
</html>
`

// Docs serves the Swagger UI page. It is public and not an operation of the catalog.
func Docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerPage))
}
