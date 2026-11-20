# Roadmap Pengembangan: EVE Market & Industry Copilot (2026)

**Codename Proyek:** `aura-forge`  
**Tujuan:** Membangun aplikasi web asisten pribadi (*copilot*) berbasis **Clean Architecture** untuk manajemen lengkap **produksi industri, perencanaan slot & jadwal mingguan, rantai pasok bahan baku, dan perdagangan marketplace (Jita)** di EVE Online. Aplikasi berjalan **100% langsung dengan Public ESI + Manajemen Manual/Import**, sementara **Authenticated EVE SSO** disediakan sebagai **fitur opsional *plug-and-play*** yang otomatis aktif cukup dengan mengisi `ESI_CLIENT_ID` & `ESI_CLIENT_SECRET` di file `.env`.

---

## 1. Tech Stack & Prinsip Clean Architecture

* **Backend:** Go (`golang`) + `go-chi/chi/v5`
* **Templating UI:** `a-h/templ` (komponen HTML *type-safe* terkompilasi ke Go)
* **Interaktivitas:** `htmx` (partial HTML update tanpa *full page reload*) + `Alpine.js` (state lokal UI: modal, tab, *inline search*, dialog konfirmasi, clipboard copy, dan notifikasi *toast*)
* **Styling:** `Tailwind CSS` (tema *dark mode* industrial-tactical dengan kontras tinggi dan hierarki visual jelas)
* **Database:** **PostgreSQL 16+** menggunakan driver `github.com/jackc/pgx/v5` (`pgxpool`), disertai `docker-compose.yml` dan *SQL migrations* otomatis saat startup.
* **Integrasi EVE Online ESI (`https://esi.evetech.net/latest/`):**
  * **Default Mode (Zero-Config / Public ESI):** Langsung berjalan tanpa pendaftaran developer CCP (mengambil harga Jita *live*, rata-rata volume penjualan harian, *System Cost Index*, dan resolusi `type_id` otomatis).
  * **Optional Authenticated Mode (EVE SSO OAuth2):** Aktif otomatis apabila `ESI_CLIENT_ID` dan `ESI_CLIENT_SECRET` diisi di `.env` (menambahkan sinkronisasi saldo dompet, order pasar aktif + deteksi *undercut*, antrean *industry jobs*, *blueprints*, dan pemotongan stok hangar pada *shopping list*).
* **Aturan Clean Architecture & Kualitas Kode (Wajib Diterapkan AI):**
  * **Dependency Inversion dengan Go Interfaces:** Definisikan kontrak `interface` eksplisit (`ItemRepository`, `BatchRepository`, `PlanRepository`, `SettingsRepository`, `TokenRepository`, `ESICacheRepository`, `MarketDataProvider`, dan `CharacterSyncProvider`) di `internal/domain/` sehingga `service` bergantung pada abstraksi *interface*, bukan langsung pada `pgx` atau HTTP client ESI.
  * **Pemisahan Lapisan Tegas:**
    * `internal/domain` (Entities, Value Objects, Interfaces & Pure Business Rules — bebas dari `pgx`, `net/http`, dan `templ`).
    * `internal/service` (Application Use Cases — mendukung operasi penuh di *Standard Mode* maupun *Authenticated ESI Mode*).
    * `internal/store/postgres` (Adapter PostgreSQL).
    * `internal/esi` (Adapter Public Market ESI & Optional EVE SSO Character Sync).
    * `internal/http` & `internal/web` (Delivery Layer: Chi handlers, conditional auth routes & Templ UI).
  * Gunakan *strongly typed structs*, konstanta (`const`) eksplisit, dan *guard clauses* (hindari *magic strings* atau `map[string]any`).
  * Seluruh perhitungan ISK menggunakan tipe presisi (`NUMERIC(20,2)` di PostgreSQL dan pembulatan/pengecekan *edge case* di Go).

---

## 2. Spesifikasi Fitur Lengkap (Full Assistant)

