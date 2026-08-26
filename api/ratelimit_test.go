package main

import (
	"testing"
	"time"
)

func TestIPRateLimiter_AllowsUpToBurstThenRejects(t *testing.T) {
	limiter := newIPRateLimiter(1, 3) // 1 req/s refill, burst of 3

	for i := 0; i < 3; i++ {
		if !limiter.allow("1.2.3.4") {
			t.Fatalf("request %d within burst should be allowed", i)
		}
	}
	if limiter.allow("1.2.3.4") {
		t.Fatal("request past burst should be rejected")
	}
}

func TestIPRateLimiter_TracksClientsIndependently(t *testing.T) {
	limiter := newIPRateLimiter(1, 1)

	if !limiter.allow("1.2.3.4") {
		t.Fatal("first client's first request should be allowed")
	}
	if limiter.allow("1.2.3.4") {
		t.Fatal("first client's second request should be rejected (burst exhausted)")
	}
	if !limiter.allow("5.6.7.8") {
		t.Fatal("a different client should have its own untouched budget")
	}
}

func TestIPRateLimiter_RefillsOverTime(t *testing.T) {
	limiter := newIPRateLimiter(1000, 1) // fast refill so the test doesn't sleep long

	if !limiter.allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if limiter.allow("1.2.3.4") {
		t.Fatal("immediate second request should be rejected")
	}
	time.Sleep(5 * time.Millisecond)
	if !limiter.allow("1.2.3.4") {
		t.Fatal("request after the refill window should be allowed")
	}
}

func TestIPRateLimiter_CleanupLoopEvictsIdleVisitors(t *testing.T) {
	limiter := newIPRateLimiter(1, 1)
	limiter.allow("1.2.3.4")

	done := make(chan struct{})
	defer close(done)
	go limiter.cleanupLoop(done, 5*time.Millisecond, 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)

	limiter.mu.Lock()
	_, stillTracked := limiter.visitors["1.2.3.4"]
	limiter.mu.Unlock()
	if stillTracked {
		t.Fatal("visitor idle past maxIdle should have been evicted by the cleanup loop")
	}
}
