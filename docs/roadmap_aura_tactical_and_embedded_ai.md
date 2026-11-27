# Aura Tactical (`/tactical`) & Embedded AI Architecture — Product & Engineering Roadmap

> **Status:** Architectural Design & Long-Term Roadmap (Discussion Capture — No Code Changes Applied Yet)  
> **Scope:** Standalone Ship Fitting & Combat Strategy Studio (`/tactical`) + Invisible/Embedded Background AI Architecture  
> **Primary Focus:** PvE First (Missions, Ratting, Abyssal, Anti-Gank Hauling) → PvP (1v1, Faction Warfare, Small Gang)  
> **Deployment Model:** Modular Monolith inside the `aura-forge` repository & same domain (PostgreSQL-only, zero Redis, single Docker Compose stack)

---

## 1. Executive Summary & Architectural Vision

### 1.1 Why Two Focused Studios in One Project?
- **Aura Forge (`/`)** remains 100% focused on **Manufacturing, Planetary Industry, and Marketplace Trading** toward **100B+ ISK** without cluttering its sidebar or workflows with combat tools.
- **Aura Tactical (`/tactical`)** lives inside the **same Go binary, same domain, and same PostgreSQL database**, but renders its own **fullscreen EVE-inspired tactical layout** (`tactical_layout.templ`) completely decoupled from the Aura Forge sidebar.
- A minimal top-bar **Studio Switcher** (or direct URL navigation `/tactical`) lets pilots jump seamlessly between **Industry/Market Mode** and **Combat & Fitting Mode**.

### 1.2 Shared Infrastructure Advantages (Zero Duplication)
| Shared Asset | Benefit for Aura Tactical |
| :--- | :--- |
| **CCP SDE (`sde_types` + `traits_json`)** | 22,000+ EVE items, official Ship Hull Bonuses (`typeBonus.jsonl`), slot counts, and Dogma attributes are already imported locally. |
| **Live Trade Hub Prices (`esi_cache`)** | Every ship fitting automatically shows live Jita/Amarr/Dodixie/Rens/Hek total fit cost (ISK) and supports 1-click **Copy EVE Multibuy**. |
| **App Auth & Optional EVE SSO** | Single login; when EVE SSO is connected, Aura Tactical can optionally simulate fittings using the pilot's **actual trained skills** instead of only assuming *All Level V*. |
| **PostgreSQL-Only Stack (No Redis)** | Zero extra infrastructure — fittings, simulation presets, and AI structured-output caches are persisted cleanly in PostgreSQL. |

---

## 2. Part I: Invisible / Embedded AI Architecture (Beyond Chat)

Most AI integrations default to a chat sidebar where users must write prompts and read long paragraphs. For high-speed EVE Online decision-making, **Aura Forge** and **Aura Tactical** adopt an **Embedded Background AI** pattern:

```text
User Context / Trigger
        │
        ▼
Go Background Goroutine (Bounded Concurrency + Context Timeout)
        │
        ├──► Check PostgreSQL Cache (TTL + Context Hash)
        │         └── Hit  ──► Return Cached Structured JSON Instantly
        │
        └──► Miss ──► LLM Call (Strict JSON Schema Output)
                       │
                       ▼
              Validate & Persist JSON in PostgreSQL
                       │
                       ▼
         HTMX + Templ Renders Native UI Cards / Badges / Action Buttons
```

### Core Design Principles
1. **Structured JSON Only:** AI never returns free-form essays for operational screens. Every endpoint enforces a typed Go struct (e.g., `verdict`, `confidence_score`, `weaknesses[]`, `recommended_modules[]`, `engagement_steps[]`).
2. **Non-Blocking UX:** Pages render immediately with deterministic math/SDE data; AI insight strips hydrate asynchronously via HTMX (`hx-trigger="load"`) or pre-warmed background workers.
3. **Token & Cost Efficiency:** Deduplicated via `singleflight` and cached in PostgreSQL keyed by `(feature, entity_hash, market_or_fit_signature)` with configurable TTL.
4. **Chat Remains Optional:** The existing Aura AI chat drawer stays available only for open-ended free-form exploration, while 90% of operational guidance surfaces natively inside the UI.

### High-Value Embedded AI Surfaces (Reference Concepts)
- **Dashboard Daily Action Strip:** Top 3 concrete prioritized action cards (*Craft*, *Relist*, *Avoid*) generated from wallet, active orders, jobs, and Opportunity Radar.
- **Marketplace Order Book Verdict:** Compact inline strip (`STATION TRADE` / `CRAFT IT` / `HAUL` / `AVOID TRAP`) summarizing sell walls and price anomalies.
- **Compare Popup "Beginner Pick":** Concise winner badge and 2-bullet rationale when comparing 2–3 ships or modules.
- **Capital Allocator ("Auto-Build Weekly Plan"):** Pilot enters ISK budget + risk profile → background AI outputs a diversified, ready-to-save manufacturing plan.
- **Aura Tactical Combat Strategist:** Automatically audits ship fittings against target NPC/PvP profiles and outputs concrete module swaps and engagement playbooks (detailed in Phase 4 below).

