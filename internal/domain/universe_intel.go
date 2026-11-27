package domain

// NPCBPOSeedOrder represents an official CCP NPC market seed sell order (duration=365) for a BPO.
type NPCBPOSeedOrder struct {
	OrderID     int64   `json:"order_id"`
	TypeID      int32   `json:"type_id"`
	StationID   int64   `json:"station_id"`
	StationName string  `json:"station_name"`
	SystemID    int32   `json:"system_id"`
	SystemName  string  `json:"system_name,omitempty"`
	RegionName  string  `json:"region_name"`
	Price       float64 `json:"price"`
	Duration    int     `json:"duration"`
}

// LoyaltyStoreRequiredItem represents an item required to redeem an LP Store offer.
type LoyaltyStoreRequiredItem struct {
	TypeID   int32  `json:"type_id"`
	Name     string `json:"name,omitempty"`
	Quantity int    `json:"quantity"`
}

// LoyaltyStoreOffer represents one NPC Corporation LP Store offer (e.g. Faction BPC or Faction module).
type LoyaltyStoreOffer struct {
	OfferID         int32                      `json:"offer_id"`
	CorporationID   int32                      `json:"corporation_id"`
	CorporationName string                     `json:"corporation_name"`
	StationHint     string                     `json:"station_hint,omitempty"`
	TypeID          int32                      `json:"type_id"`
	TypeName        string                     `json:"type_name,omitempty"`
	Quantity        int                        `json:"quantity"`
	LPCost          int                        `json:"lp_cost"`
	ISKCost         int64                      `json:"isk_cost"`
	RequiredItems   []LoyaltyStoreRequiredItem `json:"required_items,omitempty"`
}

// SolarSystemInfo holds basic public security and naming metadata for an EVE solar system.
type SolarSystemInfo struct {
	SystemID       int32   `json:"system_id"`
	Name           string  `json:"name"`
	SecurityStatus float64 `json:"security_status"`
}

// SystemKillStat holds 1-hour public PvP/NPC kill counts for an EVE solar system.
type SystemKillStat struct {
	SystemID  int32 `json:"system_id"`
	ShipKills int   `json:"ship_kills"`
	PodKills  int   `json:"pod_kills"`
	NPCKills  int   `json:"npc_kills"`
}
