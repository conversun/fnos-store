.PHONY: dev build-linux-x86 build-linux-arm build-frontend build-all fpk clean

BINARY_NAME := fnos-store
BUILD_DIR := build

dev:
	PROJECT_ROOT=$(CURDIR) go run ./cmd/server/

build-linux-x86:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server/

build-linux-arm:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/server/

build-frontend:
	cd frontend && npm run build && cp -r dist/ ../web/

build-all: build-frontend build-linux-x86 build-linux-arm

fpk:
	bash build.sh

clean:
	rm -rf $(BUILD_DIR) fnos-apps-store_*.fpk
