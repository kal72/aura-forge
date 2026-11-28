package domain

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DScanShipRole groups ships detected on EVE Directional Scanner into actionable combat roles.
type DScanRoleSummary struct {
	Role  string   `json:"role"` // Tackle | DPS | Ganker | Logistics | EWAR | Recon/Probes | Hauler | Other
	Count int      `json:"count"`
	Ships []string `json:"ships"`
	Color string   `json:"color"` // rose | amber | sky | emerald | violet | slate
}

// DScanAnalysisResult is the deterministic instant threat report from an EVE D-Scan clipboard paste.
type DScanAnalysisResult struct {
	TotalObjects   int                `json:"total_objects"`
	ThreatLevel    string             `json:"threat_level"` // SAFE | CAUTION | GANK ALERT | GATECAMP DANGER
	Headline       string             `json:"headline"`
	Recommendation string             `json:"recommendation"`
	HasProbes      bool               `json:"has_probes"`
	GankerCount    int                `json:"ganker_count"`
	TackleCount    int                `json:"tackle_count"`
	LogiCount      int                `json:"logi_count"`
	Roles          []DScanRoleSummary `json:"roles"`
}

// AnalyzeDScan parses a live EVE Online Directional Scanner paste (tab-separated) locally with 0 ESI calls.
func AnalyzeDScan(raw string) (*DScanAnalysisResult, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	roleOrder := []struct {
		name  string
		color string
	}{
		{"Ganker", "rose"},
		{"Tackle", "amber"},
		{"Recon / Probes", "violet"},
		{"DPS", "sky"},
		{"Logistics", "emerald"},
		{"EWAR", "orange"},
		{"Hauler / Industrial", "slate"},
	}
	byRole := make(map[string]map[string]int)
	total := 0
	hasProbes := false

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		cols := strings.Split(t, "\t")
		shipType := ""
		if len(cols) >= 3 {
			shipType = strings.TrimSpace(cols[2])
		} else if len(cols) == 2 {
			shipType = strings.TrimSpace(cols[1])
		} else {
			shipType = t
		}
		if shipType == "" {
			continue
		}
		total++
		role := classifyDScanShip(shipType)
		if strings.Contains(strings.ToLower(shipType), "probe") {
			hasProbes = true
		}
		if byRole[role] == nil {
			byRole[role] = make(map[string]int)
		}
		byRole[role][shipType]++
	}

	if total == 0 {
		return nil, fmt.Errorf("no valid D-Scan lines found — press Ctrl+A then Ctrl+C inside EVE Directional Scanner")
	}

	roles := make([]DScanRoleSummary, 0, len(roleOrder))
	gankers, tackle, logi, dps := 0, 0, 0, 0
	for _, ro := range roleOrder {
		m := byRole[ro.name]
		if len(m) == 0 {
			continue
		}
		cnt := 0
		ships := make([]string, 0, len(m))
		for s, c := range m {
			cnt += c
			if c > 1 {
				ships = append(ships, fmt.Sprintf("%s ×%d", s, c))
			} else {
				ships = append(ships, s)
			}
		}
		sort.Strings(ships)
		roles = append(roles, DScanRoleSummary{
			Role:  ro.name,
			Count: cnt,
			Ships: ships,
			Color: ro.color,
		})
		switch ro.name {
		case "Ganker":
			gankers += cnt
		case "Tackle":
			tackle += cnt
		case "Logistics":
			logi += cnt
		case "DPS":
			dps += cnt
		}
	}

	level := "SAFE"
	headline := fmt.Sprintf("Low Threat (%d contacts on scan)", total)
	rec := "No immediate gank fleet or gatecamp signature detected. Keep D-Scan refreshing every 5–10 seconds."

	switch {
	case gankers >= 2:
		level = "GANK ALERT"
		headline = fmt.Sprintf("Suicide Gank Signature: %d Ganker Hulls Detected!", gankers)
		rec = "Do NOT undock or warp to gates with valuable cargo. Dock up, wait for criminals to clear, or use an Insta-Warp (<2.0s) scout."
	case tackle >= 2 && (dps >= 2 || logi >= 1):
		level = "GATECAMP DANGER"
		headline = fmt.Sprintf("Organized Fleet / Gatecamp (%d Tackle, %d DPS, %d Logi)", tackle, dps, logi)
		rec = "High probability of warp disruption / bubble camp. Warp to a safe spot or bounce off a celestial before approaching."
	case hasProbes || tackle >= 1 || gankers == 1:
		level = "CAUTION"
		headline = fmt.Sprintf("Active Hunter / Scanner Presence (%d contacts)", total)
		rec = "Align to a station or safe bookmark immediately. If Combat Scanner Probes are on short scan (<1 AU), warp off grid now."
	}

	return &DScanAnalysisResult{
		TotalObjects:   total,
		ThreatLevel:    level,
		Headline:       headline,
		Recommendation: rec,
		HasProbes:      hasProbes,
		GankerCount:    gankers,
		TackleCount:    tackle,
		LogiCount:      logi,
		Roles:          roles,
	}, nil
}