### A. Public ESI Market Intelligence (Aktif Otomatis Tanpa Setup)
1. **Auto-Resolve Item & Type ID (`POST /universe/ids/`):** Mengubah nama item dari teks *copy-paste* menjadi `type_id` resmi EVE Online.
2. **Live Jita Price Sync (`GET /markets/10000002/orders/`):** Mengambil harga *Sell Order* terendah dan *Buy Order* tertinggi di Jita 4-4.
3. **Daily Sales Volume & Liquidity Check (`GET /markets/10000002/history/`):** Menghitung rata-rata unit terjual per hari (7 & 30 hari terakhir) untuk menyaring item yang sepi pembeli.
4. **System Cost Index / SCI (`GET /industry/systems/`):** Mengambil tarif pajak manufaktur/invention terbaru di sistem pabrik yang dipilih pengguna.
5. **Caching & Rate-Limit Protection:** Menyimpan cache respons ESI ke tabel `esi_cache` PostgreSQL menggunakan header `Expires`/`ETag` dan memantau `X-ESI-Error-Limit-Remain`.

### B. Optional Authenticated EVE SSO (Aktif Otomatis Jika `.env` Diisi)
Aplikasi mendeteksi status `config.IsSSOEnabled()` (`ESI_CLIENT_ID != "" && ESI_CLIENT_SECRET != ""`):
* **Jika `.env` kosong (Default):** Aplikasi berjalan mulus tanpa error; tombol login EVE SSO menampilkan badge *"Optional: Isi ESI_CLIENT_ID & SECRET di .env untuk Auto-Sync Karakter"*, sementara saldo modal, stok bahan, dan status *batch* dikelola penuh melalui UI.
* **Jika `.env` diisi:**
  1. **Login EVE SSO & Auto-Refresh Token:** OAuth2 login, simpan token di `eve_tokens`, dan perbarui *access token* otomatis.
  2. **Wallet & Transaction Sync:** Membaca saldo dompet karakter dan transaksi penjualan aktual.
  3. **Market Orders & Undercut Radar:** Mendeteksi *Sell Order* aktif yang tersalip harga pesaing di Jita dan menghitung batas aman *relist*.
  4. **Industry Jobs & Slot Monitor:** Membaca antrean manufaktur/invention secara langsung dan menyinkronkan progres ke *Tracker*.
  5. **Blueprints & Hangar Assets Deduction:** Memotong kebutuhan belanja di *Weekly Planner* berdasarkan stok bahan yang sudah ada di hangar.

### C. Trap Guard & Risk Advisor (Detektor Jebakan Pemula)
Setiap kandidat item dievaluasi otomatis sebelum masuk ke rencana produksi:
1. **Deteksi BPC Langka (Faction / Deadspace / Storyline / Officer / Subverted / Abyssal):**
   * Memberi peringatan keras **JEBAKAN BPC LANGKA** jika item membutuhkan BPC khusus sementara biaya BPC tambahan (`ExtraSetupCost`) masih `0`.
2. **Deteksi Non-Manufaktur:**
   * Menandai item seperti *Compressed Ice* atau komoditas mentah yang bukan hasil manufaktur *blueprint*.
3. **Deteksi Likuiditas Rendah (Berbasis Volume Public ESI):**
   * Memberi peringatan *Slow Mover* jika rata-rata volume penjualan harian di Jita terlalu kecil dibanding target produksi.
4. **Deteksi Profit Receh & Lock-Up Modal:**
   * Memperingatkan jika profit per *batch* terlalu kecil dibanding pemakaian *slot* dan *broker fee*, atau modal bahan melebihi kas tersedia.
5. **Penjelasan Human-Readable (*Copilot Note*):**
   * Memberikan status (`LAYAK_PRODUKSI`, `WASPADA_MODAL`, `WASPADA_NICHE`, `JEBAKAN`), level risiko (`LOW`, `MEDIUM`, `HIGH`, `CRITICAL`), alasan spesifik, dan rekomendasi langkah berikutnya.

