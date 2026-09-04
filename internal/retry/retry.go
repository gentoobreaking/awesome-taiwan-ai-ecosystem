// Package retry provides retry, backoff, and rate limiting for HTTP clients.
// Retry config: max_retry=3, base_delay=1s, max_delay=30s (§22 Retry Policy).
package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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

			retryable := rc.isRetryableStatus(resp.StatusCode)
			resp.Body.Close()

			if !retryable {
				// Non-retryable error - return immediately
				return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
			}

			// Calculate backoff
			delay := rc.calculateBackoff(attempt, resp)
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
		delay := rc.calculateBackoff(attempt, nil)
		if delay > 0 {
			select {
			case <-time.After(delay):
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

func (rc *RetryableClient) isRetryableStatus(statusCode int) bool {
	// 429 Too Many Requests - retryable
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	// 5xx errors - retryable
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	// 4xx (except 429) - not retryable
	return false
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
