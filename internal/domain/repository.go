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

// MarketBrowserProvider is an optional Public ESI extension for full order book & multi-hub marketplace browsing.
type MarketBrowserProvider interface {
	GetRegionOrderBook(ctx context.Context, hub TradeHub, typeID int32, limit int) (sells []MarketOrderBookEntry, buys []MarketOrderBookEntry, summary HubPriceSummary, err error)
	GetMarketHistory(ctx context.Context, regionID int32, typeID int32, days int) ([]MarketHistoryPoint, error)
}

// GlobalMarketPriceProvider is an optional Public ESI extension for 1-call global /markets/prices/ snapshots
// and batch /universe/names/ resolution.
type GlobalMarketPriceProvider interface {
	GetGlobalMarketPrices(ctx context.Context) ([]GlobalMarketPrice, error)
	ResolveTypeNames(ctx context.Context, typeIDs []int32) (map[int32]string, error)
}

// GlobalPriceSnapshotRepository persists daily global EVE market price snapshots for day-over-day mover detection.
type GlobalPriceSnapshotRepository interface {
	UpsertDailySnapshot(ctx context.Context, snapshotDate time.Time, prices []GlobalMarketPrice) (int, error)
	ListTopPriceMovers(ctx context.Context, limit int, minPrevPrice float64) ([]GlobalPriceMover, error)
	GetSnapshotStats(ctx context.Context) (*GlobalSnapshotStats, error)
	PruneOldSnapshots(ctx context.Context, keepDays int) error
}

// SDERepository specifies storage operations for EVE Online Static Data Export blueprints and materials.
type SDERepository interface {
	SearchBlueprints(ctx context.Context, query, category string, limit int) ([]SDEBlueprint, error)
	GetBlueprintByID(ctx context.Context, blueprintTypeID int32) (*SDEBlueprint, error)
	GetBlueprintByProductID(ctx context.Context, productTypeID int32) (*SDEBlueprint, error)
	GetBlueprintMaterials(ctx context.Context, blueprintTypeID int32) ([]SDEMaterialRequirement, error)
	UpsertSDEData(ctx context.Context, types []SDEType, blueprints []SDEBlueprint, materials map[int32][]SDEMaterialRequirement) error
}

// SDEExtendedRepository is an optional extension on SDERepository supporting paginated modal browsing and SDE metadata.
type SDEExtendedRepository interface {
	SearchBlueprintsPaged(ctx context.Context, query, category string, techLevel int, page, pageSize int) (*BlueprintPageResult, error)
	GetSDEMetadata(ctx context.Context) (*SDEMetadata, error)
	SaveSDEMetadata(ctx context.Context, meta *SDEMetadata) error
}

// SDEInventionRepository is an optional extension on SDERepository for T2 invention pathways and reaction formulas.
type SDEInventionRepository interface {
	UpsertInventionData(ctx context.Context, inventions []SDEInvention, materials map[int32][]SDEInventionMaterial) error
	SearchInventions(ctx context.Context, query string, limit int) ([]SDEInvention, error)
	GetInventionByT2BlueprintID(ctx context.Context, t2BlueprintTypeID int32) (*SDEInvention, []SDEInventionMaterial, error)
	ListReactionBlueprints(ctx context.Context, query string, limit int) ([]SDEBlueprint, error)
}

// SDETypeSearchRepository is an optional extension on SDERepository for fast local Marketplace autocomplete & type browsing.
type SDETypeSearchRepository interface {
	SearchTypes(ctx context.Context, query string, limit int) ([]SDEType, error)
	BrowseTypesPaged(ctx context.Context, query, category, groupName string, page, pageSize int) (*MarketTypePageResult, error)
}

// SDETypeNameResolver is an optional extension on SDERepository for instant local TypeID -> Name resolution.
type SDETypeNameResolver interface {
	GetTypeNamesByIDs(ctx context.Context, typeIDs []int32) (map[int32]string, error)
}

