// Package challenge adds the resource metadata to the 401 and 403 challenges of niceauth/verify.
package challenge

import "net/http"

// With adds the resource metadata to the challenges of niceauth/verify, so an MCP client that gets 401 or 403
// finds the authorization server (RFC 9728, section 5.1).
func With(metadataURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&challengeWriter{ResponseWriter: w, metadata: `resource_metadata="` + metadataURL + `"`}, r)
	})
}

type challengeWriter struct {
	http.ResponseWriter
	metadata    string
	wroteHeader bool
}

func (c *challengeWriter) WriteHeader(status int) {
	if !c.wroteHeader {
		c.wroteHeader = true
		h := c.Header()
		switch status {
		case http.StatusUnauthorized:
			if v := h.Get("WWW-Authenticate"); v == "" || v == "Bearer" {
				h.Set("WWW-Authenticate", "Bearer "+c.metadata)
			} else {
				h.Set("WWW-Authenticate", v+", "+c.metadata)
			}
		case http.StatusForbidden:
			h.Set("WWW-Authenticate", `Bearer error="insufficient_scope", `+c.metadata)
		}
	}
	c.ResponseWriter.WriteHeader(status)
}

func (c *challengeWriter) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.ResponseWriter.Write(b)
}

// Flush keeps the streaming of the MCP handler working through the wrapper.
func (c *challengeWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (c *challengeWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
