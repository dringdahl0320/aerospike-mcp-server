// Copyright 2024 OnChain Media Corporation
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	enabled    bool
}

// RateLimitConfig holds rate limiter configuration.
type RateLimitConfig struct {
	Enabled        bool    `json:"enabled"`
	RequestsPerSec float64 `json:"requests_per_second"`
	BurstSize      int     `json:"burst_size"`
}

// DefaultRateLimitConfig returns default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:        true,
		RequestsPerSec: 100,
		BurstSize:      200,
	}
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	maxTokens := float64(cfg.BurstSize)
	if maxTokens <= 0 {
		maxTokens = 200
	}

	refillRate := cfg.RequestsPerSec
	if refillRate <= 0 {
		refillRate = 100
	}

	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
		enabled:    cfg.Enabled,
	}
}

// Allow checks if a request is allowed under the rate limit.
func (r *RateLimiter) Allow() bool {
	if !r.enabled {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= 1 {
		r.tokens--
		return true
	}

	return false
}

// AllowN checks if n requests are allowed.
func (r *RateLimiter) AllowN(n int) bool {
	if !r.enabled {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	needed := float64(n)
	if r.tokens >= needed {
		r.tokens -= needed
		return true
	}

	return false
}

// Wait blocks until a request is allowed or returns error if rate limited.
func (r *RateLimiter) Wait() error {
	if !r.enabled {
		return nil
	}

	for i := 0; i < 100; i++ { // Max 10 seconds wait
		if r.Allow() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("rate limit exceeded")
}

// refill adds tokens based on elapsed time.
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.lastRefill = now

	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
}

// GetStats returns current rate limiter statistics.
func (r *RateLimiter) GetStats() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	return map[string]interface{}{
		"enabled":          r.enabled,
		"available_tokens": r.tokens,
		"max_tokens":       r.maxTokens,
		"refill_rate":      r.refillRate,
	}
}
