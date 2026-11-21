package domain

import "time"

// GlobalMarketPrice represents one item entry returned by EVE ESI GET /markets/prices/.
type GlobalMarketPrice struct {
	TypeID        int32   `json:"type_id"`
	AveragePrice  float64 `json:"average_price"`
	AdjustedPrice float64 `json:"adjusted_price"`
}

// GlobalPriceMover represents an item whose global average price increased between two daily snapshots.
type GlobalPriceMover struct {
	TypeID        int32     `json:"type_id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	CurrentAvg    float64   `json:"current_avg"`
	PreviousAvg   float64   `json:"previous_avg"`
	ChangePercent float64   `json:"change_percent"`
	SnapshotDate  time.Time `json:"snapshot_date"`
}

// GlobalSnapshotStats summarizes the state of stored daily GET /markets/prices/ snapshots in PostgreSQL.
type GlobalSnapshotStats struct {
	DaysAvailable int        `json:"days_available"`
	LatestItems   int        `json:"latest_items"`
	LatestDate    *time.Time `json:"latest_date,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}
