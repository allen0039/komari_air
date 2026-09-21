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

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

install_packages() {
  if command_exists apt-get; then
    log "Installing packages with apt-get: $*"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y --no-install-recommends "$@"
  elif command_exists dnf; then
    log "Installing packages with dnf: $*"
    dnf install -y "$@"
  elif command_exists yum; then
    log "Installing packages with yum: $*"
    yum install -y "$@"
  elif command_exists apk; then
    log "Installing packages with apk: $*"
    apk add --no-cache "$@"
  elif command_exists pacman; then
    log "Installing packages with pacman: $*"
    pacman -Sy --noconfirm --needed "$@"
  elif command_exists zypper; then
    log "Installing packages with zypper: $*"
    zypper --non-interactive install "$@"
  else
    fail "No supported package manager was found; install $* manually"
  fi
}

ensure_base_dependencies() {
  local packages=()

  if ! command_exists curl; then
    packages+=(curl ca-certificates)
  fi
  if ! command_exists git; then
    packages+=(git)
  fi

  if (( ${#packages[@]} > 0 )); then
    install_packages "${packages[@]}"
  fi

  command_exists curl || fail "Failed to install required command: curl"
  command_exists git || fail "Failed to install required command: git"
}

install_docker() {
  log "Docker is not installed; installing it now..."

  if command_exists apk; then
    install_packages docker docker-cli-compose
  elif command_exists pacman; then
    install_packages docker docker-compose
  elif command_exists zypper; then
    install_packages docker docker-compose
  else
    local installer
    installer="$(mktemp)"
    if ! curl -fsSL --retry 3 https://get.docker.com -o "${installer}"; then
      rm -f "${installer}"
      fail "Failed to download the official Docker installer"
    fi
    if ! sh "${installer}"; then
      rm -f "${installer}"
      fail "The official Docker installer failed"
    fi
    rm -f "${installer}"
  fi

  command_exists docker || fail "Docker installation did not provide the docker command"
}

start_docker() {
  if docker info >/dev/null 2>&1; then
    return
  fi

  log "Starting Docker..."
  if command_exists systemctl; then
    systemctl enable --now docker >/dev/null 2>&1 || systemctl start docker >/dev/null 2>&1 || true
  elif command_exists rc-service; then
    rc-update add docker default >/dev/null 2>&1 || true
    rc-service docker start >/dev/null 2>&1 || true
  elif command_exists service; then
    service docker start >/dev/null 2>&1 || true
  fi

  local attempt
  for ((attempt = 1; attempt <= 30; attempt++)); do
    if docker info >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done

  fail "Docker is installed but the daemon could not be started"
}

install_compose_plugin() {
  local asset
  local architecture
  local actual_checksum
  local expected_checksum
  local plugin_dir="/usr/local/lib/docker/cli-plugins"
  local plugin_path="${plugin_dir}/docker-compose"

  case "$(uname -m)" in
    x86_64|amd64)
      architecture="x86_64"
      ;;
    aarch64|arm64)
      architecture="aarch64"
      ;;
    armv7l|armv7)
      architecture="armv7"
      ;;
    ppc64le|s390x|riscv64)
      architecture="$(uname -m)"
      ;;
    *)
      fail "Unsupported architecture for Docker Compose: $(uname -m)"
      ;;
  esac

  log "Docker Compose is not installed; installing the official plugin..."
  asset="docker-compose-linux-${architecture}"
  mkdir -p "${plugin_dir}"
  curl -fsSL --retry 3 \
    "https://github.com/docker/compose/releases/latest/download/${asset}" \
    -o "${plugin_path}.tmp"
  expected_checksum="$(
    curl -fsSL --retry 3 https://github.com/docker/compose/releases/latest/download/checksums.txt \
      | awk -v asset="${asset}" '$2 == asset || $2 == "*" asset { print $1; exit }'
  )"
  if [[ -z "${expected_checksum}" ]]; then
    rm -f "${plugin_path}.tmp"
    fail "Could not find the Docker Compose checksum"
  fi
  actual_checksum="$(sha256sum "${plugin_path}.tmp" | awk '{ print $1 }')"
  if [[ "${actual_checksum}" != "${expected_checksum}" ]]; then
    rm -f "${plugin_path}.tmp"
    fail "Docker Compose checksum verification failed"
  fi
  chmod 0755 "${plugin_path}.tmp"
  mv "${plugin_path}.tmp" "${plugin_path}"

  docker compose version >/dev/null 2>&1 || fail "Docker Compose plugin installation failed"
}

read_env_value() {
  local key="$1"
  local file="$2"
  sed -n "s/^${key}=//p" "$file" | tail -n 1
}

if [[ "${EUID}" -ne 0 ]]; then
  fail "Run this script as root, for example: curl ... | sudo bash"
fi

ensure_base_dependencies

if ! command_exists docker; then
  install_docker
fi
start_docker

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose)
elif command_exists docker-compose; then
  COMPOSE=(docker-compose -f compose.yaml)
else
  install_compose_plugin
  COMPOSE=(docker compose)
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
  KOMARI_VERSION="$(tr -d '[:space:]' < "${INSTALL_DIR}/VERSION")"

  if [[ ! "${KOMARI_VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    fail "VERSION must contain a semantic version such as 0.1.0"
  fi
  if ! [[ "${KOMARI_PORT}" =~ ^[0-9]+$ ]] || (( KOMARI_PORT < 1 || KOMARI_PORT > 65535 )); then
    fail "KOMARI_PORT must be an integer between 1 and 65535"
  fi
  if [[ "${KOMARI_TZ}" == *$'\n'* || "${KOMARI_TZ}" == *$'\r'* ]]; then
    fail "KOMARI_TZ contains an invalid newline"
  fi

  umask 077
  printf 'COMPOSE_PROJECT_NAME=komari_air\nKOMARI_VERSION=%s\nKOMARI_PORT=%s\nKOMARI_TZ=%s\n' \
    "${KOMARI_VERSION}" "${KOMARI_PORT}" "${KOMARI_TZ}" >"${env_file}"
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
