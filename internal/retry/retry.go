// Package retry provides retry, backoff, and rate limiting for HTTP clients.
// Retry config: max_retry=3, base_delay=1s, max_delay=30s (§22 Retry Policy).
package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Config defines retry and backoff behavior.
type Config struct {
	MaxRetries      int           // default: 3
	BaseDelay       time.Duration // default: 1s
	MaxDelay        time.Duration // default: 30s
	MaxConcurrency  int           // for rate limiting
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
	client  *http.Client
	config  Config
}

// NewClient creates a new RetryableClient.
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
		client: &http.Client{Timeout: 30 * time.Second},
		config: config,
	}
}
// WithHTTPClient sets a custom underlying http.Client (for testing with httptest).
func (rc *RetryableClient) WithHTTPClient(c *http.Client) *RetryableClient {
	rc.client = c
	return rc
}
// Do executes an HTTP request with retry logic.
// Returns the final response (caller must close body).
// Retry logic (§22):
//   - HTTP 429 → backoff respecting Retry-After
//   - HTTP 5xx → exponential backoff
//   - HTTP 4xx (except 429) → no retry
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

		if lastErr == nil {
			if resp.StatusCode == http.StatusOK {
				return resp, nil
			}

			// Check if retryable + calculate specific delay (rate limit)
			retryable, delay := rc.isRetryableStatus(resp.StatusCode, resp)
			resp.Body.Close()

			if !retryable {
				// Non-retryable error - return immediately
				return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
			}

			// Use specific delay (from headers) or exponential backoff
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

		// Network error - retry with backoff
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
// Returns (retryable, delay) where delay is from rate-limit headers.
func (rc *RetryableClient) isRetryableStatus(statusCode int, resp *http.Response) (bool, time.Duration) {
	// 429 Too Many Requests - retryable
	if statusCode == http.StatusTooManyRequests {
		return true, rc.getRetryDelay(resp)
	}
	// 403 Forbidden - retryable if GitHub rate limit headers present
	if statusCode == http.StatusForbidden {
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		if remaining == "0" {
			return true, rc.getRateLimitDelay(resp)
		}
	}
	// 5xx errors - retryable
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
	}
	return 0
}

// getRateLimitDelay returns delay until GitHub rate limit resets.
func (rc *RetryableClient) getRateLimitDelay(resp *http.Response) time.Duration {
	resetStr := resp.Header.Get("X-RateLimit-Reset")
	if resetStr == "" {
		return 0
	}
	if resetTime, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
		delay := time.Until(time.Unix(resetTime, 0))
		if delay > 0 {
			// Cap at 10s to avoid long waits during rate limiting
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
			return delay
		}
	}
	return 0
}

// calculateBackoff implements exponential backoff: 1s → 2s → 4s → 8s, capped at MaxDelay.
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
