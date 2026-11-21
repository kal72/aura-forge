package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// PITier represents the Planetary Industry processing tier (P0 raw -> P4 advanced).
type PITier string

const (
	PITierP0 PITier = "P0"
	PITierP1 PITier = "P1"
	PITierP2 PITier = "P2"
	PITierP3 PITier = "P3"
	PITierP4 PITier = "P4"
)

// PIInputReq defines one direct input commodity required for a PI schematic cycle.
type PIInputReq struct {
	TypeID   int32 `json:"type_id"`
	Quantity int   `json:"quantity"`
}

// PICommodity represents an EVE Online Planetary Industry resource or factory product (P0–P4).
type PICommodity struct {
	TypeID       int32        `json:"type_id"`
	Name         string       `json:"name"`
	Tier         PITier       `json:"tier"`
	OutputQty    int          `json:"output_qty"`     // output per factory cycle (20 for P1, 5 for P2, 3 for P3, 1 for P4)
	CycleMinutes int          `json:"cycle_minutes"`  // 30m for P1, 60m for P2/P3/P4
	BaseValueISK float64      `json:"base_value_isk"` // CCP POCO base export/import value per unit
	SourcePlanets []string    `json:"source_planets,omitempty"` // populated for P0 raw resources
	Inputs       []PIInputReq `json:"inputs,omitempty"`
}

// PITreeNode represents one node in a recursive Planetary Industry recipe tree.
type PITreeNode struct {
	TypeID        int32        `json:"type_id"`
	Name          string       `json:"name"`
	Tier          PITier       `json:"tier"`
	Depth         int          `json:"depth"`
	Quantity      int          `json:"quantity"`
	UnitPrice     float64      `json:"unit_price"`
	TotalValue    float64      `json:"total_value"`
	SourcePlanets []string     `json:"source_planets,omitempty"`
	Children      []PITreeNode `json:"children,omitempty"`
}

// PIP0Requirement aggregates total P0 raw resources and their source planet types needed for 1 cycle.
type PIP0Requirement struct {
	TypeID        int32    `json:"type_id"`
	Name          string   `json:"name"`
	TotalQuantity int      `json:"total_quantity"`
	SourcePlanets []string `json:"source_planets"`
}

// PIRecipeAnalysis bundles the full recursive recipe tree, source planets, and live Jita factory economics.
type PIRecipeAnalysis struct {
	Commodity          PICommodity       `json:"commodity"`
	RootNode           PITreeNode        `json:"root_node"`
	FlatTree           []PITreeNode      `json:"flat_tree"` // pre-order flattened for clean Templ rendering
	P0Requirements     []PIP0Requirement `json:"p0_requirements"`
	RequiredPlanets    []string          `json:"required_planets"`
	POCOTaxPercent     float64           `json:"poco_tax_percent"`
	OutputUnitSell     float64           `json:"output_unit_sell"`
	OutputCycleRevenue float64           `json:"output_cycle_revenue"`
	DirectInputCost    float64           `json:"direct_input_cost"`
	POCOTaxISK         float64           `json:"poco_tax_isk"`
	NetProfitPerCycle  float64           `json:"net_profit_per_cycle"`
	NetMarginPercent   float64           `json:"net_margin_percent"`
	EstDailyISK        float64           `json:"est_daily_isk"`
	EstMonthlyISK      float64           `json:"est_monthly_isk"`
	MultibuyText       string            `json:"multibuy_text"`
}

// PIProfitRow represents one ranked item in the PI Profit Scanner (P1–P4).
type PIProfitRow struct {
	TypeID            int32              `json:"type_id"`
	Name              string             `json:"name"`
	Tier              PITier             `json:"tier"`
	OutputUnitSell    float64            `json:"output_unit_sell"`
	DirectInputCost   float64            `json:"direct_input_cost"`
	POCOTaxISK        float64            `json:"poco_tax_isk"`
	NetProfitPerCycle float64            `json:"net_profit_per_cycle"`
	NetMarginPercent  float64            `json:"net_margin_percent"`
	EstMonthlyISK     float64            `json:"est_monthly_isk"`
	Verdict           OpportunityVerdict `json:"verdict"`
}

