#!/usr/bin/env bash
set -euo pipefail

# aura-forge Podman Deployment Script (executed remotely over SSH stdin; writes no secret files to VPS)
# Usage: bash -s -- <target_env> <image_name> <app_port> <app_bind>

TARGET_ENV="${1:-dev}"
IMAGE_NAME="${2:?image_name is required}"
APP_PORT="${3:-8081}"
APP_BIND="${4:-127.0.0.1}"

CONTAINER_NAME="aura-forge-${TARGET_ENV}"

echo "=========================================================="
echo " Deploying aura-forge (${TARGET_ENV})"
echo " Container : ${CONTAINER_NAME}"
echo " Image     : ${IMAGE_NAME}"
echo " Binding   : ${APP_BIND}:${APP_PORT} -> 8080"
echo "=========================================================="

if [ -n "${GHCR_TOKEN:-}" ] && [ -n "${GHCR_USER:-}" ]; then
  echo "--> Logging in to ghcr.io..."
  echo "${GHCR_TOKEN}" | podman login ghcr.io -u "${GHCR_USER}" --password-stdin >/dev/null
fi

OLD_IMAGE_ID=$(podman images -q "${IMAGE_NAME}" 2>/dev/null | head -n 1 || true)

echo "--> Pulling latest image ${IMAGE_NAME} (always replaces local tag if version matches)..."
podman pull "${IMAGE_NAME}"
NEW_IMAGE_ID=$(podman images -q "${IMAGE_NAME}" 2>/dev/null | head -n 1 || true)
if [ -n "${OLD_IMAGE_ID}" ] && [ "${OLD_IMAGE_ID}" != "${NEW_IMAGE_ID}" ]; then
  echo "--> Replaced existing tag ${IMAGE_NAME}: ${OLD_IMAGE_ID} -> ${NEW_IMAGE_ID}"
fi

if podman ps -a --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
  echo "--> Stopping and removing existing container ${CONTAINER_NAME}..."
  podman rm -f "${CONTAINER_NAME}" >/dev/null
fi

ENV_ARGS=(
  -e "PORT=8080"
  -e "DATABASE_URL=${DATABASE_URL:-}"
  -e "SESSION_SECRET=${SESSION_SECRET:-}"
  -e "ESI_USER_AGENT=${ESI_USER_AGENT:-aura-forge/1.0 (EVE Market & Industry Copilot; contact: pilot@new-eden.org)}"
  -e "ESI_CLIENT_ID=${ESI_CLIENT_ID:-}"
  -e "ESI_CLIENT_SECRET=${ESI_CLIENT_SECRET:-}"
  -e "ESI_CALLBACK_URL=${ESI_CALLBACK_URL:-}"
  -e "AI_BASE_URL=${AI_BASE_URL:-https://api.openai.com/v1}"
  -e "AI_API_KEY=${AI_API_KEY:-}"
  -e "AI_MODEL=${AI_MODEL:-gpt-4.1-mini}"
  -e "AI_TIMEOUT_SECONDS=${AI_TIMEOUT_SECONDS:-60}"
)

if [ -n "${ADMIN_USERNAME:-}" ]; then
  ENV_ARGS+=(-e "ADMIN_USERNAME=${ADMIN_USERNAME}")
fi
if [ -n "${ADMIN_PASSWORD:-}" ]; then
  ENV_ARGS+=(-e "ADMIN_PASSWORD=${ADMIN_PASSWORD}")
fi

EXTRA_ARR=()
if [ -n "${PODMAN_EXTRA_ARGS:-}" ]; then
  # shellcheck disable=SC2206
  EXTRA_ARR=(${PODMAN_EXTRA_ARGS})
fi

echo "--> Starting container ${CONTAINER_NAME}..."
podman run -d \
  --replace \
  --name "${CONTAINER_NAME}" \
  --restart unless-stopped \
  --add-host=host.docker.internal:host-gateway \
  -p "${APP_BIND}:${APP_PORT}:8080" \
  "${ENV_ARGS[@]}" \
  "${EXTRA_ARR[@]}" \
  "${IMAGE_NAME}" >/dev/null

echo "--> Waiting for /health check on http://127.0.0.1:${APP_PORT}/health..."
HEALTH_OK=0
for i in $(seq 1 15); do
  if curl -fsS "http://127.0.0.1:${APP_PORT}/health" >/dev/null 2>&1; then
    HEALTH_OK=1
    break
  fi
  sleep 1
done

if [ "${HEALTH_OK}" -eq 1 ]; then
  echo "--> Health check PASSED! ${CONTAINER_NAME} is running."
else
  echo "--> Warning: /health did not respond within 15s. Recent container logs:" >&2
  podman logs --tail 40 "${CONTAINER_NAME}" >&2 || true
  exit 1
fi

echo "--> Pruning all unused & dangling images (keeping active container images)..."
podman image prune -af || true

echo "--> Deployment complete."
