// Package fallback provides Yahoo Finance data source adapters for global markets.
//
// Stability features:
//   - Rate-limit tracking with exponential backoff
//   - Subdomain rotation (query1/query2)
//   - PowerShell WinHTTP fallback for TLS fingerprint bypass
//   - Local SQLite cache fallback when all network sources fail
//   - Circuit breaker pattern for failed sources
package fallback

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/data/datasource"
	"go-stock/backend/logger"
	"strings"
	"sync"
	"time"
)

// --- Circuit Breaker for Yahoo Finance ---

type circuitState int

const (
	circuitClosed circuitState = iota   // normal
	circuitOpen                         // failing, reject fast
	circuitHalfOpen                     // testing recovery
)

type yahooCircuit struct {
	mu            sync.RWMutex
	state         circuitState
	failCount     int
	failThreshold int
	lastFailTime  time.Time
	cooldown      time.Duration
	successCount int
}

func newYahooCircuit() *yahooCircuit {
	return &yahooCircuit{
		failThreshold: 5,           // 连续失败5次后熔断
		cooldown:      2 * time.Minute, // 熔断后冷却2分钟
	}
}

func (cb *yahooCircuit) Allow() bool {
	cb.mu.RLock()
	state := cb.state
	lastFail := cb.lastFailTime
	cb.mu.RUnlock()

	if state == circuitClosed {
		return true
	}
	if state == circuitOpen {
		if time.Since(lastFail) > cb.cooldown {
			cb.mu.Lock()
			cb.state = circuitHalfOpen
			cb.mu.Unlock()
			return true
		}
		return false
	}
	// half-open
	return true
}

func (cb *yahooCircuit) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failCount = 0
	if cb.state == circuitHalfOpen {
		cb.successCount++
		if cb.successCount >= 3 {
			cb.state = circuitClosed
			cb.successCount = 0
			logger.SugaredLogger.Info("yahoo circuit breaker: recovered to CLOSED")
		}
	}
}

func (cb *yahooCircuit) RecordFail() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failCount++
	cb.lastFailTime = time.Now()
	if cb.failCount >= cb.failThreshold {
		cb.state = circuitOpen
		logger.SugaredLogger.Warnf("yahoo circuit breaker: OPEN (failCount=%d)", cb.failCount)
	}
}

// --- Yahoo Quote Provider ---

type YahooQuoteProvider struct {
	api     *data.YahooFinanceApi
	circuit *yahooCircuit
}

func NewYahooQuoteProvider() *YahooQuoteProvider {
	return &YahooQuoteProvider{
		api:     data.NewYahooFinanceApi(),
		circuit: newYahooCircuit(),
	}
}

func (p *YahooQuoteProvider) Name() string  { return "yahoo" }
func (p *YahooQuoteProvider) Priority() int { return 25 }

func (p *YahooQuoteProvider) Available(ctx context.Context) bool {
	// 熔断器快速拒绝
	if !p.circuit.Allow() {
		logger.SugaredLogger.Debug("yahoo quote: circuit breaker OPEN, skipping")
		return false
	}
	return true
}

func (p *YahooQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// Yahoo 对 A股支持很差，直接跳过避免浪费请求
	if isA股(code) {
		return nil, fmt.Errorf("yahoo quote: skip A-share %s", code)
	}

	quote, err := p.api.GetQuote(ctx, code)
	if err != nil {
		p.circuit.RecordFail()
		return nil, fmt.Errorf("yahoo quote %s: %w", code, err)
	}

	p.circuit.RecordSuccess()
	return quote, nil
}

// --- Yahoo KLine Provider ---

type YahooKLineProvider struct {
	api     *data.YahooFinanceApi
	circuit *yahooCircuit
}

func NewYahooKLineProvider() *YahooKLineProvider {
	return &YahooKLineProvider{
		api:     data.NewYahooFinanceApi(),
		circuit: newYahooCircuit(),
	}
}

func (p *YahooKLineProvider) Name() string  { return "yahoo_kline" }
func (p *YahooKLineProvider) Priority() int { return 25 }

func (p *YahooKLineProvider) Available(ctx context.Context) bool {
	if !p.circuit.Allow() {
		logger.SugaredLogger.Debug("yahoo kline: circuit breaker OPEN, skipping")
		return false
	}
	return true
}

func (p *YahooKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
	if isA股(code) {
		return nil, fmt.Errorf("yahoo kline: skip A-share %s", code)
	}

	kline, err := p.api.GetKLine(ctx, code, period, count)
	if err != nil {
		p.circuit.RecordFail()
		return nil, fmt.Errorf("yahoo kline %s: %w", code, err)
	}

	p.circuit.RecordSuccess()
	return kline, nil
}

// --- Yahoo Fundamental Provider ---

type YahooFundamentalProvider struct {
	api     *data.YahooFundamentalApi
	circuit *yahooCircuit
}

func NewYahooFundamentalProvider() *YahooFundamentalProvider {
	return &YahooFundamentalProvider{
		api:     data.NewYahooFundamentalApi(),
		circuit: newYahooCircuit(),
	}
}

func (p *YahooFundamentalProvider) Name() string  { return "yahoo_fundamental" }
func (p *YahooFundamentalProvider) Priority() int { return 25 }

func (p *YahooFundamentalProvider) Available(ctx context.Context) bool {
	if !p.circuit.Allow() {
		return false
	}
	return true
}

func (p *YahooFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	if isA股(code) {
		return nil, fmt.Errorf("yahoo fundamental: skip A-share %s", code)
	}

	fd, err := p.api.GetFundamental(ctx, code)
	if err != nil {
		p.circuit.RecordFail()
		return nil, fmt.Errorf("yahoo fundamental %s: %w", code, err)
	}

	p.circuit.RecordSuccess()
	return fd, nil
}

// --- helpers ---

func isA股(code string) bool {
	lc := strings.ToLower(code)
	return strings.HasPrefix(lc, "sh") || strings.HasPrefix(lc, "sz") || strings.HasPrefix(lc, "bj")
}