// PIColony tracks an active player Planetary Industry colony and its extractor expiry timer.
type PIColony struct {
	ID                 int64     `json:"id"`
	PlanetName         string    `json:"planet_name"`
	SolarSystem        string    `json:"solar_system"`
	PlanetType         string    `json:"planet_type"`
	ProductTypeID      int32     `json:"product_type_id"`
	ProductName        string    `json:"product_name"`
	Tier               PITier    `json:"tier"`
	ExtractorExpiresAt time.Time `json:"extractor_expires_at"`
	EstDailyISK        float64   `json:"est_daily_isk"`
	Note               string    `json:"note"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// StatusLabel returns ACTIVE, EXPIRING, or EXPIRED along with a human-readable time remaining label.
func (c PIColony) StatusLabel(now time.Time) (status string, remaining string) {
	diff := c.ExtractorExpiresAt.Sub(now)
	if diff <= 0 {
		return "EXPIRED", "Expired"
	}
	hours := int(diff.Hours())
	mins := int(diff.Minutes()) % 60
	days := hours / 24
	remH := hours % 24
	var lbl string
	if days > 0 {
		lbl = fmt.Sprintf("%dd %dh left", days, remH)
	} else if hours > 0 {
		lbl = fmt.Sprintf("%dh %dm left", hours, mins)
	} else {
		lbl = fmt.Sprintf("%dm left", mins)
	}
	if diff <= 6*time.Hour {
		return "EXPIRING", lbl
	}
	return "ACTIVE", lbl
}

// AllPlanetTypes lists the 8 EVE Online colonizable planet types.
var AllPlanetTypes = []string{"Barren", "Temperate", "Lava", "Plasma", "Gas", "Oceanic", "Storm", "Ice"}

// Complete official EVE Online Planetary Industry Catalog (P0 -> P4).
var piCatalogList = []PICommodity{
	// === P0 RAW RESOURCES (15) ===
	{TypeID: 2267, Name: "Base Metals", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Barren", "Gas", "Lava", "Plasma", "Storm"}},
	{TypeID: 2268, Name: "Aqueous Liquids", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Barren", "Gas", "Ice", "Oceanic", "Storm", "Temperate"}},
	{TypeID: 2270, Name: "Noble Metals", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Barren", "Plasma"}},
	{TypeID: 2272, Name: "Heavy Metals", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Ice", "Lava", "Plasma"}},
	{TypeID: 2286, Name: "Planktic Colonies", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Ice", "Oceanic"}},
	{TypeID: 2287, Name: "Complex Organisms", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Oceanic", "Temperate"}},
	{TypeID: 2288, Name: "Carbon Compounds", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Barren", "Oceanic", "Temperate"}},
	{TypeID: 2305, Name: "Autotrophs", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Temperate"}},
	{TypeID: 2306, Name: "Non-CS Crystals", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Lava", "Plasma"}},
	{TypeID: 2307, Name: "Felsic Magma", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Lava"}},
	{TypeID: 2308, Name: "Suspended Plasma", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Gas", "Lava", "Plasma", "Storm"}},
	{TypeID: 2309, Name: "Ionic Solutions", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Gas", "Storm"}},
	{TypeID: 2310, Name: "Noble Gas", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Gas", "Ice", "Storm"}},
	{TypeID: 2311, Name: "Reactive Gas", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Gas"}},
	{TypeID: 2073, Name: "Microorganisms", Tier: PITierP0, BaseValueISK: 5, SourcePlanets: []string{"Barren", "Ice", "Oceanic", "Temperate"}},

	// === P1 BASIC COMMODITIES (15) — 3000 P0 -> 20 P1 per 30m ===
	{TypeID: 2389, Name: "Bacteria", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2073, 3000}}},
	{TypeID: 2390, Name: "Electrolytes", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2309, 3000}}},
	{TypeID: 2392, Name: "Industrial Fibers", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2305, 3000}}},
	{TypeID: 2393, Name: "Biofuels", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2288, 3000}}},
	{TypeID: 2395, Name: "Proteins", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2287, 3000}}},
	{TypeID: 2396, Name: "Biomass", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2286, 3000}}},
	{TypeID: 2397, Name: "Silicon", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2307, 3000}}},
	{TypeID: 2398, Name: "Reactive Metals", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2267, 3000}}},
	{TypeID: 2399, Name: "Precious Metals", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2270, 3000}}},
	{TypeID: 2400, Name: "Toxic Metals", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2272, 3000}}},
	{TypeID: 2401, Name: "Chiral Structures", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2306, 3000}}},
	{TypeID: 3645, Name: "Water", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2268, 3000}}},
	{TypeID: 3683, Name: "Oxygen", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2310, 3000}}},
	{TypeID: 3779, Name: "Oxidizing Compound", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2311, 3000}}},
	{TypeID: 9828, Name: "Plasmoids", Tier: PITierP1, OutputQty: 20, CycleMinutes: 30, BaseValueISK: 400, Inputs: []PIInputReq{{2308, 3000}}},

	// === P2 REFINED COMMODITIES (24) — 40 P1 + 40 P1 -> 5 P2 per 60m ===
	{TypeID: 2312, Name: "Supertensile Plastics", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2396, 40}, {3683, 40}}},
	{TypeID: 2317, Name: "Oxides", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{3683, 40}, {3779, 40}}},
	{TypeID: 2319, Name: "Test Cultures", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{3645, 40}, {2389, 40}}},
	{TypeID: 2321, Name: "Polyaramids", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2392, 40}, {3779, 40}}},
	{TypeID: 2327, Name: "Microfiber Shielding", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2397, 40}, {2392, 40}}},
	{TypeID: 2328, Name: "Water-Cooled CPU", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{3645, 40}, {2398, 40}}},
	{TypeID: 2329, Name: "Biocells", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2393, 40}, {2399, 40}}},
	{TypeID: 2463, Name: "Nanites", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2398, 40}, {2389, 40}}},
	{TypeID: 3689, Name: "Mechanical Parts", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2399, 40}, {2398, 40}}},
	{TypeID: 3691, Name: "Synthetic Oil", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{3683, 40}, {2390, 40}}},
	{TypeID: 3693, Name: "Fertilizer", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2395, 40}, {2389, 40}}},
	{TypeID: 3695, Name: "Polytextiles", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2392, 40}, {2393, 40}}},
	{TypeID: 3697, Name: "Silicate Glass", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2397, 40}, {3779, 40}}},
	{TypeID: 3725, Name: "Livestock", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2393, 40}, {2395, 40}}},
	{TypeID: 3775, Name: "Viral Agent", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2396, 40}, {2389, 40}}},
	{TypeID: 3828, Name: "Construction Blocks", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2400, 40}, {2398, 40}}},
	{TypeID: 9830, Name: "Rocket Fuel", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2390, 40}, {9828, 40}}},
	{TypeID: 9832, Name: "Coolant", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{3645, 40}, {2390, 40}}},
	{TypeID: 9836, Name: "Consumer Electronics", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2401, 40}, {2400, 40}}},
	{TypeID: 9838, Name: "Superconductors", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{3645, 40}, {9828, 40}}},
	{TypeID: 9840, Name: "Transmitter", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2401, 40}, {9828, 40}}},
	{TypeID: 9842, Name: "Miniature Electronics", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2397, 40}, {2401, 40}}},
	{TypeID: 44, Name: "Enriched Uranium", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2400, 40}, {2399, 40}}},
	{TypeID: 15317, Name: "Genetically Enhanced Livestock", Tier: PITierP2, OutputQty: 5, CycleMinutes: 60, BaseValueISK: 7200, Inputs: []PIInputReq{{2396, 40}, {2395, 40}}},

	// === P3 SPECIALIZED COMMODITIES (21) — 10 P2 each -> 3 P3 per 60m ===
	{TypeID: 9848, Name: "Robotics", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3689, 10}, {9836, 10}}},
	{TypeID: 9834, Name: "Guidance Systems", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2328, 10}, {9840, 10}}},
	{TypeID: 9846, Name: "Planetary Vehicles", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2312, 10}, {3689, 10}, {9842, 10}}},
	{TypeID: 2344, Name: "Condensates", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2317, 10}, {9832, 10}}},
	{TypeID: 2345, Name: "Camera Drones", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3697, 10}, {9830, 10}}},
	{TypeID: 2346, Name: "Synthetic Synapses", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2312, 10}, {2319, 10}}},
	{TypeID: 2348, Name: "Gel-Matrix Biopaste", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2317, 10}, {2329, 10}, {9838, 10}}},
	{TypeID: 2349, Name: "Supercomputers", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2328, 10}, {9832, 10}, {9836, 10}}},
	{TypeID: 2351, Name: "Smartfab Units", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3828, 10}, {9842, 10}}},
	{TypeID: 2352, Name: "Nuclear Reactors", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2327, 10}, {44, 10}}},
	{TypeID: 2354, Name: "Neocoms", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2329, 10}, {3697, 10}}},
	{TypeID: 2358, Name: "Biotech Research Reports", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2463, 10}, {3725, 10}, {3828, 10}}},
	{TypeID: 2360, Name: "Industrial Explosives", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3693, 10}, {3695, 10}}},
	{TypeID: 2361, Name: "Hermetic Membranes", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2321, 10}, {15317, 10}}},
	{TypeID: 2366, Name: "Hazmat Detection Systems", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3695, 10}, {3775, 10}, {9840, 10}}},
	{TypeID: 2367, Name: "Cryoprotectant Solution", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2319, 10}, {3691, 10}, {3693, 10}}},
	{TypeID: 12836, Name: "Transcranial Microcontrollers", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2329, 10}, {2463, 10}}},
	{TypeID: 17136, Name: "Ukomi Superconductors", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3691, 10}, {9838, 10}}},
	{TypeID: 17392, Name: "Data Chips", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2312, 10}, {2327, 10}}},
	{TypeID: 17898, Name: "High-Tech Transmitters", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{2321, 10}, {9840, 10}}},
	{TypeID: 28974, Name: "Vaccines", Tier: PITierP3, OutputQty: 3, CycleMinutes: 60, BaseValueISK: 60000, Inputs: []PIInputReq{{3725, 10}, {3775, 10}}},

	// === P4 ADVANCED COMMODITIES (8) — 6 P3 each (+ 40 P1 where applicable) -> 1 P4 per 60m ===
	{TypeID: 2867, Name: "Broadcast Node", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2354, 6}, {17392, 6}, {17898, 6}}},
	{TypeID: 2868, Name: "Integrity Response Drones", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{9846, 6}, {2348, 6}, {2366, 6}}},
	{TypeID: 2869, Name: "Nano-Factory", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2360, 6}, {17136, 6}, {2398, 40}}},
	{TypeID: 2870, Name: "Organic Mortar Applicators", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2344, 6}, {9848, 6}, {2389, 40}}},
	{TypeID: 2871, Name: "Recursive Computing Module", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2346, 6}, {9834, 6}, {12836, 6}}},
	{TypeID: 2872, Name: "Self-Harmonizing Power Core", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2345, 6}, {2352, 6}, {2361, 6}}},
	{TypeID: 2875, Name: "Sterile Conduits", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2351, 6}, {28974, 6}, {3645, 40}}},
	{TypeID: 2876, Name: "Wetware Mainframe", Tier: PITierP4, OutputQty: 1, CycleMinutes: 60, BaseValueISK: 1200000, Inputs: []PIInputReq{{2349, 6}, {2358, 6}, {2367, 6}}},
}

var piCatalogByID map[int32]PICommodity

func init() {
	piCatalogByID = make(map[int32]PICommodity, len(piCatalogList))
	for _, c := range piCatalogList {
		piCatalogByID[c.TypeID] = c
	}
}

// GetPICommodity looks up a PI item by official EVE TypeID.
func GetPICommodity(typeID int32) (PICommodity, bool) {
	c, ok := piCatalogByID[typeID]
	return c, ok
}

// ListProduciblePICommodities returns P1–P4 PI items filtered by optional tier and search query.
func ListProduciblePICommodities(tierFilter string, query string) []PICommodity {
	tf := strings.ToUpper(strings.TrimSpace(tierFilter))
	q := strings.ToLower(strings.TrimSpace(query))

	out := make([]PICommodity, 0, len(piCatalogList))
	for _, c := range piCatalogList {
		if c.Tier == PITierP0 {
			continue
		}
		if tf != "" && tf != "ALL" && string(c.Tier) != tf {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name), q) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// CollectPITreeTypeIDs collects the target TypeID and all recursive input TypeIDs (P1–P4) for batched ESI pricing.
func CollectPITreeTypeIDs(typeID int32) []int32 {
	seen := make(map[int32]struct{})
	out := make([]int32, 0)
	var walk func(id int32)
	walk = func(id int32) {
		c, ok := piCatalogByID[id]
		if !ok || c.Tier == PITierP0 {
			return
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			out = append(out, id)
		}
		for _, in := range c.Inputs {
			walk(in.TypeID)
		}
	}
	walk(typeID)
	return out
}

// BuildPIRecipeTree builds the full recursive recipe tree down to P0 raw resources & source planets,
// and computes live Jita factory economics (with POCO Customs Tax).
func BuildPIRecipeTree(typeID int32, prices map[int32]JitaPrice, pocoTaxPct float64) (*PIRecipeAnalysis, error) {
	rootComm, ok := piCatalogByID[typeID]
	if !ok || rootComm.Tier == PITierP0 {
		return nil, fmt.Errorf("producible PI item #%d not found", typeID)
	}
	if pocoTaxPct < 0 {
		pocoTaxPct = 0
	}

	p0Totals := make(map[int32]int)
	planetSet := make(map[string]struct{})
	flat := make([]PITreeNode, 0, 32)

	var buildNode func(id int32, qty int, depth int) PITreeNode
	buildNode = func(id int32, qty int, depth int) PITreeNode {
		c := piCatalogByID[id]
		unitPrice := 0.0
		if jp, exists := prices[id]; exists && jp.LowestSell > 0 {
			unitPrice = jp.LowestSell
		}
		node := PITreeNode{
			TypeID:        c.TypeID,
			Name:          c.Name,
			Tier:          c.Tier,
			Depth:         depth,
			Quantity:      qty,
			UnitPrice:     RoundISK(unitPrice),
			TotalValue:    RoundISK(unitPrice * float64(qty)),
			SourcePlanets: c.SourcePlanets,
		}
		flat = append(flat, node)

		if c.Tier == PITierP0 {
			p0Totals[c.TypeID] += qty
			for _, pl := range c.SourcePlanets {
				planetSet[pl] = struct{}{}
			}
			return node
		}

		// Scale child inputs to produce `qty` units of `c`
		outPerCycle := c.OutputQty
		if outPerCycle <= 0 {
			outPerCycle = 1
		}
		cyclesNeeded := float64(qty) / float64(outPerCycle)
		children := make([]PITreeNode, 0, len(c.Inputs))
		for _, req := range c.Inputs {
			childQty := int(math.Ceil(float64(req.Quantity) * cyclesNeeded))
			children = append(children, buildNode(req.TypeID, childQty, depth+1))
		}
		node.Children = children
		return node
	}

	rootNode := buildNode(rootComm.TypeID, rootComm.OutputQty, 0)

	// Build P0 summary & sorted required planet list
	p0List := make([]PIP0Requirement, 0, len(p0Totals))
	for id, totalQty := range p0Totals {
		c := piCatalogByID[id]
		p0List = append(p0List, PIP0Requirement{
			TypeID:        id,
			Name:          c.Name,
			TotalQuantity: totalQty,
			SourcePlanets: c.SourcePlanets,
		})
	}
	sort.Slice(p0List, func(i, j int) bool { return p0List[i].Name < p0List[j].Name })

	planets := make([]string, 0, len(planetSet))
	for pl := range planetSet {
		planets = append(planets, pl)
	}
	sort.Strings(planets)

	// Compute 1-cycle factory economics (buying direct inputs from Jita -> exporting output)
	outSell := prices[rootComm.TypeID].LowestSell
	cycleRev := RoundISK(outSell * float64(rootComm.OutputQty))

	var directInputCost float64
	var pocoImportTax float64
	multibuyLines := make([]string, 0, len(rootComm.Inputs))
	for _, req := range rootComm.Inputs {
		inComm := piCatalogByID[req.TypeID]
		inPrice := prices[req.TypeID].LowestSell
		directInputCost += inPrice * float64(req.Quantity)
		// CCP POCO Import Tax = 0.5 * BaseValue * TaxRate
		if inComm.Tier != PITierP0 {
			pocoImportTax += (inComm.BaseValueISK * 0.5 * (pocoTaxPct / 100.0)) * float64(req.Quantity)
			multibuyLines = append(multibuyLines, fmt.Sprintf("%s\t%d", inComm.Name, req.Quantity))
		}
	}
	// CCP POCO Export Tax = BaseValue * TaxRate
	pocoExportTax := (rootComm.BaseValueISK * (pocoTaxPct / 100.0)) * float64(rootComm.OutputQty)
	totalPocoTax := RoundISK(pocoExportTax + pocoImportTax)
	directInputCost = RoundISK(directInputCost)

	netProfit := RoundISK(cycleRev - directInputCost - totalPocoTax)
	marginPct := 0.0
	if cycleRev > 0 {
		marginPct = RoundPercent((netProfit / cycleRev) * 100.0)
	}

	cyclesPerDay := 24.0
	if rootComm.CycleMinutes > 0 {
		cyclesPerDay = (24.0 * 60.0) / float64(rootComm.CycleMinutes)
	}
	estDaily := RoundISK(netProfit * cyclesPerDay)
	estMonthly := RoundISK(estDaily * 30.0)

	return &PIRecipeAnalysis{
		Commodity:          rootComm,
		RootNode:           rootNode,
		FlatTree:           flat,
		P0Requirements:     p0List,
		RequiredPlanets:    planets,
		POCOTaxPercent:     pocoTaxPct,
		OutputUnitSell:     RoundISK(outSell),
		OutputCycleRevenue: cycleRev,
		DirectInputCost:    directInputCost,
		POCOTaxISK:         totalPocoTax,
		NetProfitPerCycle:  netProfit,
		NetMarginPercent:   marginPct,
		EstDailyISK:        estDaily,
		EstMonthlyISK:      estMonthly,
		MultibuyText:       strings.Join(multibuyLines, "\n"),
	}, nil
}
