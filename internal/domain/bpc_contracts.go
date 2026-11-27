package domain

import "time"

// PublicBPCContract represents a live Public Item Exchange Contract selling a Blueprint Copy (or researched BPO) in a region.
type PublicBPCContract struct {
	ContractID         int64     `json:"contract_id"`
	BlueprintTypeID    int32     `json:"blueprint_type_id"`
	BlueprintName      string    `json:"blueprint_name"`
	ProductName        string    `json:"product_name"`
	Price              float64   `json:"price"`
	PricePerRun        float64   `json:"price_per_run"`
	Runs               int       `json:"runs"`
	MaterialEfficiency int       `json:"material_efficiency"`
	TimeEfficiency     int       `json:"time_efficiency"`
	IsBlueprintCopy    bool      `json:"is_blueprint_copy"`
	Quantity           int       `json:"quantity"`
	StartLocationID    int64     `json:"start_location_id"`
	StationName        string    `json:"station_name"`
	RegionName         string    `json:"region_name"`
	DateExpired        time.Time `json:"date_expired"`
	Title              string    `json:"title,omitempty"`
}

// BPCFinderResult aggregates live Public BPC Contracts, NPC BPO Seed Orders, and LP Store Offers for a blueprint.
type BPCFinderResult struct {
	Query            string              `json:"query"`
	Hub              TradeHub            `json:"hub"`
	Blueprint        *SDEBlueprint       `json:"blueprint,omitempty"`
	BPCContracts     []PublicBPCContract `json:"bpc_contracts"`
	NPCBPOOrders     []NPCBPOSeedOrder   `json:"npc_bpo_orders"`
	LPStoreOffers    []LoyaltyStoreOffer `json:"lp_store_offers"`
	ScannedContracts int                 `json:"scanned_contracts"`
	ScannedPages     int                 `json:"scanned_pages"`
	TotalPages       int                 `json:"total_pages"`
	Notice           string              `json:"notice,omitempty"`
}