---

## 3. Part II: Aura Tactical (`/tactical`) — Phased Roadmap to Best-in-Class

### Phase 1: Ship Fitting Workbench Foundation & EFT Interop
**Goal:** Deliver a fast, EVE-authentic web fitting workbench with real-time resource/defense math and live market valuation.

- **Dedicated Fullscreen Layout (`/tactical`):**
  - Independent `tactical_layout.templ` with the *Seamless Deep Orbital Space* & *Day/Night* theme, zero Aura Forge sidebar, and a compact switcher back to `/`.
- **Visual Slot Rack Workbench:**
  - Ship selector (local SDE autocomplete + Browse by Hull Class: Frigate, Destroyer, Cruiser, Battlecruiser, Battleship, Industrial/Hauler).
  - Interactive slot racks: **High Slots**, **Medium Slots**, **Low Slots**, **Rig Slots**, **Drone Bay**, and **Charges / Ammunition**.
  - Live Ship Hull Bonuses (*Traits*) panel pulled directly from `sde_types.traits_json`.
- **Core Real-Time Fitting Metrics:**
  - **Resources:** CPU Usage vs Output (`tf`), Powergrid Usage vs Output (`MW`), Calibration Usage vs Output (`points`), Turret & Launcher Hardpoints.
  - **Defense & Tank:** Raw Hit Points (Shield / Armor / Structure), Resistance Profiles (Electromagnetic / Thermal / Kinetic / Explosive), and **Effective Hit Points (EHP)**.
  - **Navigation & Utility:** Maximum Velocity (`m/s`), Warp Speed (`AU/s`), Inertia Modifier & **Align Time** (`s`), Cargo Capacity (`m³`), Drone Bandwidth & Bay.
