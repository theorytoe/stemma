package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientSendsTheUserAgent(t *testing.T) {
	var ua, accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		accept = r.Header.Get("Accept")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.sleep = func(context.Context, time.Duration) error { return nil }
	resp, err := c.Get(context.Background(), srv.URL, "application/x-bibtex")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if !strings.HasPrefix(ua, "stemma/") {
		t.Errorf("user agent = %q, want it to start with stemma/", ua)
	}
	if !strings.Contains(ua, "github.com/theorytoe/stemma") {
		t.Errorf("user agent = %q, want it to carry the project URL", ua)
	}
	if accept != "application/x-bibtex" {
		t.Errorf("accept = %q", accept)
	}
}

func TestClientRetriesTransientFailures(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.sleep = func(context.Context, time.Duration) error { return nil }
	resp, err := c.Get(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := atomic.LoadInt32(&n); got != 3 {
		t.Errorf("requests = %d, want 3 (two retries)", got)
	}
}

func TestClientGivesUpAfterRetries(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewClient()
	c.sleep = func(context.Context, time.Duration) error { return nil }
	resp, err := c.Get(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := atomic.LoadInt32(&n); got != int32(c.Retries+1) {
		t.Errorf("requests = %d, want %d", got, c.Retries+1)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

func TestClientDoesNotRetryAnAnswer(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient()
	c.sleep = func(context.Context, time.Duration) error { return nil }
	resp, err := c.Get(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := atomic.LoadInt32(&n); got != 1 {
		t.Errorf("requests = %d, want 1 (a 404 is an answer)", got)
	}
}

func TestClientHonoursRetryAfter(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) == 1 {
			w.Header().Set("Retry-After", "2")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	var waits []time.Duration
	c := NewClient()
	c.sleep = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	resp, err := c.Get(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if len(waits) != 1 || waits[0] != 2*time.Second {
		t.Errorf("waits = %v, want one wait of 2s", waits)
	}
}

func TestBackoffForCapsAndBacksOff(t *testing.T) {
	base := 100 * time.Millisecond
	first := backoffFor(base, 0, "")
	if first < base || first > base*3/2 {
		t.Errorf("first backoff = %v, want between %v and %v", first, base, base*3/2)
	}
	// 100ms << 20 is far past the cap, and the cap is applied before jitter.
	big := backoffFor(base, 20, "")
	if big < maxBackoff || big > maxBackoff*3/2 {
		t.Errorf("capped backoff = %v, want around %v", big, maxBackoff)
	}
	if got := backoffFor(base, 0, "9999"); got != maxBackoff {
		t.Errorf("retry-after over the cap = %v, want %v", got, maxBackoff)
	}
	if got := backoffFor(base, 0, "-1"); got != 0 {
		t.Errorf("negative retry-after = %v, want 0", got)
	}
}

func TestClientStopsWhenTheContextIsCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewClient()
	if _, err := c.Get(ctx, srv.URL, ""); err == nil {
		t.Error("Get succeeded with a cancelled context")
	}
}
