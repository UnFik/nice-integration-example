package failure

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nice-platform/example/agent/backend/internal/domain"
)

const leaked = "eyJhbGciOiJSUzI1NiJ9.payload.sig"

func TestFailuresNeverCarryATokenOrARawBody(t *testing.T) {
	for name, f := range map[string]any{
		"service": Service("nadi", 500, []byte(`{"error":"`+leaked+`","error_description":"stack trace at line 3","debug":"raw"}`)),
		"grant":   Exchange("nadi", &domain.GrantError{Code: "invalid_grant", Description: "subject " + leaked}, "VARS"),
		"invalid": Exchange("nadi", &domain.InvalidCallError{Reason: "path parameter \"id\" holds " + leaked}, "VARS"),
		"api":     Discovery("nadi", &domain.APIError{Status: 400, Code: leaked}, "VARS"),
	} {
		raw, _ := json.Marshal(f)
		if strings.Contains(string(raw), "eyJ") || strings.Contains(string(raw), `"debug"`) {
			t.Errorf("%s: failure shows a token or the raw body: %s", name, raw)
		}
	}
}

func TestPlain(t *testing.T) {
	for in, want := range map[string]string{
		"Delete a device":         "Delete a device",
		"Hapus perangkat\n" + "x": "Hapus perangkat?x",
		"token " + leaked:         "",
		strings.Repeat("a", 130):  strings.Repeat("a", 117) + "...",
	} {
		if got := Plain(in); got != want {
			t.Errorf("Plain(%q) = %q, want %q", in, got, want)
		}
	}
}
