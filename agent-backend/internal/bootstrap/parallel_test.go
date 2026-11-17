package bootstrap

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTenCallsInOneContextShareOneExchange(t *testing.T) {
	e := newTestEnv(t)
	s := e.connect(e.userToken(orgA, prjP))
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if res, _ := callTool(t, s, "inventaris_list_items", nil); res.IsError {
				t.Errorf("parallel call failed: %s", resultText(t, res))
			}
		})
	}
	wg.Wait()
	for range 5 {
		if res, _ := callTool(t, s, "inventaris_list_items", nil); res.IsError {
			t.Fatalf("call failed: %s", resultText(t, res))
		}
	}
	if n := len(e.nice.Exchanges()); n != 1 {
		t.Fatalf("exchanges = %d for ten calls in one context, want 1", n)
	}
	if n := len(e.inv.Received()); n != 10 {
		t.Fatalf("inventaris received %d calls, want 10", n)
	}
}

func TestTwoContextsExchangeAtTheSameTime(t *testing.T) {
	e := newTestEnv(t)
	var inFlight, most atomic.Int32
	both := make(chan struct{})
	var once sync.Once
	// Each exchange waits until the other one has started: run one after the other, they would time out.
	e.nice.onExchange = func(string) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		if n == 2 {
			once.Do(func() { close(both) })
		}
		select {
		case <-both:
		case <-time.After(3 * time.Second):
		}
	}
	s := e.connect(e.userToken(orgA, prjP))
	var wg sync.WaitGroup
	for _, where := range []map[string]any{nil, {"organization_id": orgB, "project_id": prjQ}} {
		wg.Go(func() {
			if res, _ := callTool(t, s, "inventaris_list_items", where); res.IsError {
				t.Errorf("call for %v failed: %s", where, resultText(t, res))
			}
		})
	}
	wg.Wait()
	if most.Load() != 2 {
		t.Fatalf("at most %d exchange in flight, want 2: calls for different contexts must not wait for each other", most.Load())
	}
	if n := len(e.nice.Exchanges()); n != 2 {
		t.Fatalf("exchanges = %d, want one per context", n)
	}
}
