// Package fallback provides Yahoo Finance data source adapters for global markets.
//
// Stability features:
//   - Single shared circuit breaker across all three providers (quote/kline/
//     fundamental): one Yahoo-wide outage trips once, not 3x per provider
//   - Cooldown backs off exponentially per repeated trip (2min → 30min cap)
//   - Available() also honors the data-layer rate-limit flag so the HTTP
//     channel and the breaker share one gate
//   - A-share requests are skipped with datasource.ErrUnsupported (expected,
//     logged at debug) instead of a failure warning
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

// --- Circuit Breaker for Yahoo Finance (shared by all providers) ---

type circuitState int

const (
	circuitClosed circuitState = iota // normal
	circuitOpen                       // failing, reject fast
	circuitHalfOpen                   // testing recovery
)

type yahooCircuit struct {
	mu            sync.RWMutex
	state         circuitState
	failCount     int
	failThreshold int
	lastFailTime  time.Time
	cooldown      time.Duration
	opens         int // how many times the breaker has tripped, drives backoff
	successCount  int
}

// sharedYahooCircuit is intentionally process-wide: Yahoo outages are
// endpoint-wide, so quote/kline/fundamental must trip together instead of
// each paying their own failure threshold.
var (
	sharedYahooCircuitOnce sync.Once
	sharedYahooCircuit     *yahooCircuit
)

func getYahooCircuit() *yahooCircuit {
	sharedYahooCircuitOnce.Do(func() {
		sharedYahooCircuit = &yahooCircuit{
			failThreshold: 5,
			cooldown:      2 * time.Minute,
		}
	})
	return sharedYahooCircuit
}

func (cb *yahooCircuit) Allow() bool {
	cb.mu.RLock()
	state := cb.state
	lastFail := cb.lastFailTime
	cooldown := cb.cooldown
	cb.mu.RUnlock()

	if state == circuitClosed {
		return true
	}
	if state == circuitOpen {
		if time.Since(lastFail) > cooldown {
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
			cb.opens = 0
			cb.cooldown = 2 * time.Minute
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
		if cb.state != circuitOpen {
			cb.opens++
		}
		cb.state = circuitOpen
		// Exponential backoff: 2min → 4min → 8min → … capped at 30min.
		cooldown := 2 * time.Minute << (cb.opens - 1)
		if cooldown > 30*time.Minute || cooldown <= 0 {
			cooldown = 30 * time.Minute
		}
		cb.cooldown = cooldown
		logger.SugaredLogger.Warnf("yahoo circuit breaker: OPEN (failCount=%d, cooldown=%s)", cb.failCount, cooldown)
	}
}

// yahooAvailable is the single gate every provider consults: the shared
// circuit breaker plus the data-layer rate-limit flag (set when all HTTP
// subdomains failed, including the PowerShell fallback path).
func yahooAvailable(ctx context.Context) bool {
	if !getYahooCircuit().Allow() {
		return false
	}
	if data.YahooRateLimited() {
		return false
	}
	return true
}

// --- Yahoo Quote Provider ---

type YahooQuoteProvider struct {
	api     *data.YahooFinanceApi
	circuit *yahooCircuit
}

func NewYahooQuoteProvider() *YahooQuoteProvider {
	return &YahooQuoteProvider{
		api:     data.NewYahooFinanceApi(),
		circuit: getYahooCircuit(),
	}
}

func (p *YahooQuoteProvider) Name() string  { return "yahoo" }
func (p *YahooQuoteProvider) Priority() int { return 25 }

func (p *YahooQuoteProvider) Available(ctx context.Context) bool {
	if !yahooAvailable(ctx) {
		logger.SugaredLogger.Debug("yahoo quote: circuit breaker OPEN or rate-limited, skipping")
		return false
	}
	return true
}

func (p *YahooQuoteProvider) GetQuote(ctx context.Context, code string) (*datasource.QuoteData, error) {
	// Yahoo 对 A股支持很差，直接跳过避免浪费请求
	if isA股(code) {
		return nil, fmt.Errorf("%w: yahoo quote does not cover A-share %s", datasource.ErrUnsupported, code)
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
		circuit: getYahooCircuit(),
	}
}

func (p *YahooKLineProvider) Name() string  { return "yahoo_kline" }
func (p *YahooKLineProvider) Priority() int { return 25 }

func (p *YahooKLineProvider) Available(ctx context.Context) bool {
	if !yahooAvailable(ctx) {
		logger.SugaredLogger.Debug("yahoo kline: circuit breaker OPEN or rate-limited, skipping")
		return false
	}
	return true
}

func (p *YahooKLineProvider) GetKLine(ctx context.Context, code, period string, count int) (*datasource.KLineData, error) {
	if isA股(code) {
		return nil, fmt.Errorf("%w: yahoo kline does not cover A-share %s", datasource.ErrUnsupported, code)
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
		circuit: getYahooCircuit(),
	}
}

func (p *YahooFundamentalProvider) Name() string  { return "yahoo_fundamental" }
func (p *YahooFundamentalProvider) Priority() int { return 25 }

func (p *YahooFundamentalProvider) Available(ctx context.Context) bool {
	return yahooAvailable(ctx)
}

func (p *YahooFundamentalProvider) GetFundamental(ctx context.Context, code string) (*datasource.FundamentalData, error) {
	if isA股(code) {
		return nil, fmt.Errorf("%w: yahoo fundamental does not cover A-share %s", datasource.ErrUnsupported, code)
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