- **EFT Format Import / Export & Market Integration:**
  - 1-click **Import EFT** (`Ctrl+V` from EVE Online / Pyfa / zKillboard) and **Export EFT** (`Ctrl+C` directly into EVE Online's Fitting Window).
  - Real-time **Total Fit Cost (ISK)** at the pilot's chosen Trade Hub + **Copy EVE Multibuy** list.
- **Saved Fittings Library (PostgreSQL):**
  - Table `tactical_fittings` to save, tag (`PvE`, `PvP`, `Hauling`, `Abyssal`), duplicate, and organize personal ship doctrines.

---

### Phase 2: Full Dogma Stacking Engine, Skill Profiles & PvE Combat Simulator (Priority)
**Goal:** Achieve Pyfa-grade calculation accuracy and build a beginner-friendly **PvE Combat & Survival Simulator** covering Missions, Ratting, Abyssal, and Anti-Gank Hauling.

- **EVE Online Dogma Math Engine:**
  - Official **Stacking Penalty** formula ($S(n) = e^{-(n/2.67)^2}$) for resistance hardeners, damage mods, tracking computers, and propulsion enhancers.
  - **Capacitor Stability Simulator:** Exact capacitor peak recharge vs active module drain (Cap Stable `%` or exact **Time Until Cap Empty** in `mm:ss`).
  - **Offense Calculator:** Turret DPS & Volley, Missile DPS & Volley, Drone DPS, combined Total DPS, and damage type breakdown (Electromagnetic / Thermal / Kinetic / Explosive).
  - **Active Tank Calculator:** Sustained Shield Boost (`HP/s` and `EHP/s`) and Armor Repair (`HP/s` and `EHP/s`) including cap-limited sustainability.
- **Pilot Skill Profiles:**
  - Preset profiles: **Beginner (All Level III)**, **Intermediate (All Level IV)**, **Maxed (All Level V)**.
  - **Live EVE SSO Skill Sync (Optional):** Fetch `/characters/{id}/skills/` via existing EVE SSO token so the simulator reflects the pilot's *real* skill levels and highlights missing prerequisites or high-impact skill upgrades.
- **PvE Target & Pirate Faction Library (Priority Focus):**
  - Built-in EVE NPC damage & weakness presets:
    - **Guristas** (Deals Kinetic/Thermal • Weak to Kinetic/Thermal)
    - **Sansha's Nation** (Deals EM/Thermal • Weak to EM/Thermal)
    - **Blood Raiders** (Deals EM/Thermal • Weak to EM/Thermal + Capacitor Neutralizer warning)
    - **Serpentis** (Deals Thermal/Kinetic • Weak to Kinetic/Thermal + Sensor Dampener warning)
    - **Angel Cartel** (Deals Explosive/Kinetic • Weak to Explosive/Kinetic)
    - **Triglavian / Abyssal Weather Profiles** (Electrical, Exotic, Firestorm, Gamma, Dark)
  - **Mission & Ratting Readiness Evaluator (Level 1–4 & Anomalies):**
    - Compares your ship's **Sustained EHP/s** against the chosen faction's incoming DPS profile.
    - Flags fatal gaps immediately (e.g., *"EM Shield Hole 0% against Sansha — Critical Tank Failure"* or *"Insufficient Capacitor for sustained Shield Booster"*).
  - **Anti-Gank Hauler Check:**
    - Evaluates Industrial / Hauler EHP & Align Time against typical High-Sec suicide gank thresholds (e.g., Catalyst / Tornado volley benchmarks) alongside cargo ISK value.

---

### Phase 3: Engagement Physics & PvP 1v1 / Small Gang Timeline Simulator
**Goal:** Model real EVE Online combat geometry, damage application, and second-by-second engagement outcomes for both advanced PvE and PvP.

- **Range & Damage Application Curves (`0–100 km`):**
  - **Turret Application:** Optimal Range, Accuracy Falloff, Tracking Speed vs Target Transversal/Angular Velocity and Signature Radius.
  - **Missile Application:** Flight Range (`velocity × flight time`), Explosion Radius vs Target Signature Radius, Explosion Velocity vs Target Speed.
  - Visual interactive chart showing **Applied DPS vs Distance (`km`)** so pilots immediately see their ideal engagement envelope.
- **Fit vs Fit 1v1 Duel Simulator (PvP & Boss Testing):**
  - Pit your saved fit against any second saved fit (or enemy archetype: *Brawler*, *Kiter*, *Scram-Kiter*, *Logi*, *Ganker*).
  - **Second-by-Second Combat Timeline:** Simulates Shield, Armor, Structure, and Capacitor curves over time under chosen engagement rules (*Keep at Range*, *Orbit at X km*, *Overheat* toggles, *EWAR* effects like Stasis Webifier, Warp Scrambler, Energy Neutralizer).
  - Clear verdict: **Estimated Time-to-Kill (TTK)** vs **Time-to-Die (TTD)** and **Capacitor Burnout Point**.
- **Doctrine & Fleet Role Support (Small Gang Foundation):**
  - Group multiple saved fits into a named Doctrine (e.g., *Caracal Fleet*, *Abyssal Trio*) with total fleet ISK cost, EHP/DPS summary, and 1-click alliance/corp EFT export.

---

### Phase 4: Embedded AI Tactical Strategist & Auto-Optimization
**Goal:** Combine deterministic EVE Dogma simulation with background structured-JSON AI to deliver actionable combat coaching without chat friction.

- **Background AI Fit Auditor (Native Tactical Card):**
  - Triggered asynchronously when a fit is saved or evaluated against a PvE/PvP scenario.
  - Returns structured JSON rendered as crisp visual callouts:
    - **Survivability Grade (`S` / `A` / `B` / `C` / `Risk`)**
    - **Critical Weaknesses** (resistance holes, cap instability, wasted slots, mismatched weapon/hull bonuses)
    - **Concrete Module Swap Recommendations** (with ISK delta and EHP/DPS impact)
- **Step-by-Step Engagement Playbook:**
  - Automatically generates a concise tactical checklist for the selected matchup:
    - *Pre-Warp Prep* (ammo type, drones, hardeners)
    - *Engagement Range & Positioning* (orbit distance, propulsion timing)
    - *Threat Priority* (which NPC EWAR frigates or enemy roles to clear first)
    - *Disengage Trigger* (exact Shield/Cap threshold to warp out safely)
- **Budget-Aware Fit Optimizer:**
  - Pilot specifies: *"Mission Level 3 vs Guristas, Max Budget 60M ISK, Beginner Skills"* → engine + background AI suggests the safest cost-effective hull & fitting variants, linking directly to **Aura Forge Marketplace / Manufacture** if the pilot wants to build or buy the hull and modules themselves.

---

### Phase 5: Flagship Innovations & Combat Intelligence
**Goal:** Differentiate Aura Tactical from every existing EVE Online fitting tool by combining visual 2D tactical geometry, live D-Scan & combat-log telemetry, pilot skill adaptation, and ISK ROI awareness.

- **Interactive 2D Tactical Range & Orbit Sandbox (Canvas):**
  - Top-down interactive 2D radar centered on the pilot's ship showing live concentric range rings: **Optimal**, **Falloff**, **Missile Flight Range**, **Stasis Webifier (10 km)**, **Warp Scrambler (9 km)**, and **Warp Disruptor (24 km)**.
  - Dragging a target icon and adjusting its orbit speed/angle updates **Applied DPS**, **Tracking Hit %**, and **Time-to-Kill** in real time — making EVE combat geometry immediately intuitive for beginners.
- **D-Scan Instant Threat Analyzer (`Ctrl+V` from Game):**
  - Paste a live **Directional Scanner (D-Scan)** clipboard dump from EVE Online to classify nearby ships into tactical roles (*Tackle*, *DPS*, *Logistics*, *Gankers*, *EWAR*, *Combat Probes*) using local `sde_types` (0 ESI calls).
  - Surfaces an instant **Threat Verdict Card** (e.g., high-sec suicide gank warning or low-sec gatecamp composition) plus recommended escape/counter action.
- **Combat Log Post-Mortem Autopsy (Hybrid Deterministic + Optional AI):**
  - Drag-and-drop or paste EVE Online `Gamelogs/*.txt` combat logs after a mission, Abyssal run, or PvP fight.
  - **Deterministic Go Parser (80% — zero AI required, zero token cost):** Parses timestamps, incoming/outgoing DPS timelines, hit quality distribution (*Miss / Glances Off / Hits / Penetrates / Smashes*), top threat sources, and EWAR events (*Neutralizer / Scramble / Tracking Disruption*).
  - **Optional Embedded AI Synthesis (20% — background structured JSON):** Reads only the compact parsed summary (never raw multi-thousand-line logs) to produce a concise root-cause autopsy and concrete module/tactic fixes. Works 100% with deterministic charts even when AI is disabled.
- **"Fly Today" Auto-Downgrade & Skill Gap Calculator (Via EVE SSO):**
  - Compares any imported EFT fitting against the pilot's live EVE SSO trained skills, listing missing skill prerequisites and estimated training time.
  - Provides a 1-click **"Adapt Fit to Fly Today"** action that swaps locked Tech 2 modules for the best available **Meta 4 / Compact** equivalents within CPU/Powergrid limits so beginners can undock immediately.
- **Combat ROI & Payback Bridge (Connected to Aura Forge's 100B+ ISK Mission):**
  - Links PvE fits (Missions, Ratting, Abyssal) directly to live market replacement cost, computing **Estimated Payback Hours** (*Fit ISK Cost ÷ Expected ISK/Hour*), disconnect/loss risk exposure, and cost-benefit upgrade paths (*"+35M ISK module upgrade increases clear speed by 15% and cuts payback time"*).

---

## 4. Clean Architecture & Code Isolation Blueprint

To keep `aura-forge` modular and maintainable, Aura Tactical will live in dedicated, cleanly separated modules:

```text
internal/
├── domain/
│   ├── fitting.go              # ShipFit, FitSlot, FitStats, EFT parser/exporter
│   ├── combat_sim.go           # Stacking penalty, EHP/DPS/Cap math, Faction profiles, Timeline engine
│   └── tactical_ai.go          # Structured JSON schemas for Embedded AI Tactical Audit
├── service/
│   ├── tactical_service.go     # Orchestrates SDE traits, Dogma specs, live hub pricing, & simulations
│   └── tactical_ai_service.go  # Background goroutine AI evaluator + PostgreSQL JSON cache
├── store/postgres/
│   └── tactical_repo.go        # CRUD for tactical_fittings & ai_tactical_cache
├── http/
│   └── tactical_handlers.go    # /tactical/* routes (SSR + HTMX partials + JSON APIs)
└── web/
    ├── tactical_layout.templ   # Dedicated fullscreen layout (no Aura Forge sidebar)
    ├── tactical_workbench.templ# Ship fitting canvas, racks, live metrics, EFT import/export
    └── tactical_combat.templ   # PvE/PvP simulator, range curves, timeline, AI Tactical Cards

migrations/
└── 019_aura_tactical.up.sql    # tactical_fittings & ai_tactical_cache tables
```

### Guardrails & Non-Goals
- **Zero Impact on Aura Forge Core:** No breaking changes to `/`, `/marketplace`, `/manufacture`, `/scanner`, `/pi`, `/pilot`, or `/settings`.
- **Deterministic Math First, AI Second:** All CPU, Powergrid, EHP, DPS, Resistance, Capacitor, and Range numbers MUST be computed deterministically in Go from official CCP SDE/Dogma data — AI is strictly for tactical synthesis, weakness explanation, and strategy coaching, never for guessing raw stats.
- **Beginner-First Clarity:** Follow the same zero-cryptic-abbreviation standard (spell out `Medium`, `Electromagnetic`, `Thermal`, `Kinetic`, `Explosive`, `Capacitor`, `Level 1–5`, keeping only standard measurement units).
- **ESI Safety:** All ship/module Dogma attributes are cached in PostgreSQL (`esi_cache`) or imported via CCP SDE so fitting adjustments require **0 live ESI calls**.
