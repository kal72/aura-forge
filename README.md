# aura-forge // EVE Market & Industry Copilot (2026)

Asisten web pribadi (*copilot*) berbasis **Clean Architecture** untuk manajemen lengkap **produksi industri, perencanaan slot & jadwal mingguan, rantai pasok bahan baku, dan perdagangan marketplace (Jita)** di EVE Online.

Dibangun menggunakan **Go (`go-chi/chi/v5`) + `a-h/templ` + HTMX + Alpine.js + Tailwind CSS + PostgreSQL 16 (`jackc/pgx/v5`) + EVE Online ESI API**.

---

## Fitur Utama

1. **Public ESI Market Intelligence (Default Zero-Config):**
   * Auto-resolve `type_id` dari nama item (`POST /universe/ids/`).
   * Sinkronisasi harga Jita 4-4 *live* (`GET /markets/10000002/orders/`).
   * Volume penjualan harian 7 & 30 hari (`GET /markets/10000002/history/`).
   * *System Cost Index* (SCI) terbaru (`GET /industry/systems/`).
   * Caching respons ESI ke PostgreSQL (`esi_cache` berbasis `Expires`/`ETag`) & proteksi `X-ESI-Error-Limit-Remain`.
2. **Trap Guard & Risk Advisor:**
   * Deteksi otomatis **JEBAKAN BPC LANGKA** (Faction / Deadspace / Storyline / Officer / Subverted / Abyssal saat `ExtraSetupCost == 0`), item non-manufaktur (*Compressed Ice*), *Slow Mover*, profit receh, dan *lock-up* modal.
3. **Net Profit, Tax & Break-Even Calculator (HTMX Real-Time):**
   * Menghitung *Gross Revenue*, *Broker Fee*, *Sales Tax*, *Relist Fee*, *Job Install Cost* (SCI), *Break-Even Unit Price*, ROI, dan proyeksi profit mingguan, lengkap dengan tombol **Cek Jita ESI**.
4. **Smart Capital Allocator (4 Keranjang Modal):**
   * Membagi kas tersedia (manual atau saldo dompet SSO) ke 4 keranjang: *Fast Cashflow*, *Mid-Tier*, *High ROI*, dan *Cash Buffer* (minimal `10%–15%`) dengan konversi 1-klik ke **Weekly Planner** atau **Draft Batches**.
5. **Weekly Production & Slot Planner:**
   * Validasi kapasitas *Manufacturing* & *Science/Invention Slots* beserta total jam produksi.
   * Jadwal eksekusi **Day 1 – Day 7 Blueprint**.
   * **Consolidated Shopping List** (otomatis memotong stok hangar saat SSO aktif) + tombol **1-Klik Copy Format EVE Multibuy**.
   * Simulasi pertumbuhan modal **Minggu 1 → Minggu 4** dan eksekusi 1-klik ke *Tracker*.
6. **Production, Inventory & Market Order Tracker + Bulk Importer:**
   * Siklus batch `DRAFT → IN_PRODUCTION → LISTED → SOLD` (atau `SLOW_MOVER`), simulator **Relist & Undercut Guard**, serta **Bulk Paste / CSV Importer** (mendukung `K/M/B` & `%` dengan laporan error per baris).

---

## Cara Menjalankan (Standard Mode — Public ESI Tanpa Setup Kredensial)

Aplikasi langsung berjalan 100% tanpa perlu mendaftar developer CCP:

### Opsi A: Full Docker Compose (PostgreSQL + aura-forge)

```bash
# 1. Salin konfigurasi environment (biarkan ESI_CLIENT_ID & ESI_CLIENT_SECRET kosong)
cp .env.example .env

# 2. Rebuild image & jalankan seluruh stack di background
make docker-run
# (atau foreground dengan log langsung: make docker-run)

# Lihat log aplikasi
make docker-logs
```

### Opsi B: Pengembangan Lokal (Go Lokal + PostgreSQL Docker)

```bash
# 1. Jalankan PostgreSQL 16 saja
make db-up

# 2. Jalankan server aura-forge lokal
make run
```

Buka browser di **`http://localhost:8080`**, lalu klik **"Load Sample Data"** di Dashboard atau Catalog untuk langsung mencoba contoh item produksi, detektor jebakan BPC/Slow Mover, dan sinkronisasi harga Jita live via **Sync Jita Prices**.

