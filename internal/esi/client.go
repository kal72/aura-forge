package esi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aura-forge/internal/domain"
)

const (
	DefaultESIBaseURL   = "https://esi.evetech.net/latest"
	TheForgeRegionID    = int32(10000002)
	Jita44StationID     = int64(60003760)
	JitaSolarSystemID   = int32(30000142)
	MinErrorLimitRemain = int32(10)
	DefaultCacheTTL     = 5 * time.Minute
	DefaultHTTPTimeout  = 15 * time.Second
	DefaultRetryAfter   = 30 * time.Second
	MaxOrderPages       = 5
)

var (
	ErrErrorLimitReached = errors.New("ESI error limit threshold reached; pausing requests until X-ESI-Error-Limit-Reset")
	ErrRateLimitCooldown = errors.New("ESI rate-limit/downtime cooldown active (Retry-After); pausing requests")
	ErrSSODisabled       = errors.New("EVE SSO is not configured (set ESI_CLIENT_ID and ESI_CLIENT_SECRET in .env to enable)")
)

type inflightCall struct {
	done    chan struct{}
	data    []byte
	headers http.Header
	err     error
}

// Client is a low-level HTTP client for EVE Online ESI with User-Agent, PostgreSQL caching,
// in-flight request deduplication, X-ESI-Error-Limit-Remain/Reset auto-recovery, and Retry-After cooldown.
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
	cache      domain.ESICacheRepository

	rateMu           sync.Mutex
	errorLimitRemain int32
	errorResetAt     time.Time
	cooldownUntil    time.Time

	inflightMu sync.Mutex
	inflight   map[string]*inflightCall
}

// NewClient constructs a new ESI HTTP client.
func NewClient(baseURL, userAgent string, cache domain.ESICacheRepository, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultESIBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if strings.TrimSpace(userAgent) == "" {
		userAgent = "aura-forge/1.0 (EVE Market & Industry Copilot)"
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultHTTPTimeout}
	}

	return &Client{
		baseURL:          baseURL,
		userAgent:        userAgent,
		httpClient:       httpClient,
		cache:            cache,
		errorLimitRemain: 100,
		inflight:         make(map[string]*inflightCall),
	}
}

// ErrorLimitRemain returns the last observed X-ESI-Error-Limit-Remain value (auto-restoring after reset window).
func (c *Client) ErrorLimitRemain() int32 {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	c.maybeResetErrorLimitLocked()
	return c.errorLimitRemain
}

func (c *Client) maybeResetErrorLimitLocked() {
	if c.errorLimitRemain < MinErrorLimitRemain && !c.errorResetAt.IsZero() && time.Now().After(c.errorResetAt) {
		c.errorLimitRemain = 100
		c.errorResetAt = time.Time{}
	}
}

func (c *Client) checkRateLimit() error {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	now := time.Now()
	if !c.cooldownUntil.IsZero() && now.Before(c.cooldownUntil) {
		return fmt.Errorf("%w (retry in %s)", ErrRateLimitCooldown, time.Until(c.cooldownUntil).Round(time.Second))
	}

	c.maybeResetErrorLimitLocked()
	if c.errorLimitRemain < MinErrorLimitRemain {
		return ErrErrorLimitReached
	}
	return nil
}

// DoGet performs a GET request to ESI with optional caching, in-flight deduplication, and bearer auth.
func (c *Client) DoGet(ctx context.Context, path string, accessToken string, useCache bool) ([]byte, error) {
	body, _, err := c.DoGetWithHeaders(ctx, path, accessToken, useCache)
	return body, err
}

