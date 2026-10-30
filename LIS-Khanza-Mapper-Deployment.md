# Panduan Deployment Produksi LIS Khanza Mapper — mode SIMRS

Dokumen ini adalah panduan deployment **mode `simrs`**: aplikasi terhubung ke MySQL/MariaDB Khanza, memetakan tes LIS, dan (opsional) menjembatani order serta hasil dengan MedQLab. Mode `standalone` (tanpa DB Khanza) memakai `.env.standalone.example` dan tidak dibahas di sini.

Repositori: [https://github.com/enjaytarigan/lis-khanza-mapper](https://github.com/enjaytarigan/lis-khanza-mapper)

---

## 1. Ringkasan

LIS Khanza Mapper adalah aplikasi web Go yang:

1. Memetakan tes laboratorium dari **LIS** ke template laboratorium **SIMRS Khanza** (`template_laboratorium`)
2. Menyediakan API agar **MedQLab dapat push hasil lab** ke SIMRS (`periksa_lab`, `detail_periksa_lab`, `saran_kesan_lab`) dan mencatat jurnal akuntansi (`jurnal` / `detailjurnal`) dari tarif INSERT baru, memakai COA `set_akun_ralan` / `set_akun_ranap`
3. (Opsional) Membaca order baru di `permintaan_lab` dan mengirimnya ke MedQLab `saveOrder`, lalu mencatat nomor lab di `lis_order_outbox`

Aplikasi menggunakan **database MySQL/MariaDB produksi SIMRS** yang sudah berjalan di rumah sakit.

| Komponen | Keterangan |
|----------|------------|
| Runtime | Container Docker; **image dibangun di server produksi** dari `Dockerfile` (`docker compose build`) |
| Orkestrasi | Docker Compose |
| Database | MySQL/MariaDB SIMRS (eksternal terhadap container aplikasi) |
| Migrasi DB | Berjalan **otomatis** setiap kali aplikasi start (`lis_tests`, `lis_mapping_tests`, `lis_hasil_inbox`, `lis_order_outbox`) |
| Autentikasi UI/API mapping | HTTP Basic Auth (`AUTH_USERNAME` / `AUTH_PASSWORD`) |
| Autentikasi push hasil | API key (`MEDQLAB_WEBHOOK_API_KEY`) |
| UI audit | `/bridging-logs` — log hasil yang di-push |
| Poller order | Opt-in (`MEDQLAB_ORDER_POLL`); uji dengan `MEDQLAB_ORDER_ONLY_RM` |

### 1.1 Alur data

Dua arah independen. Boleh dinyalakan sendiri-sendiri.

```text
Khanza permintaan_lab
        |
        v  poller (MEDQLAB_ORDER_POLL=true)
lis_order_outbox  ----POST /api/v1/saveOrder---->  MedQLab
                                                    |
                                                    v  push hasil
                         POST /api/v1/medqlab/hasil
                                                    |
                                                    v
                              periksa_lab / detail_periksa_lab / saran_kesan_lab
```

- **Order:** mapper membaca `permintaan_lab`, menyusun JSON seperti `ApiMEDQLAB.kirimRalan` / `kirimRanap`, POST ke MedQLab, menyimpan `no_laboratorium` di `lis_order_outbox`.
- **Hasil:** MedQLab memanggil mapper; mapper menulis tabel pemeriksaan Khanza. Tidak memakai filter RM.

Jangan jalankan poller mapper bersamaan dengan kirim order dari UI Khanza (`kirimRalan` / `kirimRanap`) untuk order yang sama — MedQLab bisa menerima duplikat.

### 1.2 Tiga profil operasi

Set `MAPPER_MODE=simrs` pada ketiganya. UI mapping selalu aktif.

| Profil | Order ke MedQLab | Hasil ke Khanza | Env kunci |
|--------|------------------|-----------------|-----------|
| Hasil saja | Mati | Hidup | `MEDQLAB_ORDER_POLL=false` (default) + isi `MEDQLAB_WEBHOOK_API_KEY` |
| Order saja | Hidup | Mati | `MEDQLAB_ORDER_POLL=true` + URL/cons/sign/since; **kosongkan** `MEDQLAB_WEBHOOK_API_KEY` |
| Keduanya | Hidup | Hidup | Poller on + isi API key |

Hasil tidak punya flag `true`/`false` tersendiri. Endpoint `POST /api/v1/medqlab/hasil` menolak request jika API key kosong (HTTP 503).

### 1.3 Jaringan

| Arah | Syarat |
|------|--------|
| Mapper → MySQL Khanza | `DATABASE_DSN` reachable (biasanya port 3306) |
| Mapper → MedQLab (order) | Host `MEDQLAB_BASE_URL` reachable dari container; path yang dipakai `/api/v1/saveOrder` |
| MedQLab → mapper (hasil) | MedQLab reachable ke `http://<host-mapper>:<APP_PORT>/api/v1/medqlab/hasil` dengan `X-API-Key` |

Firewall: buka `APP_PORT` hanya di jaringan internal/RS. Reverse proxy TLS disarankan jika MedQLab di luar host mapper.

---

## 2. Prasyarat

Server produksi (VM/bare metal) harus memiliki:

- **Docker Engine** 24+ (atau setara)
- **Docker Compose** v2 (`docker compose`)
- Akses jaringan ke **MySQL/MariaDB SIMRS produksi** (port biasanya `3306`)
- Jika poller order on: akses HTTPS/HTTP dari server mapper ke origin MedQLab (`MEDQLAB_BASE_URL`)
- Jika hasil on: MedQLab harus bisa memanggil `APP_PORT` mapper
- **Git** (clone/pull kode di server produksi)
- Ruang disk dan CPU memadai untuk **build image** di server (kompilasi Go di dalam Docker build)
- Port `APP_PORT` (default `8080`) terbuka sesuai kebijakan akses internal/rs

```bash
docker --version
docker compose version
```

Instalasi Docker: [https://docs.docker.com/engine/install/](https://docs.docker.com/engine/install/)

---

## 3. Clone repositori

```bash
git clone https://github.com/enjaytarigan/lis-khanza-mapper.git
cd lis-khanza-mapper
```

---

## 4. File `docker-compose.yml`

Buat file berikut di **root** proyek (sejajar dengan `Dockerfile`), atau gunakan file yang sudah ada di repositori.

Layanan aplikasi saja; database SIMRS produksi sudah tersedia di server lain atau di host yang sama.

```yaml
# LIS Khanza Mapper — produksi
#
#   cp .env.example .env
#   # Edit DATABASE_DSN, AUTH_*, MEDQLAB_*
#   docker compose up -d --build

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: lis-khanza-mapper
    restart: unless-stopped
    ports:
      - "${APP_PORT:-8080}:8080"
    env_file:
      - .env
    environment:
      APP_LISTEN: ":8080"
      APP_ENV: production
```

> **Healthcheck:** Image runtime Alpine default tidak menyertakan `wget`. Untuk pemantauan produksi, gunakan probe eksternal (`curl` ke `/healthz` dan `/readyz` dari load balancer atau monitoring) alih-alih healthcheck container bawaan, kecuali Anda menambahkan alat probe ke image.

---

## 5. Variabel lingkungan dan rahasia

### 5.1 File `.env`

Salin template dan batasi hak akses file:

```bash
cp .env.example .env
chmod 600 .env
```

Contoh isi **produksi**:

```env
# MySQL SIMRS produksi
DATABASE_DSN=simrs_user:KataSandiKuat@tcp(192.168.1.10:3306)/sik?parseTime=true&loc=Local&charset=utf8mb4

# Login aplikasi (HTTP Basic Auth) — UI + API mapping
AUTH_USERNAME=admin
AUTH_PASSWORD=GantiDenganKataSandiKuat

APP_ENV=production
APP_LISTEN=:8080
APP_PORT=8080

# Zona waktu rumah sakit (IANA). Contoh: Asia/Jakarta | Asia/Makassar | Asia/Jayapura | Asia/Pontianak
APP_TIMEZONE=Asia/Jakarta

MAPPER_MODE=simrs

# Hasil: kosongkan MEDQLAB_WEBHOOK_API_KEY untuk menonaktifkan push hasil
MEDQLAB_WEBHOOK_API_KEY=ganti-dengan-openssl-rand-hex-32
MEDQLAB_BRIDGING_NIP=198001012000011001

# Order: false = hasil saja. true membutuhkan URL/cons/sign/since.
MEDQLAB_ORDER_POLL=false
MEDQLAB_BASE_URL=https://lis.example
MEDQLAB_X_CONS=consid-khanza
MEDQLAB_X_SIGN=secret-mentah-bukan-hmac
MEDQLAB_ORDER_SINCE=2026-10-01
MEDQLAB_ORDER_INTERVAL=5s
MEDQLAB_ORDER_CLAIM_TIMEOUT=2m
# Uji: hanya nomor RM ini yang dikirim. Kosongkan saat go-live semua pasien.
MEDQLAB_ORDER_ONLY_RM=
```

Generate API key:

```bash
openssl rand -hex 32
```

#### Format `DATABASE_DSN`

```text
user:password@tcp(host:port)/nama_database?parseTime=true&loc=Local&charset=utf8mb4
```

| Skenario produksi | Host di DSN |
|-------------------|-------------|
| MySQL SIMRS di **server terpisah** | IP atau hostname server DB, mis. `192.168.1.10` |

Contoh MySQL:

```env
DATABASE_DSN=spv:server@tcp(192.168.1.10:3306)/sik?parseTime=true&loc=Local&charset=utf8mb4
```

### 5.2 Daftar variabel

| Variabel | Wajib | Default | Deskripsi |
|----------|-------|---------|-----------|
| `MAPPER_MODE` | Tidak | `simrs` | Harus `simrs` untuk dokumen ini |
| `DATABASE_DSN` | Ya | — | DSN ke database SIMRS produksi |
| `AUTH_USERNAME` | Ya | — | Username HTTP Basic Auth (UI + API mapping) |
| `AUTH_PASSWORD` | Ya | — | Password HTTP Basic Auth |
| `MEDQLAB_WEBHOOK_API_KEY` | Untuk push hasil | — | API key `POST /api/v1/medqlab/hasil`. Jika kosong → HTTP 503 (hasil mati) |
| `MEDQLAB_BRIDGING_NIP` | Fallback hasil | — | NIP untuk `periksa_lab.nip` jika payload tidak punya `idEmployeeVerify` |
| `MEDQLAB_ORDER_POLL` | Tidak | `false` | `true` untuk mengirim order Khanza ke MedQLab |
| `MEDQLAB_BASE_URL` | Jika poll on | — | Origin MedQLab (tanpa trailing slash) |
| `MEDQLAB_X_CONS` | Jika poll on | — | Header `X-cons` (cons id Khanza) |
| `MEDQLAB_X_SIGN` | Jika poll on | — | Header `X-sign` (secret mentah, bukan HMAC) |
| `MEDQLAB_ORDER_SINCE` | Jika poll on | — | `yyyy-MM-dd`; hanya order dengan `tgl_permintaan` pada/sesudah tanggal ini |
| `MEDQLAB_ORDER_INTERVAL` | Tidak | `5s` | Jeda antar tick poller |
| `MEDQLAB_ORDER_CLAIM_TIMEOUT` | Tidak | `2m` | Klaim `00` yang macet boleh diambil ulang setelah durasi ini |
| `MEDQLAB_ORDER_ONLY_RM` | Tidak | kosong | Daftar `no_rkm_medis` dipisah koma. Kosong = semua pasien. Terisi = hanya RM itu (mode uji) |
| `APP_LISTEN` | Tidak | `:8080` | Bind di dalam container (gunakan `:8080`) |
| `APP_PORT` | Tidak | `8080` | Port yang dipublish ke host |
| `APP_ENV` | Tidak | `production` | Tetap `production` di server produksi |
| `APP_TIMEZONE` | Tidak | process local | IANA TZ untuk konversi datetime MedQLab ke DATE/TIME SIMRS (`Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, …). Fallback: `TZ` |

| Variabel | Deskripsi |
|----------|-----------|
| `MIGRATE_ONLY=true` | Hanya jalankan migrasi lalu keluar (pemeliharaan/troubleshooting) |

> **Migrasi:** Tabel `lis_tests`, `lis_mapping_tests`, `lis_hasil_inbox`, dan `lis_order_outbox` dibuat/diperbarui **otomatis** saat aplikasi start. Tidak perlu menjalankan skrip SQL manual pada deployment rutin.

### 5.3 Keamanan rahasia (secret)

| Praktik | Keterangan |
|---------|------------|
| **Jangan commit `.env`** | File termasuk dalam `.gitignore` |
| **`chmod 600 .env`** | Hanya akun yang menjalankan deploy yang boleh membaca |
| **Password kuat** | `AUTH_PASSWORD`, `MEDQLAB_WEBHOOK_API_KEY`, dan kredensial MySQL harus kuat |
| **Satu server, satu `.env`** | File konfigurasi disimpan hanya di server produksi |
| **Docker Secrets (Swarm)** | Jika memakai Swarm, map secret ke environment variable |
| **CI/CD** (jika dipakai) | Hanya untuk inject variabel ke server; build tetap di server produksi, bukan push/pull image registry |

Contoh yang **tidak boleh** dilakukan:

```bash
git add .env
```

Alternatif tanpa menyimpan `.env` di disk (mis. dari orchestrator):

```bash
export DATABASE_DSN='user:pass@tcp(10.0.0.5:3306)/sik?parseTime=true&loc=Local&charset=utf8mb4'
export AUTH_USERNAME=admin
export AUTH_PASSWORD='...'
export MEDQLAB_WEBHOOK_API_KEY='...'
export MEDQLAB_BRIDGING_NIP='...'
export APP_ENV=production
docker compose up -d --build
```

---

## 6. Langkah deployment

### 6.1 Persiapan database SIMRS produksi

1. Buat atau gunakan user MySQL dengan hak sesuai profil. Semua profil butuh hak inti mapping.

**Inti (selalu):**

| Hak | Objek |
|-----|--------|
| **SELECT** | `template_laboratorium`, `jns_perawatan_lab` |
| **CREATE / ALTER / INSERT / UPDATE / SELECT** | `lis_tests`, `lis_mapping_tests` |

**Tambahan hasil:**

| Hak | Objek |
|-----|--------|
| **SELECT** | `permintaan_lab`, `reg_periksa`, `set_akun_ralan`, `set_akun_ranap` |
| **CREATE / ALTER / INSERT / UPDATE / SELECT** | `lis_hasil_inbox` |
| **INSERT / UPDATE / SELECT** | `periksa_lab`, `detail_periksa_lab`, `saran_kesan_lab` |
| **INSERT / SELECT** | `jurnal`, `detailjurnal` |
| **UPDATE** | `permintaan_lab` (`tgl_hasil` / `jam_hasil`; plus `tgl_sampel` / `jam_sampel` jika ada `collectDate`) |

**Tambahan poller order:**

| Hak | Objek |
|-----|--------|
| **SELECT** | `permintaan_lab`, `permintaan_pemeriksaan_lab`, `permintaan_detail_permintaan_lab`, `reg_periksa`, `pasien`, `dokter`, `penjab`, `poliklinik`, `kamar_inap`, `kamar`, `bangsal` |
| **CREATE / ALTER / INSERT / UPDATE / SELECT** | `lis_order_outbox` |

2. Mapping LIS ↔ template dikelola lewat UI sebelum uji order/hasil. Poller hanya mengirim tes yang ada di permintaan Khanza; hasil yang tidak termapping tercatat di `/bridging-logs`.

### 6.2 Build di server dan jalankan

Image **tidak** diambil dari registry; seluruh build dilakukan di server produksi setelah clone repositori.

```bash
cd lis-khanza-mapper

cp .env.example .env
nano .env   # sesuaikan DATABASE_DSN, AUTH_*, MEDQLAB_*

# Build image dari Dockerfile (kompilasi di dalam Docker)
docker compose build

# Jalankan container
docker compose up -d
```

Atau build dan start sekaligus:

```bash
docker compose up -d --build
```

Periksa log:

```bash
docker compose logs -f app
```

Output yang diharapkan:

```text
migrations applied
listening on :8080 (env=production mode=simrs timezone=Asia/Jakarta order_poll=false)
```

Jika poller on, log juga memuat `[orderpoll] started ...` dan `order_only_rm=...` atau `all patients`.

### 6.3 Verifikasi

```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/readyz
curl -u admin:KataSandiAnda http://localhost:8080/
curl -u admin:KataSandiAnda http://localhost:8080/bridging-logs
```

Akses operator: `http://<IP-server-produksi>:8080` dengan kredensial `AUTH_USERNAME` / `AUTH_PASSWORD`.

- **Log Bridging:** `http://<IP-server>:8080/bridging-logs`
- Disarankan membatasi akses jaringan (VPN, firewall internal, reverse proxy dengan TLS).

### 6.4 Pembaruan versi

Tarik kode terbaru, build ulang image di server yang sama, lalu ganti container:

```bash
cd lis-khanza-mapper
git pull origin master
docker compose build --no-cache
docker compose up -d
```

Migrasi dijalankan ulang saat container baru start (perintah `CREATE TABLE IF NOT EXISTS` bersifat idempotent).

### 6.5 Penghentian layanan

```bash
docker compose down
```

### 6.6 Uji poller order dengan nomor RM (lalu go-live)

Gunakan ini sebelum membuka poller ke semua pasien.

1. Pastikan mapping LIS → template sudah ada di UI (`/map/bulk`).
2. Pilih satu atau dua `no_rkm_medis` uji yang punya `permintaan_lab` baru (`tgl_permintaan` ≥ `MEDQLAB_ORDER_SINCE`).
3. Set:

```env
MEDQLAB_ORDER_POLL=true
MEDQLAB_ORDER_ONLY_RM=000022,000023
```

4. Restart: `docker compose up -d`. Cek log `[orderpoll] started ... order_only_rm=000022,000023`.
5. Buat order lab di Khanza untuk RM itu (atau tunggu order yang sudah ada). Jangan kirim lewat tombol `ApiMEDQLAB` Khanza.
6. Verifikasi:

```sql
SELECT noorder, no_rawat, no_laboratorium, status, error_message, claimed_at, sent_at
FROM lis_order_outbox
ORDER BY updated_at DESC
LIMIT 20;
```

   - `sent` + nomor lab: MedQLab menerima order.
   - `failed`: baca `error_message` (order pendek, tanpa item, kamar ranap tidak ketemu, ditolak LIS).
   - Order RM lain **tidak** muncul di outbox.

7. Jika hasil juga diuji, MedQLab push ke `/api/v1/medqlab/hasil`; cek UI `/bridging-logs`.
8. Go-live semua pasien: kosongkan `MEDQLAB_ORDER_ONLY_RM` (atau hapus baris itu), restart. Log harus `all patients`.

`MEDQLAB_ORDER_ONLY_RM` tidak memfilter push hasil. Kosongkan nilai itu jika tidak sedang menguji order.

---

## 7. Troubleshooting

| Gejala | Kemungkinan penyebab | Solusi |
|--------|---------------------|--------|
| `database ping: connection refused` | DSN / firewall / MySQL mati | Periksa host, port, status MySQL; buka firewall DB |
| `Access denied for user` | User/password salah atau hak kurang | Perbaiki `DATABASE_DSN`; berikan grant sesuai §6.1 |
| `migrate: ... syntax error` | Versi DB tidak kompatibel | Gunakan MariaDB 10.x / MySQL 5.7+ sesuai SIMRS |
| Tidak bisa diakses dari workstation | Firewall / port | Buka `APP_PORT`; gunakan IP server produksi |
| `host.docker.internal` gagal | Host gateway (Linux) | Pastikan `extra_hosts: host-gateway` di Compose |
| Login ditolak | Kredensial aplikasi salah | Periksa `AUTH_USERNAME` / `AUTH_PASSWORD` |
| Template kosong | DSN bukan DB SIMRS | Pastikan database `sik` (atau nama DB SIMRS Anda) benar |
| Push hasil `401` | API key salah | Periksa `MEDQLAB_WEBHOOK_API_KEY` |
| Push hasil `503` | Key/NIP belum di-set | Isi `MEDQLAB_WEBHOOK_API_KEY` dan `MEDQLAB_BRIDGING_NIP`, restart |
| Hasil tidak masuk SIMRS | Order/mapping tidak cocok | Cek log app dan UI `/bridging-logs` |
| Order tidak masuk MedQLab | Poller off / since terlalu baru / outbox `failed` / RM tidak di allowlist | Set `MEDQLAB_ORDER_POLL=true` dan `MEDQLAB_ORDER_SINCE`; jika uji, pastikan RM ada di `MEDQLAB_ORDER_ONLY_RM`; cek log `[orderpoll]` dan `lis_order_outbox` |
| Order terkirim ganda | `ApiMEDQLAB` Khanza dan poller sama-sama aktif | Matikan kirim order dari UI Khanza untuk order yang di-poll mapper |
| Order pasien uji tidak terkirim | RM tidak cocok / `tgl_permintaan` sebelum since | Samakan `no_rkm_medis` (termasuk nol di depan) dengan `MEDQLAB_ORDER_ONLY_RM` |

```bash
docker compose ps
docker compose logs app --tail 100
curl -s http://localhost:8080/readyz
```

---

## 8. Ringkasan perintah

```bash
git clone https://github.com/enjaytarigan/lis-khanza-mapper.git
cd lis-khanza-mapper

cp .env.example .env && chmod 600 .env
# edit .env — DATABASE_DSN, AUTH_*, MEDQLAB_WEBHOOK_API_KEY, MEDQLAB_BRIDGING_NIP
# optional poller: MEDQLAB_ORDER_POLL, MEDQLAB_BASE_URL, MEDQLAB_X_CONS, MEDQLAB_X_SIGN, MEDQLAB_ORDER_SINCE
# uji: MEDQLAB_ORDER_ONLY_RM=000022  (kosongkan saat go-live)

docker compose up -d --build
docker compose logs -f app
docker compose ps
```

---

## 9. Referensi

- Repositori: [github.com/enjaytarigan/lis-khanza-mapper](https://github.com/enjaytarigan/lis-khanza-mapper)
- README proyek: `README.md`
- Template env (SIMRS): `.env.example` → `.env`
- Template env (standalone, no Khanza DB): `.env.standalone.example` → `.env`
- Contoh query integrasi SIMRS (Java): lihat bagian integrasi di `README.md`
