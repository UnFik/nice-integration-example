package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCheckMessage(t *testing.T) {
	if err := CheckMessage("daftar perangkat saya"); err != nil {
		t.Fatal(err)
	}
	var invalid InvalidInputError
	for _, s := range []string{"", "   \n", "\xff"} {
		if err := CheckMessage(s); !errors.As(err, &invalid) {
			t.Errorf("%q: %v", s, err)
		}
	}
	if err := CheckMessage(strings.Repeat("é", MaxMessageRunes)); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	var long TooLongError
	if err := CheckMessage(strings.Repeat("a", MaxMessageRunes+1)); !errors.As(err, &long) {
		t.Errorf("over the limit: %v", err)
	}
}

func TestCleanTitle(t *testing.T) {
	if got, err := CleanTitle("  Perangkat Demo  "); err != nil || got != "Perangkat Demo" {
		t.Fatalf("%q %v", got, err)
	}
	for _, s := range []string{"", "  ", strings.Repeat("x", MaxTitleRunes+1), "a\x00b", "a\nb"} {
		if _, err := CleanTitle(s); err == nil {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestTitleFrom(t *testing.T) {
	if got := TitleFrom("  hapus\nperangkat   lama  "); got != "hapus perangkat lama" {
		t.Fatalf("%q", got)
	}
	got := TitleFrom(strings.Repeat("kata ", 40))
	if utf8.RuneCountInString(got) > 60 || !strings.HasSuffix(got, "…") {
		t.Fatalf("%q", got)
	}
}
