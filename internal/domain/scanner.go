package domain

import (
	"math"
	"sort"
	"strings"
)

// OpportunityVerdict classifies the safety and profitability of a scanned opportunity.
type OpportunityVerdict string

const (
	VerdictStrong   OpportunityVerdict = "STRONG"
	VerdictModerate OpportunityVerdict = "MODERATE"
	VerdictWeak     OpportunityVerdict = "WEAK"
	VerdictAvoid    OpportunityVerdict = "AVOID"

	// HighSecGankSafeCapISK is the recommended maximum cargo value for a T1 hauler in High-Sec.
	HighSecGankSafeCapISK = 200_000_000.0
)

// RoundEVETick rounds an ISK price to EVE Online's 4 significant figures market tick rule.
func RoundEVETick(val float64) float64 {
	if val <= 0 || math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	if val < 100 {
		return math.Round(val*100) / 100
	}
	digits := math.Floor(math.Log10(val)) + 1
	scale := math.Pow(10, digits-4)
	if scale <= 0 {
		return math.Round(val*100) / 100
	}
	return math.Round(val/scale) * scale
}

// ManufacturingOpportunity represents one ranked blueprint result from the Manufacturing Margin Scanner.
type ManufacturingOpportunity struct {
	BlueprintTypeID   int32              `json:"blueprint_type_id"`
	ProductTypeID     int32              `json:"product_type_id"`
	ProductName       string             `json:"product_name"`
	Category          string             `json:"category"`
	TechLevel         int                `json:"tech_level"`
	HubKey            string             `json:"hub_key"`
	HubName           string             `json:"hub_name"`
	MaterialHubKey    string             `json:"material_hub_key,omitempty"`
	MaterialHubName   string             `json:"material_hub_name,omitempty"`
	Runs              int                `json:"runs"`
	ME                int                `json:"me"`
	TE                int                `json:"te"`
	OwnedBPO          bool               `json:"owned_bpo"`
	AutoSized         bool               `json:"auto_sized"`
	RunSizingReason   string             `json:"run_sizing_reason,omitempty"`
	OutputQty         int                `json:"output_qty"`
	MaterialCost      float64            `json:"material_cost"`
	ProductUnitSell   float64            `json:"product_unit_sell"`
	RecommendedSell   float64            `json:"recommended_sell"`
	BreakEvenSell     float64            `json:"break_even_sell"`
	DailyVolume7d     float64            `json:"daily_volume_7d"`
	FitsCapital       bool               `json:"fits_capital"`
	NetProfit         float64            `json:"net_profit"`
	NetMarginPercent  float64            `json:"net_margin_percent"`
	ROIPercent        float64            `json:"roi_percent"`
	ISKPerHour        float64            `json:"isk_per_hour"`
	DurationFormatted string             `json:"duration_formatted"`
	IsPriceSpike      bool               `json:"is_price_spike,omitempty"`
	PriceSpikePct     float64            `json:"price_spike_pct,omitempty"`
	Verdict           OpportunityVerdict `json:"verdict"`
}

// StationTradeOpportunity represents one station-trading flip candidate (Buy Order -> Sell Order) at any trade hub.
type StationTradeOpportunity struct {
	TypeID               int32              `json:"type_id"`
	Name                 string             `json:"name"`
	Category             string             `json:"category"`
	HubKey               string             `json:"hub_key"`
	HubName              string             `json:"hub_name"`
	HighestBuy           float64            `json:"highest_buy"`
	LowestSell           float64            `json:"lowest_sell"`
	RecommendedBuyOrder  float64            `json:"recommended_buy_order"`
	RecommendedSellOrder float64            `json:"recommended_sell_order"`
	DailyVolume7d        float64            `json:"daily_volume_7d"`
	EstDailyFlipProfit   float64            `json:"est_daily_flip_profit"`
	GrossSpreadISK       float64            `json:"gross_spread_isk"`
	GrossSpreadPct       float64            `json:"gross_spread_pct"`
	NetFlipProfitUnit    float64            `json:"net_flip_profit_unit"`
	NetFlipMarginPct     float64            `json:"net_flip_margin_pct"`
	ROIPercent           float64            `json:"roi_percent"`
	SellOrdersCount      int                `json:"sell_orders_count"`
	BuyOrdersCount       int                `json:"buy_orders_count"`
	Verdict              OpportunityVerdict `json:"verdict"`
}

