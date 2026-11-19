package bootstrap

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Two steps of six minutes pass the time niceclient renews a token (10 minutes after it was issued) and stay before
// its expiry (30). The reading between them matters: the SDK renews in the background only a token that was used
// within the last renewal interval, as the token of a running watch always is.
const step = 6 * time.Minute

func TestWatchUsesOneTokenForManyReadings(t *testing.T) {
	w := newWorld(t)
	if code := w.run(context.Background(), &scriptWaiter{stop: 4}, "watch"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	if calls, tokens := len(w.nadi.Calls()), len(w.nice.Requests()); calls != 4 || tokens != 1 {
		t.Fatalf("calls to nadi = %d, token requests = %d", calls, tokens)
	}
	if n := strings.Count(w.out.String(), "# 1 device(s)"); n != 4 {
		t.Fatalf("readings printed = %d: %s", n, w.out)
	}
}

func TestWatchRenewsTheTokenBeforeItExpires(t *testing.T) {
	w := newWorld(t)
	waiter := &scriptWaiter{stop: 4, steps: map[int]func(){
		1: func() { w.clock.Advance(step) },
		2: func() {
			w.clock.Advance(step)
			// Nothing calls NICE here: the background renewal of the SDK does.
			eventually(t, "the renewed token", func() bool { return len(w.nice.Issued()) == 2 })
		},
		// The SDK stores the new token a moment after NICE answered.
		3: func() { time.Sleep(50 * time.Millisecond) },
	}}
	if code := w.run(context.Background(), waiter, "watch"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	calls, issued := w.nadi.Calls(), w.nice.Issued()
	if len(calls) != 4 || calls[0].Bearer() != issued[0] || calls[1].Bearer() != issued[0] || calls[3].Bearer() != issued[1] {
		t.Fatalf("calls = %d, tokens = %d", len(calls), len(issued))
	}
}

func TestWatchStopsWhenTheKeyIsRevoked(t *testing.T) {
	w := newWorld(t)
	waiter := &scriptWaiter{steps: map[int]func(){
		1: func() { w.clock.Advance(step) },
		2: func() {
			w.nice.Refuse(&refusal{401, "invalid_client", "client authentication failed"})
			w.clock.Advance(step)
			eventually(t, "the refused renewal", func() bool { return len(w.nice.Requests()) == 2 })
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if code := w.run(ctx, waiter, "watch"); code != 1 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	if ctx.Err() != nil {
		t.Fatal("watch went on until the test gave up")
	}
	for _, want := range []string{"invalid_client", "checklist step 4"} {
		if !strings.Contains(w.err.String(), want) {
			t.Errorf("stderr lacks %q: %s", want, w.err)
		}
	}
	// The SDK asked NICE once more for the renewal and then stopped asking.
	if n := len(w.nice.Requests()); n != 2 {
		t.Fatalf("token requests = %d, want 2", n)
	}
}

func TestWatchKeepsItsTokenWhileNICEIsDown(t *testing.T) {
	w := newWorld(t)
	waiter := &scriptWaiter{stop: 5, steps: map[int]func(){
		1: func() { w.clock.Advance(step) },
		2: func() {
			w.nice.Refuse(&refusal{503, "temporarily_unavailable", ""})
			w.clock.Advance(step)
			eventually(t, "the failed renewal", func() bool { return len(w.nice.Requests()) >= 2 })
		},
	}}
	if code := w.run(context.Background(), waiter, "watch"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	calls := w.nadi.Calls()
	if len(calls) != 5 || calls[4].Bearer() != w.nice.Issued()[0] {
		t.Fatalf("calls to nadi = %d", len(calls))
	}
	if strings.Contains(w.err.String(), "FAILED") || strings.Count(w.out.String(), "# 1 device(s)") != 5 {
		t.Fatalf("stdout = %s\nstderr = %s", w.out, w.err)
	}
}

func TestWatchTriesAgainAfterNadiDidNotAnswer(t *testing.T) {
	w := newWorld(t)
	waiter := &scriptWaiter{stop: 3, steps: map[int]func(){
		1: func() { w.nadiNet.down.Store(true) },
		2: func() { w.nadiNet.down.Store(false) },
	}}
	if code := w.run(context.Background(), waiter, "watch"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
	if !strings.Contains(w.err.String(), "service_unavailable") || strings.Count(w.out.String(), "# 1 device(s)") != 2 {
		t.Fatalf("stdout = %s\nstderr = %s", w.out, w.err)
	}
}

func TestWatchEndsWithZeroWhenInterrupted(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := w.run(ctx, &scriptWaiter{}, "watch"); code != 0 {
		t.Fatalf("exit %d: %s", code, w.err)
	}
}
