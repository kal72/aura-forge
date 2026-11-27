// aura-forge Interactive HTML5 Canvas Planet Colony Map Visualizer
// Features Semantic Zoom (nodes spread apart when zooming while shapes & labels stay crisp constant size).
(function () {
	const ROLE_STYLE = {
		command_center:   { color: '#fbbf24', fill: 'rgba(251,191,36,0.24)',  short: 'CC',  r: 17 },
		extractor:        { color: '#f59e0b', fill: 'rgba(245,158,11,0.24)',  short: 'EXT', r: 17 },
		launchpad:        { color: '#34d399', fill: 'rgba(52,211,153,0.24)',  short: 'PAD', r: 16 },
		storage:          { color: '#22d3ee', fill: 'rgba(34,211,238,0.22)',  short: 'STR', r: 15 },
		basic_factory:    { color: '#a78bfa', fill: 'rgba(167,139,250,0.24)', short: 'P1',  r: 15 },
		advanced_factory: { color: '#38bdf8', fill: 'rgba(56,189,248,0.24)',  short: 'P2+', r: 15 },
		hightech_factory: { color: '#e879f9', fill: 'rgba(232,121,249,0.26)', short: 'P4',  r: 16 },
		facility:         { color: '#94a3b8', fill: 'rgba(148,163,184,0.20)', short: 'PIN', r: 14 },
	};

	window.afPIColonyMap = function () {
		return {
			open: false,
			loading: false,
			error: '',
			planetName: '',
			planetIconTypeId: 11,
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
			resizeObserver: null,
			phase: 0,
			projected: [],
			pinById: {},
			scaleRad: 1,
			viewW: 800,
			viewH: 540,

			attachResizeObserver() {
				const wrap = this.$refs.piCanvasWrap || (this.$refs.piCanvas && this.$refs.piCanvas.parentElement);
				if (!wrap || typeof ResizeObserver === 'undefined') return;
				if (this.resizeObserver) {
					this.resizeObserver.disconnect();
				}
				this.resizeObserver = new ResizeObserver(() => {
					if (!this.open || !this.layout) return;
					const rect = wrap.getBoundingClientRect();
					const w = Math.floor(rect.width);
					const h = Math.floor(rect.height);
					if (w > 0 && h > 0 && (w !== this.viewW || h !== this.viewH)) {
						this.computeProjection();
					}
				});
				this.resizeObserver.observe(wrap);
			},

			async openPlanet(planetId, planetName, planetIconId) {
				this.open = true;
				this.loading = true;
				this.error = '';
				this.planetName = planetName || ('Planet #' + planetId);
				this.planetIconTypeId = planetIconId || 11;
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
					if (data.planet_icon_type_id) this.planetIconTypeId = data.planet_icon_type_id;
					this.$nextTick(() => {
						requestAnimationFrame(() => {
							this.attachResizeObserver();
							this.computeProjection();
							this.startAnim();
						});
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
				if (this.resizeObserver) {
					this.resizeObserver.disconnect();
					this.resizeObserver = null;
				}
			},

			resetView() {
				this.zoom = 1;
				this.panX = 0;
				this.panY = 0;
			},

			zoomBy(factor) {
				this.zoom = Math.max(0.4, Math.min(18.0, this.zoom * factor));
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

			// Semantic zoom projection: expands distance between pins while keeping icons constant size
			toScreen(wx, wy) {
				return {
					x: this.viewW / 2 + (wx - this.viewW / 2) * this.zoom + this.panX,
					y: this.viewH / 2 + (wy - this.viewH / 2) * this.zoom + this.panY,
				};
			},

			computeProjection() {
				const canvas = this.$refs.piCanvas;
				if (!canvas || !this.layout || !this.layout.pins || this.layout.pins.length === 0) {
					this.projected = [];
					this.pinById = {};
					return;
				}
				const wrap = this.$refs.piCanvasWrap || canvas.parentElement;
				const rect = (wrap || canvas).getBoundingClientRect();
				const w = Math.floor(rect.width);
				const h = Math.floor(rect.height);
				if (w < 40 || h < 40) return;
				this.viewW = w;
				this.viewH = h;
				const dpr = window.devicePixelRatio || 1;
				canvas.style.width = w + 'px';
				canvas.style.height = h + 'px';
				canvas.width = Math.round(w * dpr);
				canvas.height = Math.round(h * dpr);

				const pts = [];
				for (const p of this.layout.pins) {
					pts.push({ lat: p.latitude, lon: p.longitude });
					if (Array.isArray(p.heads)) {
						for (const hd of p.heads) pts.push({ lat: hd.latitude, lon: hd.longitude });
					}
				}

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
				for (const pt of pts) {
					const x = normDeltaLon(pt.lon) * cosLat;
					const y = -(pt.lat - centerLat);
					if (x < minX) minX = x;
					if (x > maxX) maxX = x;
					if (y < minY) minY = y;
					if (y > maxY) maxY = y;
				}

				const spanX = Math.max(0.005, maxX - minX);
				const spanY = Math.max(0.005, maxY - minY);
				const pad = 80;
				const scale = Math.min((w - pad * 2) / spanX, (h - pad * 2) / spanY);
				const midX = (minX + maxX) / 2;
				const midY = (minY + maxY) / 2;
				this.scaleRad = scale;

				const toWorld = (lat, lon) => ({
					x: w / 2 + (normDeltaLon(lon) * cosLat - midX) * scale,
					y: h / 2 + (-(lat - centerLat) - midY) * scale,
				});

				this.projected = [];
				this.pinById = {};
				for (const p of this.layout.pins) {
					const pos = toWorld(p.latitude, p.longitude);
					const heads = (p.heads || []).map(hd => ({
						...hd,
						...toWorld(hd.latitude, hd.longitude),
					}));
					const item = { ...p, x: pos.x, y: pos.y, pHeads: heads };
					this.projected.push(item);
					this.pinById[String(p.pin_id)] = item;
				}
			},

			onWheel(e) {
				e.preventDefault();
				const factor = e.deltaY < 0 ? 1.18 : 0.85;
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

				// Hit test directly in screen space (matches Semantic Zoom constant node radii!)
				let best = null;
				let bestDist = 26;
				for (const p of this.projected) {
					const sp = this.toScreen(p.x, p.y);
					const d = Math.hypot(sp.x - cx, sp.y - cy);
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
				const w = this.viewW;
				const h = this.viewH;

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

				// 1. Draw Extractor Head circles & tethers (Semantic Zoom positions)
				if (this.showHeads) {
					for (const p of this.projected) {
						if (!p.pHeads || !p.pHeads.length) continue;
						const sp = this.toScreen(p.x, p.y);
						const headR = Math.max(7, Math.min(36, (p.head_radius || 0.015) * this.scaleRad * Math.sqrt(this.zoom)));
						for (const hd of p.pHeads) {
							const sh = this.toScreen(hd.x, hd.y);
							ctx.strokeStyle = 'rgba(245, 158, 11, 0.35)';
							ctx.lineWidth = 1;
							ctx.setLineDash([3, 3]);
							ctx.beginPath();
							ctx.moveTo(sp.x, sp.y);
							ctx.lineTo(sh.x, sh.y);
							ctx.stroke();
							ctx.setLineDash([]);

							ctx.fillStyle = 'rgba(245, 158, 11, 0.14)';
							ctx.strokeStyle = 'rgba(251, 191, 36, 0.55)';
							ctx.beginPath();
							ctx.arc(sh.x, sh.y, headR, 0, Math.PI * 2);
							ctx.fill();
							ctx.stroke();

							ctx.fillStyle = '#fde68a';
							ctx.beginPath();
							ctx.arc(sh.x, sh.y, 2.8, 0, Math.PI * 2);
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
						const sa = this.toScreen(a.x, a.y);
						const sb = this.toScreen(b.x, b.y);
						ctx.strokeStyle = 'rgba(148, 163, 184, 0.40)';
						ctx.lineWidth = 1.6 + Math.min(2.5, (l.link_level || 0) * 0.35);
						ctx.beginPath();
						ctx.moveTo(sa.x, sa.y);
						ctx.lineTo(sb.x, sb.y);
						ctx.stroke();
					}
				}

				// 3. Draw animated Material Routes
				if (this.showRoutes && this.layout && this.layout.routes) {
					for (const r of this.layout.routes) {
						const a = this.pinById[String(r.source_pin_id)];
						const b = this.pinById[String(r.destination_pin_id)];
						if (!a || !b) continue;
						const sa = this.toScreen(a.x, a.y);
						const sb = this.toScreen(b.x, b.y);
						ctx.strokeStyle = 'rgba(56, 189, 248, 0.45)';
						ctx.lineWidth = 1.6;
						ctx.setLineDash([5, 5]);
						ctx.beginPath();
						ctx.moveTo(sa.x, sa.y);
						ctx.lineTo(sb.x, sb.y);
						ctx.stroke();
						ctx.setLineDash([]);

						const px = sa.x + (sb.x - sa.x) * this.phase;
						const py = sa.y + (sb.y - sa.y) * this.phase;
						ctx.fillStyle = '#38bdf8';
						ctx.beginPath();
						ctx.arc(px, py, 3, 0, Math.PI * 2);
						ctx.fill();
					}
				}

				// 4. Draw Crisp Constant-Size Pin Nodes (clean vector shapes + high-contrast text)
				for (const p of this.projected) {
					const st = ROLE_STYLE[p.role] || ROLE_STYLE.facility;
					const sp = this.toScreen(p.x, p.y);
					const isSelected = this.selectedPin && this.selectedPin.pin_id === p.pin_id;
					const r = st.r;

					if (isSelected) {
						ctx.strokeStyle = '#ffffff';
						ctx.lineWidth = 2.5;
						ctx.beginPath();
						ctx.arc(sp.x, sp.y, r + 5, 0, Math.PI * 2);
						ctx.stroke();
					}

					// Solid dark backdrop + role-tinted fill + bright border
					ctx.beginPath();
					if (p.role === 'extractor') {
						ctx.moveTo(sp.x, sp.y - r - 2);
						ctx.lineTo(sp.x + r + 2, sp.y);
						ctx.lineTo(sp.x, sp.y + r + 2);
						ctx.lineTo(sp.x - r - 2, sp.y);
						ctx.closePath();
					} else if (p.role === 'launchpad' || p.role === 'storage') {
						ctx.roundRect(sp.x - r, sp.y - r, r * 2, r * 2, 5);
					} else {
						ctx.arc(sp.x, sp.y, r, 0, Math.PI * 2);
					}
					ctx.fillStyle = '#0c0f22';
					ctx.fill();
					ctx.fillStyle = st.fill;
					ctx.fill();
					ctx.strokeStyle = st.color;
					ctx.lineWidth = 2;
					ctx.stroke();

					// Crisp centered short role code inside pin
					ctx.fillStyle = '#f8fafc';
					ctx.font = 'bold 9.5px ui-monospace, SFMono-Regular, monospace';
					ctx.textAlign = 'center';
					ctx.textBaseline = 'middle';
					ctx.fillText(st.short, sp.x, sp.y);

					// Crisp constant-size label below pin
					const caption = p.schematic_name || p.product_name || '';
					if (caption) {
						ctx.fillStyle = 'rgba(226, 232, 240, 0.92)';
						ctx.font = '600 10.5px system-ui, -apple-system, sans-serif';
						ctx.textAlign = 'center';
						ctx.textBaseline = 'top';
						ctx.fillText(caption.length > 20 ? caption.slice(0, 19) + '…' : caption, sp.x, sp.y + r + 5);
					}
				}

				// Zoom indicator badge in top-right of canvas
				ctx.fillStyle = 'rgba(15, 17, 38, 0.82)';
				ctx.strokeStyle = 'rgba(139, 92, 246, 0.35)';
				ctx.lineWidth = 1;
				ctx.beginPath();
				ctx.roundRect(w - 118, 12, 106, 24, 6);
				ctx.fill();
				ctx.stroke();
				ctx.fillStyle = '#cbd5e1';
				ctx.font = 'bold 10.5px ui-monospace, SFMono-Regular, monospace';
				ctx.textAlign = 'center';
				ctx.textBaseline = 'middle';
				ctx.fillText('ZOOM ' + this.zoom.toFixed(1) + '×', w - 65, 24);

				ctx.restore();
			}
		};
	};
})();
