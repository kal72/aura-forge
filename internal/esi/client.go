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
	"sync/atomic"
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
)

var (
	ErrErrorLimitReached = errors.New("ESI error limit threshold reached; pausing requests to prevent IP ban")
	ErrSSODisabled       = errors.New("EVE SSO is not configured (set ESI_CLIENT_ID and ESI_CLIENT_SECRET in .env to enable)")
)

// Client is a low-level HTTP client for EVE Online ESI with User-Agent, caching, and error-limit tracking.
type Client struct {
	baseURL          string
	userAgent        string
	httpClient       *http.Client
	cache            domain.ESICacheRepository
	errorLimitRemain atomic.Int32
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

	c := &Client{
		baseURL:    baseURL,
		userAgent:  userAgent,
		httpClient: httpClient,
		cache:      cache,
	}
	c.errorLimitRemain.Store(100)
	return c
}

// ErrorLimitRemain returns the last observed X-ESI-Error-Limit-Remain value.
func (c *Client) ErrorLimitRemain() int32 {
	return c.errorLimitRemain.Load()
}

// DoGet performs a GET request to ESI with optional caching and bearer authentication.
func (c *Client) DoGet(ctx context.Context, path string, accessToken string, useCache bool) ([]byte, error) {
	if c.errorLimitRemain.Load() < MinErrorLimitRemain {
		return nil, ErrErrorLimitReached
	}

	fullURL := c.baseURL + path
	cacheKey := "GET:" + fullURL

	var cachedData []byte
	var cachedETag string
	var cacheFound bool

	if useCache && c.cache != nil {
		data, etag, expiresAt, found, err := c.cache.Get(ctx, cacheKey)
		if err == nil && found {
			if time.Now().Before(expiresAt) {
				return data, nil
			}
			cachedData = data
			cachedETag = etag
			cacheFound = true
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating ESI GET request: %w", err)
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
		return nil, fmt.Errorf("executing ESI GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	c.trackErrorLimit(resp.Header)

	if resp.StatusCode == http.StatusNotModified && cacheFound {
		expiresAt := parseExpiresHeader(resp.Header.Get("Expires"))
		if useCache && c.cache != nil {
			_ = c.cache.Set(ctx, cacheKey, cachedData, cachedETag, expiresAt)
		}
		return cachedData, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ESI response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ESI GET %s returned status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if useCache && c.cache != nil {
		etag := resp.Header.Get("ETag")
		expiresAt := parseExpiresHeader(resp.Header.Get("Expires"))
		_ = c.cache.Set(ctx, cacheKey, body, etag, expiresAt)
	}

	return body, nil
}

// DoPost performs a POST request to ESI with JSON body and error-limit monitoring.
func (c *Client) DoPost(ctx context.Context, path string, payload any) ([]byte, error) {
	if c.errorLimitRemain.Load() < MinErrorLimitRemain {
		return nil, ErrErrorLimitReached
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

	c.trackErrorLimit(resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ESI POST response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ESI POST %s returned status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return body, nil
}

func (c *Client) trackErrorLimit(headers http.Header) {
	raw := headers.Get("X-ESI-Error-Limit-Remain")
	if raw == "" {
		return
	}
	if val, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		c.errorLimitRemain.Store(int32(val))
	}
}

func parseExpiresHeader(raw string) time.Time {
	if strings.TrimSpace(raw) != "" {
		if t, err := http.ParseTime(raw); err == nil {
			return t
		}
	}
	return time.Now().Add(DefaultCacheTTL)
}
