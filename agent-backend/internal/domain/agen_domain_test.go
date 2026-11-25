package domain

import "testing"

func TestValidServiceSlug(t *testing.T) {
	for s, want := range map[string]bool{
		"nadi": true, "pengiriman": true, "a": true, "svc-2": true,
		"": false, "Nadi": false, "nadi x": false, "nadi/ops": false, "-nadi": false, "2nadi": false,
		"nadi nice:aud:nice": false, "../nice": false,
	} {
		if got := ValidServiceSlug(s); got != want {
			t.Errorf("ValidServiceSlug(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestSecureEndpoint(t *testing.T) {
	for e, want := range map[string]bool{
		"https://nadi.example.com": true, "http://127.0.0.1:8081": true, "http://localhost:8081": true,
		"http://[::1]:8081": true, "http://nadi:8081": false, "http://10.0.0.5": false, "ftp://nadi": false,
		"": false, "nadi:8081": false,
	} {
		if got := SecureEndpoint(e); got != want {
			t.Errorf("SecureEndpoint(%q) = %v, want %v", e, got, want)
		}
	}
}
