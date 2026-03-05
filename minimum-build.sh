#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)
IMAGE_NAME=${MIN_IMAGE_NAME:-ytm-tui:minimal}
TEST_QUERY=${1:-"milk-v duo"}
TMP_DOCKERFILE=$(mktemp)

cleanup() {
	rm -f "$TMP_DOCKERFILE"
}
trap cleanup EXIT

cat >"$TMP_DOCKERFILE" <<'EOF'
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ytm ./cmd/ytm

FROM alpine:3.19
ENV PATH="/usr/local/bin:${PATH}"
RUN apk add --no-cache bash curl python3 ca-certificates && \
    curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod +x /usr/local/bin/yt-dlp
COPY --from=builder /out/ytm /usr/local/bin/ytm
ENTRYPOINT ["ytm"]
EOF

echo "[minimum-build] Building minimal image $IMAGE_NAME"
docker build -f "$TMP_DOCKERFILE" -t "$IMAGE_NAME" "$ROOT_DIR"

echo "[minimum-build] Running smoke test: ytm search '$TEST_QUERY'"
docker run --rm \
	--name ytm-tui-min-test \
	-e YTM_YTDLP_ARGS="${YTM_YTDLP_ARGS:-}" \
	-e YTM_YTDLP_EXTRACTOR_ARGS="${YTM_YTDLP_EXTRACTOR_ARGS:-}" \
	"$IMAGE_NAME" search --no-fzf --no-history --limit 5 "$TEST_QUERY"

echo "[minimum-build] Success"
