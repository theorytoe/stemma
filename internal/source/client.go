package source

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/theorytoe/stemma/internal/version"
)

// The network contract. These are the numbers the whole package agrees on:
// long enough for a slow resolver, bounded so a dead network fails in well under
// a minute, and retrying only the failures that a retry can fix.
const (
	// DefaultTimeout bounds one request, including its redirects.
	DefaultTimeout = 15 * time.Second
	// DefaultRetries is how many times a transient failure is retried, so a
	// request is made at most DefaultRetries+1 times.
	DefaultRetries = 2
	// baseBackoff is the first wait between retries; it doubles each time.
	baseBackoff = 500 * time.Millisecond
	// maxBackoff caps a wait, so a hostile Retry-After cannot stall a run.
	maxBackoff = 8 * time.Second
)

// Client is the HTTP client resolution talks through. It exists so the network
// contract has one implementation, and so a test can replace the transport and
// the sleep instead of reaching the real network.
type Client struct {
	// HTTP performs the requests. Its Timeout is the per-request bound.
	HTTP *http.Client
	// UA is the User-Agent every request carries, so operators can see who is
	// calling and where to complain.
	UA string
	// Retries is how many additional attempts a transient failure gets.
	Retries int
	// Backoff is the first wait between retries.
	Backoff time.Duration

	// sleep waits between attempts. It is a field so a test does not have to.
	sleep func(context.Context, time.Duration) error
}

// NewClient returns a client with the contract the package documents.
func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: DefaultTimeout},
		UA:      "stemma/" + version.Version + " (+https://github.com/theorytoe/stemma)",
		Retries: DefaultRetries,
		Backoff: baseBackoff,
		sleep:   sleepContext,
	}
}

// Get performs a GET under the network contract and returns the final response,
// open for the caller to read and close.
//
// A network error and a 429 or 5xx are retried with an exponential backoff,
// honouring Retry-After when the server sends one. Anything else is returned as
// it came, because a 404 or a 400 is an answer and not a failure to reach one.
// Retries stop early when the context is cancelled.
func (c *Client) Get(ctx context.Context, url, accept string) (*http.Response, error) {
	retries := c.Retries
	backoff := c.Backoff
	if backoff <= 0 {
		backoff = baseBackoff
	}
	sleep := c.sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		if c.UA != "" {
			req.Header.Set("User-Agent", c.UA)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt >= retries {
				return nil, err
			}
			if err := sleep(ctx, backoffFor(backoff, attempt, "")); err != nil {
				return nil, err
			}
			continue
		}

		if !retryableStatus(resp.StatusCode) || attempt >= retries {
			return resp, nil
		}
		retryAfter := resp.Header.Get("Retry-After")
		drain(resp.Body)
		if err := sleep(ctx, backoffFor(backoff, attempt, retryAfter)); err != nil {
			return nil, err
		}
	}
}

// retryableStatus reports whether a status is worth trying again. A 429 and a
// 5xx are the server saying "not now"; the rest are answers.
func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// backoffFor is how long to wait before attempt+1. A Retry-After the server
// sent wins, capped; otherwise the wait doubles per attempt with a little
// jitter so that many clients do not return in lockstep.
func backoffFor(base time.Duration, attempt int, retryAfter string) time.Duration {
	if d, ok := parseRetryAfter(retryAfter); ok {
		if d < 0 {
			return 0
		}
		if d > maxBackoff {
			return maxBackoff
		}
		return d
	}
	d := base << attempt
	if d > maxBackoff {
		d = maxBackoff
	}
	if d > 0 {
		d += time.Duration(rand.Int64N(int64(d/2) + 1))
	}
	return d
}

// parseRetryAfter reads the header's two allowed forms: a number of seconds, or
// an HTTP date.
func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t), true
	}
	return 0, false
}

// drain empties a response that is about to be retried, so the connection can
// be reused, then closes it.
func drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	_ = body.Close()
}

// sleepContext waits, and gives up early when the context is cancelled.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
