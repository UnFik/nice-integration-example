package approval

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func digest(t *testing.T, v any) string {
	t.Helper()
	d, err := Digest(v)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDigestIgnoresMapOrderButNotValues(t *testing.T) {
	a := digest(t, map[string]any{"op": "deleteDevice", "params": map[string]string{"id": "1", "x": "2"}})
	b := digest(t, map[string]any{"params": map[string]string{"x": "2", "id": "1"}, "op": "deleteDevice"})
	c := digest(t, map[string]any{"op": "deleteDevice", "params": map[string]string{"id": "2", "x": "2"}})
	if a != b || a == c {
		t.Fatalf("digests %s %s %s", a, b, c)
	}
}

func TestRedeemOnceForTheSameUserAndCall(t *testing.T) {
	c := &clock{time.Unix(1_800_000_000, 0)}
	b := New(c.now)
	d := digest(t, "call-1")
	state := b.Issue("usr_1", d)
	if strings.Contains(state, "usr_1") {
		t.Fatalf("state %q shows the subject in clear", state)
	}
	if err := b.Redeem(state, "usr_1", d); err != nil {
		t.Fatalf("first redeem: %v", err)
	}
	if err := b.Redeem(state, "usr_1", d); !errors.Is(err, ErrInvalid) {
		t.Fatalf("second redeem = %v, want ErrInvalid", err)
	}
}

func TestRedeemRefusesAnotherUserCallKeyOrTime(t *testing.T) {
	c := &clock{time.Unix(1_800_000_000, 0)}
	b := New(c.now)
	d := digest(t, "call-1")
	cases := map[string]func() error{
		"other user": func() error { return b.Redeem(b.Issue("usr_1", d), "usr_2", d) },
		"other call": func() error { return b.Redeem(b.Issue("usr_1", d), "usr_1", digest(t, "call-2")) },
		"other key":  func() error { return New(c.now).Redeem(b.Issue("usr_1", d), "usr_1", d) },
		"altered": func() error {
			s := b.Issue("usr_1", d)
			body, sig, _ := strings.Cut(s, ".")
			return b.Redeem(body[:len(body)-1]+"A."+sig, "usr_1", d)
		},
		"no signature": func() error { body, _, _ := strings.Cut(b.Issue("usr_1", d), "."); return b.Redeem(body, "usr_1", d) },
		"empty":        func() error { return b.Redeem("", "usr_1", d) },
		"expired": func() error {
			s := b.Issue("usr_1", d)
			c.t = c.t.Add(TTL)
			defer func() { c.t = c.t.Add(-TTL) }()
			return b.Redeem(s, "usr_1", d)
		},
	}
	for name, redeem := range cases {
		if err := redeem(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: redeem = %v, want ErrInvalid", name, err)
		}
	}
}

func TestRedeemForgetsExpiredNonces(t *testing.T) {
	c := &clock{time.Unix(1_800_000_000, 0)}
	b := New(c.now)
	d := digest(t, "call-1")
	if err := b.Redeem(b.Issue("usr_1", d), "usr_1", d); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(TTL + time.Second)
	if err := b.Redeem(b.Issue("usr_1", d), "usr_1", d); err != nil {
		t.Fatal(err)
	}
	if n := len(b.used); n != 1 {
		t.Fatalf("nonces kept = %d, want the expired one dropped", n)
	}
}