// DoGetWithHeaders performs a GET request to ESI and returns both body bytes and response headers (e.g. X-Pages).
func (c *Client) DoGetWithHeaders(ctx context.Context, path string, accessToken string, useCache bool) ([]byte, http.Header, error) {
	if err := c.checkRateLimit(); err != nil {
		return nil, nil, err
	}

	fullURL := c.baseURL + path
	cacheKey := "GET:" + fullURL

	// Fast-path cache check before deduplication
	if useCache && c.cache != nil {
		data, _, expiresAt, found, err := c.cache.Get(ctx, cacheKey)
		if err == nil && found && time.Now().Before(expiresAt) {
			return data, http.Header{}, nil
		}
	}

	// Singleflight deduplication for identical concurrent unauthenticated GETs
	if accessToken == "" {
		c.inflightMu.Lock()
		if existing, ok := c.inflight[cacheKey]; ok {
			c.inflightMu.Unlock()
			select {
			case <-existing.done:
				return existing.data, existing.headers, existing.err
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		call := &inflightCall{done: make(chan struct{})}
		c.inflight[cacheKey] = call
		c.inflightMu.Unlock()

		data, hdrs, err := c.executeGet(ctx, fullURL, path, cacheKey, accessToken, useCache)
		call.data = data
		call.headers = hdrs
		call.err = err
		close(call.done)

		c.inflightMu.Lock()
		delete(c.inflight, cacheKey)
		c.inflightMu.Unlock()
		return data, hdrs, err
	}

	return c.executeGet(ctx, fullURL, path, cacheKey, accessToken, useCache)
}

func (c *Client) executeGet(ctx context.Context, fullURL, path, cacheKey, accessToken string, useCache bool) ([]byte, http.Header, error) {
	var cachedData []byte
	var cachedETag string
	var cacheFound bool

	if useCache && c.cache != nil {
		data, etag, expiresAt, found, err := c.cache.Get(ctx, cacheKey)
		if err == nil && found {
			if time.Now().Before(expiresAt) {
				return data, http.Header{}, nil
			}
			cachedData = data
			cachedETag = etag
			cacheFound = true
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating ESI GET request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if cacheFound && cachedETag != "" {
		req.Header.Set("If-None-Match", cachedETag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing ESI GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	c.trackRateHeaders(resp.StatusCode, resp.Header)

	if resp.StatusCode == http.StatusNotModified && cacheFound {
		expiresAt := parseExpiresHeader(resp.Header.Get("Expires"))
		if useCache && c.cache != nil {
			_ = c.cache.Set(ctx, cacheKey, cachedData, cachedETag, expiresAt)
		}
		return cachedData, resp.Header.Clone(), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header.Clone(), fmt.Errorf("reading ESI response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Clone(), fmt.Errorf("ESI GET %s returned status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if useCache && c.cache != nil {
		etag := resp.Header.Get("ETag")
		expiresAt := parseExpiresHeader(resp.Header.Get("Expires"))
		_ = c.cache.Set(ctx, cacheKey, body, etag, expiresAt)
	}

	return body, resp.Header.Clone(), nil
}

// DoPost performs a POST request to ESI with JSON body, error-limit monitoring, and Retry-After respect.
func (c *Client) DoPost(ctx context.Context, path string, payload any) ([]byte, error) {
	if err := c.checkRateLimit(); err != nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling ESI POST payload: %w", err)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("creating ESI POST request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing ESI POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	c.trackRateHeaders(resp.StatusCode, resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ESI POST response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ESI POST %s returned status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return body, nil
}

func (c *Client) trackRateHeaders(statusCode int, headers http.Header) {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	if rawRemain := strings.TrimSpace(headers.Get("X-ESI-Error-Limit-Remain")); rawRemain != "" {
		if val, err := strconv.Atoi(rawRemain); err == nil {
			c.errorLimitRemain = int32(val)
		}
	}

	if rawReset := strings.TrimSpace(headers.Get("X-ESI-Error-Limit-Reset")); rawReset != "" {
		if secs, err := strconv.Atoi(rawReset); err == nil && secs > 0 {
			c.errorResetAt = time.Now().Add(time.Duration(secs) * time.Second)
		}
	}

	// Respect Retry-After on 420 (ESI Error Limited), 429 (Too Many Requests), or 503 (Downtime)
	if statusCode == 420 || statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable {
		cooldown := parseRetryAfter(headers.Get("Retry-After"))
		if cooldown <= 0 && (statusCode == 420 || statusCode == http.StatusTooManyRequests) {
			cooldown = DefaultRetryAfter
		}
		if cooldown > 0 {
			until := time.Now().Add(cooldown)
			if until.After(c.cooldownUntil) {
				c.cooldownUntil = until
			}
		}
	}
}

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(raw); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

func parseExpiresHeader(raw string) time.Time {
	if strings.TrimSpace(raw) != "" {
		if t, err := http.ParseTime(raw); err == nil {
			return t
		}
	}
	return time.Now().Add(DefaultCacheTTL)
}
