// Package retry provides retry, backoff, and rate limiting for HTTP clients.
// Retry config: max_retry=3, base_delay=1s, max_delay=30s (§22 Retry Policy).
package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	github_ratelimit "github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_primary_ratelimit"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit/github_secondary_ratelimit"
)

// Config defines retry and backoff behavior.
type Config struct {
	MaxRetries     int           // default: 3
	BaseDelay      time.Duration // default: 1s
	MaxDelay       time.Duration // default: 30s
	MaxConcurrency int           // for rate limiting
}

// DefaultConfig returns the standard retry configuration (§22).
func DefaultConfig() Config {
	return Config{
		MaxRetries:     3,
		BaseDelay:      1 * time.Second,
		MaxDelay:       30 * time.Second,
		MaxConcurrency: 2,
	}
}

// RetryableClient wraps http.Client with retry and rate limiting.
type RetryableClient struct {
	client *http.Client
	config Config
}

// newRateLimitedTransport wraps base with go-github-ratelimit (primary+secondary).
// Secondary is configured with SingleSleepLimit 60s and callbacks that log Warn,
// per audit recommendation A (minimal invasive transport).
func newRateLimitedTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return github_ratelimit.New(base,
		github_primary_ratelimit.WithLimitDetectedCallback(func(ctx *github_primary_ratelimit.CallbackContext) {
			slog.Warn("github primary rate limit detected", "category", ctx.Category, "reset", ctx.ResetTime)
		}),
		github_primary_ratelimit.WithRequestPreventedCallback(func(ctx *github_primary_ratelimit.CallbackContext) {
			slog.Warn("github primary rate limit request prevented", "category", ctx.Category, "reset", ctx.ResetTime)
		}),
		github_secondary_ratelimit.WithLimitDetectedCallback(func(ctx *github_secondary_ratelimit.CallbackContext) {
			slog.Warn("github secondary rate limit detected", "reset", ctx.ResetTime, "totalSleep", ctx.TotalSleepTime)
		}),
		github_secondary_ratelimit.WithSingleSleepLimit(60*time.Second, func(ctx *github_secondary_ratelimit.CallbackContext) {
			slog.Warn("github secondary single sleep limit exceeded", "reset", ctx.ResetTime, "totalSleep", ctx.TotalSleepTime)
		}),
		github_secondary_ratelimit.WithTotalSleepLimit(5*time.Minute, func(ctx *github_secondary_ratelimit.CallbackContext) {
			slog.Warn("github secondary total sleep limit exceeded", "reset", ctx.ResetTime, "totalSleep", ctx.TotalSleepTime)
		}),
	)
}

// NewClient creates a new RetryableClient with GitHub rate limiting enabled.
// Transport is automatically wrapped by github_ratelimit (primary + secondary).
func NewClient(config Config) *RetryableClient {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.BaseDelay == 0 {
		config.BaseDelay = 1 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 2
	}
	return &RetryableClient{
		client: &http.Client{
			Transport: newRateLimitedTransport(nil),
			Timeout:   30 * time.Second,
		},
		config: config,
	}
}

// NewClientWithRateLimit creates a new RetryableClient with explicit rate limiting.
// Provided for audit recommendation A — functionally identical to NewClient (which already wraps Transport).
func NewClientWithRateLimit(config Config) *RetryableClient {
	return NewClient(config)
}

// NewClientWithTransport creates a client wrapping a custom base transport with rate limiting.
func NewClientWithTransport(config Config, base http.RoundTripper) *RetryableClient {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.BaseDelay == 0 {
		config.BaseDelay = 1 * time.Second
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 2
	}
	return &RetryableClient{
		client: &http.Client{
			Transport: newRateLimitedTransport(base),
			Timeout:   30 * time.Second,
		},
		config: config,
	}
}

// WithHTTPClient sets a custom underlying http.Client (for testing with httptest).
// If the provided client has a Transport, it will be wrapped with rate limiting to preserve guarantees.
func (rc *RetryableClient) WithHTTPClient(c *http.Client) *RetryableClient {
	if c != nil {
		if c.Transport != nil {
			c.Transport = newRateLimitedTransport(c.Transport)
		} else {
			c.Transport = newRateLimitedTransport(nil)
		}
		rc.client = c
	}
	return rc
}

