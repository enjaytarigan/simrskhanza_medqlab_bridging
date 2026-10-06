# Panduan Deployment Produksi LIS Khanza Mapper (Docker Compose)

Dokumen ini menjelaskan cara men-deploy aplikasi **LIS Khanza Mapper** ke **lingkungan produksi** menggunakan Docker Compose: clone repositori, definisi Compose, serta konfigurasi variabel lingkungan dan rahasia (secret).

Repositori: [https://github.com/enjaytarigan/lis-khanza-mapper](https://github.com/enjaytarigan/lis-khanza-mapper)

---

## 1. Ringkasan

LIS Khanza Mapper adalah aplikasi web Go yang:

1. Memetakan tes laboratorium dari **LIS** ke template laboratorium **SIMRS Khanza** (`template_laboratorium`)
2. Menyediakan API agar **MedQLab dapat push hasil lab** ke SIMRS (`periksa_lab`, `detail_periksa_lab`, `saran_kesan_lab`) — tanpa posting jurnal akuntansi

Aplikasi menggunakan **database MySQL/MariaDB produksi SIMRS** yang sudah berjalan di rumah sakit.

| Komponen | Keterangan |
|----------|------------|
| Runtime | Container Docker; **image dibangun di server produksi** dari `Dockerfile` (`docker compose build`) |
| Orkestrasi | Docker Compose |
| Database | MySQL/MariaDB SIMRS (eksternal terhadap container aplikasi) |
| Migrasi DB | Berjalan **otomatis** setiap kali aplikasi start (`lis_tests`, `lis_mapping_tests`, `lis_hasil_inbox`) |
| Autentikasi UI/API mapping | HTTP Basic Auth (`AUTH_USERNAME` / `AUTH_PASSWORD`) |
| Autentikasi push hasil | API key (`MEDQLAB_WEBHOOK_API_KEY`) |
| UI audit | `/bridging-logs` — log hasil yang di-push |

---

## 2. Prasyarat

Server produksi (VM/bare metal) harus memiliki:

- **Docker Engine** 24+ (atau setara)
- **Docker Compose** v2 (`docker compose`)
- Akses jaringan ke **MySQL/MariaDB SIMRS produksi** (port biasanya `3306`)
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

# MedQLab push result API (POST /api/v1/medqlab/hasil)
MEDQLAB_WEBHOOK_API_KEY=ganti-dengan-openssl-rand-hex-32
MEDQLAB_BRIDGING_NIP=198001012000011001
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
| `DATABASE_DSN` | Ya | — | DSN ke database SIMRS produksi |
| `AUTH_USERNAME` | Ya | — | Username HTTP Basic Auth (UI + API mapping) |
| `AUTH_PASSWORD` | Ya | — | Password HTTP Basic Auth |
| `MEDQLAB_WEBHOOK_API_KEY` | Untuk push hasil | — | API key `POST /api/v1/medqlab/hasil`. Jika kosong → HTTP 503 |
| `MEDQLAB_BRIDGING_NIP` | Fallback | — | NIP untuk `periksa_lab.nip` jika payload tidak punya `idEmployeeVerify` |
| `APP_LISTEN` | Tidak | `:8080` | Bind di dalam container (gunakan `:8080`) |
| `APP_PORT` | Tidak | `8080` | Port yang dipublish ke host |
| `APP_ENV` | Tidak | `production` | Tetap `production` di server produksi |
| `APP_TIMEZONE` | Tidak | process local | IANA TZ untuk konversi datetime MedQLab ke DATE/TIME SIMRS (`Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, …). Fallback: `TZ` |

| Variabel | Deskripsi |
|----------|-----------|
| `MIGRATE_ONLY=true` | Hanya jalankan migrasi lalu keluar (pemeliharaan/troubleshooting) |

> **Migrasi:** Tabel `lis_tests`, `lis_mapping_tests`, dan `lis_hasil_inbox` dibuat/diperbarui **otomatis** saat aplikasi start. Tidak perlu menjalankan skrip SQL manual pada deployment rutin.

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

1. Buat atau gunakan user MySQL dengan hak minimal:

| Hak | Objek |
|-----|--------|
| **SELECT** | `template_laboratorium`, `jns_perawatan_lab`, `permintaan_lab`, `reg_periksa` |
| **CREATE / ALTER / INSERT / UPDATE / SELECT** | `lis_tests`, `lis_mapping_tests`, `lis_hasil_inbox` |
| **INSERT / UPDATE / SELECT** | `periksa_lab`, `detail_periksa_lab`, `saran_kesan_lab` |
| **UPDATE** | `permintaan_lab` (kolom `tgl_hasil` / `jam_hasil`; plus `tgl_sampel` / `jam_sampel` jika ada `collectDate`) |

2. Mapping LIS ↔ template dikelola lewat UI; hasil lab ditulis otomatis saat MedQLab push ke API.

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
listening on :8080 (env=production)
```

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

docker compose up -d --build
docker compose logs -f app
docker compose ps
```

---

## 9. Referensi

- Repositori: [github.com/enjaytarigan/lis-khanza-mapper](https://github.com/enjaytarigan/lis-khanza-mapper)
- README proyek: `README.md`
- Template env: `.env.example`
- Contoh query integrasi SIMRS (Java): lihat bagian integrasi di `README.md`
