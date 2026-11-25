package domain

import "math"

// OrderWatchStatus classifies whether a pilot's open market order is currently #1 or undercut.
type OrderWatchStatus string

const (
	OrderStatusTop      OrderWatchStatus = "TOP"
	OrderStatusUndercut OrderWatchStatus = "UNDERCUT"
	OrderStatusUnknown  OrderWatchStatus = "UNKNOWN"
)

// OrderWatchResult holds live market comparison data for a single open character order.
type OrderWatchResult struct {
	OrderID          int64            `json:"order_id"`
	Status           OrderWatchStatus `json:"status"`
	MarketBestPrice  float64          `json:"market_best_price"`
	GapISK           float64          `json:"gap_isk"`
	GapPercent       float64          `json:"gap_percent"`
	RecommendedPrice float64          `json:"recommended_price"`
}

// EvaluateOrderUndercut compares a pilot's open Buy or Sell order against the live hub best price
// and computes a 4-significant-figure EVE tick update price when undercut.
func EvaluateOrderUndercut(order CharacterOrder, hubPrice JitaPrice) OrderWatchResult {
	res := OrderWatchResult{
		OrderID: order.OrderID,
		Status:  OrderStatusUnknown,
	}
	if order.Price <= 0 {
		return res
	}

	if order.IsBuyOrder {
		if hubPrice.HighestBuy <= 0 {
			return res
		}
		res.MarketBestPrice = RoundISK(hubPrice.HighestBuy)
		// Tolerance for floating-point representation
		if order.Price+0.005 >= hubPrice.HighestBuy {
			res.Status = OrderStatusTop
			return res
		}
		gap := hubPrice.HighestBuy - order.Price
		res.Status = OrderStatusUndercut
		res.GapISK = RoundISK(gap)
		res.GapPercent = RoundPercent((gap / order.Price) * 100.0)
		rec := RoundEVETick(hubPrice.HighestBuy * 1.001)
		if rec <= hubPrice.HighestBuy {
			rec = RoundISK(hubPrice.HighestBuy + 0.01)
		}
		res.RecommendedPrice = rec
		return res
	}

	// Sell Order
	if hubPrice.LowestSell <= 0 {
		return res
	}
	res.MarketBestPrice = RoundISK(hubPrice.LowestSell)
	if order.Price-0.005 <= hubPrice.LowestSell {
		res.Status = OrderStatusTop
		return res
	}
	gap := order.Price - hubPrice.LowestSell
	res.Status = OrderStatusUndercut
	res.GapISK = RoundISK(gap)
	res.GapPercent = RoundPercent((gap / order.Price) * 100.0)
	rec := RoundEVETick(hubPrice.LowestSell * 0.999)
	if rec >= hubPrice.LowestSell || rec <= 0 {
		rec = math.Max(0.01, RoundISK(hubPrice.LowestSell-0.01))
	}
	res.RecommendedPrice = rec
	return res
}

// LookupTradeHubByRegion resolves a TradeHub from an EVE RegionID, falling back to the provided default hub.
func LookupTradeHubByRegion(regionID int32, fallback TradeHub) TradeHub {
	if regionID > 0 {
		for _, h := range MajorTradeHubs {
			if h.RegionID == regionID {
				return h
			}
		}
	}
	return fallback
}