// ArbitrageOpportunity represents one inter-hub hauling/trading opportunity (Origin Sell -> Dest Sell).
type ArbitrageOpportunity struct {
	TypeID              int32              `json:"type_id"`
	Name                string             `json:"name"`
	Category            string             `json:"category"`
	OriginHub           TradeHub           `json:"origin_hub"`
	DestHub             TradeHub           `json:"dest_hub"`
	OriginBuyPrice      float64            `json:"origin_buy_price"` // Lowest Sell at origin (instant buy)
	DestSellPrice       float64            `json:"dest_sell_price"`  // Lowest Sell at destination
	DestBuyPrice        float64            `json:"dest_buy_price"`   // Highest Buy at destination (instant dump)
	RecommendedSell     float64            `json:"recommended_sell"` // Suggested 4-sig-fig EVE sell order price at destination
	BreakEvenSell       float64            `json:"break_even_sell"`  // Minimum sell price to break even after hauling + fees
	UnitVolumeM3        float64            `json:"unit_volume_m3"`
	DestDailyVolume7d   float64            `json:"dest_daily_volume_7d"`
	HaulCostPerUnit     float64            `json:"haul_cost_per_unit"`
	NetProfitUnit       float64            `json:"net_profit_unit"`
	NetMarginPct        float64            `json:"net_margin_pct"`
	ROIPercent          float64            `json:"roi_percent"`
	MaxTripUnits        int                `json:"max_trip_units"`
	TripCargoM3         float64            `json:"trip_cargo_m3"`
	TripCapitalRequired float64            `json:"trip_capital_required"`
	EstProfitPerTrip    float64            `json:"est_profit_per_trip"`
	GankRiskWarning     bool               `json:"gank_risk_warning"`
	Verdict             OpportunityVerdict `json:"verdict"`
}

// StockoutSignal classifies local trade-hub supply tightness for high-demand items.
type StockoutSignal string

const (
	SignalStockout     StockoutSignal = "STOCKOUT"      // 0 active sell orders at hub despite regional demand
	SignalThinSupply   StockoutSignal = "THIN_SUPPLY"   // <= 3 active sell orders (very low undercut competition)
	SignalLocalPremium StockoutSignal = "LOCAL_PREMIUM" // Hub sell trades >= 15% above Jita benchmark
)

// StockoutOpportunity represents a local trade-hub supply crunch or low-competition premium item.
type StockoutOpportunity struct {
	TypeID             int32              `json:"type_id"`
	Name               string             `json:"name"`
	Category           string             `json:"category"`
	HubKey             string             `json:"hub_key"`
	HubName            string             `json:"hub_name"`
	HubLowestSell      float64            `json:"hub_lowest_sell"`
	JitaLowestSell     float64            `json:"jita_lowest_sell"`
	RecommendedSell    float64            `json:"recommended_sell"`
	PremiumOverJitaPct float64            `json:"premium_over_jita_pct"`
	SellOrdersCount    int                `json:"sell_orders_count"`
	BuyOrdersCount     int                `json:"buy_orders_count"`
	DailyVolume7d      float64            `json:"daily_volume_7d"`
	EstDailyMarketISK  float64            `json:"est_daily_market_isk"`
	Signal             StockoutSignal     `json:"signal"`
	Verdict            OpportunityVerdict `json:"verdict"`
}

