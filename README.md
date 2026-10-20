# LIS Khanza Mapper

Go web app to map **LIS tests** (`lis_tests`) to **SIMRS Khanza** lab templates (`id_template`). Two modes:

| `MAPPER_MODE` | Database | Hasil push |
|---------------|----------|------------|
| `simrs` (default) | Same MySQL as SIMRS Khanza | `POST /api/v1/medqlab/hasil` writes `periksa_lab` |
| `standalone` | Bundled MariaDB owned by this app | Hasil push returns 503 |

Use **standalone** when HIS IT will not host this app or open the Khanza database (for example puruk-cahu). Bridging then calls `POST /api/v1/mappings/resolve` to fill `testid_simrs`.


## Quick start (Docker)

```bash
cp .env.example .env
# Edit DATABASE_DSN (SIMRS MySQL), AUTH_USERNAME, AUTH_PASSWORD
# For MedQLab push: set MEDQLAB_WEBHOOK_API_KEY; MEDQLAB_BRIDGING_NIP is fallback NIP only
./scripts/deploy.sh
```

Open `http://localhost:8080` and sign in with HTTP Basic auth.

**Migrations** (`lis_tests`, `lis_mapping_tests`, `lis_hasil_inbox`, `simrs_panels`, `simrs_templates`) run automatically every time the app starts.

## Standalone (no Khanza DB)

HIS IT does not deploy this app on their server. Run it with its own MariaDB, import the SIMRS catalog, then map LIS tests. puruk-cahu (`medqlabv2_bridging`) looks up `id_template` at getResult time.

```bash
cp .env.standalone.example .env.standalone
# Set AUTH_USERNAME, AUTH_PASSWORD, MEDQLAB_WEBHOOK_API_KEY, APP_PORT
docker compose --env-file .env.standalone -f docker-compose.standalone.yml up -d --build
```

`--env-file` is required so `APP_PORT` / `MYSQL_PORT` / `MYSQL_DATA_DIR` apply to published ports (service `env_file` alone does not).

- App: `http://localhost:${APP_PORT:-8080}`
- MariaDB: `localhost:3307` (user `mapper`, password `mapper`, database `mapper`)
- Data: host directory `./data/mysql` (`MYSQL_DATA_DIR`)

Catalog CSV columns (re-import updates by `id_template`):

```
id_template,pemeriksaan,satuan,urut,kd_jenis_prw,nm_perawatan,biaya_item,kategori,total_byr,status
```

Keep **one active mapping per LIS test**. If a test maps to several templates, include the ordered `id_template` or `kd_jenis_prw` in `order_ids` so resolve can pick one. Ambiguous or unmapped tests are not guessed.

## Docker Compose files

| File | Env | Purpose |
|------|-----|---------|
| `docker-compose.yml` | `.env` from `.env.example` | App only; Khanza MySQL |
| `docker-compose.standalone.yml` | `.env.standalone` from `.env.standalone.example` | App + MariaDB; no Khanza DB |

Khanza-connected deploy:

```bash
docker compose up -d --build
```

Local Go against `.env`:

```bash
./scripts/run-local.sh
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `MAPPER_MODE` | No | `simrs` (default) or `standalone` |
| `DATABASE_DSN` | Yes | MySQL DSN (Khanza DB in `simrs` mode; mapper DB in `standalone`) |
| `AUTH_USERNAME` | Yes | HTTP Basic user (UI + mapping API) |
| `AUTH_PASSWORD` | Yes | HTTP Basic password |
| `MEDQLAB_WEBHOOK_API_KEY` | For hasil push and resolve | API key for `POST /api/v1/medqlab/hasil` and `POST /api/v1/mappings/resolve` |
| `MEDQLAB_BRIDGING_NIP` | Fallback | Used for `periksa_lab.nip` only when payload has no `idEmployeeVerify` on verified leaves |
| `APP_LISTEN` | No | Default `:8080` (use `:8080` in Docker) |
| `APP_PORT` | No | Host port published by Compose (default `8080`) |
| `APP_ENV` | No | `development` or `production` (default `production`) |
| `APP_TIMEZONE` | No | IANA TZ for MedQLab datetime → SIMRS wall-clock (falls back to `TZ`, then process local). Examples: `Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, `Asia/Pontianak` |
| `MYSQL_PORT` | No | Host port for bundled MariaDB (default `3307`) |
| `MYSQL_DATA_DIR` | Standalone | Host path for MariaDB files (default `./data/mysql`). Not a Docker named volume. |
| `MIGRATE_ONLY` | No | If `true`, run migrations then exit |