### D. Net Profit, Tax & Break-Even Calculator
* **Input:** Nama Item (auto-fill harga Jita & volume harian dari Public ESI), Output Qty per Batch, Harga Jual, Biaya Bahan, Biaya Ekstra BPC/Decryptor, *Job Installation Cost* (SCI), Biaya Angkut, *Broker Fee (%)*, *Sales Tax (%)*, Estimasi *Relist*, dan Target *Batches/Week*.
* **Output Real-Time (`htmx`):** *Gross Revenue*, *Total Biaya Aktual*, *Net Profit per Batch*, *Net Margin (%)*, *ROI (%)*, *Break-Even Unit Price*, dan *Proyeksi Profit Mingguan*.

### E. Smart Capital Allocator (Pembagi Modal Pemula)
* Membagi kas tersedia (baik dari input pengaturan modal maupun saldo dompet jika SSO aktif) ke **4 keranjang**:
  1. **Arus Kas Harian (*Fast Cashflow*)**
  2. **Profit Menengah (*Mid-Tier*)**
  3. **Pendongkrak Profit (*High ROI / High Profit*)**
  4. **Cadangan Kas Operasional (*Cash Buffer* minimal `10%–15%`)**
* Mendukung konversi 1-klik ke **Weekly Production & Slot Planner** atau langsung menjadi *Draft Batches*.

### F. Weekly Production & Slot Planner (Manajemen Jadwal & Rantai Pasok)
1. **Slot & Job Time Capacity Planner:**
   * Mengatur jumlah *Manufacturing Slots* & *Science/Invention Slots* yang tersedia (dari pengaturan karakter atau *live sync* jika SSO aktif) dan memvalidasi apakah rencana *batch* mingguan muat di kapasitas slot.
2. **Jadwal Eksekusi Harian (Day 1 – Day 7 Blueprint):**
   * Menyusun jadwal belanja *Buy Order* (Hari 1), produksi & *listing* batch awal (Hari 2–3), *re-batching* dari hasil penjualan cepat (Hari 4–6), dan panen/audit mingguan (Hari 7).
3. **Consolidated Shopping List & EVE Multibuy Exporter:**
   * Menggabungkan seluruh kebutuhan bahan baku dari rencana mingguan (otomatis dikurangi stok hangar jika SSO aktif, atau stok manual yang dicatat pengguna), menghitung total estimasi biaya & volume kargo (`m³`), dan menyediakan tombol **1-Klik Copy Format EVE Multibuy**.
4. **Multi-Week Growth Projection (Minggu 1 -> Minggu 4):**
   * Simulasi pertumbuhan modal bertahap dan rekomendasi kapan kas cukup untuk naik kelas ke item berprofit lebih besar.

### G. Production, Inventory & Market Order Tracker
* **Status Siklus:** `DRAFT` -> `IN_PRODUCTION` -> `LISTED` -> `SOLD` (atau `SLOW_MOVER`).
* **Relist & Undercut Guard:** Menghitung sisa margin aman sebelum pengguna menurunkan harga *Sell Order* agar tidak merugi karena *broker fee* dan *sales tax*.
* **Bulk Paste / CSV Importer:** Menempelkan tabel data eksternal (seperti hasil EVE-HUB) lalu meng-enrich data dengan harga Jita dan volume harian dari Public ESI secara otomatis.

---

## 3. Struktur Direktori Proyek (`aura-forge`)

