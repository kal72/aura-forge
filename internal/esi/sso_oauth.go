package esi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aura-forge/internal/domain"
)

const (
	DefaultSSOAuthorizeURL = "https://login.eveonline.com/v2/oauth/authorize"
	DefaultSSOTokenURL     = "https://login.eveonline.com/v2/oauth/token"
)

// DefaultSSOScopes lists the EVE ESI scopes required for optional character auto-sync.
var DefaultSSOScopes = []string{
	"esi-wallet.read_character_wallet.v1",
	"esi-markets.read_character_orders.v1",
	"esi-industry.read_character_jobs.v1",
	"esi-characters.read_blueprints.v1",
	"esi-assets.read_assets.v1",
}

// SSOClient handles optional EVE Online OAuth2 SSO login and token refresh.
type SSOClient struct {
	clientID     string
	clientSecret string
	callbackURL  string
	authorizeURL string
	tokenURL     string
	httpClient   *http.Client
}

// NewSSOClient constructs an SSOClient. If clientID or clientSecret is empty, SSO remains safely disabled.
func NewSSOClient(clientID, clientSecret, callbackURL string, httpClient *http.Client) *SSOClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultHTTPTimeout}
	}
	return &SSOClient{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		callbackURL:  strings.TrimSpace(callbackURL),
		authorizeURL: DefaultSSOAuthorizeURL,
		tokenURL:     DefaultSSOTokenURL,
		httpClient:   httpClient,
	}
}

// SetEndpoints overrides OAuth2 endpoints (primarily for httptest unit testing).
func (s *SSOClient) SetEndpoints(authorizeURL, tokenURL string) {
	if authorizeURL != "" {
		s.authorizeURL = authorizeURL
	}
	if tokenURL != "" {
		s.tokenURL = tokenURL
	}
}

// IsEnabled returns true if both client ID and secret are configured.
func (s *SSOClient) IsEnabled() bool {
	return s.clientID != "" && s.clientSecret != ""
}

// AuthURL builds the EVE SSO OAuth2 authorization redirect URL.
func (s *SSOClient) AuthURL(state string) (string, error) {
	if !s.IsEnabled() {
		return "", ErrSSODisabled
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("redirect_uri", s.callbackURL)
	params.Set("client_id", s.clientID)
	params.Set("scope", strings.Join(DefaultSSOScopes, " "))
	params.Set("state", state)

	return s.authorizeURL + "?" + params.Encode(), nil
}

type ssoTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// ExchangeCode exchanges an OAuth2 authorization code for an EVE access & refresh token.
func (s *SSOClient) ExchangeCode(ctx context.Context, code string) (*domain.EVEToken, error) {
	if !s.IsEnabled() {
		return nil, ErrSSODisabled
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("authorization code cannot be empty")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)

	return s.requestToken(ctx, form)
}

// RefreshToken refreshes an expired EVE SSO token using its refresh_token.
func (s *SSOClient) RefreshToken(ctx context.Context, token *domain.EVEToken) (*domain.EVEToken, error) {
	if !s.IsEnabled() {
		return nil, ErrSSODisabled
	}
	if token == nil || strings.TrimSpace(token.RefreshToken) == "" {
		return nil, fmt.Errorf("valid refresh token is required")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", token.RefreshToken)

	refreshed, err := s.requestToken(ctx, form)
	if err != nil {
		return nil, err
	}

	// Preserve existing identity if JWT claims weren't present
	if refreshed.CharacterID == 0 {
		refreshed.CharacterID = token.CharacterID
	}
	if refreshed.CharacterName == "" {
		refreshed.CharacterName = token.CharacterName
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}

	return refreshed, nil
}

// EnsureValidToken checks if the token is expiring within 60 seconds and refreshes + persists it automatically.
func (s *SSOClient) EnsureValidToken(ctx context.Context, token *domain.EVEToken, repo domain.CharacterRepository) (*domain.EVEToken, error) {
	if !s.IsEnabled() {
		return nil, ErrSSODisabled
	}
	if token == nil {
		return nil, fmt.Errorf("no character token available")
	}
	if time.Until(token.ExpiresAt) > 60*time.Second {
		return token, nil
	}

	refreshed, err := s.RefreshToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("auto-refreshing EVE token: %w", err)
	}
	if repo != nil {
		_ = repo.SaveToken(ctx, refreshed)
	}
	return refreshed, nil
}

func (s *SSOClient) requestToken(ctx context.Context, form url.Values) (*domain.EVEToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating SSO token request: %w", err)
	}

	basicAuth := base64.StdEncoding.EncodeToString([]byte(s.clientID + ":" + s.clientSecret))
	req.Header.Set("Authorization", "Basic "+basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing SSO token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading SSO token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("EVE SSO token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed ssoTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding SSO token response: %w", err)
	}

	charID, charName, scope := parseEVEJWTClaims(parsed.AccessToken)
	expiresIn := parsed.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 1199
	}

	return &domain.EVEToken{
		CharacterID:   charID,
		CharacterName: charName,
		AccessToken:   parsed.AccessToken,
		RefreshToken:  parsed.RefreshToken,
		ExpiresAt:     time.Now().Add(time.Duration(expiresIn) * time.Second),
		Scope:         scope,
		UpdatedAt:     time.Now(),
	}, nil
}

type eveJWTClaims struct {
	Sub  string `json:"sub"`
	Name string `json:"name"`
	Scp  any    `json:"scp"`
}

// parseEVEJWTClaims extracts CharacterID, CharacterName, and scopes from an EVE SSO v2 JWT access token.
func parseEVEJWTClaims(jwtToken string) (int64, string, string) {
	parts := strings.Split(jwtToken, ".")
	if len(parts) < 2 {
		return 0, "", ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, "", ""
	}

	var claims eveJWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, "", ""
	}

	// Sub format: "CHARACTER:EVE:<character_id>"
	var charID int64
	subParts := strings.Split(claims.Sub, ":")
	if len(subParts) == 3 {
		if id, err := strconv.ParseInt(subParts[2], 10, 64); err == nil {
			charID = id
		}
	}

	var scopeStr string
	switch v := claims.Scp.(type) {
	case string:
		scopeStr = v
	case []any:
		scopes := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				scopes = append(scopes, s)
			}
		}
		scopeStr = strings.Join(scopes, " ")
	}

	return charID, claims.Name, scopeStr
}
