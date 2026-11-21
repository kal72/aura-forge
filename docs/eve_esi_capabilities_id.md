# Referensi Kemampuan EVE Online ESI API (Public vs. Private)

Dokumen ini merangkum data yang dapat diambil dari **Public ESI** (tanpa login) maupun **Private ESI** (membutuhkan EVE SSO OAuth2) yang relevan untuk membangun dan menskalakan bisnis industri serta trading di EVE Online menuju **100B+ ISK**, beserta status implementasinya di `aura-forge`.

---

## 1. Public ESI API *(Tanpa Login EVE SSO — Gratis & Langsung Jalan)*

| Kategori | Data yang Bisa Diambil | Status di `aura-forge` | Manfaat untuk Bisnis 100B+ ISK |
| :--- | :--- | :---: | :--- |
| **Market Orders** | Daftar Sell & Buy Order live per region (`/markets/{region_id}/orders/`) | **Sudah dipakai** | Order Book 5 Trade Hub, Station Trading, Arbitrase, Undercut Guard |
| **Market History** | Riwayat harga harian (avg/high/low) & volume 1 tahun (`/markets/{region_id}/history/`) | **Sudah dipakai** | Deteksi *slow mover*, grafik tren 30 hari, hindari *price spike* palsu |
| **Global Prices (EIV)** | Harga rata-rata & *adjusted price* seluruh item (`/markets/prices/`) | *Bisa ditambah* | Menghitung **Estimated Item Value (EIV)** resmi CCP untuk *Job Install Cost* yang 100% presisi |
| **Market Groups** | Pohon kategori market seperti di dalam game (`/markets/groups/`) | *Bisa ditambah* | Browsing katalog ribuan item EVE tanpa harus hafal nama |
| **Industry SCI** | *System Cost Index* seluruh solar system (`/industry/systems/`) | **Sudah dipakai** | Sync SCI live (Jita, Perimeter, Amarr, dll.) agar biaya produksi akurat |
| **Universe IDs & Info** | Nama ↔ TypeID, volume `m³`, deskripsi (`/universe/ids/`, `/universe/types/{id}/`) | **Sebagian dipakai** | Auto-fill item baru & kalkulasi ongkos angkut (`m³`) |
| **Route & Gank Radar** | Jumlah *jump* aman High-Sec (`/route/`) + statistik kill kapal (`/universe/system_kills/`) | *Bisa ditambah* | Cek rute teraman & peringatan sistem rawan *gank* (mis. Uedama) sebelum hauling |
| **Public Contracts** | Daftar kontrak publik per region (`/contracts/public/{region_id}/`) | *Bisa ditambah* | Berburu **BPC murah**, paket kapal, dan arbitrase kontrak |

---

## 2. Private ESI API *(Butuh Login EVE SSO OAuth2 — Data Spesifik Karakter/Corp)*

| Kategori | Data yang Bisa Diambil (Scope) | Status di `aura-forge` | Manfaat untuk Bisnis 100B+ ISK |
| :--- | :--- | :---: | :--- |
| **Wallet Balance** | Saldo ISK dompet karakter (`esi-wallet.read_character_wallet.v1`) | **Sudah ada** | Auto-update kas tersedia & Net Worth harian |
| **Wallet Transactions & Journal** | Riwayat asli setiap pembelian bahan, penjualan barang, pajak & broker fee | *Bisa ditambah* | **Auto-Accounting 100%:** hitung profit bersih nyata tanpa catat manual |
| **Character Market Orders** | Order Buy/Sell aktif milik karakter + nilai Escrow (`esi-markets.read_character_orders.v1`) | **Sudah ada** | Auto-Undercut Radar langsung terhadap order asli di Jita |
| **Industry Jobs** | Job manufaktur/riset/invensi aktif & kapan selesai (`esi-industry.read_character_jobs.v1`) | **Sudah ada** | Pantau slot kosong secara real-time agar mesin produksi tidak pernah *idle* |
| **Character Blueprints** | Daftar BPO/BPC milik karakter lengkap dengan ME/TE/Runs (`esi-characters.read_blueprints.v1`) | **Sudah ada** | Kalkulator otomatis mengetahui ME/TE BPO asli milik karakter |
| **Hangar Assets** | Seluruh isi hangar/inventory karakter (`esi-assets.read_assets.v1`) | **Sudah ada** | Potong otomatis daftar belanja Multibuy & hitung nilai aset inventory |
| **Character Skills & Standings** | Level *Accounting, Broker Relations, Mass Production, Science* + Standing NPC | *Bisa ditambah* | Auto-hitung Broker Fee, Sales Tax, jumlah slot, dan peluang T2 Invention secara presisi |
| **Structure Markets** | Order Book di Citadel/Upwell milik player (`esi-markets.structure_markets.v1`) | *Bisa ditambah* | Akses harga di pasar Citadel (Perimeter / Null-Sec) yang tidak ada di Public ESI |
| **Planetary Industry (PI)** | Status ekstraktor & pabrik planet (`esi-planets.manage_planets.v1`) | *Bisa ditambah* | Pantau suplai bahan baku PI untuk produksi T2/T3 |
| **Corporation ESI** | Wallet, Jobs, Blueprints, dan Assets milik *Alt Corp* industri | *Bisa ditambah* | Penting saat skala 10B → 100B+ ISK dengan banyak karakter (*alts*) |

---

## 3. Catatan Penting (Aturan CCP Games)
- ESI **tidak menyediakan endpoint untuk membuat/mengubah order market atau memulai job manufaktur** secara otomatis (untuk mencegah bot).
- Peran `aura-forge` adalah sebagai **otak analisis, proteksi margin, dan pemantauan otomatis** (mencari peluang, menghitung margin/pajak/BOM, mendeteksi undercut, dan menyiapkan daftar Multibuy), sedangkan eksekusi klik beli/jual/start job tetap dilakukan di dalam game EVE Online.
- Untuk mengaktifkan Private ESI secara opsional di `aura-forge`, cukup daftarkan aplikasi gratis di `https://developers.eveonline.com/` dengan callback `http://localhost:8080/auth/callback`, lalu isi `ESI_CLIENT_ID` dan `ESI_CLIENT_SECRET` di file `.env`.