```text
aura-forge/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── domain/
│   │   ├── item.go
│   │   ├── character.go
│   │   ├── repository.go
│   │   ├── calculator.go
│   │   ├── trap_filter.go
│   │   ├── allocator.go
│   │   ├── planner.go
│   │   ├── tracker.go
│   │   └── importer.go
│   ├── service/
│   │   └── copilot_service.go
│   ├── store/
│   │   └── postgres/
│   │       ├── db.go
│   │       ├── item_repo.go
│   │       ├── batch_repo.go
│   │       ├── plan_repo.go
│   │       ├── character_repo.go
│   │       ├── settings_repo.go
│   │       └── migrations/
│   │           └── 001_init.sql
│   ├── esi/
│   │   ├── client.go
│   │   ├── public_market.go
│   │   ├── sso_oauth.go
│   │   └── character_sync.go
│   ├── http/
│   │   ├── router.go
│   │   ├── handlers.go
│   │   ├── auth_handlers.go
│   │   └── middleware.go
│   └── web/
│       ├── layout.templ
│       ├── dashboard.templ
│       ├── catalog.templ
│       ├── calculator.templ
│       ├── allocator.templ
│       ├── planner.templ
│       ├── tracker.templ
│       └── settings.templ
├── web/
│   └── static/
│       ├── app.css
│       └── tailwind.input.css
├── docker-compose.yml
├── .env.example
├── go.mod
├── Makefile
└── README.md
```

---

## 4. Rencana Eksekusi Bertahap (Untuk AI Pengembang)

Kerjakan proyek ini **secara berurutan dari Fase 1 hingga Fase 6**. Pastikan `go build ./...` dan `go test ./...` lulus pada setiap fase sebelum lanjut ke fase berikutnya.

### Fase 1: Fondasi Proyek, PostgreSQL Setup & Domain Contracts
* **Target:** Setup struktur `aura-forge` (Go + Chi + Templ + HTMX + Alpine + Tailwind + PostgreSQL `pgx/v5`).
* **Langkah:**
  1. Buat `go.mod` (`module aura-forge`), `docker-compose.yml` (PostgreSQL 16 dengan *healthcheck* & volume persisten), `.env.example` (`DATABASE_URL`, `PORT`, `SESSION_SECRET`, `ESI_USER_AGENT`, serta variabel opsional `ESI_CLIENT_ID`, `ESI_CLIENT_SECRET`, `ESI_CALLBACK_URL`), `Makefile`, dan `internal/config/config.go` (termasuk helper `IsSSOEnabled() bool`).
  2. Definisikan konstanta (`TrapStatus`, `RiskLevel`, `BatchStatus`, `PlanStatus`, `ItemCategory`) dan *structs* (`MarketItem`, `CharacterProfile`, `CharacterOrder`, `CharacterJob`, `CharacterBlueprint`, `HangarAsset`, `WeeklyPlan`, `ShoppingListItem`, `BatchRecord`, `UserSettings`) di `internal/domain/`, serta Go *interfaces* di `internal/domain/repository.go`.
  3. Buat migrasi PostgreSQL `internal/store/postgres/migrations/001_init.sql` (tabel `user_settings`, `eve_tokens`, `character_orders`, `character_jobs`, `character_assets`, `character_blueprints`, `market_items`, `weekly_plans`, `production_batches`, dan `esi_cache` dengan `NUMERIC(20,2)` dan `JSONB`) serta implementasi repository PostgreSQL.
* **Verifikasi Fase 1:** `docker compose up -d` sehat, `go build ./...` sukses, dan migrasi tabel berjalan otomatis saat startup.

### Fase 2: Logika Domain Murni & Unit Testing
* **Target:** Mengimplementasikan seluruh aturan bisnis murni di `internal/domain/` tanpa ketergantungan HTTP/DB.
* **Langkah:**
  1. Implementasikan `calculator.go` (margin bersih, pajak, *broker fee*, *relist fee*, *break-even price*, ROI, proyeksi mingguan) dengan *guard clauses*.
  2. Implementasikan `trap_filter.go` untuk mendeteksi BPC langka, item non-manufaktur, *slow mover* berbasis volume ESI, profit receh, dan *lock-up* modal.
  3. Implementasikan `allocator.go` (pembagi modal 4 keranjang menggunakan kas pengaturan atau saldo dompet karakter jika SSO aktif).
  4. Implementasikan `planner.go` (validasi kapasitas slot manufaktur/invention, jadwal Day 1–7, **Consolidated Shopping List** yang memotong stok manual/hangar, formatter **EVE Multibuy**, dan simulasi pertumbuhan Minggu 1–4).
  5. Implementasikan `importer.go` untuk mem-parse teks *copy-paste* tabel (`K/M/B` & `%`) dengan laporan error per baris.
  6. Buat *unit test* komprehensif di `internal/domain/domain_test.go`.
