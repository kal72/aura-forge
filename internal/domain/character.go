package domain

import "time"

// CharacterProfile represents an EVE character's synchronized profile and slot capacities.
type CharacterProfile struct {
	CharacterID              int64     `json:"character_id"`
	CharacterName            string    `json:"character_name"`
	WalletBalance            float64   `json:"wallet_balance"`
	TotalManufacturingSlots  int       `json:"total_manufacturing_slots"`
	UsedManufacturingSlots   int       `json:"used_manufacturing_slots"`
	TotalScienceSlots        int       `json:"total_science_slots"`
	UsedScienceSlots         int       `json:"used_science_slots"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// EVEToken stores OAuth2 tokens for authenticated characters.
type EVEToken struct {
	CharacterID   int64     `json:"character_id"`
	CharacterName string    `json:"character_name"`
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	ExpiresAt     time.Time `json:"expires_at"`
	Scope         string    `json:"scope"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CharacterOrder represents a market order belonging to the character.
type CharacterOrder struct {
	OrderID                int64     `json:"order_id"`
	CharacterID            int64     `json:"character_id"`
	TypeID                 int32     `json:"type_id"`
	TypeName               string    `json:"type_name"`
	Price                  float64   `json:"price"`
	VolumeRemain           int       `json:"volume_remain"`
	VolumeTotal            int       `json:"volume_total"`
	IsBuyOrder             bool      `json:"is_buy_order"`
	RegionID               int32     `json:"region_id"`
	LocationID             int64     `json:"location_id"`
	IssuedAt               time.Time `json:"issued_at"`
	IsUndercut             bool      `json:"is_undercut"`
	CompetitorLowestPrice  float64   `json:"competitor_lowest_price"`
	SafeRelistPrice        float64   `json:"safe_relist_price"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// CharacterJob represents an industry manufacturing or science job.
type CharacterJob struct {
	JobID         int64      `json:"job_id"`
	CharacterID   int64      `json:"character_id"`
	TypeID        int32      `json:"type_id"`
	TypeName      string     `json:"type_name"`
	ActivityID    int        `json:"activity_id"`
	ActivityName  string     `json:"activity_name"`
	Runs          int        `json:"runs"`
	Status        string     `json:"status"` // active, paused, ready, delivered
	StartDate     time.Time  `json:"start_date"`
	EndDate       time.Time  `json:"end_date"`
	CompletedDate *time.Time `json:"completed_date,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CharacterBlueprint represents a blueprint in character possession.
type CharacterBlueprint struct {
	ItemID             int64     `json:"item_id"`
	CharacterID        int64     `json:"character_id"`
	TypeID             int32     `json:"type_id"`
	TypeName           string    `json:"type_name"`
	LocationID         int64     `json:"location_id"`
	Quantity           int       `json:"quantity"`
	MaterialEfficiency int       `json:"material_efficiency"`
	TimeEfficiency     int       `json:"time_efficiency"`
	Runs               int       `json:"runs"` // -1 for original (BPO), >0 for copy (BPC)
	IsCopy             bool      `json:"is_copy"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// HangarAsset represents material/item stock stored in hangars.
type HangarAsset struct {
	ItemID      int64     `json:"item_id"`
	CharacterID int64     `json:"character_id"`
	TypeID      int32     `json:"type_id"`
	TypeName    string    `json:"type_name"`
	LocationID  int64     `json:"location_id"`
	Quantity    int64     `json:"quantity"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserSettings represents application parameters and manual simulation constants.
type UserSettings struct {
	ID                 int       `json:"id"`
	StartingCapital    float64   `json:"starting_capital"`
	BrokerFeePercent   float64   `json:"broker_fee_percent"`   // e.g. 3.0 for 3.0%
	SalesTaxPercent    float64   `json:"sales_tax_percent"`    // e.g. 3.6 for 3.6%
	RelistFeePercent   float64   `json:"relist_fee_percent"`   // e.g. 0.5 for 0.5%
	ManufacturingSlots int       `json:"manufacturing_slots"` // default 10
	ScienceSlots       int       `json:"science_slots"`       // default 1
	SystemCostIndex    float64   `json:"system_cost_index"`    // e.g. 0.045 for 4.5%
	FreightCostPerM3   float64   `json:"freight_cost_per_m3"`  // e.g. 300 ISK/m³
	FacilityTaxPercent float64   `json:"facility_tax_percent"` // e.g. 0.25%
	UpdatedAt          time.Time `json:"updated_at"`
}

// DefaultUserSettings provides sensible defaults for a pilot starting in highsec.
func DefaultUserSettings() UserSettings {
	return UserSettings{
		ID:                 1,
		StartingCapital:    100_000_000.0, // 100M ISK
		BrokerFeePercent:   3.0,
		SalesTaxPercent:    3.6,
		RelistFeePercent:   0.5,
		ManufacturingSlots: 10,
		ScienceSlots:       1,
		SystemCostIndex:    0.045, // 4.5%
		FreightCostPerM3:   300.0,
		FacilityTaxPercent: 0.25,
		UpdatedAt:          time.Now(),
	}
}
