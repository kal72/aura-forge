package domain

import "time"

// TradeHub represents a major EVE Online regional market hub.
type TradeHub struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	RegionName string `json:"region_name"`
	RegionID   int32  `json:"region_id"`
	StationID  int64  `json:"station_id"`
	SystemID   int32  `json:"system_id"`
}

// MajorTradeHubs lists the five primary Empire trade hubs in New Eden.
var MajorTradeHubs = []TradeHub{
	{Key: "jita", Name: "Jita 4-4", RegionName: "The Forge", RegionID: 10000002, StationID: 60003760, SystemID: 30000142},
	{Key: "amarr", Name: "Amarr VIII", RegionName: "Domain", RegionID: 10000043, StationID: 60008494, SystemID: 30002187},
	{Key: "dodixie", Name: "Dodixie IX", RegionName: "Sinq Laison", RegionID: 10000032, StationID: 60011866, SystemID: 30002659},
	{Key: "rens", Name: "Rens VI", RegionName: "Heimatar", RegionID: 10000030, StationID: 60004588, SystemID: 30002510},
	{Key: "hek", Name: "Hek VIII", RegionName: "Metropolis", RegionID: 10000042, StationID: 60005686, SystemID: 30002053},
}

// LookupTradeHub returns the TradeHub matching key (defaults to Jita if unknown).
func LookupTradeHub(key string) TradeHub {
	for _, h := range MajorTradeHubs {
		if h.Key == key {
			return h
		}
	}
	return MajorTradeHubs[0]
}

// MarketOrderBookEntry represents a single active public market order from ESI.
type MarketOrderBookEntry struct {
	OrderID      int64   `json:"order_id"`
	Price        float64 `json:"price"`
	VolumeRemain int     `json:"volume_remain"`
	VolumeTotal  int     `json:"volume_total"`
	LocationID   int64   `json:"location_id"`
	SystemID     int32   `json:"system_id"`
	IsBuyOrder   bool    `json:"is_buy_order"`
	Issued       string  `json:"issued"`
}

// MarketHistoryPoint represents one daily candle from ESI /markets/{region_id}/history/.
type MarketHistoryPoint struct {
	Date       string  `json:"date"`
	Average    float64 `json:"average"`
	Highest    float64 `json:"highest"`
	Lowest     float64 `json:"lowest"`
	Volume     int64   `json:"volume"`
	OrderCount int64   `json:"order_count"`
}

// HubPriceSummary summarizes live order book depth and spread for an item at one trade hub.
type HubPriceSummary struct {
	Hub             TradeHub `json:"hub"`
	LowestSell      float64  `json:"lowest_sell"`
	HighestBuy      float64  `json:"highest_buy"`
	SpreadISK       float64  `json:"spread_isk"`
	SpreadPercent   float64  `json:"spread_percent"`
	SellOrdersCount int      `json:"sell_orders_count"`
	BuyOrdersCount  int      `json:"buy_orders_count"`
	SellVolume      int64    `json:"sell_volume"`
	BuyVolume       int64    `json:"buy_volume"`
}

// MarketplaceSnapshot holds the full live market intelligence view for a single EVE item.
type MarketplaceSnapshot struct {
	Query             string                 `json:"query"`
	TypeID            int32                  `json:"type_id"`
	TypeName          string                 `json:"type_name"`
	SelectedHub       TradeHub               `json:"selected_hub"`
	LowestSell        float64                `json:"lowest_sell"`
	HighestBuy        float64                `json:"highest_buy"`
	SpreadISK         float64                `json:"spread_isk"`
	SpreadPercent     float64                `json:"spread_percent"`
	TotalSellVolume   int64                  `json:"total_sell_volume"`
	TotalBuyVolume    int64                  `json:"total_buy_volume"`
	SellOrdersCount   int                    `json:"sell_orders_count"`
	BuyOrdersCount    int                    `json:"buy_orders_count"`
	AvgDailyVolume7d  float64                `json:"avg_daily_volume_7d"`
	AvgDailyVolume30d float64                `json:"avg_daily_volume_30d"`
	EstimatedDailyISK float64                `json:"estimated_daily_isk"`

	// v2 Trading & Price Intelligence
	Avg7dPrice           float64  `json:"avg_7d_price"`
	Low7dPrice           float64  `json:"low_7d_price"`
	High7dPrice          float64  `json:"high_7d_price"`
	Change7dPct          float64  `json:"change_7d_pct"`
	PriceSignal          string   `json:"price_signal"` // CHEAP | FAIR | HIGH | SPIKE
	RoundTripFeePct      float64  `json:"round_trip_fee_pct"`
	NetFlipProfitUnit    float64  `json:"net_flip_profit_unit"`
	NetFlipMarginPct     float64  `json:"net_flip_margin_pct"`
	RecommendedBuyOrder  float64  `json:"recommended_buy_order"`
	RecommendedSellOrder float64  `json:"recommended_sell_order"`
	BestArbOriginHub     TradeHub `json:"best_arb_origin_hub"`
	BestArbDestHub       TradeHub `json:"best_arb_dest_hub"`
	BestArbOriginSell    float64  `json:"best_arb_origin_sell"`
	BestArbDestSell      float64  `json:"best_arb_dest_sell"`
	BestArbNetMarginPct  float64  `json:"best_arb_net_margin_pct"`
	HasBlueprint         bool     `json:"has_blueprint"`
	BlueprintTypeID      int32    `json:"blueprint_type_id"`
	EstCraftUnitCost     float64  `json:"est_craft_unit_cost"`
	CraftSavingsPct      float64  `json:"craft_savings_pct"`

	SellOrders     []MarketOrderBookEntry `json:"sell_orders"`
	BuyOrders      []MarketOrderBookEntry `json:"buy_orders"`
	History        []MarketHistoryPoint   `json:"history"`
	HubComparisons []HubPriceSummary      `json:"hub_comparisons"`
	UpdatedAt      time.Time              `json:"updated_at"`
}
