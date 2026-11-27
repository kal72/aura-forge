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
			savedFittings: initialSaved,

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
						this.$nextTick(() => this.startRadarLoop());
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
				// CPU Segmented Arc (Right-Mid — Ice Blue #38bdf8)
				for (const seg of this.cpuTicks()) {
					const stroke = seg.active ? (cpuOver ? '#f43f5e' : '#38bdf8') : 'rgba(56,189,248,0.18)';
					svg += `<line x1="${seg.x1}" y1="${seg.y1}" x2="${seg.x2}" y2="${seg.y2}" stroke="${stroke}" stroke-width="2.8" stroke-linecap="round"></line>`;
				}
				// Powergrid Segmented Arc (Bottom-Right — Crimson Coral #fb7185)
				for (const seg of this.powergridTicks()) {
					const stroke = seg.active ? (pgOver ? '#ff2a55' : '#fb7185') : 'rgba(244,63,94,0.20)';
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
		};
	};
})();
