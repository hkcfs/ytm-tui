#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)
DIST_DIR="$ROOT_DIR/dist"
TARGET_TRIPLE=${TARGET_TRIPLE:-linux/amd64}
IFS='/' read -r TARGET_GOOS TARGET_GOARCH <<<"$TARGET_TRIPLE"
ARCHIVE_NAME="ytm-${TARGET_GOOS}-${TARGET_GOARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
CONTAINER_ID=""

cleanup() {
	if [[ -n "$CONTAINER_ID" ]]; then
		docker rm -f "$CONTAINER_ID" >/dev/null 2>&1 || true
	fi
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$DIST_DIR"

echo "[build-static-bin] Building static binary for $TARGET_TRIPLE"
BUILD_CMD=$(cat <<EOF
set -Eeuo pipefail
cd /workspace
mkdir -p build
export GOOS=$TARGET_GOOS
export GOARCH=$TARGET_GOARCH
export CGO_ENABLED=0
export GOFLAGS='-buildvcs=false'
/usr/local/go/bin/go test ./...
/usr/local/go/bin/go build -ldflags '-s -w' -o build/ytm ./cmd/ytm
EOF
)

CONTAINER_ID=$(docker create --platform "linux/$TARGET_GOARCH" -w /workspace golang:1.26 bash -lc "$BUILD_CMD")
docker cp "$ROOT_DIR/." "$CONTAINER_ID:/workspace"
docker start -a "$CONTAINER_ID"
docker cp "$CONTAINER_ID:/workspace/build/ytm" "$TMP_DIR/ytm"

tar -czf "$DIST_DIR/$ARCHIVE_NAME" -C "$TMP_DIR" ytm
sha256sum "$DIST_DIR/$ARCHIVE_NAME" >"$DIST_DIR/$ARCHIVE_NAME.sha256"

echo "[build-static-bin] Artifact written to $DIST_DIR/$ARCHIVE_NAME"
