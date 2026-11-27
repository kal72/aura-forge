package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
	ItemID             int64  `json:"item_id"`
	TypeID             int32  `json:"type_id"`
	LocationID         int64  `json:"location_id"`
	LocationFlag       string `json:"location_flag"`
	Quantity           int    `json:"quantity"`
	MaterialEfficiency int    `json:"material_efficiency"`
	TimeEfficiency     int    `json:"time_efficiency"`
	Runs               int    `json:"runs"`
}

type esiCharPublicInfo struct {
	CorporationID int64 `json:"corporation_id"`
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
			LocationFlag:       r.LocationFlag,
			Quantity:           r.Quantity,
			MaterialEfficiency: r.MaterialEfficiency,
			TimeEfficiency:     r.TimeEfficiency,
			Runs:               r.Runs,
			IsCopy:             r.Runs > 0 || r.Quantity == -2,
			IsCorp:             false,
			UpdatedAt:          now,
		})
	}
	return bps, nil
}

// SyncCorpBlueprints fetches blueprints owned by the character's corporation
// (requires esi-corporations.read_blueprints.v1 scope and in-game Director role).
func (c *CharacterSyncClient) SyncCorpBlueprints(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterBlueprint, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Resolve character's corporation_id via cached public ESI
	charBody, err := c.esiClient.DoGet(ctx, fmt.Sprintf("/characters/%d/?datasource=tranquility", validTok.CharacterID), "", true)
	if err != nil {
		return nil, fmt.Errorf("resolving character corporation_id: %w", err)
	}
	var pub esiCharPublicInfo
	if err := json.Unmarshal(charBody, &pub); err != nil || pub.CorporationID <= 0 {
		return nil, fmt.Errorf("character has no valid corporation_id")
	}

	const maxCorpBPPages = 10
	now := time.Now()
	bps := make([]domain.CharacterBlueprint, 0, 64)
	totalPages := 1

	for page := 1; page <= totalPages && page <= maxCorpBPPages; page++ {
		path := fmt.Sprintf("/corporations/%d/blueprints/?datasource=tranquility&page=%d", pub.CorporationID, page)
		body, headers, err := c.esiClient.DoGetWithHeaders(ctx, path, validTok.AccessToken, false)
		if err != nil {
			if page == 1 {
				return nil, fmt.Errorf("syncing corporation blueprints: %w", err)
			}
			break
		}
		if page == 1 && headers != nil {
			if xp, pErr := strconv.Atoi(headers.Get("X-Pages")); pErr == nil && xp > 1 {
				totalPages = xp
			}
		}
		var raw []esiCharBlueprint
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("decoding corporation blueprints page %d: %w", page, err)
		}
		for _, r := range raw {
			bps = append(bps, domain.CharacterBlueprint{
				ItemID:             r.ItemID,
				CharacterID:        validTok.CharacterID,
				TypeID:             r.TypeID,
				TypeName:           fmt.Sprintf("Blueprint #%d", r.TypeID),
				LocationID:         r.LocationID,
				LocationFlag:       r.LocationFlag,
				Quantity:           r.Quantity,
				MaterialEfficiency: r.MaterialEfficiency,
				TimeEfficiency:     r.TimeEfficiency,
				Runs:               r.Runs,
				IsCopy:             r.Runs > 0 || r.Quantity == -2,
				IsCorp:             true,
				UpdatedAt:          now,
			})
		}
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

type esiWalletTransaction struct {
	TransactionID int64   `json:"transaction_id"`
	Date          string  `json:"date"`
	TypeID        int32   `json:"type_id"`
	LocationID    int64   `json:"location_id"`
	UnitPrice     float64 `json:"unit_price"`
	Quantity      int     `json:"quantity"`
	IsBuy         bool    `json:"is_buy"`
}

// SyncWalletTransactions fetches recent realized buy/sell transactions via Private ESI
// using the existing esi-wallet.read_character_wallet.v1 scope.
func (c *CharacterSyncClient) SyncWalletTransactions(ctx context.Context, token *domain.EVEToken) ([]domain.MarketTransaction, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/wallet/transactions/?datasource=tranquility", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing wallet transactions: %w", err)
	}

	var raw []esiWalletTransaction
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding wallet transactions: %w", err)
	}

	out := make([]domain.MarketTransaction, 0, len(raw))
	for _, r := range raw {
		ts, _ := time.Parse(time.RFC3339, r.Date)
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		side := domain.TransactionSideSell
		if r.IsBuy {
			side = domain.TransactionSideBuy
		}
		extID := r.TransactionID
		tx := domain.MarketTransaction{
			CharacterID:  validTok.CharacterID,
			ExternalTxID: &extID,
			TransactedAt: ts.UTC(),
			Side:         side,
			TypeID:       r.TypeID,
			ItemName:     fmt.Sprintf("Type #%d", r.TypeID),
			Quantity:     r.Quantity,
			UnitPrice:    domain.RoundISK(r.UnitPrice),
			TotalISK:     domain.RoundISK(r.UnitPrice * float64(r.Quantity)),
			LocationID:   r.LocationID,
			HubName:      mapStationIDToHubName(r.LocationID),
			Source:       domain.TransactionSourceESI,
		}
		tx.DedupHash = domain.ComputeTransactionDedupHash(tx)
		out = append(out, tx)
	}
	return out, nil
}

