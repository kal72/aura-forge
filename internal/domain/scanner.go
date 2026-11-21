package domain

import "sort"

// OpportunityVerdict classifies the safety and profitability of a scanned opportunity.
type OpportunityVerdict string

const (
	VerdictStrong   OpportunityVerdict = "STRONG"
	VerdictModerate OpportunityVerdict = "MODERATE"
	VerdictWeak     OpportunityVerdict = "WEAK"
	VerdictAvoid    OpportunityVerdict = "AVOID"
)

// ManufacturingOpportunity represents one ranked blueprint result from the Manufacturing Margin Scanner.
type ManufacturingOpportunity struct {
	BlueprintTypeID   int32              `json:"blueprint_type_id"`
	ProductTypeID     int32              `json:"product_type_id"`
	ProductName       string             `json:"product_name"`
	Category          string             `json:"category"`
	Runs              int                `json:"runs"`
	OutputQty         int                `json:"output_qty"`
	MaterialCost      float64            `json:"material_cost"`
	ProductUnitSell   float64            `json:"product_unit_sell"`
	NetProfit         float64            `json:"net_profit"`
	NetMarginPercent  float64            `json:"net_margin_percent"`
	ROIPercent        float64            `json:"roi_percent"`
	ISKPerHour        float64            `json:"isk_per_hour"`
	DurationFormatted string             `json:"duration_formatted"`
	Verdict           OpportunityVerdict `json:"verdict"`
}

// StationTradeOpportunity represents one Jita station-trading flip candidate (Buy Order -> Sell Order).
type StationTradeOpportunity struct {
	TypeID            int32              `json:"type_id"`
	Name              string             `json:"name"`
	Category          string             `json:"category"`
	HighestBuy        float64            `json:"highest_buy"`
	LowestSell        float64            `json:"lowest_sell"`
	GrossSpreadISK    float64            `json:"gross_spread_isk"`
	GrossSpreadPct    float64            `json:"gross_spread_pct"`
	NetFlipProfitUnit float64            `json:"net_flip_profit_unit"`
	NetFlipMarginPct  float64            `json:"net_flip_margin_pct"`
	ROIPercent        float64            `json:"roi_percent"`
	SellOrdersCount   int                `json:"sell_orders_count"`
	BuyOrdersCount    int                `json:"buy_orders_count"`
	Verdict           OpportunityVerdict `json:"verdict"`
}

// ArbitrageOpportunity represents one inter-hub hauling/trading opportunity (Origin Sell -> Dest Sell).
type ArbitrageOpportunity struct {
	TypeID          int32              `json:"type_id"`
	Name            string             `json:"name"`
	Category        string             `json:"category"`
	OriginHub       TradeHub           `json:"origin_hub"`
	DestHub         TradeHub           `json:"dest_hub"`
	OriginBuyPrice  float64            `json:"origin_buy_price"`  // Lowest Sell at origin (instant buy)
	DestSellPrice   float64            `json:"dest_sell_price"`   // Lowest Sell at destination
	DestBuyPrice    float64            `json:"dest_buy_price"`    // Highest Buy at destination (instant dump)
	HaulCostPerUnit float64            `json:"haul_cost_per_unit"`
	NetProfitUnit   float64            `json:"net_profit_unit"`
	NetMarginPct    float64            `json:"net_margin_pct"`
	ROIPercent      float64            `json:"roi_percent"`
	Verdict         OpportunityVerdict `json:"verdict"`
}

// EvaluateStationTrade computes true net station-trading flip profit after Buy Broker Fee, Sell Broker Fee, and Sales Tax.
func EvaluateStationTrade(typeID int32, name, category string, jp JitaPrice, brokerFeePct, salesTaxPct float64) StationTradeOpportunity {
	opp := StationTradeOpportunity{
		TypeID:          typeID,
		Name:            name,
		Category:        category,
		HighestBuy:      RoundISK(jp.HighestBuy),
		LowestSell:      RoundISK(jp.LowestSell),
		SellOrdersCount: jp.SellOrdersCount,
		BuyOrdersCount:  jp.BuyOrdersCount,
		Verdict:         VerdictAvoid,
	}
	if jp.HighestBuy <= 0 || jp.LowestSell <= 0 {
		return opp
	}

	opp.GrossSpreadISK = RoundISK(jp.LowestSell - jp.HighestBuy)
	opp.GrossSpreadPct = RoundPercent((opp.GrossSpreadISK / jp.LowestSell) * 100.0)

	// True station trading costs: Buy Order cost + Buy Broker Fee + Sell Broker Fee + Sales Tax
	buyCost := jp.HighestBuy * (1.0 + brokerFeePct/100.0)
	sellNet := jp.LowestSell * (1.0 - (brokerFeePct+salesTaxPct)/100.0)
	netProfit := RoundISK(sellNet - buyCost)

	opp.NetFlipProfitUnit = netProfit
	opp.NetFlipMarginPct = RoundPercent((netProfit / jp.LowestSell) * 100.0)
	if buyCost > 0 {
		opp.ROIPercent = RoundPercent((netProfit / buyCost) * 100.0)
	}

	switch {
	case netProfit > 0 && opp.NetFlipMarginPct >= 8.0:
		opp.Verdict = VerdictStrong
	case netProfit > 0 && opp.NetFlipMarginPct >= 3.0:
		opp.Verdict = VerdictModerate
	case netProfit > 0:
		opp.Verdict = VerdictWeak
	default:
		opp.Verdict = VerdictAvoid
	}
	return opp
}

