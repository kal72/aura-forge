package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aura-forge/internal/domain"
	"aura-forge/internal/esi"
	apphttp "aura-forge/internal/http"
	"aura-forge/internal/service"
	"aura-forge/internal/web"
)

type memSettingsRepo struct {
	cfg domain.UserSettings
}

func (m *memSettingsRepo) GetSettings(_ context.Context) (*domain.UserSettings, error) {
	cp := m.cfg
	return &cp, nil
}

func (m *memSettingsRepo) SaveSettings(_ context.Context, s *domain.UserSettings) error {
	m.cfg = *s
	return nil
}

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

func TestHandleSaveSettings_PriceUpdaterIntervalPersistsAndRendersSelected(t *testing.T) {
	def := domain.DefaultUserSettings()
	def.AutoPriceSyncEnabled = false
	def.AutoPriceSyncIntervalMinutes = 15
	repo := &memSettingsRepo{cfg: def}

	sso := esi.NewSSOClient("", "", "http://localhost:8080/auth/callback", nil)
	svc := service.NewCopilotService(
		nil, nil, nil, nil, repo,
		esi.NewPublicMarketClient(esi.NewClient("", "test", nil, nil)),
		esi.NewNoOpCharacterSyncProvider(),
		sso,
	)
	router := apphttp.NewRouter(svc, web.NewRenderer())

	form := url.Values{}
	form.Set("form_section", "price_updater")
	form.Set("auto_price_sync_enabled", "on")
	form.Set("auto_price_sync_interval_minutes", "30")

	postReq := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("HX-Request", "true")
	postRec := httptest.NewRecorder()
	router.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusOK {
		t.Fatalf("POST /settings expected 200, got %d: %s", postRec.Code, postRec.Body.String())
	}
	if got := postRec.Header().Get("HX-Redirect"); got != "/settings?saved=1" {
		t.Errorf("expected HX-Redirect /settings?saved=1, got %q", got)
	}
	if !repo.cfg.AutoPriceSyncEnabled || repo.cfg.AutoPriceSyncIntervalMinutes != 30 {
		t.Fatalf("expected persisted enabled=true interval=30, got enabled=%v interval=%d",
			repo.cfg.AutoPriceSyncEnabled, repo.cfg.AutoPriceSyncIntervalMinutes)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/settings?saved=1", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /settings?saved=1 expected 200, got %d", getRec.Code)
	}
	if cc := getRec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("expected Cache-Control to include no-store, got %q", cc)
	}
	body := getRec.Body.String()
	if !strings.Contains(body, `<option value="30" selected>`) {
		t.Errorf("expected option 30 to be selected in rendered HTML")
	}
	if strings.Contains(body, `<option value="15" selected>`) {
		t.Errorf("did not expect option 15 to remain selected in rendered HTML")
	}
	if !strings.Contains(body, "Settings saved successfully to PostgreSQL.") {
		t.Errorf("expected saved confirmation banner on /settings?saved=1")
	}
}