// NetWorthRepository specifies storage operations for daily Net Worth tracking snapshots.
type NetWorthRepository interface {
	ListSnapshots(ctx context.Context, limit int) ([]NetWorthSnapshot, error)
	UpsertSnapshot(ctx context.Context, snap *NetWorthSnapshot) error
	DeleteSnapshot(ctx context.Context, id int64) error
}

// PIRepository specifies storage operations for player Planetary Industry colonies.
type PIRepository interface {
	ListColonies(ctx context.Context) ([]PIColony, error)
	SaveColony(ctx context.Context, c *PIColony) error
	ResetExtractor(ctx context.Context, id int64, newExpiresAt time.Time) error
	DeleteColony(ctx context.Context, id int64) error
}

// CharacterSyncProvider specifies operations to fetch authenticated character data via ESI OAuth2.
type CharacterSyncProvider interface {
	SyncWallet(ctx context.Context, token *EVEToken) (float64, error)
	SyncOrders(ctx context.Context, token *EVEToken) ([]CharacterOrder, error)
	SyncJobs(ctx context.Context, token *EVEToken) ([]CharacterJob, error)
	SyncBlueprints(ctx context.Context, token *EVEToken) ([]CharacterBlueprint, error)
	SyncAssets(ctx context.Context, token *EVEToken) ([]HangarAsset, error)
}

// CharacterLedgerSyncProvider is an optional Private ESI extension for wallet transactions,
// realized tax/fee journal entries, and 90-day order history (uses existing wallet & order scopes).
type CharacterLedgerSyncProvider interface {
	SyncWalletTransactions(ctx context.Context, token *EVEToken) ([]MarketTransaction, error)
	SyncWalletJournalFees(ctx context.Context, token *EVEToken) ([]WalletFeeEntry, error)
	SyncOrderHistory(ctx context.Context, token *EVEToken) ([]CharacterOrderHistory, error)
}

// TradeLedgerRepository persists realized market transactions, fee journal entries, and historical orders.
type TradeLedgerRepository interface {
	UpsertTransactions(ctx context.Context, txs []MarketTransaction) (inserted int, skipped int, err error)
	ListTransactions(ctx context.Context, periodDays int, sideFilter string, query string, includeExcluded bool, limit int) ([]MarketTransaction, error)
	SetTransactionsExcluded(ctx context.Context, ids []int64, excluded bool) (int, error)
	CountExcludedTransactions(ctx context.Context, periodDays int) (int, error)
	DeleteTransaction(ctx context.Context, id int64) error
	UpsertFeeEntries(ctx context.Context, entries []WalletFeeEntry) (int, error)
	ListFeeEntries(ctx context.Context, periodDays int) ([]WalletFeeEntry, error)
	UpsertOrderHistory(ctx context.Context, orders []CharacterOrderHistory) (int, error)
	ListOrderHistory(ctx context.Context, characterID int64, limit int) ([]CharacterOrderHistory, error)
}

// UserRepository specifies storage operations for application users and login sessions.
type UserRepository interface {
	CountUsers(ctx context.Context) (int, error)
	ListUsers(ctx context.Context) ([]AppUser, error)
	GetUserByID(ctx context.Context, id int64) (*AppUser, error)
	GetUserByUsername(ctx context.Context, username string) (*AppUser, error)
	CreateUser(ctx context.Context, user *AppUser) error
	UpdateUser(ctx context.Context, user *AppUser) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	UpdateLastLogin(ctx context.Context, id int64, at time.Time) error
	DeleteUser(ctx context.Context, id int64) error

	CreateSession(ctx context.Context, session *AppSession) error
	GetSessionWithUser(ctx context.Context, tokenHash string) (*AppSession, *AppUser, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteSessionsByUser(ctx context.Context, userID int64) error
	CleanExpiredSessions(ctx context.Context) error
}
