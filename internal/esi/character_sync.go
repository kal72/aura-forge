package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aura-forge/internal/domain"
)

// CharacterSyncClient implements domain.CharacterSyncProvider using authenticated ESI endpoints.
// If SSO is disabled, every method fails safely with ErrSSODisabled without making network requests.
type CharacterSyncClient struct {
	esiClient *Client
	ssoClient *SSOClient
	charRepo  domain.CharacterRepository
}

// NewCharacterSyncClient constructs an authenticated CharacterSyncProvider.
func NewCharacterSyncClient(esiClient *Client, ssoClient *SSOClient, charRepo domain.CharacterRepository) *CharacterSyncClient {
	return &CharacterSyncClient{
		esiClient: esiClient,
		ssoClient: ssoClient,
		charRepo:  charRepo,
	}
}

func (c *CharacterSyncClient) requireValidToken(ctx context.Context, token *domain.EVEToken) (*domain.EVEToken, error) {
	if c.ssoClient == nil || !c.ssoClient.IsEnabled() {
		return nil, ErrSSODisabled
	}
	return c.ssoClient.EnsureValidToken(ctx, token, c.charRepo)
}

// SyncWallet fetches the character's ISK wallet balance.
func (c *CharacterSyncClient) SyncWallet(ctx context.Context, token *domain.EVEToken) (float64, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return 0, err
	}

	path := fmt.Sprintf("/characters/%d/wallet/?datasource=tranquility", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return 0, fmt.Errorf("syncing character wallet: %w", err)
	}

	var balance float64
	if err := json.Unmarshal(body, &balance); err != nil {
		return 0, fmt.Errorf("decoding wallet balance: %w", err)
	}
	return domain.RoundISK(balance), nil
}

type esiCharOrder struct {
	OrderID      int64   `json:"order_id"`
	TypeID       int32   `json:"type_id"`
	Price        float64 `json:"price"`
	VolumeRemain int     `json:"volume_remain"`
	VolumeTotal  int     `json:"volume_total"`
	IsBuyOrder   bool    `json:"is_buy_order"`
	RegionID     int32   `json:"region_id"`
	LocationID   int64   `json:"location_id"`
	Issued       string  `json:"issued"`
}

// SyncOrders fetches active character market orders.
func (c *CharacterSyncClient) SyncOrders(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterOrder, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/orders/?datasource=tranquility", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing character orders: %w", err)
	}

	var raw []esiCharOrder
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding character orders: %w", err)
	}

	now := time.Now()
	orders := make([]domain.CharacterOrder, 0, len(raw))
	for _, r := range raw {
		issued, _ := time.Parse(time.RFC3339, r.Issued)
		if issued.IsZero() {
			issued = now
		}
		orders = append(orders, domain.CharacterOrder{
			OrderID:      r.OrderID,
			CharacterID:  validTok.CharacterID,
			TypeID:       r.TypeID,
			TypeName:     fmt.Sprintf("Type #%d", r.TypeID),
			Price:        domain.RoundISK(r.Price),
			VolumeRemain: r.VolumeRemain,
			VolumeTotal:  r.VolumeTotal,
			IsBuyOrder:   r.IsBuyOrder,
			RegionID:     r.RegionID,
			LocationID:   r.LocationID,
			IssuedAt:     issued,
			UpdatedAt:    now,
		})
	}
	return orders, nil
}

type esiCharJob struct {
	JobID               int64  `json:"job_id"`
	BlueprintTypeID     int32  `json:"blueprint_type_id"`
	ProductTypeID       int32  `json:"product_type_id"`
	ActivityID          int    `json:"activity_id"`
	Runs                int    `json:"runs"`
	Status              string `json:"status"`
	StartDate           string `json:"start_date"`
	EndDate             string `json:"end_date"`
	CompletedDate       string `json:"completed_date,omitempty"`
}

