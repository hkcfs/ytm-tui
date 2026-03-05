# First stage: build Go CLI
FROM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /opt/ytm ./cmd/ytm

# Second stage: runtime with required CLI deps
FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash fzf yt-dlp mpv jq socat curl ca-certificates git ncurses-bin \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /opt/ytm /usr/local/bin/ytm
COPY scripts/ytm-tui.sh /usr/local/share/ytm/ytm-tui.sh
RUN chmod +x /usr/local/share/ytm/ytm-tui.sh
ENV PATH="/usr/local/bin:${PATH}"
ENTRYPOINT ["ytm"]
CMD ["--help"]
