package esi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
)

// inMemoryCache implements domain.ESICacheRepository for testing.
type inMemoryCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	data      []byte
	etag      string
	expiresAt time.Time
}

func newInMemoryCache() *inMemoryCache {
	return &inMemoryCache{entries: make(map[string]cacheEntry)}
}

func (c *inMemoryCache) Get(ctx context.Context, key string) ([]byte, string, time.Time, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, "", time.Time{}, false, nil
	}
	return e.data, e.etag, e.expiresAt, true, nil
}

func (c *inMemoryCache) Set(ctx context.Context, key string, data []byte, etag string, expiresAt time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{data: data, etag: etag, expiresAt: expiresAt}
	return nil
}

func (c *inMemoryCache) CleanExpired(ctx context.Context) error {
	return nil
}

func makeFakeEVEJWT(charID int64, charName string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(map[string]any{
		"sub":  "CHARACTER:EVE:" + stringFromInt64(charID),
		"name": charName,
		"scp":  esi.DefaultSSOScopes,
	})
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".fakesig"
}

func stringFromInt64(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestPublicMarketESI_AllEndpoints(t *testing.T) {
	var seenUserAgent string
	orderHits := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("X-ESI-Error-Limit-Remain", "95")
		w.Header().Set("ETag", `"etag-v1"`)
		w.Header().Set("Expires", time.Now().Add(5*time.Minute).UTC().Format(http.TimeFormat))

		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/universe/ids/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"inventory_types":[{"id":2456,"name":"Hobgoblin I"},{"id":34,"name":"Tritanium"}]}`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/markets/10000002/orders/"):
			orderHits++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"order_id":1,"type_id":2456,"location_id":60003760,"system_id":30000142,"price":12500.5,"volume_remain":50,"is_buy_order":false},
				{"order_id":2,"type_id":2456,"location_id":60003760,"system_id":30000142,"price":12000.0,"volume_remain":100,"is_buy_order":false},
				{"order_id":3,"type_id":2456,"location_id":60003760,"system_id":30000142,"price":10500.0,"volume_remain":200,"is_buy_order":true},
				{"order_id":4,"type_id":2456,"location_id":99999999,"system_id":30000144,"price":5000.0,"volume_remain":10,"is_buy_order":false}
			]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/markets/10000002/history/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"date":"2026-11-10","volume":1000},
				{"date":"2026-11-11","volume":2000},
				{"date":"2026-11-12","volume":3000},
				{"date":"2026-11-13","volume":4000},
				{"date":"2026-11-14","volume":5000},
				{"date":"2026-11-15","volume":6000},
				{"date":"2026-11-16","volume":7000}
			]`))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/industry/systems/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"solar_system_id":30000142,"cost_indices":[{"activity":"manufacturing","cost_index":0.065},{"activity":"invention","cost_index":0.032}]}
			]`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cache := newInMemoryCache()
	client := esi.NewClient(srv.URL, "aura-forge-test/1.0", cache, srv.Client())
	pub := esi.NewPublicMarketClient(client)
	ctx := context.Background()

	// 1. ResolveTypeIDs
	ids, err := pub.ResolveTypeIDs(ctx, []string{"Hobgoblin I", "Tritanium"})
	if err != nil {
		t.Fatalf("ResolveTypeIDs failed: %v", err)
	}
	if ids["Hobgoblin I"] != 2456 || ids["Tritanium"] != 34 {
		t.Errorf("unexpected resolved IDs: %+v", ids)
	}
	if seenUserAgent != "aura-forge-test/1.0" {
		t.Errorf("expected custom User-Agent, got %q", seenUserAgent)
	}

	// 2. GetJitaPrices (must filter Jita 4-4, ignoring 5000 ISK order in non-Jita system)
	prices, err := pub.GetJitaPrices(ctx, []int32{2456})
	if err != nil {
		t.Fatalf("GetJitaPrices failed: %v", err)
	}
	jp := prices[2456]
	if jp.LowestSell != 12000.0 || jp.HighestBuy != 10500.0 || jp.SellOrdersCount != 2 || jp.BuyOrdersCount != 1 {
		t.Errorf("unexpected Jita price: %+v", jp)
	}

	// Verify cache hit on second call (orderHits must remain 1)
	if _, err := pub.GetJitaPrices(ctx, []int32{2456}); err != nil {
		t.Fatalf("second GetJitaPrices failed: %v", err)
	}
	if orderHits != 1 {
		t.Errorf("expected cache hit (orderHits == 1), got %d", orderHits)
	}

	// 3. GetDailyVolume
	avg7, avg30, err := pub.GetDailyVolume(ctx, 2456)
	if err != nil {
		t.Fatalf("GetDailyVolume failed: %v", err)
	}
	if avg7 != 4000.0 || avg30 != 4000.0 {
		t.Errorf("expected avg 4000.0, got avg7=%.2f avg30=%.2f", avg7, avg30)
	}

	// 4. GetSystemCostIndex
	sci, err := pub.GetSystemCostIndex(ctx, esi.JitaSolarSystemID)
	if err != nil {
		t.Fatalf("GetSystemCostIndex failed: %v", err)
	}
	if sci["manufacturing"] != 0.065 || sci["invention"] != 0.032 {
		t.Errorf("unexpected SCI: %+v", sci)
	}
	if client.ErrorLimitRemain() != 95 {
		t.Errorf("expected ErrorLimitRemain 95, got %d", client.ErrorLimitRemain())
	}
}

func TestESIClient_ErrorLimitProtection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-ESI-Error-Limit-Remain", "5")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := esi.NewClient(srv.URL, "aura-forge-test/1.0", nil, srv.Client())
	ctx := context.Background()

	// First request succeeds and drops error limit to 5 (< MinErrorLimitRemain=10)
	if _, err := client.DoGet(ctx, "/test", "", false); err != nil {
		t.Fatalf("first request should succeed: %v", err)
	}

	// Second request must be blocked by ErrErrorLimitReached
	_, err := client.DoGet(ctx, "/test", "", false)
	if !errors.Is(err, esi.ErrErrorLimitReached) {
		t.Fatalf("expected ErrErrorLimitReached, got %v", err)
	}
}

func TestSSO_DisabledFallback(t *testing.T) {
	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)
	if sso.IsEnabled() {
		t.Fatalf("expected SSO to be disabled when credentials are empty")
	}

	if _, err := sso.AuthURL("state123"); !errors.Is(err, esi.ErrSSODisabled) {
		t.Errorf("expected ErrSSODisabled from AuthURL, got %v", err)
	}

	if _, err := sso.ExchangeCode(context.Background(), "abc"); !errors.Is(err, esi.ErrSSODisabled) {
		t.Errorf("expected ErrSSODisabled from ExchangeCode, got %v", err)
	}

	syncClient := esi.NewCharacterSyncClient(esi.NewClient("", "", nil, nil), sso, nil)
	if _, err := syncClient.SyncWallet(context.Background(), &domain.EVEToken{}); !errors.Is(err, esi.ErrSSODisabled) {
		t.Errorf("expected ErrSSODisabled from SyncWallet, got %v", err)
	}
}

func TestSSO_EnabledOAuthAndCharacterSync(t *testing.T) {
	fakeJWT := makeFakeEVEJWT(90000001, "Aura Industrialist")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-ESI-Error-Limit-Remain", "100")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/oauth/token":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  fakeJWT,
				"token_type":    "Bearer",
				"expires_in":    1200,
				"refresh_token": "refresh-tok-xyz",
			})

		case r.URL.Path == "/characters/90000001/wallet/":
			if r.Header.Get("Authorization") != "Bearer "+fakeJWT {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`245750125.50`))

		case r.URL.Path == "/characters/90000001/orders/":
			_, _ = w.Write([]byte(`[{"order_id":555,"type_id":2456,"price":12100,"volume_remain":40,"volume_total":100,"is_buy_order":false,"region_id":10000002,"location_id":60003760,"issued":"2026-11-19T10:00:00Z"}]`))

		case r.URL.Path == "/characters/90000001/industry/jobs/":
			_, _ = w.Write([]byte(`[{"job_id":777,"blueprint_type_id":2457,"product_type_id":2456,"activity_id":1,"runs":10,"status":"active","start_date":"2026-11-19T10:00:00Z","end_date":"2026-11-20T10:00:00Z"}]`))

		case r.URL.Path == "/characters/90000001/blueprints/":
			_, _ = w.Write([]byte(`[{"item_id":888,"type_id":2457,"location_id":60003760,"quantity":-1,"material_efficiency":10,"time_efficiency":20,"runs":-1}]`))

		case r.URL.Path == "/characters/90000001/assets/":
			_, _ = w.Write([]byte(`[{"item_id":999,"type_id":34,"location_id":60003760,"quantity":50000}]`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	sso := esi.NewSSOClient("client-id", "client-secret", "http://localhost:8080/auth/callback", srv.Client())
	sso.SetEndpoints(srv.URL+"/v2/oauth/authorize", srv.URL+"/v2/oauth/token")

	if !sso.IsEnabled() {
		t.Fatalf("expected SSO to be enabled")
	}

	authURL, err := sso.AuthURL("csrf-token")
	if err != nil || !strings.Contains(authURL, "client_id=client-id") || !strings.Contains(authURL, "state=csrf-token") {
		t.Fatalf("unexpected AuthURL: %s (err: %v)", authURL, err)
	}

	ctx := context.Background()
	tok, err := sso.ExchangeCode(ctx, "auth-code-123")
	if err != nil {
		t.Fatalf("ExchangeCode failed: %v", err)
	}
	if tok.CharacterID != 90000001 || tok.CharacterName != "Aura Industrialist" {
		t.Errorf("unexpected parsed JWT identity: id=%d name=%q", tok.CharacterID, tok.CharacterName)
	}

	esiClient := esi.NewClient(srv.URL, "aura-forge-test/1.0", nil, srv.Client())
	syncClient := esi.NewCharacterSyncClient(esiClient, sso, nil)

	bal, err := syncClient.SyncWallet(ctx, tok)
	if err != nil || bal != 245750125.50 {
		t.Errorf("SyncWallet failed: bal=%.2f err=%v", bal, err)
	}

	orders, err := syncClient.SyncOrders(ctx, tok)
	if err != nil || len(orders) != 1 || orders[0].OrderID != 555 {
		t.Errorf("SyncOrders failed: orders=%+v err=%v", orders, err)
	}

	jobs, err := syncClient.SyncJobs(ctx, tok)
	if err != nil || len(jobs) != 1 || jobs[0].ActivityName != "Manufacturing" {
		t.Errorf("SyncJobs failed: jobs=%+v err=%v", jobs, err)
	}

	bps, err := syncClient.SyncBlueprints(ctx, tok)
	if err != nil || len(bps) != 1 || bps[0].MaterialEfficiency != 10 {
		t.Errorf("SyncBlueprints failed: bps=%+v err=%v", bps, err)
	}

	assets, err := syncClient.SyncAssets(ctx, tok)
	if err != nil || len(assets) != 1 || assets[0].Quantity != 50000 {
		t.Errorf("SyncAssets failed: assets=%+v err=%v", assets, err)
	}
}
