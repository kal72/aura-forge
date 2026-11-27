// Aura Tactical — Interactive Orbital Ship Fitting Wheel + 2D Tactical Radar Engine
// Pure Alpine.js + SVG Radial Geometry + HTML5 Canvas (Zero external build dependencies)
(function () {
	const FACTION_PALETTE = {
		amarr:      { glow: 'rgba(245, 158, 11, 0.42)', rim: '#fbbf24', nebula: 'radial-gradient(circle at 50% 45%, rgba(217,119,6,0.34), rgba(120,53,15,0.22) 48%, rgba(11,13,26,0.94) 82%)' },
		caldari:    { glow: 'rgba(56, 189, 248, 0.38)', rim: '#38bdf8', nebula: 'radial-gradient(circle at 50% 45%, rgba(14,165,233,0.30), rgba(30,58,138,0.22) 48%, rgba(11,13,26,0.94) 82%)' },
		gallente:   { glow: 'rgba(52, 211, 153, 0.38)', rim: '#34d399', nebula: 'radial-gradient(circle at 50% 45%, rgba(16,185,129,0.28), rgba(6,78,59,0.24) 48%, rgba(11,13,26,0.94) 82%)' },
		minmatar:   { glow: 'rgba(251, 146, 60, 0.38)', rim: '#fb923c', nebula: 'radial-gradient(circle at 50% 45%, rgba(234,88,12,0.30), rgba(124,45,18,0.24) 48%, rgba(11,13,26,0.94) 82%)' },
		triglavian: { glow: 'rgba(244, 63, 94, 0.44)',  rim: '#f43f5e', nebula: 'radial-gradient(circle at 50% 45%, rgba(225,29,72,0.34), rgba(88,28,135,0.24) 48%, rgba(11,13,26,0.94) 82%)' },
		neutral:    { glow: 'rgba(139, 92, 246, 0.40)', rim: '#a78bfa', nebula: 'radial-gradient(circle at 50% 45%, rgba(124,58,237,0.30), rgba(49,46,129,0.24) 48%, rgba(11,13,26,0.94) 82%)' },
	};

	function degToRad(deg) {
		return (deg * Math.PI) / 180;
	}

	function polarPoint(cx, cy, r, deg) {
		const a = degToRad(deg);
		return { x: cx + r * Math.cos(a), y: cy + r * Math.sin(a) };
	}

	// Builds an exact curved-trapezoid (annular sector) SVG path between rIn and rOut from startDeg to endDeg.
	function annularSectorPath(cx, cy, rIn, rOut, startDeg, endDeg) {
		const p1 = polarPoint(cx, cy, rOut, startDeg);
		const p2 = polarPoint(cx, cy, rOut, endDeg);
		const p3 = polarPoint(cx, cy, rIn, endDeg);
		const p4 = polarPoint(cx, cy, rIn, startDeg);
		const largeArc = (endDeg - startDeg) > 180 ? 1 : 0;
		return [
			`M ${p1.x.toFixed(2)} ${p1.y.toFixed(2)}`,
			`A ${rOut} ${rOut} 0 ${largeArc} 1 ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`,
			`L ${p3.x.toFixed(2)} ${p3.y.toFixed(2)}`,
			`A ${rIn} ${rIn} 0 ${largeArc} 0 ${p4.x.toFixed(2)} ${p4.y.toFixed(2)}`,
			'Z',
		].join(' ');
	}

	function buildTickSegments(cx, cy, rInner, rOuter, startDeg, endDeg, count, fillPct) {
		const segs = [];
		const clamped = Math.max(0, Math.min(100, Number(fillPct) || 0));
		const activeCount = Math.round((clamped / 100) * count);
		const step = (endDeg - startDeg) / Math.max(count - 1, 1);
		for (let i = 0; i < count; i++) {
			const deg = startDeg + i * step;
			const p1 = polarPoint(cx, cy, rInner, deg);
			const p2 = polarPoint(cx, cy, rOuter, deg);
			segs.push({
				x1: p1.x.toFixed(1),
				y1: p1.y.toFixed(1),
				x2: p2.x.toFixed(1),
				y2: p2.y.toFixed(1),
				active: i < activeCount,
			});
		}
		return segs;
	}

	function formatISK(val) {
		const v = Number(val) || 0;
		const abs = Math.abs(v);
		if (abs >= 1e9) return (v / 1e9).toFixed(2) + 'B ISK';
		if (abs >= 1e6) return (v / 1e6).toFixed(2) + 'M ISK';
		if (abs >= 1e3) return (v / 1e3).toFixed(1) + 'K ISK';
		return v.toFixed(0) + ' ISK';
	}

	function formatNum(val, dec = 0) {
		const n = Number(val) || 0;
		return n.toLocaleString('en-US', { minimumFractionDigits: dec, maximumFractionDigits: dec });
	}

	window.afTacticalStudio = function (bootData) {
		if (!bootData) {
			try {
				const el = document.getElementById('af-tactical-boot');
				bootData = el ? JSON.parse(el.textContent || '{}') : {};
			} catch (_) {
				bootData = {};
			}
		}
		const presets = (bootData && bootData.presets) || [];
		const factions = (bootData && bootData.factions) || [];
		const duelOpponents = (bootData && bootData.duel_opponents) || [];
		const initialSaved = (bootData && bootData.saved_fittings) || [];
		const defaultFit = presets.length > 0 ? JSON.parse(JSON.stringify(presets[0].fit)) : {
			name: 'Omen — Imperial Pulse Cruiser',
			ship_type_id: 2006,
			ship_name: 'Omen',
			hull_class: 'Cruiser',
			faction_theme: 'amarr',
			tag: 'PvE',
			skill_profile: 'intermediate',
			target_preset: 'sansha',
			modules: [],
			drones: [],
		};

		return {
			// Workspace mode: 'wheel' | 'radar' | 'doctrines'
			activeMode: 'wheel',
			// Left drawer tab: 'modules' | 'hulls' | 'saved'
			leftTab: 'modules',
			// Active slot filter for module browser
			activeSlotFilter: 'high',
			selectedSlot: null, // { kind, index }
			moduleQuery: '',
			moduleResults: [],
			loadingModules: false,

			hullClassFilter: 'all',
			shipQuery: '',
			shipResults: [],
			loadingShips: false,

			presets: presets,
			factions: factions,
			duelOpponents: duelOpponents,
			savedFittings: initialSaved,

			// Phase 3: 1v1 Duel Timeline state
			duelOpponentKey: 'l3_boss',
			duelRangeKM: 18,
			duelEnemyWeb: false,
			duelEnemyScram: false,
			duelEnemyNeut: false,
			duelResult: null,
			duelRunning: false,
			duelScrubIdx: 0,

			// Phase 4: Embedded Background AI Tactical Audit state
			aiAudit: null,
			aiAuditing: false,

			// Phase 5: D-Scan & Combat Log Autopsy state
			dscanInput: '',
			dscanResult: null,
			dscanError: '',
			dscanRunning: false,
			combatLogInput: '',
			combatLogResult: null,
			combatLogError: '',
			combatLogRunning: false,

			// Batch 3: Fly Today Auto-Downgrade & Combat ROI Payback Bridge
			flyTodayRunning: false,
			roiActivityKey: 'l3_mission',
			roiActivities: [
				{ key: 'l2_mission', label: 'L2 Missions', iskPerHour: 12000000 },
				{ key: 'l3_mission', label: 'L3 Missions', iskPerHour: 28000000 },
				{ key: 'l4_mission', label: 'L4 Missions', iskPerHour: 55000000 },
				{ key: 'ratting',    label: 'Belt / Anom Ratting', iskPerHour: 45000000 },
				{ key: 'abyssal_t2', label: 'Abyssal T2–T3', iskPerHour: 65000000 },
			],

			// Batch 5: Live SSO Pilot Skills, Ammo Picker, Side-by-Side Compare & Fleet Builder
			ssoConnected: !!(bootData && bootData.mode && bootData.mode.sso_connected),
			ssoCharName: (bootData && bootData.mode && bootData.mode.character_name) || '',
			compareKeyA: presets.length > 0 ? ('preset:' + presets[0].key) : '',
			compareKeyB: presets.length > 1 ? ('preset:' + presets[1].key) : '',
			compareResult: null,
			compareRunning: false,
			fleetCounts: {},

			fit: defaultFit,
			sim: null,
			simulating: false,
			simError: '',
			toastMsg: '',
			toastTimer: null,

			// Drag & drop + Ghost hover state
			draggingModule: null,
			dragOverSlotKey: '',
			hoverSlotKey: '',
			hoverModule: null,
			ghostSim: null,
			ghostSlotKey: '',
			ghostTimer: null,
			ghostSeq: 0,

			// Modals
			eftModalOpen: false,
			eftInputText: '',
			eftImporting: false,
			eftError: '',
			traitsExpanded: false,
			savingFit: false,

			// Hologram parallax tilt
			tiltX: 0,
			tiltY: 0,

			// 2D Tactical Radar state
			radarTargetDistKM: 18,
			radarTargetAngle: -0.65,
			radarTargetSpeedMS: 240,
			radarTargetSigM: 125,
			radarDragging: false,
			radarAnimId: null,
			radarPulsePhase: 0,

			init() {
				this.runSimulate();
				this.fetchModules();
				this.fetchShips();
				this.$watch('activeMode', (val) => {
					if (val === 'radar') {
						this.$nextTick(() => {
							this.startRadarLoop();
							if (!this.duelResult) this.runDuelSim();
						});
					} else {
						this.stopRadarLoop();
					}
				});
			},

			notify(msg) {
				this.toastMsg = msg;
				if (this.toastTimer) clearTimeout(this.toastTimer);
				this.toastTimer = setTimeout(() => { this.toastMsg = ''; }, 2800);
			},

			formatISK,
			formatNum,

			factionTheme() {
				const key = (this.sim && this.sim.fit && this.sim.fit.faction_theme) || this.fit.faction_theme || 'amarr';
				return FACTION_PALETTE[key] || FACTION_PALETTE.neutral;
			},

			shipRenderURL(typeID) {
				return 'https://images.evetech.net/types/' + (typeID || 2006) + '/render?size=512';
			},

			itemIconURL(typeID) {
				return 'https://images.evetech.net/types/' + (typeID || 34) + '/icon?size=64';
			},

			handleWheelParallax(e) {
				const rect = e.currentTarget.getBoundingClientRect();
				const nx = ((e.clientX - rect.left) / rect.width - 0.5) * 2;
				const ny = ((e.clientY - rect.top) / rect.height - 0.5) * 2;
				this.tiltY = (nx * 7).toFixed(2);
				this.tiltX = (-ny * 7).toFixed(2);
			},

			resetParallax() {
				this.tiltX = 0;
				this.tiltY = 0;
			},

			async runSimulate() {
				this.simulating = true;
				this.simError = '';
				try {
					const res = await fetch('/tactical/api/simulate', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify(this.fit),
					});
					const data = await res.json();
					if (!res.ok) {
						this.simError = data.error || 'Simulation failed';
						return;
					}
					this.sim = data;
					if (data.fit) {
						this.fit.ship_name = data.fit.ship_name;
						this.fit.hull_class = data.fit.hull_class;
						this.fit.faction_theme = data.fit.faction_theme;
						this.fit.modules = data.fit.modules || [];
						this.fit.drones = data.fit.drones || [];
					}
				} catch (err) {
					this.simError = 'Network error running fitting simulation';
				} finally {
					this.simulating = false;
				}
			},

			async fetchShips() {
				this.loadingShips = true;
				try {
					const params = new URLSearchParams({ class: this.hullClassFilter, q: this.shipQuery });
					const res = await fetch('/tactical/api/ships?' + params.toString());
					const data = await res.json();
					this.shipResults = data.ships || [];
				} catch (_) {
					this.shipResults = [];
				} finally {
					this.loadingShips = false;
				}
			},

			async fetchModules() {
				this.loadingModules = true;
				try {
					const params = new URLSearchParams({ slot: this.activeSlotFilter, q: this.moduleQuery });
					const res = await fetch('/tactical/api/modules?' + params.toString());
					const data = await res.json();
					this.moduleResults = data.modules || [];
				} catch (_) {
					this.moduleResults = [];
				} finally {
					this.loadingModules = false;
				}
			},

			selectShip(ship) {
				this.fit = {
					id: 0,
					name: ship.name + ' Tactical Fit',
					ship_type_id: ship.type_id,
					ship_name: ship.name,
					hull_class: ship.group_name || '',
					faction_theme: '',
					tag: this.fit.tag || 'PvE',
					skill_profile: this.fit.skill_profile || 'intermediate',
					target_preset: this.fit.target_preset || 'guristas',
					modules: [],
					drones: [],
				};
				this.selectedSlot = null;
				this.leftTab = 'modules';
				this.activeSlotFilter = 'high';
				this.fetchModules();
				this.runSimulate();
				this.notify('Loaded hull: ' + ship.name);
			},

			loadPreset(preset) {
				this.fit = JSON.parse(JSON.stringify(preset.fit));
				this.selectedSlot = null;
				this.activeMode = 'wheel';
				this.runSimulate();
				this.notify('Loaded doctrine: ' + preset.name);
			},

			loadSavedFitting(rec) {
				if (!rec || !rec.fit) return;
				this.fit = JSON.parse(JSON.stringify(rec.fit));
				this.fit.id = rec.id;
				this.fit.name = rec.name;
				this.selectedSlot = null;
				this.activeMode = 'wheel';
				this.runSimulate();
				this.notify('Opened saved fit: ' + rec.name);
			},

			// Radial Slot Calculations (560x560 coordinate space, center 280,280, ring radius 224)
			// Every socket is tangentially rotated (deg + 90) so slots hug the circular bezel with uniform 9px gaps & zero overlap.
			slotCounts() {
				const s = (this.sim && this.sim.slots) || { high_slots: 5, mid_slots: 4, low_slots: 5, rig_slots: 3 };
				return {
					high: Math.max(0, Math.min(8, s.high_slots || 0)),
					mid:  Math.max(0, Math.min(8, s.mid_slots || 0)),
					low:  Math.max(0, Math.min(8, s.low_slots || 0)),
					rig:  Math.max(0, Math.min(3, s.rig_slots || 0)),
				};
			},

			getModuleAt(kind, index) {
				return (this.fit.modules || []).find((m) => m.slot_kind === kind && m.slot_index === index) || null;
			},

			radialSlots() {
				const counts = this.slotCounts();
				const cx = 280, cy = 280, r = 224;
				const racks = [
					{ kind: 'high', max: 8, count: counts.high, startDeg: -134, stepDeg: 12.5 },
					{ kind: 'mid',  max: 8, count: counts.mid,  startDeg: -26,  stepDeg: 12.5 },
					{ kind: 'low',  max: 8, count: counts.low,  startDeg: 80,   stepDeg: 12.5 },
					{ kind: 'rig',  max: 3, count: counts.rig,  startDeg: 184,  stepDeg: 13.0 },
				];
				const out = [];
				for (const rk of racks) {
					// Center each ship's actual slots cleanly inside its quadrant arc for balanced, uncluttered visual symmetry
					const span = Math.max(0, rk.count - 1) * rk.stepDeg;
					const quadCenter = rk.startDeg + ((rk.max - 1) * rk.stepDeg) / 2;
					const firstDeg = quadCenter - span / 2;
					for (let i = 0; i < rk.count; i++) {
						const deg = firstDeg + i * rk.stepDeg;
						const pt = polarPoint(cx, cy, r, deg);
						const mod = this.getModuleAt(rk.kind, i);
						out.push({
							key: rk.kind + '-' + i,
							kind: rk.kind,
							index: i,
							available: true,
							x: pt.x,
							y: pt.y,
							pctX: (pt.x / 560) * 100,
							pctY: (pt.y / 560) * 100,
							deg: deg,
							module: mod,
						});
					}
				}
				return out;
			},

			// Outer Segmented Arc Gauges (Calibration left, CPU right-mid, Powergrid bottom-right)
			calibrationTicks() {
				const pct = (this.sim && this.sim.calibration && this.sim.calibration.percent) || 0;
				return buildTickSegments(280, 280, 258, 273, 180, 232, 28, pct);
			},

			cpuTicks() {
				const pct = (this.sim && this.sim.cpu && this.sim.cpu.percent) || 0;
				return buildTickSegments(280, 280, 258, 273, 0, 52, 28, pct);
			},

			powergridTicks() {
				const pct = (this.sim && this.sim.powergrid && this.sim.powergrid.percent) || 0;
				return buildTickSegments(280, 280, 258, 273, 58, 112, 28, pct);
			},

			hardpointPips() {
				const s = (this.sim && this.sim.slots) || { turret_slots: 0, launcher_slots: 0 };
				const tUsed = (this.sim && this.sim.turrets_used) || 0;
				const lUsed = (this.sim && this.sim.launchers_used) || 0;
				const pips = [];
				const cx = 280, cy = 280, r = 266;
				let idx = 0;
				for (let i = 0; i < Math.min(8, s.turret_slots || 0); i++) {
					const pt = polarPoint(cx, cy, r, -56 + idx * 4.4);
					pips.push({ x: pt.x.toFixed(1), y: pt.y.toFixed(1), kind: 'turret', used: i < tUsed });
					idx++;
				}
				for (let i = 0; i < Math.min(8, s.launcher_slots || 0); i++) {
					const pt = polarPoint(cx, cy, r, -56 + idx * 4.4);
					pips.push({ x: pt.x.toFixed(1), y: pt.y.toFixed(1), kind: 'launcher', used: i < lUsed });
					idx++;
				}
				return pips;
			},

			// Pure SVG markup for the outer orbital bezel, segmented gauges (Calibration, CPU, Powergrid), and Hardpoint Pips.
			// Rendered via x-html to avoid invalid <template> elements inside the SVG namespace.
			wheelOuterSVG() {
				const calOver = !!(this.sim && this.sim.calibration && this.sim.calibration.overloaded);
				const cpuOver = !!(this.sim && this.sim.cpu && this.sim.cpu.overloaded);
				const pgOver  = !!(this.sim && this.sim.powergrid && this.sim.powergrid.overloaded);

				let svg = '';
				// Dark Orbital Bezel Ring & Precision Concentric Track [r=198 .. r=250, center r=224]
				svg += '<circle cx="280" cy="280" r="251" fill="rgba(7, 9, 19, 0.86)" stroke="rgba(148, 163, 184, 0.24)" stroke-width="1.5"></circle>';
				svg += '<circle cx="280" cy="280" r="224" fill="none" stroke="rgba(15, 23, 42, 0.72)" stroke-width="46"></circle>';
				svg += '<circle cx="280" cy="280" r="198" fill="none" stroke="rgba(251, 191, 36, 0.34)" stroke-width="1.4"></circle>';
				svg += '<circle cx="280" cy="280" r="250" fill="none" stroke="rgba(148, 163, 184, 0.16)" stroke-width="1"></circle>';

				// Subtle Rack Dividers between High / Mid / Low / Rig
				for (const divDeg of [-36, 70.5, 175.5, 219]) {
					const p1 = polarPoint(280, 280, 200, divDeg);
					const p2 = polarPoint(280, 280, 248, divDeg);
					svg += `<line x1="${p1.x.toFixed(1)}" y1="${p1.y.toFixed(1)}" x2="${p2.x.toFixed(1)}" y2="${p2.y.toFixed(1)}" stroke="rgba(251,191,36,0.22)" stroke-width="1" stroke-dasharray="2 3"></line>`;
				}

				// Curved Trapezoid (Annular Sector) Slot Cradles hugging r=201..247
				const halfSpan = 5.45; // 10.9 deg wedge inside 12.5 deg step -> crisp 1.6 deg radial gap
				for (const slot of this.radialSlots()) {
					const isSelected = !!(this.selectedSlot && this.selectedSlot.kind === slot.kind && this.selectedSlot.index === slot.index);
					const isDrop = this.dragOverSlotKey === slot.key;
					const isHover = this.hoverSlotKey === slot.key;
					const isGhost = this.ghostSlotKey === slot.key;
					const d = annularSectorPath(280, 280, 202, 246, slot.deg - halfSpan, slot.deg + halfSpan);

					let fill = 'rgba(3, 5, 12, 0.92)';
					let stroke = 'none';
					let strokeW = '0';

					if (slot.module) {
						fill = slot.module.state === 'offline' ? 'rgba(6, 9, 20, 0.65)' : 'rgba(15, 23, 42, 0.92)';
						if (slot.module.state === 'active') {
							stroke = 'rgba(74, 222, 128, 0.75)';
							strokeW = '1';
						} else if (slot.module.state === 'overheated') {
							stroke = 'rgba(251, 113, 133, 0.90)';
							strokeW = '1.2';
						}
					} else if (isHover) {
						fill = 'rgba(20, 28, 48, 0.92)';
					}

					if (isGhost) {
						stroke = '#38bdf8';
						strokeW = '1.4';
						fill = 'rgba(14, 165, 233, 0.25)';
					}
					if (isDrop || isSelected) {
						stroke = '#fbbf24';
						strokeW = '1.4';
						fill = 'rgba(30, 27, 75, 0.88)';
					}

					svg += `<path d="${d}" fill="${fill}" stroke="${stroke}" stroke-width="${strokeW}" stroke-linejoin="round"></path>`;

					// Subtle outer emerald/rose active accent arc on fitted active/overheated trapezoids
					if (slot.module && (slot.module.state === 'active' || slot.module.state === 'overheated')) {
						const pA = polarPoint(280, 280, 247.5, slot.deg - halfSpan + 0.8);
						const pB = polarPoint(280, 280, 247.5, slot.deg + halfSpan - 0.8);
						const arcColor = slot.module.state === 'overheated' ? '#fb7185' : '#4ade80';
						svg += `<path d="M ${pA.x.toFixed(2)} ${pA.y.toFixed(2)} A 247.5 247.5 0 0 1 ${pB.x.toFixed(2)} ${pB.y.toFixed(2)}" fill="none" stroke="${arcColor}" stroke-width="2" stroke-linecap="round"></path>`;
					}
				}

				// Calibration Segmented Arc (Left — Silver/White)
				for (const seg of this.calibrationTicks()) {
					const stroke = seg.active ? (calOver ? '#f43f5e' : '#e2e8f0') : 'rgba(148,163,184,0.20)';
					svg += `<line x1="${seg.x1}" y1="${seg.y1}" x2="${seg.x2}" y2="${seg.y2}" stroke="${stroke}" stroke-width="2.6" stroke-linecap="round"></line>`;
				}
				// CPU Segmented Arc (Right-Mid — Ice Blue #38bdf8, plus Amber Ghost Preview ticks!)
				const cpuGhostPct = (this.ghostSim && this.ghostSim.cpu && this.ghostSim.cpu.percent) || 0;
				const cpuBaseTicks = this.cpuTicks();
				const cpuGhostTicks = buildTickSegments(280, 280, 258, 273, 0, 52, 28, cpuGhostPct);
				for (let i = 0; i < cpuBaseTicks.length; i++) {
					const seg = cpuBaseTicks[i];
					const gActive = this.ghostSim && cpuGhostTicks[i] && cpuGhostTicks[i].active;
					let stroke = 'rgba(56,189,248,0.18)';
					if (seg.active) stroke = cpuOver ? '#f43f5e' : '#38bdf8';
					else if (gActive) stroke = (this.ghostSim.cpu.overloaded ? '#f43f5e' : '#fbbf24');
					svg += `<line x1="${seg.x1}" y1="${seg.y1}" x2="${seg.x2}" y2="${seg.y2}" stroke="${stroke}" stroke-width="2.8" stroke-linecap="round"></line>`;
				}
				// Powergrid Segmented Arc (Bottom-Right — Crimson Coral #fb7185, plus Amber Ghost Preview ticks!)
				const pgGhostPct = (this.ghostSim && this.ghostSim.powergrid && this.ghostSim.powergrid.percent) || 0;
				const pgBaseTicks = this.powergridTicks();
				const pgGhostTicks = buildTickSegments(280, 280, 258, 273, 58, 112, 28, pgGhostPct);
				for (let i = 0; i < pgBaseTicks.length; i++) {
					const seg = pgBaseTicks[i];
					const gActive = this.ghostSim && pgGhostTicks[i] && pgGhostTicks[i].active;
					let stroke = 'rgba(244,63,94,0.20)';
					if (seg.active) stroke = pgOver ? '#ff2a55' : '#fb7185';
					else if (gActive) stroke = (this.ghostSim.powergrid.overloaded ? '#ff2a55' : '#fbbf24');
					svg += `<line x1="${seg.x1}" y1="${seg.y1}" x2="${seg.x2}" y2="${seg.y2}" stroke="${stroke}" stroke-width="2.8" stroke-linecap="round"></line>`;
				}
				// Hardpoint Pips (Top-Right Collar)
				for (const pip of this.hardpointPips()) {
					const fill = pip.used ? (pip.kind === 'turret' ? '#fde68a' : '#38bdf8') : 'rgba(148,163,184,0.24)';
					const stroke = pip.used ? '#ffffff' : 'rgba(148,163,184,0.50)';
					svg += `<circle cx="${pip.x}" cy="${pip.y}" r="4.2" fill="${fill}" stroke="${stroke}" stroke-width="1.1"></circle>`;
				}
				return svg;
			},

			clickSlot(slot) {
				if (this.selectedSlot && this.selectedSlot.kind === slot.kind && this.selectedSlot.index === slot.index) {
					this.selectedSlot = null;
					return;
				}
				this.selectedSlot = { kind: slot.kind, index: slot.index };
				this.leftTab = 'modules';
				if (this.activeSlotFilter !== slot.kind) {
					this.activeSlotFilter = slot.kind;
					this.moduleQuery = '';
					this.fetchModules();
				}
			},

			equipModuleToSlot(item, targetKind, targetIndex) {
				if (!item || !item.type_id) return;
				if (targetKind === 'drone' || this.activeSlotFilter === 'drone') {
					const existing = (this.fit.drones || []).find((d) => d.type_id === item.type_id);
					if (existing) {
						existing.quantity += 1;
					} else {
						this.fit.drones = [...(this.fit.drones || []), { type_id: item.type_id, name: item.name, quantity: 5, active: true }];
					}
					this.runSimulate();
					this.notify('Added drone: ' + item.name);
					return;
				}

				const kind = targetKind || (this.selectedSlot && this.selectedSlot.kind) || this.activeSlotFilter || 'high';
				const maxCount = this.slotCounts()[kind] || 0;
				if (maxCount <= 0) {
					this.notify('This hull has no ' + kind + ' slots');
					return;
				}

				let idx = typeof targetIndex === 'number' ? targetIndex : -1;
				if (idx < 0 && this.selectedSlot && this.selectedSlot.kind === kind) {
					idx = this.selectedSlot.index;
				}
				if (idx < 0) {
					for (let i = 0; i < maxCount; i++) {
						if (!this.getModuleAt(kind, i)) {
							idx = i;
							break;
						}
					}
				}
				if (idx < 0) {
					this.notify('All ' + kind.toUpperCase() + ' slots are full — click a slot to replace it');
					return;
				}

				const nextMods = (this.fit.modules || []).filter((m) => !(m.slot_kind === kind && m.slot_index === idx));
				nextMods.push({
					slot_kind: kind,
					slot_index: idx,
					type_id: item.type_id,
					name: item.name,
					group_name: item.group_name || '',
					state: kind === 'rig' ? 'passive' : 'active',
				});
				this.fit.modules = nextMods;
				this.selectedSlot = null;
				this.runSimulate();
				this.notify('Fitted ' + item.name);
			},

			setModuleState(kind, index, newState) {
				const m = this.getModuleAt(kind, index);
				if (!m) return;
				m.state = newState;
				this.runSimulate();
			},

			cycleModuleState(kind, index) {
				const m = this.getModuleAt(kind, index);
				if (!m || kind === 'rig') return;
				const order = ['active', 'overheated', 'offline'];
				const next = order[(order.indexOf(m.state) + 1) % order.length];
				m.state = next;
				this.runSimulate();
			},

			removeModule(kind, index) {
				this.fit.modules = (this.fit.modules || []).filter((m) => !(m.slot_kind === kind && m.slot_index === index));
				this.selectedSlot = null;
				this.runSimulate();
			},

			toggleDrone(idx) {
				if (!this.fit.drones || !this.fit.drones[idx]) return;
				this.fit.drones[idx].active = !this.fit.drones[idx].active;
				this.runSimulate();
			},

			removeDrone(idx) {
				this.fit.drones.splice(idx, 1);
				this.runSimulate();
			},

			onDragStartModule(item, e) {
				this.draggingModule = item;
				if (e && e.dataTransfer) {
					e.dataTransfer.effectAllowed = 'copy';
					e.dataTransfer.setData('text/plain', String(item.type_id));
				}
			},

			onDropOnSlot(slot) {
				this.dragOverSlotKey = '';
				if (!this.draggingModule) return;
				this.equipModuleToSlot(this.draggingModule, slot.kind, slot.index);
				this.draggingModule = null;
			},

			setSkillProfile(prof) {
				this.fit.skill_profile = prof;
				this.runSimulate();
			},

			setTargetFaction(key) {
				this.fit.target_preset = key;
				this.runSimulate();
			},

			async importEFT() {
				if (!this.eftInputText.trim()) return;
				this.eftImporting = true;
				this.eftError = '';
				try {
					const res = await fetch('/tactical/api/import-eft', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							eft: this.eftInputText,
							skill_profile: this.fit.skill_profile,
							target_preset: this.fit.target_preset,
						}),
					});
					const data = await res.json();
					if (!res.ok) {
						this.eftError = data.error || 'Failed to parse EFT block';
						return;
					}
					this.sim = data;
					this.fit = data.fit;
					this.eftModalOpen = false;
					this.eftInputText = '';
					this.notify('Imported EFT: ' + this.fit.name);
				} catch (_) {
					this.eftError = 'Network error importing EFT';
				} finally {
					this.eftImporting = false;
				}
			},

			async saveFitting() {
				this.savingFit = true;
				try {
					const res = await fetch('/tactical/api/fittings', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify(this.fit),
					});
					const rec = await res.json();
					if (!res.ok) {
						this.notify(rec.error || 'Failed to save fit');
						return;
					}
					this.fit.id = rec.id;
					const idx = this.savedFittings.findIndex((f) => f.id === rec.id);
					if (idx >= 0) this.savedFittings.splice(idx, 1, rec);
					else this.savedFittings.unshift(rec);
					this.notify('Saved fit to Doctrine Library!');
				} catch (_) {
					this.notify('Network error saving fit');
				} finally {
					this.savingFit = false;
				}
			},

			async deleteSavedFitting(id) {
				try {
					const res = await fetch('/tactical/api/fittings/' + id, { method: 'DELETE' });
					if (res.ok) {
						this.savedFittings = this.savedFittings.filter((f) => f.id !== id);
						if (this.fit.id === id) this.fit.id = 0;
						this.notify('Deleted saved fit');
					}
				} catch (_) {}
			},

			copyToClipboard(text, label) {
				if (!text) return;
				navigator.clipboard.writeText(text).then(() => {
					this.notify('Copied ' + label + ' to clipboard');
				});
			},

			// =================================================================
			// Interactive 2D Tactical Range & Orbit Radar Sandbox (HTML5 Canvas)
			// =================================================================
			radarAppliedDPS() {
				if (!this.sim || !this.sim.offense) return { dps: 0, hitPct: 0, ttk: '—' };
				const off = this.sim.offense;
				const d = this.radarTargetDistKM;
				const opt = Math.max(off.optimal_range_km || 12, 1);
				const fall = Math.max(off.falloff_range_km || 8, 1);

				let rangeFactor = 1.0;
				if (d > opt) {
					const r = (d - opt) / fall;
					rangeFactor = Math.pow(0.5, r * r);
				}
				// Transversal tracking penalty approximation
				const angular = this.radarTargetSpeedMS / Math.max(d * 1000, 1000);
				const trackingPenalty = Math.max(0.18, 1.0 - Math.min(0.75, angular * 3.2 * (125 / Math.max(this.radarTargetSigM, 25))));
				const turretApp = (off.turret_dps || 0) * rangeFactor * trackingPenalty;
				const missileApp = d <= Math.max(opt, 32) ? (off.missile_dps || 0) * Math.min(1.0, this.radarTargetSigM / 90) : 0;
				const droneApp = d <= 55 ? (off.drone_dps || 0) : 0;
				const total = Math.round((turretApp + missileApp + droneApp) * 10) / 10;
				const maxBase = Math.max(off.total_dps || 1, 1);
				const hitPct = Math.min(100, Math.round((total / maxBase) * 100));
				const dummyTargetEHP = 18000;
				const ttk = total > 1 ? Math.round(dummyTargetEHP / total) + 's' : '∞';
				return { dps: total, hitPct, ttk };
			},

			startRadarLoop() {
				this.stopRadarLoop();
				const step = () => {
					this.radarPulsePhase = (this.radarPulsePhase + 0.015) % 1;
					this.drawRadarCanvas();
					this.radarAnimId = requestAnimationFrame(step);
				};
				this.radarAnimId = requestAnimationFrame(step);
			},

			stopRadarLoop() {
				if (this.radarAnimId) {
					cancelAnimationFrame(this.radarAnimId);
					this.radarAnimId = null;
				}
			},

			handleRadarPointer(e, isDown) {
				const canvas = this.$refs.radarCanvas;
				if (!canvas) return;
				if (isDown) this.radarDragging = true;
				if (!this.radarDragging) return;
				const rect = canvas.getBoundingClientRect();
				const cx = rect.width / 2;
				const cy = rect.height / 2;
				const dx = e.clientX - rect.left - cx;
				const dy = e.clientY - rect.top - cy;
				const maxRadiusPx = Math.min(cx, cy) - 24;
				const distPx = Math.sqrt(dx * dx + dy * dy);
				this.radarTargetDistKM = Math.max(1, Math.min(60, Math.round((distPx / maxRadiusPx) * 60)));
				this.radarTargetAngle = Math.atan2(dy, dx);
			},

			drawRadarCanvas() {
				const canvas = this.$refs.radarCanvas;
				if (!canvas) return;
				const dpr = window.devicePixelRatio || 1;
				const w = canvas.clientWidth || 540;
				const h = canvas.clientHeight || 420;
				if (canvas.width !== Math.floor(w * dpr) || canvas.height !== Math.floor(h * dpr)) {
					canvas.width = Math.floor(w * dpr);
					canvas.height = Math.floor(h * dpr);
				}
				const ctx = canvas.getContext('2d');
				ctx.save();
				ctx.scale(dpr, dpr);
				ctx.clearRect(0, 0, w, h);

				const cx = w / 2;
				const cy = h / 2;
				const maxR = Math.min(cx, cy) - 24;
				const kmToPx = (km) => (Math.min(km, 60) / 60) * maxR;

				// Deep space tactical grid
				ctx.strokeStyle = 'rgba(139, 92, 246, 0.14)';
				ctx.lineWidth = 1;
				[15, 30, 45, 60].forEach((km) => {
					const r = kmToPx(km);
					ctx.beginPath();
					ctx.arc(cx, cy, r, 0, Math.PI * 2);
					ctx.stroke();
					ctx.fillStyle = 'rgba(148, 163, 184, 0.55)';
					ctx.font = '10px monospace';
					ctx.fillText(km + ' km', cx + 4, cy - r + 11);
				});

				// Crosshair axes
				ctx.beginPath();
				ctx.moveTo(cx - maxR, cy); ctx.lineTo(cx + maxR, cy);
				ctx.moveTo(cx, cy - maxR); ctx.lineTo(cx, cy + maxR);
				ctx.stroke();

				// Animated radar sweep pulse
				ctx.beginPath();
				ctx.arc(cx, cy, this.radarPulsePhase * maxR, 0, Math.PI * 2);
				ctx.strokeStyle = 'rgba(56, 189, 248, ' + (0.28 * (1 - this.radarPulsePhase)).toFixed(2) + ')';
				ctx.lineWidth = 1.5;
				ctx.stroke();

				// EWAR Tactical Rings: Scram 9km, Web 10km, Disruptor 24km
				const drawRing = (km, color, dash, label) => {
					const r = kmToPx(km);
					ctx.save();
					ctx.setLineDash(dash);
					ctx.strokeStyle = color;
					ctx.lineWidth = 1.6;
					ctx.beginPath();
					ctx.arc(cx, cy, r, 0, Math.PI * 2);
					ctx.stroke();
					ctx.fillStyle = color;
					ctx.font = 'bold 9.5px monospace';
					ctx.fillText(label, cx + r * 0.72, cy + r * 0.72);
					ctx.restore();
				};

				const optKM = (this.sim && this.sim.offense && this.sim.offense.optimal_range_km) || 14;
				const fallKM = (this.sim && this.sim.offense && this.sim.offense.falloff_range_km) || 8;
				drawRing(9, 'rgba(244, 63, 94, 0.55)', [3, 3], 'SCRAM 9km');
				drawRing(24, 'rgba(251, 191, 36, 0.45)', [5, 4], 'POINT 24km');
				drawRing(optKM, 'rgba(52, 211, 153, 0.85)', [], 'OPTIMAL ' + optKM + 'km');
				if (fallKM > 0) {
					drawRing(optKM + fallKM, 'rgba(56, 189, 248, 0.60)', [6, 4], 'FALLOFF');
				}

				// Pilot ship center beacon
				ctx.fillStyle = '#fbbf24';
				ctx.beginPath();
				ctx.arc(cx, cy, 6, 0, Math.PI * 2);
				ctx.fill();
				ctx.strokeStyle = 'rgba(251, 191, 36, 0.45)';
				ctx.lineWidth = 5;
				ctx.stroke();

				// Draggable Enemy Target Blip
				const tR = kmToPx(this.radarTargetDistKM);
				const tx = cx + Math.cos(this.radarTargetAngle) * tR;
				const ty = cy + Math.sin(this.radarTargetAngle) * tR;

				// Engagement line
				ctx.strokeStyle = 'rgba(244, 63, 94, 0.75)';
				ctx.lineWidth = 1.5;
				ctx.setLineDash([4, 3]);
				ctx.beginPath();
				ctx.moveTo(cx, cy);
				ctx.lineTo(tx, ty);
				ctx.stroke();
				ctx.setLineDash([]);

				// Orbit tangent vector arrow
				const tanAngle = this.radarTargetAngle + Math.PI / 2;
				const arrowLen = 18 + Math.min(34, this.radarTargetSpeedMS / 25);
				const ax = tx + Math.cos(tanAngle) * arrowLen;
				const ay = ty + Math.sin(tanAngle) * arrowLen;
				ctx.strokeStyle = '#38bdf8';
				ctx.lineWidth = 2;
				ctx.beginPath();
				ctx.moveTo(tx, ty);
				ctx.lineTo(ax, ay);
				ctx.stroke();

				// Enemy reticle
				ctx.fillStyle = '#f43f5e';
				ctx.beginPath();
				ctx.arc(tx, ty, 8, 0, Math.PI * 2);
				ctx.fill();
				ctx.strokeStyle = '#fecdd3';
				ctx.lineWidth = 2;
				ctx.stroke();

				ctx.fillStyle = '#ffffff';
				ctx.font = 'bold 11px monospace';
				ctx.fillText('TARGET • ' + this.radarTargetDistKM + ' km', tx + 13, ty - 6);

				ctx.restore();
			},

			// =================================================================
			// Phase 3: 1v1 Duel & Boss Combat Timeline Simulator + SVG Chart
			// =================================================================
			async runDuelSim() {
				this.duelRunning = true;
				try {
					const res = await fetch('/tactical/api/duel', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							fit: this.fit,
							opponent_key: this.duelOpponentKey,
							range_km: Number(this.duelRangeKM) || 18,
							enemy_web: this.duelEnemyWeb,
							enemy_scram: this.duelEnemyScram,
							enemy_neut: this.duelEnemyNeut,
						}),
					});
					const data = await res.json();
					if (res.ok) {
						this.duelResult = data;
						this.duelScrubIdx = Math.max(0, (data.points || []).length - 1);
					}
				} catch (_) {
				} finally {
					this.duelRunning = false;
				}
			},

			duelScrubPoint() {
				if (!this.duelResult || !this.duelResult.points || this.duelResult.points.length === 0) return null;
				const idx = Math.max(0, Math.min(this.duelResult.points.length - 1, this.duelScrubIdx));
				return this.duelResult.points[idx];
			},

			duelTimelineSVG() {
				if (!this.duelResult || !this.duelResult.points || this.duelResult.points.length < 2) return '';
				const pts = this.duelResult.points;
				const W = 560, H = 170, padL = 34, padR = 14, padT = 14, padB = 22;
				const innerW = W - padL - padR;
				const innerH = H - padT - padB;
				const maxT = Math.max(this.duelResult.duration_sec || 1, 1);

				const xOf = (sec) => (padL + (sec / maxT) * innerW).toFixed(1);
				const yOf = (pct) => (padT + (1 - Math.max(0, Math.min(100, pct)) / 100) * innerH).toFixed(1);

				const poly = (getter) => pts.map((p) => `${xOf(p.second)},${yOf(getter(p))}`).join(' ');

				let svg = '';
				// Horizontal grid lines (0%, 50%, 100%)
				for (const pct of [0, 50, 100]) {
					const y = yOf(pct);
					svg += `<line x1="${padL}" y1="${y}" x2="${W - padR}" y2="${y}" stroke="rgba(148,163,184,0.16)" stroke-dasharray="3 4"></line>`;
					svg += `<text x="${padL - 5}" y="${Number(y) + 3}" text-anchor="end" fill="#94a3b8" font-size="9" font-family="monospace">${pct}%</text>`;
				}
				// Pilot HP (Emerald), Enemy HP (Rose), Pilot Cap (Amber dashed)
				svg += `<polyline fill="none" stroke="#34d399" stroke-width="2.4" points="${poly((p) => p.pilot_hp_pct)}"></polyline>`;
				svg += `<polyline fill="none" stroke="#fb7185" stroke-width="2.4" points="${poly((p) => p.enemy_hp_pct)}"></polyline>`;
				svg += `<polyline fill="none" stroke="#fbbf24" stroke-width="1.5" stroke-dasharray="4 3" points="${poly((p) => p.pilot_cap_pct)}"></polyline>`;

				// Cap Burnout Marker
				if (this.duelResult.cap_burnout_sec > 0) {
					const bx = xOf(this.duelResult.cap_burnout_sec);
					svg += `<line x1="${bx}" y1="${padT}" x2="${bx}" y2="${H - padB}" stroke="#f59e0b" stroke-width="1.2" stroke-dasharray="2 2"></line>`;
					svg += `<text x="${bx}" y="${padT + 9}" text-anchor="middle" fill="#fbbf24" font-size="8.5" font-family="monospace">CAP DRY ${this.duelResult.cap_burnout_sec}s</text>`;
				}

				// Interactive Scrubber Vertical Line
				const sp = this.duelScrubPoint();
				if (sp) {
					const sx = xOf(sp.second);
					svg += `<line x1="${sx}" y1="${padT}" x2="${sx}" y2="${H - padB}" stroke="#e2e8f0" stroke-width="1.2"></line>`;
					svg += `<circle cx="${sx}" cy="${yOf(sp.pilot_hp_pct)}" r="3.8" fill="#34d399" stroke="#0f172a" stroke-width="1.2"></circle>`;
					svg += `<circle cx="${sx}" cy="${yOf(sp.enemy_hp_pct)}" r="3.8" fill="#fb7185" stroke="#0f172a" stroke-width="1.2"></circle>`;
					svg += `<text x="${sx}" y="${H - 6}" text-anchor="middle" fill="#e2e8f0" font-size="9" font-family="monospace">t=${sp.second}s</text>`;
				}
				return svg;
			},

			// =================================================================
			// Phase 4: Embedded Background AI Tactical Audit + 1-Click Swap
			// =================================================================
			async runAIAudit() {
				this.aiAuditing = true;
				try {
					const res = await fetch('/tactical/api/ai-audit', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify(this.fit),
					});
					const data = await res.json();
					if (res.ok) {
						this.aiAudit = data;
						this.notify(data.source === 'ai_cached' ? 'AI Audit loaded instantly (cached)' : 'Tactical Audit ready');
					}
				} catch (_) {
					this.notify('Failed to run Tactical Audit');
				} finally {
					this.aiAuditing = false;
				}
			},

			applyAISwap(swap) {
				if (!swap || !swap.suggested_type_id) {
					this.notify('Search "' + (swap && swap.suggested_module) + '" in Module Drawer to equip');
					return;
				}
				this.equipModuleToSlot(
					{ type_id: swap.suggested_type_id, name: swap.suggested_module, group_name: '' },
					swap.slot_kind || 'mid'
				);
			},

			// =================================================================
			// Phase 5: D-Scan Instant Threat Radar & Combat Log Post-Mortem
			// =================================================================
			loadSampleDScan() {
				this.dscanInput = "16240\tUedama Ganker 1\tCatalyst\t0.4 AU\n16240\tUedama Ganker 2\tCatalyst\t0.4 AU\n16240\tUedama Ganker 3\tCatalyst\t0.4 AU\n17918\tCombat Scanner Probe\tCombat Scanner Probe\t1,200 km\n11198\tGate Scout\tStiletto\t850 km";
				this.runDScan();
			},

			async runDScan() {
				if (!this.dscanInput.trim()) return;
				this.dscanRunning = true;
				this.dscanError = '';
				try {
					const res = await fetch('/tactical/api/dscan', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ raw: this.dscanInput }),
					});
					const data = await res.json();
					if (!res.ok) {
						this.dscanError = data.error || 'Failed to parse D-Scan';
						return;
					}
					this.dscanResult = data;
				} catch (_) {
					this.dscanError = 'Network error analyzing D-Scan';
				} finally {
					this.dscanRunning = false;
				}
			},

			loadSampleCombatLog() {
				this.combatLogInput = "[ 2026.11.27 14:22:01 ] (combat) 340 from Sansha's Centurion - Penetrates\n[ 2026.11.27 14:22:03 ] (combat) 290 from Sansha's Centurion - Hits\n[ 2026.11.27 14:22:05 ] (combat) 410 to Sansha's Centurion - Smashes\n[ 2026.11.27 14:22:07 ] (combat) 85 from Sansha's Frigate - Glances Off\n[ 2026.11.27 14:22:09 ] (combat) Energy Neutralizer drained 60 GJ\n[ 2026.11.27 14:22:11 ] (combat) 480 to Sansha's Centurion - Wrecks";
				this.runCombatLog();
			},

			async runCombatLog() {
				if (!this.combatLogInput.trim()) return;
				this.combatLogRunning = true;
				this.combatLogError = '';
				try {
					const res = await fetch('/tactical/api/combat-log', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ raw: this.combatLogInput }),
					});
					const data = await res.json();
					if (!res.ok) {
						this.combatLogError = data.error || 'Failed to parse Combat Log';
						return;
					}
					this.combatLogResult = data;
				} catch (_) {
					this.combatLogError = 'Network error parsing Combat Log';
				} finally {
					this.combatLogRunning = false;
				}
			},

			// =================================================================
			// Batch 3: Fly Today (T2 -> Beginner Meta/T1) & Combat ROI Bridge
			// =================================================================
			async runFlyToday() {
				this.flyTodayRunning = true;
				try {
					const res = await fetch('/tactical/api/fly-today', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify(this.fit),
					});
					const data = await res.json();
					if (!res.ok) {
						this.notify(data.error || 'Could not adapt fit');
						return;
					}
					if (data.simulation) {
						this.sim = data.simulation;
						this.fit = data.simulation.fit;
					}
					if (data.swapped_count > 0) {
						this.notify('Adapted ' + data.swapped_count + ' T2 modules for beginners (Saved ' + formatISK(data.isk_saved) + ')');
					} else {
						this.notify('Fit is already beginner-friendly (no Tech 2 swaps needed)');
					}
				} catch (_) {
					this.notify('Network error adapting fit');
				} finally {
					this.flyTodayRunning = false;
				}
			},

			roiMetrics() {
				const cost = (this.sim && this.sim.total_fit_cost_isk) || 0;
				const act = this.roiActivities.find((a) => a.key === this.roiActivityKey) || this.roiActivities[1];
				const hrs = act.iskPerHour > 0 ? cost / act.iskPerHour : 0;
				let risk = 'SAFE';
				if (hrs > 6) risk = 'HIGH EXPOSURE';
				else if (hrs > 2.5) risk = 'MODERATE';
				return {
					iskPerHour: act.iskPerHour,
					paybackHours: Math.round(hrs * 10) / 10,
					risk,
					barPct: Math.min(100, Math.round((hrs / 8) * 100)),
				};
			},

			rangeCurveSVG() {
				if (!this.sim || !this.sim.range_curve || this.sim.range_curve.length < 2) return '';
				const pts = this.sim.range_curve;
				const W = 560, H = 128, padL = 38, padR = 12, padT = 12, padB = 20;
				const innerW = W - padL - padR;
				const innerH = H - padT - padB;
				const maxDPS = Math.max(10, ...pts.map((p) => p.applied_dps));
				const maxKM = 80;

				const xOf = (km) => (padL + (Math.min(km, maxKM) / maxKM) * innerW).toFixed(1);
				const yOf = (dps) => (padT + (1 - Math.max(0, dps) / maxDPS) * innerH).toFixed(1);

				const linePts = pts.map((p) => `${xOf(p.distance_km)},${yOf(p.applied_dps)}`).join(' ');
				const areaPts = `${xOf(0)},${yOf(0)} ${linePts} ${xOf(pts[pts.length - 1].distance_km)},${yOf(0)}`;

				let svg = '';
				for (const frac of [0, 0.5, 1]) {
					const y = yOf(maxDPS * frac);
					svg += `<line x1="${padL}" y1="${y}" x2="${W - padR}" y2="${y}" stroke="rgba(148,163,184,0.14)" stroke-dasharray="3 4"></line>`;
					svg += `<text x="${padL - 5}" y="${Number(y) + 3}" text-anchor="end" fill="#94a3b8" font-size="8.5" font-family="monospace">${Math.round(maxDPS * frac)}</text>`;
				}
				svg += `<polygon points="${areaPts}" fill="rgba(56,189,248,0.18)"></polygon>`;
				svg += `<polyline fill="none" stroke="#38bdf8" stroke-width="2.2" points="${linePts}"></polyline>`;

				// Synced vertical crosshair matching 2D Radar Target Distance
				const cx = xOf(this.radarTargetDistKM);
				svg += `<line x1="${cx}" y1="${padT}" x2="${cx}" y2="${H - padB}" stroke="#fbbf24" stroke-width="1.5" stroke-dasharray="3 2"></line>`;
				svg += `<text x="${cx}" y="${H - 5}" text-anchor="middle" fill="#fbbf24" font-size="8.5" font-family="monospace">${this.radarTargetDistKM} km</text>`;
				return svg;
			},

			// =================================================================
			// Batch 5: Skill-Gap Lock Helpers, Interactive Ammo Picker, & Compare
			// =================================================================
			isSlotLocked(slotKey) {
				if (!this.sim || !this.sim.locked_slot_keys) return false;
				return this.sim.locked_slot_keys.includes(slotKey);
			},

			suggestedChargesForModule(mod) {
				if (!mod || !mod.name) return [];
				const low = mod.name.toLowerCase();
				if (low.includes('pulse') || low.includes('beam') || low.includes('laser')) {
					return [
						{ type_id: 237, name: 'Multifrequency M', tag: 'Short • High DPS' },
						{ type_id: 12814, name: 'Scorch M', tag: 'Long • T2 Pulse' },
						{ type_id: 12818, name: 'Conflagration M', tag: 'Close • Max DPS' },
						{ type_id: 243, name: 'Radio M', tag: 'Max Range' },
					];
				}
				if (low.includes('missile') || low.includes('launcher')) {
					return [
						{ type_id: 209, name: 'Scourge Light Missile', tag: 'Kinetic' },
						{ type_id: 210, name: 'Inferno Light Missile', tag: 'Thermal' },
						{ type_id: 211, name: 'Mjolnir Light Missile', tag: 'EM' },
						{ type_id: 212, name: 'Nova Light Missile', tag: 'Explosive' },
					];
				}
				if (low.includes('blaster') || low.includes('railgun')) {
					return [
						{ type_id: 222, name: 'Antimatter Charge M', tag: 'High DPS' },
						{ type_id: 12789, name: 'Void M', tag: 'Max Close DPS' },
						{ type_id: 12791, name: 'Null M', tag: 'Extended Range' },
						{ type_id: 216, name: 'Iron Charge M', tag: 'Long Range' },
					];
				}
				if (low.includes('autocannon') || low.includes('artillery')) {
					return [
						{ type_id: 192, name: 'EMP M', tag: 'EM Damage' },
						{ type_id: 193, name: 'Phased Plasma M', tag: 'Thermal' },
						{ type_id: 194, name: 'Fusion M', tag: 'Explosive' },
						{ type_id: 12779, name: 'Barrage M', tag: 'Falloff Range' },
					];
				}
				return [];
			},

			loadChargeOnSlot(kind, index, charge, applyAllSame) {
				const targetMod = this.getModuleAt(kind, index);
				if (!targetMod) return;
				const applyOne = (m) => {
					m.charge_type_id = charge ? charge.type_id : 0;
					m.charge_name = charge ? charge.name : '';
				};
				if (applyAllSame) {
					for (const m of (this.fit.modules || [])) {
						if (m.slot_kind === kind && m.type_id === targetMod.type_id) {
							applyOne(m);
						}
					}
					this.notify(charge ? ('Loaded ' + charge.name + ' into all matching weapons') : 'Unloaded matching weapons');
				} else {
					applyOne(targetMod);
					this.notify(charge ? ('Loaded ' + charge.name) : 'Unloaded charge');
				}
				this.runSimulate();
			},

			allDoctrineOptions() {
				const opts = [];
				for (const pr of this.presets) {
					opts.push({ id: 'preset:' + pr.key, label: '[Starter] ' + pr.name, fit: pr.fit });
				}
				for (const sf of this.savedFittings) {
					opts.push({ id: 'saved:' + sf.id, label: '[Saved] ' + sf.name, fit: sf.fit });
				}
				return opts;
			},

			findDoctrineFit(id) {
				const found = this.allDoctrineOptions().find((o) => o.id === id);
				return found ? found.fit : null;
			},

			async runFitComparison() {
				const fitA = this.findDoctrineFit(this.compareKeyA);
				const fitB = this.findDoctrineFit(this.compareKeyB);
				if (!fitA || !fitB) {
					this.notify('Select two doctrines to compare');
					return;
				}
				this.compareRunning = true;
				try {
					const res = await fetch('/tactical/api/compare-fits', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ fit_a: fitA, fit_b: fitB }),
					});
					const data = await res.json();
					if (res.ok) {
						this.compareResult = data;
					}
				} catch (_) {
					this.notify('Failed to compare fits');
				} finally {
					this.compareRunning = false;
				}
			},

			adjustFleetCount(key, delta) {
				const cur = Number(this.fleetCounts[key] || 0);
				this.fleetCounts[key] = Math.max(0, Math.min(20, cur + delta));
			},

			fleetSummary() {
				let totalShips = 0;
				let totalISK = 0;
				let totalEHP = 0;
				let totalDPS = 0;
				const multibuyLines = [];
				for (const sf of this.savedFittings) {
					const n = Number(this.fleetCounts['saved:' + sf.id] || 0);
					if (n <= 0) continue;
					totalShips += n;
					totalISK += (sf.estimated_cost_isk || 0) * n;
					totalEHP += (sf.ehp || 0) * n;
					totalDPS += (sf.dps || 0) * n;
					multibuyLines.push(sf.ship_name + ' x' + n);
				}
				return { totalShips, totalISK, totalEHP, totalDPS, multibuy: multibuyLines.join('\n') };
			},

			// =================================================================
			// Batch 6: Hover Ghost Preview & Abyssal Weather / Implants Modifiers
			// =================================================================
			startGhostPreview(mod) {
				if (!mod || !mod.type_id || this.activeSlotFilter === 'drone') return;
				this.hoverModule = mod;
				if (this.ghostTimer) clearTimeout(this.ghostTimer);
				const seq = ++this.ghostSeq;
				this.ghostTimer = setTimeout(async () => {
					const kind = (this.selectedSlot && this.selectedSlot.kind) || this.activeSlotFilter || 'high';
					const maxCount = this.slotCounts()[kind] || 0;
					if (maxCount <= 0) return;
					let idx = (this.selectedSlot && this.selectedSlot.kind === kind) ? this.selectedSlot.index : -1;
					if (idx < 0) {
						for (let i = 0; i < maxCount; i++) {
							if (!this.getModuleAt(kind, i)) { idx = i; break; }
						}
					}
					if (idx < 0) idx = maxCount - 1;

					const cloned = JSON.parse(JSON.stringify(this.fit));
					cloned.modules = (cloned.modules || []).filter((m) => !(m.slot_kind === kind && m.slot_index === idx));
					cloned.modules.push({
						slot_kind: kind,
						slot_index: idx,
						type_id: mod.type_id,
						name: mod.name,
						state: kind === 'rig' ? 'passive' : 'active',
					});
					try {
						const res = await fetch('/tactical/api/simulate', {
							method: 'POST',
							headers: { 'Content-Type': 'application/json' },
							body: JSON.stringify(cloned),
						});
						const data = await res.json();
						if (res.ok && seq === this.ghostSeq) {
							this.ghostSim = data;
							this.ghostSlotKey = kind + '-' + idx;
						}
					} catch (_) {}
				}, 180);
			},

			clearGhostPreview() {
				this.ghostSeq++;
				if (this.ghostTimer) clearTimeout(this.ghostTimer);
				this.hoverModule = null;
				this.ghostSim = null;
				this.ghostSlotKey = '';
			},

			ghostDeltas() {
				if (!this.ghostSim || !this.sim) return null;
				return {
					name: (this.hoverModule && this.hoverModule.name) || 'Module Preview',
					ehp: Math.round(this.ghostSim.total_ehp - this.sim.total_ehp),
					dps: Math.round((this.ghostSim.offense.total_dps - this.sim.offense.total_dps) * 10) / 10,
					cpu: Math.round((this.ghostSim.cpu.used - this.sim.cpu.used) * 10) / 10,
					pg: Math.round((this.ghostSim.powergrid.used - this.sim.powergrid.used) * 10) / 10,
					overloaded: !!(this.ghostSim.cpu.overloaded || this.ghostSim.powergrid.overloaded || this.ghostSim.calibration.overloaded),
				};
			},

			setAbyssalWeather(w) {
				this.fit.abyssal_weather = (this.fit.abyssal_weather === w) ? '' : w;
				this.runSimulate();
			},

			toggleImplant(key) {
				const list = Array.isArray(this.fit.implants) ? [...this.fit.implants] : [];
				const idx = list.indexOf(key);
				if (idx >= 0) list.splice(idx, 1);
				else list.push(key);
				this.fit.implants = list;
				this.runSimulate();
			},

			hasImplant(key) {
				return Array.isArray(this.fit.implants) && this.fit.implants.includes(key);
			},
		};
	};
})();