// EvaluateArbitrage computes true net inter-hub arbitrage profit after destination sell taxes and hauling cost.
func EvaluateArbitrage(typeID int32, name, category string, originHub, destHub TradeHub, originLowestSell, destLowestSell, destHighestBuy, unitVolumeM3, freightPerM3, brokerFeePct, salesTaxPct float64) ArbitrageOpportunity {
	opp := ArbitrageOpportunity{
		TypeID:          typeID,
		Name:            name,
		Category:        category,
		OriginHub:       originHub,
		DestHub:         destHub,
		OriginBuyPrice:  RoundISK(originLowestSell),
		DestSellPrice:   RoundISK(destLowestSell),
		DestBuyPrice:    RoundISK(destHighestBuy),
		HaulCostPerUnit: RoundISK(unitVolumeM3 * freightPerM3),
		Verdict:         VerdictAvoid,
	}
	if originLowestSell <= 0 || destLowestSell <= 0 {
		return opp
	}

	totalCost := originLowestSell + opp.HaulCostPerUnit
	netRevenue := destLowestSell * (1.0 - (brokerFeePct+salesTaxPct)/100.0)
	netProfit := RoundISK(netRevenue - totalCost)

	opp.NetProfitUnit = netProfit
	opp.NetMarginPct = RoundPercent((netProfit / destLowestSell) * 100.0)
	if totalCost > 0 {
		opp.ROIPercent = RoundPercent((netProfit / totalCost) * 100.0)
	}

	switch {
	case netProfit > 0 && opp.NetMarginPct >= 10.0:
		opp.Verdict = VerdictStrong
	case netProfit > 0 && opp.NetMarginPct >= 4.0:
		opp.Verdict = VerdictModerate
	case netProfit > 0:
		opp.Verdict = VerdictWeak
	default:
		opp.Verdict = VerdictAvoid
	}
	return opp
}

// ClassifyManufacturingVerdict assigns a beginner-friendly verdict to a manufacturing margin result.
func ClassifyManufacturingVerdict(netProfit, marginPct, iskPerHour float64) OpportunityVerdict {
	switch {
	case netProfit > 0 && marginPct >= 12.0 && iskPerHour >= 250_000:
		return VerdictStrong
	case netProfit > 0 && marginPct >= 5.0:
		return VerdictModerate
	case netProfit > 0:
		return VerdictWeak
	default:
		return VerdictAvoid
	}
}

// SortManufacturingOpportunities orders opportunities by ISK/Hour descending, then Net Margin descending.
func SortManufacturingOpportunities(list []ManufacturingOpportunity) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].ISKPerHour != list[j].ISKPerHour {
			return list[i].ISKPerHour > list[j].ISKPerHour
		}
		return list[i].NetMarginPercent > list[j].NetMarginPercent
	})
}

// SortStationTradeOpportunities orders station trade flips by Net Flip Margin % descending.
func SortStationTradeOpportunities(list []StationTradeOpportunity) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].NetFlipMarginPct != list[j].NetFlipMarginPct {
			return list[i].NetFlipMarginPct > list[j].NetFlipMarginPct
		}
		return list[i].NetFlipProfitUnit > list[j].NetFlipProfitUnit
	})
}

// SortArbitrageOpportunities orders arbitrage candidates by Net Margin % descending.
func SortArbitrageOpportunities(list []ArbitrageOpportunity) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].NetMarginPct != list[j].NetMarginPct {
			return list[i].NetMarginPct > list[j].NetMarginPct
		}
		return list[i].NetProfitUnit > list[j].NetProfitUnit
	})
}
