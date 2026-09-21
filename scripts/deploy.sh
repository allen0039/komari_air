#!/usr/bin/env bash

set -Eeuo pipefail

REPO_URL="${KOMARI_REPO_URL:-https://github.com/allen0039/komari_air.git}"
BRANCH="${KOMARI_BRANCH:-main}"
INSTALL_DIR="${KOMARI_INSTALL_DIR:-/opt/komari_air}"
ACTION="${1:-deploy}"

log() {
  printf '[komari_air] %s\n' "$*"
}

fail() {
  printf '[komari_air] ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

read_env_value() {
  local key="$1"
  local file="$2"
  sed -n "s/^${key}=//p" "$file" | tail -n 1
}

if [[ "${EUID}" -ne 0 ]]; then
  fail "Run this script as root, for example: curl ... | sudo bash"
fi

require_command git
require_command docker

if ! docker info >/dev/null 2>&1; then
  fail "Docker is installed but the daemon is not available"
fi

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE=(docker-compose)
else
  fail "Docker Compose is required"
fi

sync_repository() {
  if [[ -d "${INSTALL_DIR}/.git" ]]; then
    local current_origin
    current_origin="$(git -C "${INSTALL_DIR}" remote get-url origin)"
    if [[ "${current_origin}" != "${REPO_URL}" && "${current_origin}" != "${REPO_URL%.git}" ]]; then
      fail "Existing repository origin is ${current_origin}, expected ${REPO_URL}"
    fi
    if ! git -C "${INSTALL_DIR}" diff --quiet || ! git -C "${INSTALL_DIR}" diff --cached --quiet; then
      fail "Tracked local changes exist in ${INSTALL_DIR}; commit or discard them before updating"
    fi

    log "Fetching ${BRANCH}..."
    git -C "${INSTALL_DIR}" fetch --prune origin "${BRANCH}"
    if git -C "${INSTALL_DIR}" show-ref --verify --quiet "refs/heads/${BRANCH}"; then
      git -C "${INSTALL_DIR}" checkout "${BRANCH}"
    else
      git -C "${INSTALL_DIR}" checkout -b "${BRANCH}" --track "origin/${BRANCH}"
    fi
    git -C "${INSTALL_DIR}" merge --ff-only "origin/${BRANCH}"
    return
  fi

  if [[ -e "${INSTALL_DIR}" ]] && [[ -n "$(find "${INSTALL_DIR}" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
    fail "${INSTALL_DIR} exists and is not an empty Git checkout"
  fi

  log "Cloning ${REPO_URL}..."
  mkdir -p "$(dirname "${INSTALL_DIR}")"
  git clone --depth 1 --branch "${BRANCH}" "${REPO_URL}" "${INSTALL_DIR}"
}

load_deploy_config() {
  local env_file="${INSTALL_DIR}/.env"
  local saved_port=""
  local saved_timezone=""

  if [[ -f "${env_file}" ]]; then
    saved_port="$(read_env_value KOMARI_PORT "${env_file}")"
    saved_timezone="$(read_env_value KOMARI_TZ "${env_file}")"
  fi

  KOMARI_PORT="${KOMARI_PORT:-${saved_port:-25774}}"
  KOMARI_TZ="${KOMARI_TZ:-${saved_timezone:-Asia/Shanghai}}"

  if ! [[ "${KOMARI_PORT}" =~ ^[0-9]+$ ]] || (( KOMARI_PORT < 1 || KOMARI_PORT > 65535 )); then
    fail "KOMARI_PORT must be an integer between 1 and 65535"
  fi
  if [[ "${KOMARI_TZ}" == *$'\n'* || "${KOMARI_TZ}" == *$'\r'* ]]; then
    fail "KOMARI_TZ contains an invalid newline"
  fi

  umask 077
  printf 'COMPOSE_PROJECT_NAME=komari_air\nKOMARI_PORT=%s\nKOMARI_TZ=%s\n' \
    "${KOMARI_PORT}" "${KOMARI_TZ}" >"${env_file}"
}

require_checkout() {
  [[ -f "${INSTALL_DIR}/compose.yaml" ]] || fail "No deployment found at ${INSTALL_DIR}"
  cd "${INSTALL_DIR}"
}

wait_for_health() {
  local container_id=""
  local state=""

  container_id="$("${COMPOSE[@]}" ps -q komari)"
  [[ -n "${container_id}" ]] || fail "Komari container was not created"

  for _ in $(seq 1 60); do
    state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}")"
    case "${state}" in
      healthy)
        return 0
        ;;
      exited|dead|unhealthy)
        "${COMPOSE[@]}" logs --tail=100 komari >&2 || true
        fail "Komari container entered state: ${state}"
        ;;
    esac
    sleep 2
  done

  "${COMPOSE[@]}" logs --tail=100 komari >&2 || true
  fail "Timed out waiting for Komari to become healthy"
}

deploy() {
  sync_repository
  load_deploy_config
  require_checkout

  log "Building the latest image..."
  "${COMPOSE[@]}" build --pull komari
  log "Starting Komari..."
  "${COMPOSE[@]}" up -d --remove-orphans
  wait_for_health

  log "Deployment completed"
  log "Open http://SERVER_IP:${KOMARI_PORT} to finish setup or sign in"
}

case "${ACTION}" in
  deploy|install|update)
    deploy
    ;;
  status)
    require_checkout
    "${COMPOSE[@]}" ps
    ;;
  logs)
    require_checkout
    "${COMPOSE[@]}" logs -f --tail=200 komari
    ;;
  restart)
    require_checkout
    "${COMPOSE[@]}" restart komari
    ;;
  stop)
    require_checkout
    "${COMPOSE[@]}" down
    log "Komari stopped; the komari_air_data volume was preserved"
    ;;
  *)
    fail "Unknown action '${ACTION}'. Use deploy, update, status, logs, restart, or stop"
    ;;
esac
