# Aturan Pembuatan SQL Migrations (`aura-forge`)

Seluruh perubahan skema database PostgreSQL dikelola melalui file SQL berversi di direktori ini dan dicatat otomatis di tabel `schema_migrations`.

## 1. Konvensi Penamaan (Wajib)
Setiap migrasi **harus** terdiri dari pasangan `.up.sql` dan `.down.sql` dengan nomor urut 3 digit (`NNN`) dan nama `snake_case`:

```text
001_init.up.sql
001_init.down.sql
002_add_manual_hangar_stock.up.sql
002_add_manual_hangar_stock.down.sql
```

Gunakan perintah berikut agar penomoran otomatis dan tidak bentrok:
```bash
make migrate-new NAME=add_manual_hangar_stock
```

## 2. Aturan Kualitas & Keamanan Skema
1. **Immutability (Jangan Ubah Migrasi Lama):** File migrasi yang sudah diterapkan (`APPLIED`) tidak boleh diedit. Untuk mengubah kolom/tabel, buat file migrasi baru (`002_...`, `003_...`, dst.).
2. **Presisi Data ISK Wajib `NUMERIC(20,2)`:** Semua kolom mata uang ISK (harga, modal, profit, biaya, saldo) **wajib** menggunakan `NUMERIC(20,2)` — dilarang menggunakan `FLOAT` / `REAL` / `DOUBLE PRECISION`.
3. **Standar Tipe Data Lain:**
   * Timestamp: `TIMESTAMPTZ NOT NULL DEFAULT NOW()`
   * Data semi-terstruktur (jadwal, analisis trap, array item): `JSONB` dengan default eksplisit (`'{}'::jsonb` atau `'[]'::jsonb`)
   * EVE ID (`character_id`, `order_id`, `job_id`, `location_id`, `item_id`): `BIGINT`
   * EVE `type_id` / `region_id`: `INT`
4. **Idempotent & Transactional:**
   * Selalu gunakan `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS` di `.up.sql`.
   * Selalu gunakan `DROP TABLE IF EXISTS`, `DROP INDEX IF EXISTS`, `DROP COLUMN IF EXISTS` di `.down.sql` dalam urutan terbalik dependensi (*foreign key*).
   * Setiap migrasi dijalankan di dalam transaksi PostgreSQL (`BEGIN ... COMMIT`), sehingga hindari statement yang tidak boleh berjalan di dalam transaksi (seperti `CREATE INDEX CONCURRENTLY`) kecuali dijalankan manual.
5. **Wajib Ada Pasangan Rollback (`.down.sql`):** Loader migrasi (`LoadEmbeddedMigrations`) akan menolak build/test jika sebuah `.up.sql` tidak memiliki pasangan `.down.sql` yang valid.

## 3. Perintah Eksekusi Migrasi
```bash
# Terapkan semua migrasi yang belum dijalankan (juga berjalan otomatis saat server start)
make migrate-up

# Cek status seluruh migrasi (APPLIED / PENDING)
make migrate-status

# Rollback 1 migrasi terakhir
make migrate-down

# Buat pasangan file migrasi baru (.up.sql & .down.sql)
make migrate-new NAME=nama_perubahan
```
