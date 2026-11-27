// aura-forge Interactive HTML5 Canvas Planet Colony Map Visualizer
// Renders EVE Online PI pins (Command Center, Extractor + Heads, Launchpad, Storage, Factories),
// links, and animated material routes using exact spherical (latitude/longitude) coordinates from ESI.
(function () {
	const ROLE_STYLE = {
		command_center:   { color: '#fbbf24', fill: 'rgba(251,191,36,0.22)',  short: 'CC',  label: 'Command Center',     r: 16 },
		extractor:        { color: '#f59e0b', fill: 'rgba(245,158,11,0.22)',  short: 'EXT', label: 'Extractor Control',  r: 15 },
		launchpad:        { color: '#34d399', fill: 'rgba(52,211,153,0.22)',  short: 'PAD', label: 'Launchpad',          r: 15 },
		storage:          { color: '#22d3ee', fill: 'rgba(34,211,238,0.20)',  short: 'STR', label: 'Storage Facility',   r: 13 },
		basic_factory:    { color: '#a78bfa', fill: 'rgba(167,139,250,0.22)', short: 'P1',  label: 'Basic Factory',      r: 13 },
		advanced_factory: { color: '#38bdf8', fill: 'rgba(56,189,248,0.22)',  short: 'P2+', label: 'Advanced Factory',   r: 13 },
		hightech_factory: { color: '#e879f9', fill: 'rgba(232,121,249,0.24)', short: 'P4',  label: 'High-Tech Factory',  r: 14 },
		facility:         { color: '#94a3b8', fill: 'rgba(148,163,184,0.18)', short: 'PIN', label: 'Facility',           r: 12 },
	};

	window.afPIColonyMap = function () {
		return {
			open: false,
			loading: false,
			error: '',
			planetName: '',
			layout: null,
			selectedPin: null,
			showRoutes: true,
			showHeads: true,
			zoom: 1,
			panX: 0,
			panY: 0,
			dragging: false,
			dragStartX: 0,
			dragStartY: 0,
			animFrame: null,
			phase: 0,
			projected: [],
			pinById: {},
			scaleRad: 1,

			async openPlanet(planetId, planetName) {
				this.open = true;
				this.loading = true;
				this.error = '';
				this.planetName = planetName || ('Planet #' + planetId);
				this.layout = null;
				this.selectedPin = null;
				this.zoom = 1;
				this.panX = 0;
				this.panY = 0;
				this.stopAnim();
				try {
					const res = await fetch('/pi/colonies/' + planetId + '/layout');
					const data = await res.json();
					if (!res.ok) {
						this.error = data.error || 'Failed to load planet layout';
						return;
					}
					this.layout = data;
					if (data.planet_name) this.planetName = data.planet_name;
					this.$nextTick(() => {
						this.computeProjection();
						this.startAnim();
					});
				} catch (e) {
					this.error = 'Network error loading planet layout';
				} finally {
					this.loading = false;
				}
			},

			closeModal() {
				this.open = false;
				this.stopAnim();
			},

			resetView() {
				this.zoom = 1;
				this.panX = 0;
				this.panY = 0;
			},

			zoomBy(factor) {
				this.zoom = Math.max(0.4, Math.min(5.0, this.zoom * factor));
			},

			stopAnim() {
				if (this.animFrame) {
					cancelAnimationFrame(this.animFrame);
					this.animFrame = null;
				}
			},

			startAnim() {
				this.stopAnim();
				const tick = () => {
					if (!this.open) return;
					this.phase = (this.phase + 0.012) % 1;
					this.draw();
					this.animFrame = requestAnimationFrame(tick);
				};
				this.animFrame = requestAnimationFrame(tick);
			},

			computeProjection() {
				const canvas = this.$refs.piCanvas;
				if (!canvas || !this.layout || !this.layout.pins || this.layout.pins.length === 0) {
					this.projected = [];
					this.pinById = {};
					return;
				}
				const rect = canvas.getBoundingClientRect();
				const w = Math.max(480, Math.floor(rect.width || 720));
				const h = Math.max(380, Math.floor(rect.height || 460));
				const dpr = window.devicePixelRatio || 1;
				canvas.width = w * dpr;
				canvas.height = h * dpr;

				// Collect all coordinates (pins + extractor heads) for accurate centering & scale
				const pts = [];
				for (const p of this.layout.pins) {
					pts.push({ lat: p.latitude, lon: p.longitude });
					if (Array.isArray(p.heads)) {
						for (const hd of p.heads) pts.push({ lat: hd.latitude, lon: hd.longitude });
					}
				}

				// Circular mean for longitude (handles antimeridian wrap cleanly)
				let sinLon = 0, cosLon = 0, sumLat = 0;
				for (const pt of pts) {
					sinLon += Math.sin(pt.lon);
					cosLon += Math.cos(pt.lon);
					sumLat += pt.lat;
				}
				const centerLon = Math.atan2(sinLon / pts.length, cosLon / pts.length);
				const centerLat = sumLat / pts.length;
				const cosLat = Math.max(0.25, Math.cos(centerLat));

				const normDeltaLon = (lon) => {
					let d = lon - centerLon;
					while (d > Math.PI) d -= 2 * Math.PI;
					while (d < -Math.PI) d += 2 * Math.PI;
					return d;
				};

				let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
				const rawPts = pts.map(pt => {
					const x = normDeltaLon(pt.lon) * cosLat;
					const y = -(pt.lat - centerLat);
					if (x < minX) minX = x;
					if (x > maxX) maxX = x;
					if (y < minY) minY = y;
					if (y > maxY) maxY = y;
					return { x, y };
				});

				const spanX = Math.max(0.005, maxX - minX);
				const spanY = Math.max(0.005, maxY - minY);
				const pad = 72;
				const scale = Math.min((w - pad * 2) / spanX, (h - pad * 2) / spanY);
				const midX = (minX + maxX) / 2;
				const midY = (minY + maxY) / 2;
				this.scaleRad = scale;

				const toCanvas = (lat, lon) => ({
					x: w / 2 + (normDeltaLon(lon) * cosLat - midX) * scale,
					y: h / 2 + (-(lat - centerLat) - midY) * scale,
				});

				this.projected = [];
				this.pinById = {};
				for (const p of this.layout.pins) {
					const pos = toCanvas(p.latitude, p.longitude);
					const heads = (p.heads || []).map(hd => ({
						...hd,
						...toCanvas(hd.latitude, hd.longitude),
					}));
					const item = { ...p, x: pos.x, y: pos.y, pHeads: heads };
					this.projected.push(item);
					this.pinById[String(p.pin_id)] = item;
				}
			},

			onWheel(e) {
				e.preventDefault();
				const factor = e.deltaY < 0 ? 1.12 : 0.89;
				this.zoomBy(factor);
			},

			onMouseDown(e) {
				this.dragging = true;
				this.dragStartX = e.clientX - this.panX;
				this.dragStartY = e.clientY - this.panY;
			},

			onMouseMove(e) {
				if (!this.dragging) return;
				this.panX = e.clientX - this.dragStartX;
				this.panY = e.clientY - this.dragStartY;
			},

			onMouseUp() {
				this.dragging = false;
			},

			onCanvasClick(e) {
				const canvas = this.$refs.piCanvas;
				if (!canvas || !this.projected.length) return;
				const rect = canvas.getBoundingClientRect();
				const cx = e.clientX - rect.left;
				const cy = e.clientY - rect.top;
				const w = rect.width;
				const h = rect.height;

				// Invert transform
				const wx = (cx - w / 2 - this.panX) / this.zoom + w / 2;
				const wy = (cy - h / 2 - this.panY) / this.zoom + h / 2;

				let best = null;
				let bestDist = 24 / Math.sqrt(this.zoom);
				for (const p of this.projected) {
					const d = Math.hypot(p.x - wx, p.y - wy);
					if (d < bestDist) {
						bestDist = d;
						best = p;
					}
				}
				this.selectedPin = best;
			},

			pinRoutes(pinId) {
				if (!this.layout || !this.layout.routes) return { incoming: [], outgoing: [] };
				const idStr = String(pinId);
				const incoming = [];
				const outgoing = [];
				for (const r of this.layout.routes) {
					if (String(r.destination_pin_id) === idStr) incoming.push(r);
					if (String(r.source_pin_id) === idStr) outgoing.push(r);
				}
				return { incoming, outgoing };
			},

			draw() {
				const canvas = this.$refs.piCanvas;
				if (!canvas) return;
				const ctx = canvas.getContext('2d');
				const dpr = window.devicePixelRatio || 1;
				const w = canvas.width / dpr;
				const h = canvas.height / dpr;

				ctx.save();
				ctx.scale(dpr, dpr);

				// Deep Orbital Space planet surface backdrop
				const bgGrad = ctx.createRadialGradient(w * 0.5, h * 0.5, 20, w * 0.5, h * 0.5, Math.max(w, h) * 0.72);
				bgGrad.addColorStop(0, '#1a1738');
				bgGrad.addColorStop(0.65, '#100e24');
				bgGrad.addColorStop(1, '#090b16');
				ctx.fillStyle = bgGrad;
				ctx.fillRect(0, 0, w, h);

				// Subtle tactical grid
				ctx.strokeStyle = 'rgba(139, 92, 246, 0.08)';
				ctx.lineWidth = 1;
				for (let gx = 40; gx < w; gx += 48) {
					ctx.beginPath(); ctx.moveTo(gx, 0); ctx.lineTo(gx, h); ctx.stroke();
				}
				for (let gy = 40; gy < h; gy += 48) {
					ctx.beginPath(); ctx.moveTo(0, gy); ctx.lineTo(w, gy); ctx.stroke();
				}

				// Apply pan & zoom around center
				ctx.translate(w / 2 + this.panX, h / 2 + this.panY);
				ctx.scale(this.zoom, this.zoom);
				ctx.translate(-w / 2, -h / 2);

				// 1. Draw Extractor Head circles & tethers
				if (this.showHeads) {
					for (const p of this.projected) {
						if (!p.pHeads || !p.pHeads.length) continue;
						const headR = Math.max(6, Math.min(28, (p.head_radius || 0.015) * this.scaleRad));
						for (const hd of p.pHeads) {
							ctx.strokeStyle = 'rgba(245, 158, 11, 0.35)';
							ctx.lineWidth = 1;
							ctx.setLineDash([3, 3]);
							ctx.beginPath();
							ctx.moveTo(p.x, p.y);
							ctx.lineTo(hd.x, hd.y);
							ctx.stroke();
							ctx.setLineDash([]);

							ctx.fillStyle = 'rgba(245, 158, 11, 0.14)';
							ctx.strokeStyle = 'rgba(251, 191, 36, 0.55)';
							ctx.beginPath();
							ctx.arc(hd.x, hd.y, headR, 0, Math.PI * 2);
							ctx.fill();
							ctx.stroke();

							ctx.fillStyle = '#fde68a';
							ctx.beginPath();
							ctx.arc(hd.x, hd.y, 2.5, 0, Math.PI * 2);
							ctx.fill();
						}
					}
				}

				// 2. Draw physical Links (pipes)
				if (this.layout && this.layout.links) {
					for (const l of this.layout.links) {
						const a = this.pinById[String(l.source_pin_id)];
						const b = this.pinById[String(l.destination_pin_id)];
						if (!a || !b) continue;
						ctx.strokeStyle = 'rgba(148, 163, 184, 0.38)';
						ctx.lineWidth = 1.5 + Math.min(3, (l.link_level || 0) * 0.4);
						ctx.beginPath();
						ctx.moveTo(a.x, a.y);
						ctx.lineTo(b.x, b.y);
						ctx.stroke();
					}
				}

				// 3. Draw animated Material Routes
				if (this.showRoutes && this.layout && this.layout.routes) {
					for (const r of this.layout.routes) {
						const a = this.pinById[String(r.source_pin_id)];
						const b = this.pinById[String(r.destination_pin_id)];
						if (!a || !b) continue;
						ctx.strokeStyle = 'rgba(56, 189, 248, 0.45)';
						ctx.lineWidth = 1.6;
						ctx.setLineDash([5, 5]);
						ctx.beginPath();
						ctx.moveTo(a.x, a.y);
						ctx.lineTo(b.x, b.y);
						ctx.stroke();
						ctx.setLineDash([]);

						// Moving pulse particle along route
						const px = a.x + (b.x - a.x) * this.phase;
						const py = a.y + (b.y - a.y) * this.phase;
						ctx.fillStyle = '#38bdf8';
						ctx.beginPath();
						ctx.arc(px, py, 2.8, 0, Math.PI * 2);
						ctx.fill();
					}
				}

				// 4. Draw Pin Nodes
				for (const p of this.projected) {
					const st = ROLE_STYLE[p.role] || ROLE_STYLE.facility;
					const isSelected = this.selectedPin && this.selectedPin.pin_id === p.pin_id;
					const r = st.r;

					if (isSelected) {
						ctx.strokeStyle = '#ffffff';
						ctx.lineWidth = 2.5;
						ctx.beginPath();
						ctx.arc(p.x, p.y, r + 5, 0, Math.PI * 2);
						ctx.stroke();
					}

					ctx.fillStyle = st.fill;
					ctx.strokeStyle = st.color;
					ctx.lineWidth = 2;

					if (p.role === 'extractor') {
						// Diamond
						ctx.beginPath();
						ctx.moveTo(p.x, p.y - r);
						ctx.lineTo(p.x + r, p.y);
						ctx.lineTo(p.x, p.y + r);
						ctx.lineTo(p.x - r, p.y);
						ctx.closePath();
						ctx.fill();
						ctx.stroke();
					} else if (p.role === 'launchpad' || p.role === 'storage') {
						// Square
						ctx.beginPath();
						ctx.rect(p.x - r * 0.85, p.y - r * 0.85, r * 1.7, r * 1.7);
						ctx.fill();
						ctx.stroke();
					} else {
						// Circle
						ctx.beginPath();
						ctx.arc(p.x, p.y, r, 0, Math.PI * 2);
						ctx.fill();
						ctx.stroke();
					}

					// Short badge text inside pin
					ctx.fillStyle = '#f8fafc';
					ctx.font = 'bold 9px ui-monospace, SFMono-Regular, monospace';
					ctx.textAlign = 'center';
					ctx.textBaseline = 'middle';
					ctx.fillText(st.short, p.x, p.y);

					// Sub-label (Schematic or Extracted Product)
					const caption = p.schematic_name || p.product_name || '';
					if (caption) {
						ctx.fillStyle = 'rgba(226, 232, 240, 0.88)';
						ctx.font = '10px system-ui, -apple-system, sans-serif';
						ctx.fillText(caption.length > 18 ? caption.slice(0, 17) + '…' : caption, p.x, p.y + r + 11);
					}
				}

				ctx.restore();
			}
		};
	};
})();