// Do executes an HTTP request with retry logic.
// Returns the final response (caller must close body).
// Retry logic (§22):
//   - HTTP 429/403 with rate limiting → delegated to github_ratelimit Transport (primary returns RateLimitReachedError, secondary sleeps & retries internally)
//   - HTTP 5xx → exponential backoff with jitter
//   - HTTP 4xx (except retryable) → no retry
//   - Timeout/DNS/network error → retry
//   - Max retries = 3 (initial + 3 retries = 4 attempts)
func (rc *RetryableClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)

	var lastErr error
	var resp *http.Response

	totalAttempts := rc.config.MaxRetries + 1 // initial + retries

	for attempt := 0; attempt < totalAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, lastErr = rc.client.Do(req)

		// Primary rate limit is signaled as RateLimitReachedError from the Transport — do not retry, propagate immediately.
		if lastErr != nil {
			var rlErr *github_primary_ratelimit.RateLimitReachedError
			if errors.As(lastErr, &rlErr) {
				return nil, lastErr
			}
		}

		if lastErr == nil {
			if resp.StatusCode == http.StatusOK {
				return resp, nil
			}

			// Check if retryable + calculate specific delay (only 5xx is retried at this layer; 429/403 delegated to Transport)
			retryable, delay := rc.isRetryableStatus(resp.StatusCode, resp)
			// Close body before retry / return; if secondary detection already consumed body, it was restored by the library.
			resp.Body.Close()

			if !retryable {
				// Non-retryable error - return immediately
				return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
			}

			// Use specific delay (from headers) or exponential backoff with jitter
			if delay == 0 {
				delay = rc.calculateBackoff(attempt, resp)
			}

			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			continue
		}

		// Network error - retry with backoff (jittered)
		backoffDelay := rc.calculateBackoff(attempt, nil)
		if backoffDelay > 0 {
			select {
			case <-time.After(backoffDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", totalAttempts, lastErr)
	}
	if resp != nil {
		return resp, fmt.Errorf("request failed after %d attempts with status %d", totalAttempts, resp.StatusCode)
	}
	return nil, errors.New("request failed")
}

// isRetryableStatus checks if an HTTP status code should be retried.
// Delegates secondary detection to github_ratelimit; this layer only handles 429/5xx + 403+Remaining==0.
// Returns (retryable, delay) where delay is from rate-limit headers.
func (rc *RetryableClient) isRetryableStatus(statusCode int, resp *http.Response) (bool, time.Duration) {
	// 429 Too Many Requests - retryable (secondary case handled by Transport; primary 429 also flows via RateLimitReachedError)
	if statusCode == http.StatusTooManyRequests {
		return true, rc.getRetryDelay(resp)
	}
	// 403 Forbidden - retryable only if GitHub primary remaining==0; secondary with body is delegated to Transport
	if statusCode == http.StatusForbidden {
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		if remaining == "0" {
			return true, rc.getRateLimitDelay(resp)
		}
		// Secondary abuse limit (body sniffing) is handled inside github_secondary_ratelimit transport; do not retry here
	}
	// 5xx errors - retryable with jittered exponential backoff
	if statusCode >= 500 && statusCode < 600 {
		return true, 0
	}
	// 4xx (except 403/429 with rate limit) - not retryable
	return false, 0
}

// getRetryDelay returns delay from Retry-After header.
func (rc *RetryableClient) getRetryDelay(resp *http.Response) time.Duration {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(seconds) * time.Second
		}
		// HTTP-date format is intentionally not parsed; Transport delegates via secondary_ratelimit which only handles seconds.
	}
	return 0
}

// getRateLimitDelay returns delay until GitHub rate limit resets.
// Returns the true delay (no 10s cap) so caller can decide;配合 ratelimit Primary 行為：Primary now returns RateLimitReachedError instead of sleeping.
func (rc *RetryableClient) getRateLimitDelay(resp *http.Response) time.Duration {
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	if resetStr == "" {
		return 0
	}
	if resetTime, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
		delay := time.Until(time.Unix(resetTime, 0))
		if delay > 0 {
			return delay
		}
	}
	return 0
}

// calculateBackoff implements exponential backoff: 1s → 2s → 4s → 8s, capped at MaxDelay, with jitter 0.8-1.2.
// If response has Retry-After header, use that instead.
func (rc *RetryableClient) calculateBackoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := parseRetryAfter(retryAfter); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}

	delay := rc.config.BaseDelay * time.Duration(1<<uint(attempt))
	if delay > rc.config.MaxDelay {
		delay = rc.config.MaxDelay
	}
	// Add jitter 0.8 - 1.2 to avoid thundering herd (§R10)
	jitter := 0.8 + rand.Float64()*0.4
	delay = time.Duration(float64(delay) * jitter)
	if delay > rc.config.MaxDelay {
		delay = rc.config.MaxDelay
	}
	if delay < 0 {
		delay = rc.config.BaseDelay
	}
	return delay
}

func parseRetryAfter(header string) (int, error) {
	var seconds int
	_, err := fmt.Sscanf(header, "%d", &seconds)
	return seconds, err
}

// Get performs a GET request with retry.
func (rc *RetryableClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return rc.Do(ctx, req)
}

// Post performs a POST request with retry.
func (rc *RetryableClient) Post(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	return rc.Do(ctx, req)
}

// Client returns the underlying http.Client.
func (rc *RetryableClient) Client() *http.Client {
	return rc.client
}

// SetBaseURL not supported - requests already have full URLs.
// SetTimeout sets the request timeout.
func (rc *RetryableClient) SetTimeout(timeout time.Duration) {
	rc.client.Timeout = timeout
}