---

## Panduan Opsional: Mengaktifkan EVE SSO OAuth2 (Plug-and-Play)

Jika di kemudian hari Anda ingin menyinkronkan saldo dompet, order pasar aktif, *industry jobs*, *blueprints*, dan pemotongan stok hangar secara otomatis:

1. Daftar aplikasi baru di portal resmi developer CCP: `https://developers.eveonline.com/`.
2. Pilih tipe **Authentication & API Access** dan set *Callback URL* ke:
   ```text
   http://localhost:8080/auth/callback
   ```
3. Tambahkan *scopes* berikut:
   * `esi-wallet.read_character_wallet.v1`
   * `esi-markets.read_character_orders.v1`
   * `esi-industry.read_character_jobs.v1`
   * `esi-characters.read_blueprints.v1`
   * `esi-assets.read_assets.v1`
4. Salin `Client ID` dan `Secret Key` ke file `.env`:
   ```env
   ESI_CLIENT_ID=your_client_id_here
   ESI_CLIENT_SECRET=your_client_secret_here
   ESI_CALLBACK_URL=http://localhost:8080/auth/callback
   ```
5. Restart server (`go run ./cmd/server`). Buka halaman **Settings**, lalu klik **"Login dengan EVE SSO"**. Indikator di navbar akan otomatis berubah menjadi **EVE SSO Connected**.

---

## SQL Migrations & Aturan Pembuatan

Migrasi skema PostgreSQL dicatat di tabel `schema_migrations` dan mendukung eksekusi transaksional (`up`/`down`/`status`) serta generator otomatis untuk file migrasi baru.

```bash
# Jalankan seluruh migrasi yang pending (juga berjalan otomatis saat server start)
make migrate-up

# Cek status migrasi (APPLIED / PENDING)
make migrate-status

# Rollback 1 migrasi terakhir
make migrate-down

# Buat pasangan file migrasi baru: NNN_<name>.up.sql & NNN_<name>.down.sql
make migrate-new NAME=add_manual_hangar_stock
```

Panduan & aturan lengkap (penamaan `NNN_<name>.up.sql/.down.sql`, immutability, aturan `NUMERIC(20,2)` untuk ISK, `TIMESTAMPTZ`, dan idempotency) tersedia di [`migrations/README.md`](migrations/README.md).

---

## Backup & Restore Database PostgreSQL

```bash
# Backup seluruh database ke file SQL terkompresi
docker exec -t aura-forge-postgres pg_dump -U postgres -d aura_forge | gzip > backup_aura_forge_$(date +%Y%m%d_%H%M%S).sql.gz

# Restore dari file backup
gunzip -c backup_aura_forge_YYYYMMDD_HHMMSS.sql.gz | docker exec -i aura-forge-postgres psql -U postgres -d aura_forge
```

---

## Arsitektur & Pengembangan

```text
aura-forge/
├── cmd/
│   ├── server/            # Entrypoint & Dependency Injection wiring
│   └── migrate/           # CLI SQL Migration (up, down, status, create)
├── migrations/            # Versioned SQL Migrations (*.up.sql / *.down.sql) & embed.go
├── internal/
│   ├── config/            # Konfigurasi .env & deteksi IsSSOEnabled()
│   ├── domain/            # Entities, Pure Business Rules & Repository Interfaces
│   ├── service/           # Application Use Cases (Standard & SSO Mode)
│   ├── store/postgres/    # PostgreSQL 16 (pgx/v5) Adapter & Migration Runner
│   ├── esi/               # Public Market ESI & Optional EVE SSO Adapter
│   ├── http/              # Go-Chi Router, HTMX Handlers & Conditional Auth
│   └── web/               # Type-Safe a-h/templ UI Components
├── web/static/            # Tailwind CSS & Industrial Dark-Mode Styles
├── docker-compose.yml     # PostgreSQL 16 + healthcheck + persistent volume
└── Makefile               # build, run, test, templ, db-up, db-down
```

### Perintah Verifikasi & Pengujian

```bash
# Kompilasi template Templ
make templ

# Jalankan seluruh unit test (Config, Domain, ESI httptest, Service, HTTP/Templ)
go test -v ./...

# Build binary produksi
go build ./...
```
