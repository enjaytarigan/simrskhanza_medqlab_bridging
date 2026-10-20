#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT}"

ENV_FILE="${ROOT}/.env"
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Missing ${ENV_FILE}. Create it from the example for this mode:" >&2
  echo "  cp .env.example .env                 # simrs (Khanza MySQL)" >&2
  echo "  cp .env.standalone.example .env      # standalone (own MariaDB)" >&2
  exit 1
fi

# Do not `source` .env — DSN values contain () and ? that bash would parse.
load_env() {
  local line key value
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    key="${line%%=*}"
    value="${line#*=}"
    if [[ "$key" == "$line" || -z "$key" ]]; then
      echo "Invalid line in ${ENV_FILE}: ${line}" >&2
      exit 1
    fi
    if [[ "$value" == \"*\" && "$value" == *\" ]]; then
      value="${value:1:-1}"
    elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
      value="${value:1:-1}"
    fi
    export "${key}=${value}"
  done < "${ENV_FILE}"
}
load_env

export APP_ENV="${APP_ENV:-development}"
export RUN_MIGRATIONS="${RUN_MIGRATIONS:-true}"

echo "Starting on ${APP_LISTEN:-:8080} (APP_ENV=${APP_ENV}, RUN_MIGRATIONS=${RUN_MIGRATIONS})"
exec go run ./cmd/server
