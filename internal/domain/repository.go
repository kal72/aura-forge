package domain

import (
	"context"
	"time"
)

// JitaPrice holds market snapshot data for an item in Jita.
type JitaPrice struct {
	TypeID          int32   `json:"type_id"`
	LowestSell      float64 `json:"lowest_sell"`
	HighestBuy      float64 `json:"highest_buy"`
	SellOrdersCount int     `json:"sell_orders_count"`
	BuyOrdersCount  int     `json:"buy_orders_count"`
	UpdatedTime     time.Time `json:"updated_time"`
}

// ItemRepository specifies storage operations for catalog items.
type ItemRepository interface {
	ListItems(ctx context.Context) ([]MarketItem, error)
	GetItemByID(ctx context.Context, id int64) (*MarketItem, error)
	GetItemByTypeID(ctx context.Context, typeID int32) (*MarketItem, error)
	SaveItem(ctx context.Context, item *MarketItem) error
	DeleteItem(ctx context.Context, id int64) error
	BatchUpsertItems(ctx context.Context, items []MarketItem) error
}

// BatchRepository specifies storage operations for production batches.
type BatchRepository interface {
	ListBatches(ctx context.Context, status *BatchStatus) ([]BatchRecord, error)
	GetBatchByID(ctx context.Context, id int64) (*BatchRecord, error)
	SaveBatch(ctx context.Context, batch *BatchRecord) error
	UpdateBatchStatus(ctx context.Context, id int64, status BatchStatus) error
	DeleteBatch(ctx context.Context, id int64) error
	CreateBatchesFromPlan(ctx context.Context, planID int64, batches []BatchRecord) error
}

// PlanRepository specifies storage operations for weekly production plans.
type PlanRepository interface {
	ListPlans(ctx context.Context) ([]WeeklyPlan, error)
	GetPlanByID(ctx context.Context, id int64) (*WeeklyPlan, error)
	GetActivePlan(ctx context.Context) (*WeeklyPlan, error)
	SavePlan(ctx context.Context, plan *WeeklyPlan) error
	DeletePlan(ctx context.Context, id int64) error
}

// CharacterRepository specifies storage operations for character synced data and tokens.
type CharacterRepository interface {
	GetProfile(ctx context.Context, characterID int64) (*CharacterProfile, error)
	UpsertProfile(ctx context.Context, profile *CharacterProfile) error
	GetToken(ctx context.Context, characterID int64) (*EVEToken, error)
	GetPrimaryToken(ctx context.Context) (*EVEToken, error)
	SaveToken(ctx context.Context, token *EVEToken) error
	DeleteToken(ctx context.Context, characterID int64) error
	ListOrders(ctx context.Context, characterID int64) ([]CharacterOrder, error)
	UpsertOrders(ctx context.Context, characterID int64, orders []CharacterOrder) error
	ListJobs(ctx context.Context, characterID int64) ([]CharacterJob, error)
	UpsertJobs(ctx context.Context, characterID int64, jobs []CharacterJob) error
	ListBlueprints(ctx context.Context, characterID int64) ([]CharacterBlueprint, error)
	UpsertBlueprints(ctx context.Context, characterID int64, bps []CharacterBlueprint) error
	ListAssets(ctx context.Context, characterID int64) ([]HangarAsset, error)
	UpsertAssets(ctx context.Context, characterID int64, assets []HangarAsset) error
}

// SettingsRepository specifies storage operations for user simulation settings.
type SettingsRepository interface {
	GetSettings(ctx context.Context) (*UserSettings, error)
	SaveSettings(ctx context.Context, settings *UserSettings) error
}

// ESICacheRepository specifies caching operations for ESI HTTP responses.
type ESICacheRepository interface {
	Get(ctx context.Context, cacheKey string) (data []byte, etag string, expiresAt time.Time, found bool, err error)
	Set(ctx context.Context, cacheKey string, data []byte, etag string, expiresAt time.Time) error
	CleanExpired(ctx context.Context) error
}

// MarketDataProvider specifies operations to fetch public ESI market intelligence without authentication.
type MarketDataProvider interface {
	ResolveTypeIDs(ctx context.Context, names []string) (map[string]int32, error)
	GetJitaPrices(ctx context.Context, typeIDs []int32) (map[int32]JitaPrice, error)
	GetDailyVolume(ctx context.Context, typeID int32) (avg7d float64, avg30d float64, err error)
	GetSystemCostIndex(ctx context.Context, solarSystemID int32) (map[string]float64, error)
}

// CharacterSyncProvider specifies operations to fetch authenticated character data via ESI OAuth2.
type CharacterSyncProvider interface {
	SyncWallet(ctx context.Context, token *EVEToken) (float64, error)
	SyncOrders(ctx context.Context, token *EVEToken) ([]CharacterOrder, error)
	SyncJobs(ctx context.Context, token *EVEToken) ([]CharacterJob, error)
	SyncBlueprints(ctx context.Context, token *EVEToken) ([]CharacterBlueprint, error)
	SyncAssets(ctx context.Context, token *EVEToken) ([]HangarAsset, error)
}