* **Verifikasi Fase 2:** `go test -v ./internal/domain/...` lulus 100%.

### Fase 3: Integrasi Public Market ESI & Optional EVE SSO Adapter
* **Target:** Membangun klien ESI di `internal/esi/` di mana **Public ESI berjalan tanpa kredensial**, sedangkan **EVE SSO Character Sync aktif otomatis jika `.env` diisi**.
* **Langkah:**
  1. Buat `internal/esi/client.go` dengan `User-Agent` terkonfigurasi, caching respons ke PostgreSQL (`esi_cache`), dan monitor header `X-ESI-Error-Limit-Remain`.
  2. Buat `internal/esi/public_market.go` (tanpa auth) untuk `ResolveTypeIDs`, harga Jita *live*, rata-rata volume harian, dan *System Cost Index*.
  3. Buat `internal/esi/sso_oauth.go` dan `internal/esi/character_sync.go` sebagai adapter opsional (dengan *No-Op / Disabled fallback* yang aman saat `IsSSOEnabled() == false`) untuk login EVE SSO, *auto-refresh token*, dan sinkronisasi dompet, order, *jobs*, *blueprints*, serta aset hangar.
  4. Buat pengujian dengan `httptest.NewServer` di `internal/esi/esi_test.go` (menguji Public ESI maupun skenario SSO aktif/nonaktif).
* **Verifikasi Fase 3:** `go test -v ./internal/esi/...` lulus 100%.

### Fase 4: Service Layer (Use Cases) & HTTP/HTMX Handlers
* **Target:** Orkestrasi seluruh fitur melalui `internal/service/copilot_service.go` dan ekspos route di `internal/http/`.
* **Langkah:**
  1. Buat `copilot_service.go` yang bekerja mulus baik dalam **Standard Mode (Public ESI + Manual)** maupun **SSO Connected Mode (Auto-Sync Karakter)**.
  2. Buat `router.go`, `auth_handlers.go` (mengembalikan info jelas jika SSO belum dikonfigurasi di `.env`), `handlers.go`, dan `middleware.go` menggunakan `go-chi/chi/v5` dengan dukungan *HTMX partial rendering* (`HX-Request`).
* **Verifikasi Fase 4:** `go build ./...` sukses dan seluruh endpoint terhubung rapi.

### Fase 5: Antarmuka Templ, Tailwind CSS, HTMX & Alpine.js
* **Target:** Membangun antarmuka *dark-industrial* yang intuitif dan responsif.
* **Langkah:**
  1. Buat `layout.templ` dengan navigasi (`Dashboard`, `Catalog`, `Calculator`, `Allocator`, `Weekly Planner`, `Tracker`, `Settings`), indikator mode ESI (*Public ESI Active* atau *EVE SSO Connected* jika `.env` diisi), tombol **"Sync Jita Prices"**, serta *toast notification* (`Alpine.js`).
  2. Bangun `dashboard.templ` (KPI *Kas Tersedia*, *Modal Terkunci*, *Realized Profit*, *Kapasitas Slot*, kalkulator cepat `htmx`, dan *Checklist Asisten Harian*).
  3. Bangun `catalog.templ` (tabel item dengan *Live Jita Price*, *Avg Daily Volume*, filter status/kategori, badge *Usable* & *Sering Hancur*, penjelasan jebakan, serta *Bulk Paste Importer*).
  4. Bangun `allocator.templ` (pembagi modal otomatis -> kirim ke *Weekly Planner* atau *Draft Batches*).
  5. Bangun `planner.templ` (kapasitas slot manufaktur/invention, jadwal Day 1–7, **Consolidated Shopping List** + tombol copy **EVE Multibuy**, simulasi pertumbuhan Minggu 1–4, dan tombol eksekusi ke *Tracker*).
  6. Bangun `tracker.templ` & `settings.templ` (manajemen siklus *batch*, simulasi *relist price*, pengaturan pajak, jumlah slot karakter, serta panduan status variabel `.env` EVE SSO).
