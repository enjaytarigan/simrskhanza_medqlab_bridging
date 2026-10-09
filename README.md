# LIS Khanza Mapper

Standalone Go web app to map **LIS tests** (`lis_tests`) to **SIMRS Khanza** lab templates (`template_laboratorium.id_template`). Uses the same MySQL database as SIMRS.

It also exposes an API so **MedQLab can push lab results** into SIMRS exam tables and posts the matching lab accounting journal (same pattern as Khanza / AdamLabs).


## Quick start (Docker)

```bash
cp .env.example .env
# Edit DATABASE_DSN (SIMRS MySQL), AUTH_USERNAME, AUTH_PASSWORD
# For MedQLab push: set MEDQLAB_WEBHOOK_API_KEY; MEDQLAB_BRIDGING_NIP is fallback NIP only
./scripts/deploy.sh
```

Open `http://localhost:8080` and sign in with HTTP Basic auth.

**Migrations** (`lis_tests`, `lis_mapping_tests`, `lis_hasil_inbox`) run automatically every time the app starts.

## Local development (Docker)

Bundled MariaDB + app (good for trying the mapper without touching host MySQL):

```bash
cp .env.example .env
# Edit AUTH_USERNAME / AUTH_PASSWORD
./scripts/run-local.sh
```

- App: `http://localhost:8080`
- MariaDB: `localhost:3307` (user `mapper`, password `mapper`, database `sik`)

Use your host SIMRS database instead:

```bash
./scripts/run-local.sh --external-db
```

Set `DATABASE_DSN` in `.env` to reach MySQL on the host, e.g. `...@tcp(host.docker.internal:3306)/sik?...`.

## Docker Compose files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | App service; connects via `DATABASE_DSN` in `.env` |
| `docker-compose.local.yml` | Adds MariaDB and default DSN for local stack |

Production / deploy:

```bash
docker compose up -d --build
```

Local stack:

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml up --build
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_DSN` | Yes | MySQL DSN (same DB as SIMRS) |
| `AUTH_USERNAME` | Yes | HTTP Basic user (UI + mapping API) |
| `AUTH_PASSWORD` | Yes | HTTP Basic password |
| `MEDQLAB_WEBHOOK_API_KEY` | For MedQLab push | API key for `POST /api/v1/medqlab/hasil` |
| `MEDQLAB_BRIDGING_NIP` | Fallback | Used for `periksa_lab.nip` only when payload has no `idEmployeeVerify` on verified leaves |
| `APP_LISTEN` | No | Default `:8080` (use `:8080` in Docker) |
| `APP_PORT` | No | Host port published by Compose (default `8080`) |
| `APP_ENV` | No | `development` or `production` (default `production`) |
| `APP_TIMEZONE` | No | IANA TZ for MedQLab datetime → SIMRS wall-clock (falls back to `TZ`, then process local). Examples: `Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, `Asia/Pontianak` |
| `MYSQL_PORT` | No | Host port for bundled MariaDB (default `3307`) |
| `MIGRATE_ONLY` | No | If `true`, run migrations then exit |

Copy from `.env.example`. Generate a strong API key with `openssl rand -hex 32` if needed.

## Health

- `GET /healthz` — no auth
- `GET /readyz` — DB ping, no auth

## Web UI

HTTP Basic auth (`AUTH_USERNAME` / `AUTH_PASSWORD`):

| Path | Purpose |
|------|---------|
| `/` | Dashboard |
| `/lis-tests` | Manage LIS tests |
| `/templates` | Browse SIMRS templates |
| `/map/bulk` | Bulk map LIS → templates |
| `/mappings` | View / edit mappings |
| `/bridging-logs` | Audit log of pushed hasil |
| `/bridging-logs/{id}` | Payload + status detail |

## API

JSON under `/api/v1` (Basic auth). Endpoints cover LIS tests, SIMRS templates/panels, and bulk mappings.

### MedQLab hasil push

MedQLab pushes validated lab results to the lis-khanza-mapper API. Auth is **API key**, not Basic auth.

```http
POST /api/v1/medqlab/hasil
Content-Type: application/json
X-API-Key: <MEDQLAB_WEBHOOK_API_KEY>
```

Also accepted: `Authorization: Bearer <MEDQLAB_WEBHOOK_API_KEY>`.

On success the service resolves the SIMRS order, maps examinations via `lis_mapping_tests`, and writes `periksa_lab` / `detail_periksa_lab` / `saran_kesan_lab`. Tarif mirrors native Khanza (`DlgPeriksaLaboratorium`): mode `tindakan` when `jns_perawatan_lab.total_byr > 0` (charge on `periksa_lab`, detail tariffs **0**); mode `template` when panel `total_byr = 0` but ordered items have `biaya_item` (panel stays **0**, charge on `detail_periksa_lab` so Biaya Periksa = SUM items — e.g. Kimia Klinik 0 + 3×40k = 120k). Never charges both levels. Journal: panel INSERT in `tindakan` mode, or detail INSERT amounts in `template` mode. Re-push refreshes tarif without posting another journal for existing panels. For new charges only, amounts go to `jurnal` / `detailjurnal` via `set_akun_ralan` or `set_akun_ranap`. It also updates `permintaan_lab.tgl_hasil` / `jam_hasil`, and when MedQLab sends `demographics.collectDate`, updates `tgl_sampel` / `jam_sampel`. `periksa_lab.nip` comes from the latest leaf `idEmployeeVerify` (by `verifiedAt`), falling back to `MEDQLAB_BRIDGING_NIP`. Audit rows go to `lis_hasil_inbox` (visible under **Log Bridging**).

Requires `MEDQLAB_WEBHOOK_API_KEY` in `.env`. NIP is required from payload verify employee id or `MEDQLAB_BRIDGING_NIP` fallback.

## Tables

Created on app startup (if missing):

- `lis_tests` — LIS `testId` in `lis_test_id` (unique)
- `lis_mapping_tests` — links `lis_tests.id` → `id_template`, with `kd_jenis_prw` from `template_laboratorium`
- `lis_hasil_inbox` — push audit (payload + post status)

SIMRS tables used for hasil write (must already exist): `permintaan_lab`, `periksa_lab`, `detail_periksa_lab`, `saran_kesan_lab`, `reg_periksa`, `template_laboratorium`, `jns_perawatan_lab`, `set_akun_ralan`, `set_akun_ranap`, `jurnal`, `detailjurnal`.

## Integration (SIMRS Java)

Example SIMRS integration lookup:

```sql
SELECT m.id_template FROM lis_mapping_tests m
INNER JOIN lis_tests t ON t.id = m.lis_tests_pk
WHERE t.lis_test_id = ? AND t.status='aktif' AND m.status='aktif'
  AND m.kd_jenis_prw = ?
LIMIT 1;
```

## Deployment

See [LIS-Khanza-Mapper-Deployment.md](LIS-Khanza-Mapper-Deployment.md) for production Docker Compose steps, secrets, DB grants, and troubleshooting.
