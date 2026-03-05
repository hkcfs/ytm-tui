APP_NAME := ytm
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)

.PHONY: build run lint fmt docker-build docker-run clean

build:
	@mkdir -p $(BIN_DIR)
	GO111MODULE=on go build -o $(BIN_PATH) ./cmd/ytm

run: build
	$(BIN_PATH)

lint:
	go vet ./...
	gofmt -w ./cmd ./internal

fmt:
	gofmt -w ./cmd ./internal

docker-build:
	docker build -t ytm-tui:latest .

docker-run:
	docker run --rm -it \
		--name ytm-tui \
		-v $$HOME/.config/ytm-tui:/root/.config/ytm-tui \
		--device /dev/snd \
		ytm-tui:latest tui

clean:
	rm -rf $(BIN_DIR)
	go clean ./...
