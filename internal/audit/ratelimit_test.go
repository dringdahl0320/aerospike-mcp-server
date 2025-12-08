// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:        true,
		RequestsPerSec: 100,
		BurstSize:      200,
	}

	rl := NewRateLimiter(cfg)

	if rl == nil {
		t.Fatal("RateLimiter is nil")
	}

	if !rl.enabled {
		t.Error("RateLimiter should be enabled")
	}

	if rl.maxTokens != 200 {
		t.Errorf("Expected max tokens 200, got %f", rl.maxTokens)
	}
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:        true,
		RequestsPerSec: 10,
		BurstSize:      5,
	})

	// Should allow first 5 requests (burst size)
	for i := 0; i < 5; i++ {
		if !rl.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied (no time to refill)
	if rl.Allow() {
		t.Error("6th request should be denied")
	}
}

func TestRateLimiterAllowN(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:        true,
		RequestsPerSec: 100,
		BurstSize:      10,
	})

	// Should allow 5 requests
	if !rl.AllowN(5) {
		t.Error("Should allow 5 requests")
	}

	// Should allow another 5 requests
	if !rl.AllowN(5) {
		t.Error("Should allow another 5 requests")
	}

	// Should deny 5 more (no tokens left)
	if rl.AllowN(5) {
		t.Error("Should deny when no tokens left")
	}
}

func TestRateLimiterDisabled(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:        false,
		RequestsPerSec: 1,
		BurstSize:      1,
	})

	// Should always allow when disabled
	for i := 0; i < 100; i++ {
		if !rl.Allow() {
			t.Error("Disabled rate limiter should always allow")
		}
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:        true,
		RequestsPerSec: 1000, // 1000 per second = 1 per ms
		BurstSize:      1,
	})

	// Use up the burst
	rl.Allow()

	// Should be denied
	if rl.Allow() {
		t.Error("Should be denied after burst exhausted")
	}

	// Wait for refill
	time.Sleep(10 * time.Millisecond)

	// Should be allowed after refill
	if !rl.Allow() {
		t.Error("Should be allowed after refill")
	}
}

func TestRateLimiterGetStats(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		Enabled:        true,
		RequestsPerSec: 100,
		BurstSize:      50,
	})

	stats := rl.GetStats()

	if !stats["enabled"].(bool) {
		t.Error("Stats should show enabled=true")
	}

	if stats["max_tokens"].(float64) != 50 {
		t.Errorf("Expected max_tokens 50, got %v", stats["max_tokens"])
	}

	if stats["refill_rate"].(float64) != 100 {
		t.Errorf("Expected refill_rate 100, got %v", stats["refill_rate"])
	}
}
