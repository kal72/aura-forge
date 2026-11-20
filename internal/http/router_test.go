package http_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aura-forge/internal/esi"
	apphttp "aura-forge/internal/http"
	"aura-forge/internal/service"
	"aura-forge/internal/web"
)

func TestRouter_StandardPublicESIModeEndpoints(t *testing.T) {
	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)
	svc := service.NewCopilotService(
		nil, nil, nil, nil, nil,
		esi.NewPublicMarketClient(esi.NewClient("", "test", nil, nil)),
		esi.NewNoOpCharacterSyncProvider(),
		sso,
	)

	router := apphttp.NewRouter(svc, nil)

	getPaths := []string{
		"/health",
		"/",
		"/catalog",
		"/calculator",
		"/allocator",
		"/planner",
		"/tracker",
		"/settings",
	}

	for _, path := range getPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s expected 200, got %d (body: %s)", path, rec.Code, rec.Body.String())
		}
	}

	// Verify /auth/login returns clear info (503 JSON or 200 HTMX banner) when SSO is not configured
	reqSSO := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	recSSO := httptest.NewRecorder()
	router.ServeHTTP(recSSO, reqSSO)
	if recSSO.Code != http.StatusServiceUnavailable || !strings.Contains(recSSO.Body.String(), "PUBLIC_ESI_STANDARD") {
		t.Errorf("GET /auth/login expected 503 with PUBLIC_ESI_STANDARD info, got %d: %s", recSSO.Code, recSSO.Body.String())
	}

	// Verify HTMX POST /calculator/compute
	form := url.Values{}
	form.Set("item_name", "Hobgoblin I")
	form.Set("output_qty", "500")
	form.Set("unit_sell_price", "14500")
	form.Set("material_cost", "4900000")
	form.Set("batches_per_week", "4")
	reqCalc := httptest.NewRequest(http.MethodPost, "/calculator/compute", strings.NewReader(form.Encode()))
	reqCalc.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCalc.Header.Set("HX-Request", "true")
	recCalc := httptest.NewRecorder()
	router.ServeHTTP(recCalc, reqCalc)
	if recCalc.Code != http.StatusOK || !strings.Contains(recCalc.Body.String(), "net_profit_per_batch") {
		t.Errorf("POST /calculator/compute failed: %d %s", recCalc.Code, recCalc.Body.String())
	}
}

func TestRouter_TemplUIRendering(t *testing.T) {
	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)
	svc := service.NewCopilotService(
		nil, nil, nil, nil, nil,
		esi.NewPublicMarketClient(esi.NewClient("", "test", nil, nil)),
		esi.NewNoOpCharacterSyncProvider(),
		sso,
	)

	router := apphttp.NewRouter(svc, web.NewRenderer())

	pages := map[string]string{
		"/":           "Command Center // Dashboard",
		"/catalog":    "Catalog &amp; Trap Guard Advisor",
		"/calculator": "Net Profit, Tax &amp; Break-Even Calculator",
		"/allocator":  "Smart Capital Allocator",
		"/planner":    "Weekly Production &amp; Slot Planner",
		"/tracker":    "Production, Inventory &amp; Market Order Tracker",
		"/settings":   "Settings &amp; EVE Online ESI Mode",
	}

	for path, expectedTitle := range pages {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s expected 200, got %d", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, expectedTitle) {
			t.Errorf("GET %s missing expected heading %q", path, expectedTitle)
		}
		if !strings.Contains(body, "Public ESI Active") {
			t.Errorf("GET %s missing Public ESI Active mode indicator", path)
		}
	}
}
