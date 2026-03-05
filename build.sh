#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)
BIN_DIR="$ROOT_DIR/bin"
IMAGE_NAME="${IMAGE_NAME:-ytm-tui:latest}"
GO_IMAGE="${GO_IMAGE:-golang:1.26}"

mkdir -p "$BIN_DIR"

echo "[build.sh] Creating ephemeral Go builder container ($GO_IMAGE)"
CONTAINER_ID=$(docker create --workdir /workspace "$GO_IMAGE" bash -lc "\
  set -Eeuo pipefail && \
  mkdir -p build && \
  /usr/local/go/bin/go test ./... && \
  /usr/local/go/bin/go build -o build/ytm ./cmd/ytm \
")

cleanup() {
	if [[ -n "${CONTAINER_ID:-}" ]]; then
		docker rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

echo "[build.sh] Copying source into container"
docker cp "$ROOT_DIR/." "$CONTAINER_ID:/workspace"

echo "[build.sh] Running go test/build inside container"
docker start -a "$CONTAINER_ID"

echo "[build.sh] Retrieving compiled binary"
docker cp "$CONTAINER_ID:/workspace/build/ytm" "$BIN_DIR/ytm"
chmod +x "$BIN_DIR/ytm"

echo "[build.sh] Building runtime image ($IMAGE_NAME)"
docker build -t "$IMAGE_NAME" "$ROOT_DIR"

echo "[build.sh] Done. Binary at $BIN_DIR/ytm and image tagged $IMAGE_NAME"
