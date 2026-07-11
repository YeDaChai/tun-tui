.DEFAULT_GOAL := build

.PHONY: build build-all release run install clean help fetch-geodata

APP      := tun-tui
VERSION  ?= 0.1.5
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE   := tun-tui

# 自动识别当前平台（可用 GOOS/GOARCH 覆盖）
GOOS     ?= $(shell go env GOOS)
GOARCH   ?= $(shell go env GOARCH)

LDFLAGS  := -s -w \
            -X $(MODULE)/internal/version.Version=$(VERSION) \
            -X $(MODULE)/internal/version.Commit=$(COMMIT) \
            -X $(MODULE)/internal/version.BuildDate=$(DATE)

BIN_DIR  := bin
DIST_DIR := dist
GEODATA_DIR := internal/geodata

# macOS / Windows TUN 使用 gVisor 用户态栈；Linux 使用 system 栈
ifeq ($(GOOS),darwin)
BUILD_TAGS := with_gvisor
else ifeq ($(GOOS),windows)
BUILD_TAGS := with_gvisor
else
BUILD_TAGS :=
endif

ifneq ($(BUILD_TAGS),)
TAGS_FLAG := -tags $(BUILD_TAGS)
else
TAGS_FLAG :=
endif

ifeq ($(GOOS),windows)
BIN_EXT := .exe
else
BIN_EXT :=
endif

NATIVE_BIN := $(BIN_DIR)/$(APP)$(BIN_EXT)

PLATFORMS := \
	darwin/arm64 \
	darwin/amd64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

fetch-geodata:
	@./scripts/fetch-geodata.sh

build: fetch-geodata
	@mkdir -p $(BIN_DIR)
	@echo ">> build $(GOOS)/$(GOARCH)$(if $(BUILD_TAGS), [$(BUILD_TAGS)],)"
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(TAGS_FLAG) -ldflags "$(LDFLAGS)" -o $(NATIVE_BIN) ./cmd/app/

build-all: fetch-geodata
	@mkdir -p $(BIN_DIR)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=; [ "$$os" = windows ] && ext=.exe; \
		case "$$os/$$arch" in \
			darwin/arm64)  label=macos-apple-silicon-arm64 ;; \
			darwin/amd64)  label=macos-intel-x86_64 ;; \
			linux/amd64)   label=linux-x86_64 ;; \
			linux/arm64)   label=linux-arm64 ;; \
			windows/amd64) label=windows-x86_64 ;; \
			*)             label=$$os-$$arch ;; \
		esac; \
		tags=; [ "$$os" = darwin ] || [ "$$os" = windows ] && tags="-tags with_gvisor"; \
		echo ">> build $$label$$([ -n "$$tags" ] && echo " [with_gvisor]" || echo "")"; \
		GOOS=$$os GOARCH=$$arch go build $$tags -ldflags "$(LDFLAGS)" \
			-o $(BIN_DIR)/$(APP)-$$label$$ext ./cmd/app/ || exit 1; \
	done
	@echo "done -> $(BIN_DIR)/"

release: build-all
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		ext=; [ "$$os" = windows ] && ext=.exe; \
		case "$$os/$$arch" in \
			darwin/arm64)  label=macos-apple-silicon-arm64 ;; \
			darwin/amd64)  label=macos-intel-x86_64 ;; \
			linux/amd64)   label=linux-x86_64 ;; \
			linux/arm64)   label=linux-arm64 ;; \
			windows/amd64) label=windows-x86_64 ;; \
			*)             label=$$os-$$arch ;; \
		esac; \
		name=$(APP)-$(VERSION)-$$label; \
		tmp=$$(mktemp -d); \
		cp $(BIN_DIR)/$(APP)-$$label$$ext $$tmp/$(APP)$$ext; \
		cp $(GEODATA_DIR)/geoip.metadb $(GEODATA_DIR)/geosite.dat $$tmp/; \
		if [ "$$os" = darwin ] && command -v codesign >/dev/null 2>&1; then \
			codesign -s - --force --timestamp=none $$tmp/$(APP)$$ext >/dev/null 2>&1 || true; \
		fi; \
		if [ "$$os" = windows ]; then \
			(cd $$tmp && zip -q -r $(CURDIR)/$(DIST_DIR)/$$name.zip $(APP)$$ext geoip.metadb geosite.dat); \
		else \
			tar -czf $(DIST_DIR)/$$name.tar.gz -C $$tmp $(APP)$$ext geoip.metadb geosite.dat; \
		fi; \
		rm -rf $$tmp; \
	done
	@cd $(DIST_DIR) && (command -v sha256sum >/dev/null && sha256sum $(APP)-* > SHA256SUMS || shasum -a 256 $(APP)-* > SHA256SUMS)
	@echo "done -> $(DIST_DIR)/"

run: build
ifeq ($(GOOS),windows)
	@set TUN_TUI_DATA_DIR=./data && $(NATIVE_BIN)
else
	sudo TUN_TUI_DATA_DIR=./data $(NATIVE_BIN)
endif

PREFIX   ?= /usr/local
DESTDIR  ?=

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(NATIVE_BIN) $(DESTDIR)$(PREFIX)/bin/$(APP)$(BIN_EXT)

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

help:
	@echo "本机编译:   make build          ($(GOOS)/$(GOARCH))"
	@echo "全平台编译: make build-all      (darwin/linux/windows)"
	@echo "下载 geo:   make fetch-geodata  (geoip + geosite)"
	@echo "发布打包:   make release        (压缩包 -> dist/)"
	@echo "开发运行:   make run            (需管理员权限)"
	@echo "源码安装:   make install        (默认 /usr/local/bin)"
	@echo "用户安装:   curl -fsSL .../scripts/install.sh | sh"
	@echo ""
	@echo "覆盖平台:   make build GOOS=linux GOARCH=amd64"
	@echo "指定版本:   make release VERSION=0.2.0"
	@echo ""
	@echo "产物:"
	@echo "  本机: bin/$(APP)$(BIN_EXT)"
	@echo "  全平台: bin/$(APP)-<platform>[.exe]"
	@echo "  发布包: dist/$(APP)-<version>-<platform>.tar.gz|.zip"
	@echo "          例: $(APP)-$(VERSION)-macos-apple-silicon-arm64.tar.gz"