// SyncJobs fetches active and recent industry jobs for the character.
func (c *CharacterSyncClient) SyncJobs(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterJob, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/industry/jobs/?datasource=tranquility&include_completed=false", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing character jobs: %w", err)
	}

	var raw []esiCharJob
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding character jobs: %w", err)
	}

	now := time.Now()
	jobs := make([]domain.CharacterJob, 0, len(raw))
	for _, r := range raw {
		startDate, _ := time.Parse(time.RFC3339, r.StartDate)
		endDate, _ := time.Parse(time.RFC3339, r.EndDate)
		var completedPtr *time.Time
		if r.CompletedDate != "" {
			if t, err := time.Parse(time.RFC3339, r.CompletedDate); err == nil {
				completedPtr = &t
			}
		}

		typeID := r.ProductTypeID
		if typeID == 0 {
			typeID = r.BlueprintTypeID
		}

		jobs = append(jobs, domain.CharacterJob{
			JobID:         r.JobID,
			CharacterID:   validTok.CharacterID,
			TypeID:        typeID,
			TypeName:      fmt.Sprintf("Type #%d", typeID),
			ActivityID:    r.ActivityID,
			ActivityName:  mapActivityIDToName(r.ActivityID),
			Runs:          r.Runs,
			Status:        r.Status,
			StartDate:     startDate,
			EndDate:       endDate,
			CompletedDate: completedPtr,
			UpdatedAt:     now,
		})
	}
	return jobs, nil
}

type esiCharBlueprint struct {
	ItemID             int64 `json:"item_id"`
	TypeID             int32 `json:"type_id"`
	LocationID         int64 `json:"location_id"`
	Quantity           int   `json:"quantity"`
	MaterialEfficiency int   `json:"material_efficiency"`
	TimeEfficiency     int   `json:"time_efficiency"`
	Runs               int   `json:"runs"`
}

// SyncBlueprints fetches blueprints owned by the character.
func (c *CharacterSyncClient) SyncBlueprints(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterBlueprint, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/blueprints/?datasource=tranquility", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing character blueprints: %w", err)
	}

	var raw []esiCharBlueprint
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding character blueprints: %w", err)
	}

	now := time.Now()
	bps := make([]domain.CharacterBlueprint, 0, len(raw))
	for _, r := range raw {
		bps = append(bps, domain.CharacterBlueprint{
			ItemID:             r.ItemID,
			CharacterID:        validTok.CharacterID,
			TypeID:             r.TypeID,
			TypeName:           fmt.Sprintf("Blueprint #%d", r.TypeID),
			LocationID:         r.LocationID,
			Quantity:           r.Quantity,
			MaterialEfficiency: r.MaterialEfficiency,
			TimeEfficiency:     r.TimeEfficiency,
			Runs:               r.Runs,
			IsCopy:             r.Runs > 0 || r.Quantity == -2,
			UpdatedAt:          now,
		})
	}
	return bps, nil
}

type esiCharAsset struct {
	ItemID     int64 `json:"item_id"`
	TypeID     int32 `json:"type_id"`
	LocationID int64 `json:"location_id"`
	Quantity   int64 `json:"quantity"`
}

// SyncAssets fetches hangar assets owned by the character for shopping list deduction.
func (c *CharacterSyncClient) SyncAssets(ctx context.Context, token *domain.EVEToken) ([]domain.HangarAsset, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/assets/?datasource=tranquility", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing character assets: %w", err)
	}

	var raw []esiCharAsset
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding character assets: %w", err)
	}

	now := time.Now()
	assets := make([]domain.HangarAsset, 0, len(raw))
	for _, r := range raw {
		assets = append(assets, domain.HangarAsset{
			ItemID:      r.ItemID,
			CharacterID: validTok.CharacterID,
			TypeID:      r.TypeID,
			TypeName:    fmt.Sprintf("Type #%d", r.TypeID),
			LocationID:  r.LocationID,
			Quantity:    r.Quantity,
			UpdatedAt:   now,
		})
	}
	return assets, nil
}

func mapActivityIDToName(id int) string {
	switch id {
	case 1:
		return "Manufacturing"
	case 3:
		return "Time Efficiency Research"
	case 4:
		return "Material Efficiency Research"
	case 5:
		return "Copying"
	case 8:
		return "Invention"
	case 9:
		return "Reactions"
	default:
		return fmt.Sprintf("Activity #%d", id)
	}
}

// NoOpCharacterSyncProvider is a safe fallback when EVE SSO is not configured in .env.
type NoOpCharacterSyncProvider struct{}

func NewNoOpCharacterSyncProvider() *NoOpCharacterSyncProvider {
	return &NoOpCharacterSyncProvider{}
}

func (n *NoOpCharacterSyncProvider) SyncWallet(ctx context.Context, token *domain.EVEToken) (float64, error) {
	return 0, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncOrders(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterOrder, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncJobs(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterJob, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncBlueprints(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterBlueprint, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncAssets(ctx context.Context, token *domain.EVEToken) ([]domain.HangarAsset, error) {
	return nil, ErrSSODisabled
}