// EvaluateStockoutOpportunity checks whether an item with real regional demand is stocked out,
// has thin local competition (<=3 sellers), or commands a >=15% local premium over Jita.
func EvaluateStockoutOpportunity(
	typeID int32, name, category string,
	hub TradeHub, hubPrice, jitaPrice JitaPrice, dailyVol7d float64,
) (StockoutOpportunity, bool) {
	// Require meaningful regional demand so we never flag dead/junk items
	if dailyVol7d < 5 {
		return StockoutOpportunity{}, false
	}

	var premPct float64
	if jitaPrice.LowestSell > 0 && hubPrice.LowestSell > 0 {
		premPct = RoundPercent(((hubPrice.LowestSell - jitaPrice.LowestSell) / jitaPrice.LowestSell) * 100.0)
	}

	var signal StockoutSignal
	var verdict OpportunityVerdict

	switch {
	case hubPrice.SellOrdersCount == 0 || hubPrice.LowestSell <= 0:
		signal = SignalStockout
		verdict = VerdictStrong
	case hubPrice.SellOrdersCount <= 3 && (premPct >= 8.0 || dailyVol7d >= 25):
		signal = SignalThinSupply
		if premPct >= 15.0 || dailyVol7d >= 100 {
			verdict = VerdictStrong
		} else {
			verdict = VerdictModerate
		}
	case premPct >= 15.0 && hubPrice.SellOrdersCount <= 12:
		signal = SignalLocalPremium
		if premPct >= 25.0 {
			verdict = VerdictStrong
		} else {
			verdict = VerdictModerate
		}
	default:
		return StockoutOpportunity{}, false
	}

	refPrice := hubPrice.LowestSell
	var recSell float64
	if refPrice <= 0 {
		refPrice = jitaPrice.LowestSell * 1.18
		recSell = RoundEVETick(refPrice)
	} else {
		recSell = RoundEVETick(refPrice * 0.999)
		if recSell <= 0 {
			recSell = RoundISK(refPrice)
		}
	}
	estDailyISK := RoundISK(dailyVol7d * refPrice)

	return StockoutOpportunity{
		TypeID:             typeID,
		Name:               name,
		Category:           category,
		HubKey:             hub.Key,
		HubName:            hub.Name,
		HubLowestSell:      RoundISK(hubPrice.LowestSell),
		JitaLowestSell:     RoundISK(jitaPrice.LowestSell),
		RecommendedSell:    recSell,
		PremiumOverJitaPct: premPct,
		SellOrdersCount:    hubPrice.SellOrdersCount,
		BuyOrdersCount:     hubPrice.BuyOrdersCount,
		DailyVolume7d:      RoundISK(dailyVol7d),
		EstDailyMarketISK:  estDailyISK,
		Signal:             signal,
		Verdict:            verdict,
	}, true
}

// SortStockoutOpportunities ranks STOCKOUT first, then THIN_SUPPLY, then LOCAL_PREMIUM, breaking ties by EstDailyMarketISK.
func SortStockoutOpportunities(opps []StockoutOpportunity) {
	sigRank := func(s StockoutSignal) int {
		switch s {
		case SignalStockout:
			return 0
		case SignalThinSupply:
			return 1
		default:
			return 2
		}
	}
	sort.Slice(opps, func(i, j int) bool {
		ri, rj := sigRank(opps[i].Signal), sigRank(opps[j].Signal)
		if ri != rj {
			return ri < rj
		}
		if opps[i].PremiumOverJitaPct != opps[j].PremiumOverJitaPct {
			return opps[i].PremiumOverJitaPct > opps[j].PremiumOverJitaPct
		}
		return opps[i].EstDailyMarketISK > opps[j].EstDailyMarketISK
	})
}

// EvaluateStationTrade computes true net station-trading flip profit after Buy Broker Fee, Sell Broker Fee, and Sales Tax.
func EvaluateStationTrade(typeID int32, name, category string, jp JitaPrice, brokerFeePct, salesTaxPct float64) StationTradeOpportunity {
	return EvaluateStationTradeForHub(typeID, name, category, LookupTradeHub("jita"), jp, 0, brokerFeePct, salesTaxPct)
}