type esiWalletJournalEntry struct {
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	RefType     string  `json:"ref_type"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
}

// SyncWalletJournalFees fetches market/industry/PI tax & broker fee entries from Private ESI wallet/journal.
func (c *CharacterSyncClient) SyncWalletJournalFees(ctx context.Context, token *domain.EVEToken) ([]domain.WalletFeeEntry, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/wallet/journal/?datasource=tranquility&page=1", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing wallet journal fees: %w", err)
	}

	var raw []esiWalletJournalEntry
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding wallet journal: %w", err)
	}

	trackedTypes := map[string]bool{
		"brokers_fee":          true,
		"transaction_tax":      true,
		"industry_job_tax":     true,
		"planetary_import_tax": true,
		"planetary_export_tax": true,
	}

	out := make([]domain.WalletFeeEntry, 0, len(raw)/4+1)
	for _, r := range raw {
		if !trackedTypes[r.RefType] {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, r.Date)
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		amt := r.Amount
		if amt < 0 {
			amt = -amt
		}
		out = append(out, domain.WalletFeeEntry{
			CharacterID: validTok.CharacterID,
			JournalID:   r.ID,
			RecordedAt:  ts.UTC(),
			RefType:     r.RefType,
			AmountISK:   domain.RoundISK(amt),
			Description: r.Description,
		})
	}
	return out, nil
}

type esiOrderHistory struct {
	OrderID      int64   `json:"order_id"`
	TypeID       int32   `json:"type_id"`
	IsBuyOrder   bool    `json:"is_buy_order"`
	Price        float64 `json:"price"`
	VolumeTotal  int     `json:"volume_total"`
	VolumeRemain int     `json:"volume_remain"`
	State        string  `json:"state"`
	RegionID     int32   `json:"region_id"`
	LocationID   int64   `json:"location_id"`
	Issued       string  `json:"issued"`
	Duration     int     `json:"duration"`
}

// SyncOrderHistory fetches completed, expired, and cancelled orders from the last 90 days via Private ESI.
func (c *CharacterSyncClient) SyncOrderHistory(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterOrderHistory, error) {
	validTok, err := c.requireValidToken(ctx, token)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/characters/%d/orders/history/?datasource=tranquility&page=1", validTok.CharacterID)
	body, err := c.esiClient.DoGet(ctx, path, validTok.AccessToken, false)
	if err != nil {
		return nil, fmt.Errorf("syncing order history: %w", err)
	}

	var raw []esiOrderHistory
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding order history: %w", err)
	}

	now := time.Now().UTC()
	out := make([]domain.CharacterOrderHistory, 0, len(raw))
	for _, r := range raw {
		issued, _ := time.Parse(time.RFC3339, r.Issued)
		if issued.IsZero() {
			issued = now
		}
		out = append(out, domain.CharacterOrderHistory{
			OrderID:      r.OrderID,
			CharacterID:  validTok.CharacterID,
			TypeID:       r.TypeID,
			TypeName:     fmt.Sprintf("Type #%d", r.TypeID),
			IsBuyOrder:   r.IsBuyOrder,
			Price:        domain.RoundISK(r.Price),
			VolumeTotal:  r.VolumeTotal,
			VolumeRemain: r.VolumeRemain,
			State:        r.State,
			RegionID:     r.RegionID,
			LocationID:   r.LocationID,
			IssuedAt:     issued.UTC(),
			DurationDays: r.Duration,
			UpdatedAt:    now,
		})
	}
	return out, nil
}

func mapStationIDToHubName(locationID int64) string {
	for _, h := range domain.MajorTradeHubs {
		if h.StationID == locationID {
			return h.Name
		}
	}
	if locationID > 0 {
		return fmt.Sprintf("Station #%d", locationID)
	}
	return ""
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

func (n *NoOpCharacterSyncProvider) SyncCorpBlueprints(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterBlueprint, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncAssets(ctx context.Context, token *domain.EVEToken) ([]domain.HangarAsset, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncWalletTransactions(ctx context.Context, token *domain.EVEToken) ([]domain.MarketTransaction, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncWalletJournalFees(ctx context.Context, token *domain.EVEToken) ([]domain.WalletFeeEntry, error) {
	return nil, ErrSSODisabled
}

func (n *NoOpCharacterSyncProvider) SyncOrderHistory(ctx context.Context, token *domain.EVEToken) ([]domain.CharacterOrderHistory, error) {
	return nil, ErrSSODisabled
}