Copy `.env.example` → `.env` for `simrs`, or `.env.standalone.example` → `.env.standalone` for standalone. Generate a strong API key with `openssl rand -hex 32` if needed.

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
| `/catalog` | Standalone only: CSV import and add one template |
| `/map/bulk` | Bulk map LIS → templates |
| `/mappings` | View / edit mappings |
| `/bridging-logs` | Audit log of pushed hasil |
| `/bridging-logs/{id}` | Payload + status detail |

## API

JSON under `/api/v1` (Basic auth). Endpoints cover LIS tests, SIMRS templates/panels, catalog import (standalone), and bulk mappings.

### Resolve mapping (puruk-cahu)

API key auth (`X-API-Key` or `Authorization: Bearer`). Returns `id_template` for each MedQLab `testId`.

```http
POST /api/v1/mappings/resolve
Content-Type: application/json
X-API-Key: <MEDQLAB_WEBHOOK_API_KEY>
```

```json
{
  "tests": [
    { "test_id": "12345", "local_code": "HB", "order_ids": ["1001"] }
  ]
}
```

Each result is `{ "test_id", "id_template", "kd_jenis_prw", "pemeriksaan", "status" }` where `status` is `mapped`, `unmapped`, or `ambiguous`. Selection: active `lis_test_id`, else a unique `local_code`; one mapping wins; several mappings keep the one whose `id_template` or `kd_jenis_prw` is in `order_ids`.

### MedQLab hasil push

MedQLab pushes validated lab results to the lis-khanza-mapper API. Auth is **API key**, not Basic auth.

```http
POST /api/v1/medqlab/hasil
Content-Type: application/json
X-API-Key: <MEDQLAB_WEBHOOK_API_KEY>
```

Also accepted: `Authorization: Bearer <MEDQLAB_WEBHOOK_API_KEY>`.

On success the service resolves the SIMRS order, maps examinations via `lis_mapping_tests`, and writes `periksa_lab` / `detail_periksa_lab` / `saran_kesan_lab`. Tarif mirrors native Khanza (`DlgPeriksaLaboratorium`): mode `tindakan` when `jns_perawatan_lab.total_byr > 0` (charge on `periksa_lab`, detail tariffs **0**); mode `template` when panel `total_byr = 0` but ordered items have `biaya_item` (panel stays **0**, charge on `detail_periksa_lab` so Biaya Periksa = SUM items — e.g. Kimia Klinik 0 + 3×40k = 120k). In mode `template`, ordered templates that carry a tariff but were **not** sent as MedQLab hasil leaves are still inserted as fee-only details with empty `nilai` (Khanza fee-header pattern — e.g. ELEKTROLIT 300k under `106-A-K3` while Na+/K+/CL- hold results). Mode `tindakan` does **not** add fee-only rows (avoids double-count with panel). Journal: panel INSERT in `tindakan` mode, or detail INSERT amounts in `template` mode (hasil + fee-only). Re-push refreshes tarif without posting another journal for existing panels. For new charges only, amounts go to `jurnal` / `detailjurnal` via `set_akun_ralan` or `set_akun_ranap`. It also updates `permintaan_lab.tgl_hasil` / `jam_hasil`, and when MedQLab sends `demographics.collectDate`, updates `tgl_sampel` / `jam_sampel`. `periksa_lab.nip` comes from the latest leaf `idEmployeeVerify` (by `verifiedAt`), falling back to `MEDQLAB_BRIDGING_NIP`. Audit rows go to `lis_hasil_inbox` (visible under **Log Bridging**).

Requires `MEDQLAB_WEBHOOK_API_KEY` in `.env`. NIP is required from payload verify employee id or `MEDQLAB_BRIDGING_NIP` fallback. In `standalone` mode this endpoint returns **503** (no write access to SIMRS exam tables).

## Tables

Created on app startup (if missing):

- `lis_tests` — LIS `testId` in `lis_test_id` (unique)
- `lis_mapping_tests` — links `lis_tests.id` → `id_template`, with `kd_jenis_prw`
- `lis_hasil_inbox` — push audit (payload + post status)
- `simrs_panels` / `simrs_templates` — local Khanza catalog (used in standalone; extra unused tables in `simrs` mode)

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