func classifyDScanShip(shipType string) string {
	low := strings.ToLower(shipType)
	switch {
	case strings.Contains(low, "combat scanner probe") || strings.Contains(low, "core scanner probe") ||
		strings.Contains(low, "lachesis") || strings.Contains(low, "arazu") || strings.Contains(low, "huginn") ||
		strings.Contains(low, "rapier") || strings.Contains(low, "rook") || strings.Contains(low, "falcon") ||
		strings.Contains(low, "curse") || strings.Contains(low, "pilgrim") || strings.Contains(low, "astero") ||
		strings.Contains(low, "stratios"):
		return "Recon / Probes"
	case strings.Contains(low, "catalyst") || strings.Contains(low, "tornado") || strings.Contains(low, "thrasher") ||
		strings.Contains(low, "coercer") || strings.Contains(low, "talos") || strings.Contains(low, "brutix"):
		return "Ganker"
	case strings.Contains(low, "stiletto") || strings.Contains(low, "malediction") || strings.Contains(low, "ares") ||
		strings.Contains(low, "crow") || strings.Contains(low, "sabre") || strings.Contains(low, "flycatcher") ||
		strings.Contains(low, "eris") || strings.Contains(low, "heretic") || strings.Contains(low, "broadword") ||
		strings.Contains(low, "onyx") || strings.Contains(low, "phobos") || strings.Contains(low, "devoter") ||
		strings.Contains(low, "atron") || strings.Contains(low, "condor") || strings.Contains(low, "slasher"):
		return "Tackle"
	case strings.Contains(low, "basilisk") || strings.Contains(low, "guardian") || strings.Contains(low, "scimitar") ||
		strings.Contains(low, "oneiros") || strings.Contains(low, "osprey") || strings.Contains(low, "augoror") ||
		strings.Contains(low, "exequror") || strings.Contains(low, "scythe") || strings.Contains(low, "deacon") ||
		strings.Contains(low, "kirin") || strings.Contains(low, "thalia") || strings.Contains(low, "scalpel"):
		return "Logistics"
	case strings.Contains(low, "blackbird") || strings.Contains(low, "celestis") || strings.Contains(low, "arbitrator") ||
		strings.Contains(low, "bellicose") || strings.Contains(low, "griffin") || strings.Contains(low, "maulus") ||
		strings.Contains(low, "crucifier") || strings.Contains(low, "vigil"):
		return "EWAR"
	case strings.Contains(low, "nereus") || strings.Contains(low, "tayra") || strings.Contains(low, "badger") ||
		strings.Contains(low, "iteron") || strings.Contains(low, "bestower") || strings.Contains(low, "wreathe") ||
		strings.Contains(low, "mammoth") || strings.Contains(low, "freighter") || strings.Contains(low, "orca") ||
		strings.Contains(low, "venture") || strings.Contains(low, "retriever") || strings.Contains(low, "mackinaw"):
		return "Hauler / Industrial"
	default:
		return "DPS"
	}
}

// CombatLogThreatSource aggregates total incoming damage from one enemy entity.
type CombatLogThreatSource struct {
	SourceName string  `json:"source_name"`
	TotalDmg   float64 `json:"total_dmg"`
	Hits       int     `json:"hits"`
	SharePct   float64 `json:"share_pct"`
}

// CombatLogAutopsyResult is the deterministic 0-token post-mortem breakdown of an EVE Gamelog file.
type CombatLogAutopsyResult struct {
	TotalEvents       int                     `json:"total_events"`
	IncomingDamage    float64                 `json:"incoming_damage"`
	OutgoingDamage    float64                 `json:"outgoing_damage"`
	EstimatedInDPS    float64                 `json:"estimated_in_dps"`
	EstimatedOutDPS   float64                 `json:"estimated_out_dps"`
	HitQualityCounts  map[string]int          `json:"hit_quality_counts"`
	HitQualityPct     map[string]float64      `json:"hit_quality_pct"`
	EWARTags          []string                `json:"ewar_tags"`
	TopThreats        []CombatLogThreatSource `json:"top_threats"`
	KeyTakeaway       string                  `json:"key_takeaway"`
}

var (
	reCombatDmgLine = regexp.MustCompile(`(?i)\(combat\)\s*(?:<[^>]+>)*\s*(\d+)\s*(?:<[^>]+>)*\s*(from|to)\s+(.+?)(?:\s*-\s*([A-Za-z ]+))?$`)
	reHTMLTags      = regexp.MustCompile(`<[^>]+>`)
)

