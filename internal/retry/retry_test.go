package retry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("Expected BaseDelay=1s, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay=30s, got %v", cfg.MaxDelay)
	}
}

func TestRetryOn500(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	rc := NewClient(DefaultConfig())
	resp, err := rc.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	requests := atomic.LoadInt32(&requestCount)
	if requests > 4 {
		t.Errorf("Expected at most 4 requests (initial + 3 retries), got %d", requests)
	}
}

func TestMaxRetryExceeded(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	rc := NewClient(Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})
	_, err := rc.Get(context.Background(), server.URL)

	if err == nil {
		t.Error("Expected error after max retries exceeded")
	}

	requests := atomic.LoadInt32(&requestCount)
	if requests > 4 {
		t.Errorf("Expected at most 4 requests (initial + 3 retries), got %d", requests)
	}
	if requests < 4 {
		t.Errorf("Expected exactly 4 requests, got %d", requests)
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rc := NewClient(Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})
	_, err := rc.Get(context.Background(), server.URL)

	if err == nil {
		t.Error("Expected error for 404")
	}

	requests := atomic.LoadInt32(&requestCount)
	if requests != 1 {
		t.Errorf("Expected 1 request for 4xx (no retry), got %d", requests)
	}
}

func TestNoRetryOn429WithRetryAfter(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rc := NewClient(Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})
	_, err := rc.Get(context.Background(), server.URL)

	if err != nil {
		t.Fatalf("Expected success after 429 retry, got error: %v", err)
	}

	requests := atomic.LoadInt32(&requestCount)
	if requests != 2 {
		t.Errorf("Expected 2 requests (initial + 1 retry), got %d", requests)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rc := NewClient(Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := rc.Get(ctx, server.URL)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

func TestRetryAfterHeader(t *testing.T) {
	rc := NewClient(DefaultConfig())

	// Test Retry-After parsing
	delay := rc.calculateBackoff(0, &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"2"}},
	})
	if delay != 2*time.Second {
		t.Errorf("Expected 2s delay from Retry-After, got %v", delay)
	}
}
