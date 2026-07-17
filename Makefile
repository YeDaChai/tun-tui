.DEFAULT_GOAL := build

.PHONY: build build-all release run install clean help fetch-geodata

APP     := tun-tui
MODULE  := tun-tui
VERSION ?= 0.2.3
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# 纯 Go 交叉编译，避免本机 CGO 拖垮跨平台构建
export CGO_ENABLED := 0

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(DATE)

BIN_DIR  := bin
DIST_DIR := dist

ifeq ($(GOOS),darwin)
BUILD_TAGS := with_gvisor
else ifeq ($(GOOS),windows)
BUILD_TAGS := with_gvisor
else
BUILD_TAGS :=
endif

ifeq ($(GOOS),windows)
BIN_EXT := .exe
else
BIN_EXT :=
endif

NATIVE_BIN := $(BIN_DIR)/$(APP)$(BIN_EXT)

# os/arch/label — 一份列表同时服务 build-all 与 release
PLATFORMS := \
	darwin/arm64/macos-apple-silicon-arm64 \
	darwin/amd64/macos-intel-x86_64 \
	linux/amd64/linux-x86_64 \
	linux/arm64/linux-arm64 \
	windows/amd64/windows-x86_64

fetch-geodata:
	@./scripts/fetch-geodata.sh

build: fetch-geodata
	@mkdir -p $(BIN_DIR)
	@echo ">> build $(GOOS)/$(GOARCH)$(if $(BUILD_TAGS), [$(BUILD_TAGS)])"
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(if $(BUILD_TAGS),-tags $(BUILD_TAGS)) -ldflags "$(LDFLAGS)" -o $(NATIVE_BIN) ./cmd/app/

build-all: fetch-geodata
	@mkdir -p $(BIN_DIR)
	@for spec in $(PLATFORMS); do \
		os=$${spec%%/*}; rest=$${spec#*/}; arch=$${rest%%/*}; label=$${rest#*/}; \
		ext=; tags=; \
		case $$os in \
			windows) ext=.exe; tags=with_gvisor ;; \
			darwin)  tags=with_gvisor ;; \
		esac; \
		echo ">> build $$label$${tags:+ [$$tags]}"; \
		GOOS=$$os GOARCH=$$arch go build $${tags:+-tags $$tags} -ldflags "$(LDFLAGS)" \
			-o $(BIN_DIR)/$(APP)-$$label$$ext ./cmd/app/ || exit 1; \
	done
	@echo "done -> $(BIN_DIR)/"

# 地理数据已 go:embed 进二进制，发布包只打可执行文件
release: build-all
	@rm -rf $(DIST_DIR) && mkdir -p $(DIST_DIR)
	@for spec in $(PLATFORMS); do \
		os=$${spec%%/*}; rest=$${spec#*/}; arch=$${rest%%/*}; label=$${rest#*/}; \
		ext=; \
		[ "$$os" = windows ] && ext=.exe; \
		bin=$(BIN_DIR)/$(APP)-$$label$$ext; \
		name=$(APP)-$(VERSION)-$$label; \
		tmp=$$(mktemp -d); \
		cp "$$bin" "$$tmp/$(APP)$$ext"; \
		if [ "$$os" = darwin ] && command -v codesign >/dev/null 2>&1; then \
			codesign -s - --force --timestamp=none "$$tmp/$(APP)$$ext" >/dev/null 2>&1 || true; \
		fi; \
		if [ "$$os" = windows ]; then \
			(cd "$$tmp" && zip -q "$(CURDIR)/$(DIST_DIR)/$$name.zip" "$(APP)$$ext"); \
		else \
			tar -czf "$(DIST_DIR)/$$name.tar.gz" -C "$$tmp" "$(APP)$$ext"; \
		fi; \
		rm -rf "$$tmp"; \
	done
	@cd $(DIST_DIR) && (command -v sha256sum >/dev/null && sha256sum $(APP)-* > SHA256SUMS || shasum -a 256 $(APP)-* > SHA256SUMS)
	@echo "done -> $(DIST_DIR)/"

run: build
	@if [ "$(GOOS)" = windows ]; then \
		echo "Windows 请以管理员身份运行: $(NATIVE_BIN)"; \
		TUN_TUI_DATA_DIR=./data $(NATIVE_BIN); \
	else \
		sudo TUN_TUI_DATA_DIR=./data $(NATIVE_BIN); \
	fi

PREFIX  ?= /usr/local
DESTDIR ?=

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(NATIVE_BIN) $(DESTDIR)$(PREFIX)/bin/$(APP)$(BIN_EXT)

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

help:
	@echo "make build        本机编译 ($(GOOS)/$(GOARCH))"
	@echo "make build-all    全平台交叉编译"
	@echo "make release      打包到 dist/（仅二进制，geo 已内嵌）"
	@echo "make run          本机编译并以管理员运行（数据目录 ./data）"
	@echo "make fetch-geodata 下载/校验嵌入用地理数据"
	@echo "make install      安装到 $(PREFIX)/bin"
	@echo "make clean        清理 bin/ dist/"
	@echo ""
	@echo "覆盖平台: make build GOOS=linux GOARCH=amd64"
	@echo "指定版本: make release VERSION=0.2.3"