// AnalyzeCombatLog deterministically parses EVE Online Gamelogs/*.txt combat lines (zero AI tokens).
func AnalyzeCombatLog(raw string) (*CombatLogAutopsyResult, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	var (
		totalEvents int
		inDmg       float64
		outDmg      float64
		qualCounts  = map[string]int{"Miss": 0, "Glances Off": 0, "Hits": 0, "Penetrates": 0, "Smashes": 0, "Wrecks": 0}
		qualTotal   int
		threats     = make(map[string]*CombatLogThreatSource)
		ewarSet     = make(map[string]struct{})
	)

	for _, ln := range lines {
		clean := strings.TrimSpace(reHTMLTags.ReplaceAllString(ln, " "))
		if clean == "" || !strings.Contains(strings.ToLower(clean), "(combat)") {
			continue
		}
		low := strings.ToLower(clean)

		// Detect EWAR events
		if strings.Contains(low, "energy neutralizer") || strings.Contains(low, "neutralized") {
			ewarSet["Capacitor Neutralizer"] = struct{}{}
		}
		if strings.Contains(low, "warp scrambl") || strings.Contains(low, "warp disrupt") {
			ewarSet["Warp Scramble / Disrupt"] = struct{}{}
		}
		if strings.Contains(low, "stasis web") {
			ewarSet["Stasis Webifier"] = struct{}{}
		}
		if strings.Contains(low, "jammed") || strings.Contains(low, "ecm") {
			ewarSet["ECM Jamming"] = struct{}{}
		}
		if strings.Contains(low, "misses you completely") || strings.Contains(low, "your") && strings.Contains(low, "misses") {
			totalEvents++
			qualCounts["Miss"]++
			qualTotal++
			continue
		}

		m := reCombatDmgLine.FindStringSubmatch(clean)
		if len(m) < 4 {
			continue
		}
		dmg, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		dir := strings.ToLower(m[2])
		entity := strings.TrimSpace(m[3])
		if idx := strings.Index(entity, " - "); idx > 0 {
			entity = strings.TrimSpace(entity[:idx])
		}
		quality := ""
		if len(m) >= 5 {
			quality = strings.TrimSpace(m[4])
		}

		totalEvents++
		if dir == "from" {
			inDmg += dmg
			if threats[entity] == nil {
				threats[entity] = &CombatLogThreatSource{SourceName: entity}
			}
			threats[entity].TotalDmg += dmg
			threats[entity].Hits++
		} else {
			outDmg += dmg
		}

		ql := strings.ToLower(quality)
		switch {
		case strings.Contains(ql, "wreck"):
			qualCounts["Wrecks"]++
			qualTotal++
		case strings.Contains(ql, "smash"):
			qualCounts["Smashes"]++
			qualTotal++
		case strings.Contains(ql, "penetrat"):
			qualCounts["Penetrates"]++
			qualTotal++
		case strings.Contains(ql, "glanc"):
			qualCounts["Glances Off"]++
			qualTotal++
		case strings.Contains(ql, "hit") || strings.Contains(ql, "graz"):
			qualCounts["Hits"]++
			qualTotal++
		}
	}

	if totalEvents == 0 {
		return nil, fmt.Errorf("no (combat) lines detected — paste lines from EVE Online Documents/EVE/logs/Gamelogs/*.txt")
	}

	qualPct := make(map[string]float64, len(qualCounts))
	if qualTotal > 0 {
		for k, v := range qualCounts {
			qualPct[k] = math.Round((float64(v)/float64(qualTotal))*1000) / 10
		}
	}

	topList := make([]CombatLogThreatSource, 0, len(threats))
	for _, ts := range threats {
		if inDmg > 0 {
			ts.SharePct = math.Round((ts.TotalDmg/inDmg)*1000) / 10
		}
		topList = append(topList, *ts)
	}
	sort.Slice(topList, func(i, j int) bool { return topList[i].TotalDmg > topList[j].TotalDmg })
	if len(topList) > 6 {
		topList = topList[:6]
	}

	ewarTags := make([]string, 0, len(ewarSet))
	for tag := range ewarSet {
		ewarTags = append(ewarTags, tag)
	}
	sort.Strings(ewarTags)

	// Approximate fight duration (~2.5s per event heuristic if timestamps omitted)
	estDur := math.Max(10.0, float64(totalEvents)*2.0)
	takeaway := fmt.Sprintf("Parsed %d combat events: %.0f incoming damage vs %.0f outgoing damage.", totalEvents, inDmg, outDmg)
	if len(topList) > 0 && topList[0].SharePct >= 45 {
		takeaway = fmt.Sprintf("Primary threat was %s (%.0f%% of incoming damage). Prioritize clearing or pulling range from this source first.", topList[0].SourceName, topList[0].SharePct)
	}
	if len(ewarTags) > 0 {
		takeaway += " EWAR detected: " + strings.Join(ewarTags, ", ") + "."
	}

	return &CombatLogAutopsyResult{
		TotalEvents:      totalEvents,
		IncomingDamage:   math.Round(inDmg),
		OutgoingDamage:   math.Round(outDmg),
		EstimatedInDPS:   math.Round((inDmg/estDur)*10) / 10,
		EstimatedOutDPS:  math.Round((outDmg/estDur)*10) / 10,
		HitQualityCounts: qualCounts,
		HitQualityPct:    qualPct,
		EWARTags:         ewarTags,
		TopThreats:       topList,
		KeyTakeaway:      takeaway,
	}, nil
}