* **Verifikasi Fase 5:** `templ generate`, build CSS Tailwind, dan uji seluruh alur UI di browser.

### Fase 6: Seed Data Pemula, Dokumentasi & Verifikasi Akhir
* **Target:** Menyediakan data contoh, dokumentasi lengkap, dan verifikasi menyeluruh.
* **Langkah:**
  1. Sediakan tombol *"Load Sample Data"* untuk mengisi contoh item produksi & item jebakan agar langsung bisa dicoba bersama *Public ESI*.
  2. Tulis `README.md` lengkap berisi:
     * Cara menjalankan aplikasi langsung dalam **Standard Mode (Public ESI)** hanya dengan `docker compose up -d` dan `go run ./cmd/server`.
     * Panduan opsional mengaktifkan **EVE SSO** di kemudian hari (cukup mendaftar di `https://developers.eveonline.com/` dan mengisi `ESI_CLIENT_ID` & `ESI_CLIENT_SECRET` di `.env`).
     * Cara *backup* database PostgreSQL.
  3. Jalankan pengujian akhir `go test ./...` dan `go build ./...`.

---

## 5. Prompt Delegasi Siap Pakai untuk AI Pengembang

```text
Bangun aplikasi web "aura-forge // EVE Market & Industry Copilot (2026)" (codename: aura-forge) menggunakan Go + go-chi/chi/v5 + a-h/templ + HTMX + Alpine.js + Tailwind CSS + PostgreSQL (jackc/pgx/v5) + EVE Online ESI API mengikuti seluruh spesifikasi Clean Architecture pada roadmap ini.

Aturan eksekusi:
1. Kerjakan secara berurutan dari Fase 1 sampai Fase 6.
2. Terapkan Clean Architecture dengan Go interfaces (Dependency Inversion): pisahkan domain logic murni (`internal/domain`) dan service use-cases (`internal/service`) dari adapter PostgreSQL (`internal/store/postgres`), ESI client (`internal/esi`), HTTP handlers (`internal/http`), dan templ views (`internal/web`).
3. Gunakan strongly typed structs, konstanta eksplisit (hindari magic strings), guard clauses, dan tipe `NUMERIC(20,2)` di PostgreSQL untuk data ISK.
4. Jadikan Public ESI (harga Jita live, volume harian, System Cost Index, resolve type_id) + Manajemen Manual/Import sebagai mode default yang langsung jalan tanpa kredensial, dan jadikan Authenticated EVE SSO OAuth2 sebagai fitur opsional plug-and-play yang otomatis aktif ketika `ESI_CLIENT_ID` dan `ESI_CLIENT_SECRET` diisi di `.env`.
5. Pastikan modul Weekly Production & Slot Planner (jadwal Day 1–7, validasi kapasitas slot, Consolidated Shopping List + Copy Format EVE Multibuy, simulasi pertumbuhan Minggu 1–4, serta konversi ke Tracker) terimplementasi lengkap.
6. Tulis unit test untuk domain logic dan ESI client (menggunakan httptest) serta pastikan `go test ./...` dan `go build ./...` lulus sebelum pindah ke fase berikutnya.
7. Setelah setiap fase selesai, tampilkan ringkasan file yang dibuat/diubah beserta perintah verifikasinya.

Mulai sekarang dengan Fase 1.
```