// EvaluateStationTradeForHub evaluates a Buy->Sell flip at any trade hub, including 7d daily volume,
// EVE 4-sig-fig Recommended Buy/Sell Order prices, and conservative Est. Daily Flip Profit (10% volume share).
func EvaluateStationTradeForHub(typeID int32, name, category string, hub TradeHub, jp JitaPrice, dailyVol7d, brokerFeePct, salesTaxPct float64) StationTradeOpportunity {
	opp := StationTradeOpportunity{
		TypeID:          typeID,
		Name:            name,
		Category:        category,
		HubKey:          hub.Key,
		HubName:         hub.Name,
		HighestBuy:      RoundISK(jp.HighestBuy),
		LowestSell:      RoundISK(jp.LowestSell),
		DailyVolume7d:   dailyVol7d,
		SellOrdersCount: jp.SellOrdersCount,
		BuyOrdersCount:  jp.BuyOrdersCount,
		Verdict:         VerdictAvoid,
	}
	if jp.HighestBuy <= 0 || jp.LowestSell <= 0 {
		return opp
	}

	// Recommended 1-tick outbid above Highest Buy & 1-tick undercut below Lowest Sell (EVE 4 sig figs)
	recBuy := RoundEVETick(jp.HighestBuy * 1.001)
	if recBuy <= jp.HighestBuy {
		recBuy = RoundEVETick(jp.HighestBuy * 1.002)
	}
	recSell := RoundEVETick(jp.LowestSell * 0.999)
	if recSell >= jp.LowestSell {
		recSell = RoundEVETick(jp.LowestSell * 0.998)
	}
	if recBuy >= recSell {
		recBuy = RoundEVETick(jp.HighestBuy)
		recSell = RoundEVETick(jp.LowestSell)
	}
	opp.RecommendedBuyOrder = recBuy
	opp.RecommendedSellOrder = recSell

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
	if netProfit > 0 && dailyVol7d > 0 {
		// Conservative 10% market capture estimate for a beginner station trader
		estUnits := math.Max(1, math.Floor(dailyVol7d*0.10))
		opp.EstDailyFlipProfit = RoundISK(estUnits * netProfit)
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
	return EvaluateArbitrageWithTrip(typeID, name, category, originHub, destHub, originLowestSell, destLowestSell, destHighestBuy, unitVolumeM3, 0, 5000, 150_000_000, freightPerM3, brokerFeePct, salesTaxPct)
}

// EvaluateArbitrageWithTrip computes net arbitrage profit per unit PLUS trip-level sizing bounded by
// cargo capacity (m3), trip budget (ISK), and destination 50% daily liquidity cap, flagging >200M ISK gank risk.
func EvaluateArbitrageWithTrip(
	typeID int32, name, category string,
	originHub, destHub TradeHub,
	originLowestSell, destLowestSell, destHighestBuy float64,
	unitVolumeM3, destDailyVol7d, cargoCapM3, tripBudgetISK float64,
	freightPerM3, brokerFeePct, salesTaxPct float64,
) ArbitrageOpportunity {
	if unitVolumeM3 <= 0 {
		unitVolumeM3 = 5.0
	}
	opp := ArbitrageOpportunity{
		TypeID:            typeID,
		Name:              name,
		Category:          category,
		OriginHub:         originHub,
		DestHub:           destHub,
		OriginBuyPrice:    RoundISK(originLowestSell),
		DestSellPrice:     RoundISK(destLowestSell),
		DestBuyPrice:      RoundISK(destHighestBuy),
		UnitVolumeM3:      unitVolumeM3,
		DestDailyVolume7d: destDailyVol7d,
		HaulCostPerUnit:   RoundISK(unitVolumeM3 * freightPerM3),
		Verdict:           VerdictAvoid,
	}
	if originLowestSell <= 0 || destLowestSell <= 0 {
		return opp
	}

	totalCost := originLowestSell + opp.HaulCostPerUnit
	feeFactor := 1.0 - (brokerFeePct+salesTaxPct)/100.0
	netRevenue := destLowestSell * feeFactor
	netProfit := RoundISK(netRevenue - totalCost)

	recSell := RoundEVETick(destLowestSell * 0.999)
	if recSell <= 0 {
		recSell = RoundISK(destLowestSell)
	}
	opp.RecommendedSell = recSell
	if feeFactor > 0 {
		opp.BreakEvenSell = RoundISK(totalCost / feeFactor)
	}

	opp.NetProfitUnit = netProfit
	opp.NetMarginPct = RoundPercent((netProfit / destLowestSell) * 100.0)
	if totalCost > 0 {
		opp.ROIPercent = RoundPercent((netProfit / totalCost) * 100.0)
	}

	// Compute realistic MaxTripUnits bounded by Budget, Cargo Cap (m3), and 50% of Dest Daily Volume
	if cargoCapM3 <= 0 {
		cargoCapM3 = 5000
	}
	if tripBudgetISK <= 0 {
		tripBudgetISK = 150_000_000
	}
	maxUnits := int(math.Floor(tripBudgetISK / originLowestSell))
	maxByCargo := int(math.Floor(cargoCapM3 / unitVolumeM3))
	if maxByCargo < maxUnits {
		maxUnits = maxByCargo
	}
	if destDailyVol7d > 0 {
		maxByLiq := int(math.Max(1, math.Floor(destDailyVol7d*0.50)))
		if maxByLiq < maxUnits {
			maxUnits = maxByLiq
		}
	}
	if maxUnits < 0 {
		maxUnits = 0
	}

	opp.MaxTripUnits = maxUnits
	opp.TripCargoM3 = math.Round(float64(maxUnits)*unitVolumeM3*10) / 10
	opp.TripCapitalRequired = RoundISK(float64(maxUnits) * originLowestSell)
	opp.EstProfitPerTrip = RoundISK(float64(maxUnits) * netProfit)
	opp.GankRiskWarning = opp.TripCapitalRequired > HighSecGankSafeCapISK

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
	SortManufacturingOpportunitiesBy(list, "isk_hr")
}

// SortManufacturingOpportunitiesBy orders manufacturing scan results by isk_hr, roi, profit, or volume.
func SortManufacturingOpportunitiesBy(list []ManufacturingOpportunity, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "roi":
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].ROIPercent != list[j].ROIPercent {
				return list[i].ROIPercent > list[j].ROIPercent
			}
			return list[i].ISKPerHour > list[j].ISKPerHour
		})
	case "profit":
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].NetProfit != list[j].NetProfit {
				return list[i].NetProfit > list[j].NetProfit
			}
			return list[i].ISKPerHour > list[j].ISKPerHour
		})
	case "volume":
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].DailyVolume7d != list[j].DailyVolume7d {
				return list[i].DailyVolume7d > list[j].DailyVolume7d
			}
			return list[i].ISKPerHour > list[j].ISKPerHour
		})
	default:
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].ISKPerHour != list[j].ISKPerHour {
				return list[i].ISKPerHour > list[j].ISKPerHour
			}
			return list[i].NetMarginPercent > list[j].NetMarginPercent
		})
	}
}

// SortStationTradeOpportunities orders station trade flips by EstDailyFlipProfit / Net Flip Margin % descending.
func SortStationTradeOpportunities(list []StationTradeOpportunity) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].NetFlipMarginPct != list[j].NetFlipMarginPct {
			return list[i].NetFlipMarginPct > list[j].NetFlipMarginPct
		}
		return list[i].EstDailyFlipProfit > list[j].EstDailyFlipProfit
	})
}

// SortArbitrageOpportunities orders arbitrage candidates by EstProfitPerTrip descending (when positive), then Net Margin %.
func SortArbitrageOpportunities(list []ArbitrageOpportunity) {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].EstProfitPerTrip != list[j].EstProfitPerTrip {
			return list[i].EstProfitPerTrip > list[j].EstProfitPerTrip
		}
		return list[i].NetMarginPct > list[j].NetMarginPct
	})
}
