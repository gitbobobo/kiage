.PHONY: build build-kindle dev test deploy deploy-kindle tidy import-edge fonts fbink

KIAGE_ROOT ?= $(CURDIR)/extension
GOFLAGS ?= CGO_ENABLED=0
KINDLE_EXT ?=
KINDLE_ROOT ?= /Volumes/Kindle

build:
	$(GOFLAGS) go build -o bin/kiage ./cmd/kiage

build-kindle:
	GOOS=linux GOARCH=arm GOARM=7 $(GOFLAGS) go build -ldflags="-s -w" -o extension/bin/kiage ./cmd/kiage

dev:
	KIAGE_ROOT=$(KIAGE_ROOT) go run ./cmd/kiage dev -addr :8088

import-edge:
	KIAGE_ROOT=$(KIAGE_ROOT) ./bin/kiage import-edge

fonts:
	@mkdir -p extension/fonts
	curl -fsSL -o extension/fonts/NotoSansSC-Regular.otf \
	  "https://github.com/notofonts/noto-cjk/raw/main/Sans/OTF/SimplifiedChinese/NotoSansCJKsc-Regular.otf"
	@echo "font ready: extension/fonts/NotoSansSC-Regular.otf"

fbink:
	@KINDLE_ROOT="$(KINDLE_ROOT)" "$(CURDIR)/scripts/bundle-fbink.sh"

test:
	go test ./...

tidy:
	go mod tidy

deploy-kindle:
	@KINDLE_ROOT="$(KINDLE_ROOT)" "$(CURDIR)/scripts/bundle-fbink.sh" || echo "warn: fbink 未捆绑，设备需有 /mnt/us/libkh/bin/fbink" >&2
	@KINDLE_EXT="$(KINDLE_EXT)" "$(CURDIR)/scripts/deploy-kindle.sh"

deploy: deploy-kindle
