APP := nmf
APP_NAME := NMF
APP_ID := io.github.nekomimist.nmf
DIST := dist
WINDOWS_ZIG ?= zig
WINDOWS_OBJCOPY ?= llvm-objcopy
WINDOWS_CC_FLAGS := -Wdeprecated-non-prototype -Wl,--subsystem,windows
FYNE_TAGS := migrated_fynedo
NIX_DEVELOP ?= nix develop
WINDOWS_TEST_PACKAGES ?= ./...

.PHONY: build build-linux build-windows build-windows-in-nix build-windows-arm64 build-windows-arm64-in-nix test test-all test-race test-windows-compile test-windows-compile-in-nix test-windows-compile-arm64 test-windows-compile-arm64-in-nix test-darwin-compile debug-env clean
.NOTPARALLEL: build-windows build-windows-in-nix build-windows-arm64 build-windows-arm64-in-nix

build: build-linux

build-linux:
	@if [ "$${NMF_NIX_DEV_SHELL:-0}" = 1 ]; then \
		echo "build-linux must run outside the Nix development shell so WSLg can use the host GL/EGL libraries" >&2; \
		exit 1; \
	fi
	mkdir -p $(DIST)
	go build -tags $(FYNE_TAGS) -o $(DIST)/$(APP) .

define build-windows-target
	mkdir -p $(DIST)
	CC="$(WINDOWS_ZIG) cc -target $(1)-windows-gnu $(WINDOWS_CC_FLAGS)" \
	CXX="$(WINDOWS_ZIG) c++ -target $(1)-windows-gnu $(WINDOWS_CC_FLAGS)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=$(2) \
	fyne package --target windows --icon nmf-icon.png --app-id $(APP_ID) --name $(APP_NAME) --release
	mv $(APP_NAME).exe $(DIST)/$(3).exe
	$(WINDOWS_OBJCOPY) --subsystem windows:6.0 $(DIST)/$(3).exe
endef

build-windows:
	@if [ "$${NMF_NIX_DEV_SHELL:-0}" = 1 ]; then \
		$(MAKE) build-windows-in-nix; \
	else \
		$(NIX_DEVELOP) --command env NMF_NIX_DEV_SHELL=1 $(MAKE) build-windows-in-nix; \
	fi

build-windows-in-nix:
	$(call build-windows-target,x86_64,amd64,$(APP))

build-windows-arm64:
	@if [ "$${NMF_NIX_DEV_SHELL:-0}" = 1 ]; then \
		$(MAKE) build-windows-arm64-in-nix; \
	else \
		$(NIX_DEVELOP) --command env NMF_NIX_DEV_SHELL=1 $(MAKE) build-windows-arm64-in-nix; \
	fi

build-windows-arm64-in-nix:
	$(call build-windows-target,aarch64,arm64,$(APP)-arm64)

test:
	go test -tags $(FYNE_TAGS) ./internal/...

test-all:
	go test -tags $(FYNE_TAGS) ./...

test-race:
	go test -race -tags $(FYNE_TAGS) ./...

define test-windows-target
	CC="$(WINDOWS_ZIG) cc -target $(1)-windows-gnu $(WINDOWS_CC_FLAGS)" \
	CXX="$(WINDOWS_ZIG) c++ -target $(1)-windows-gnu $(WINDOWS_CC_FLAGS)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=$(2) \
	go test -tags $(FYNE_TAGS) -exec=true ./...
endef

test-windows-compile:
	@if [ "$${NMF_NIX_DEV_SHELL:-0}" = 1 ]; then \
		$(MAKE) test-windows-compile-in-nix; \
	else \
		$(NIX_DEVELOP) --command env NMF_NIX_DEV_SHELL=1 $(MAKE) test-windows-compile-in-nix; \
	fi

test-windows-compile-in-nix:
	$(call test-windows-target,x86_64,amd64)

test-windows-compile-arm64:
	@if [ "$${NMF_NIX_DEV_SHELL:-0}" = 1 ]; then \
		$(MAKE) test-windows-compile-arm64-in-nix; \
	else \
		$(NIX_DEVELOP) --command env NMF_NIX_DEV_SHELL=1 $(MAKE) test-windows-compile-arm64-in-nix; \
	fi

test-windows-compile-arm64-in-nix:
	$(call test-windows-target,aarch64,arm64)

# Build with the pinned Linux toolchain and execute on the WSL Windows host.
.PHONY: test-windows-wsl test-windows-wsl-in-nix
test-windows-wsl:
	@command -v powershell.exe >/dev/null && command -v wslpath >/dev/null || \
		{ echo "test-windows-wsl requires WSL Windows interop" >&2; exit 1; }
	@if [ "$${NMF_NIX_DEV_SHELL:-0}" = 1 ]; then \
		$(MAKE) test-windows-wsl-in-nix; \
	else \
		$(NIX_DEVELOP) --command env NMF_NIX_DEV_SHELL=1 $(MAKE) test-windows-wsl-in-nix; \
	fi

test-windows-wsl-in-nix:
	CC="$(WINDOWS_ZIG) cc -target x86_64-windows-gnu $(WINDOWS_CC_FLAGS)" \
	CXX="$(WINDOWS_ZIG) c++ -target x86_64-windows-gnu $(WINDOWS_CC_FLAGS)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	go test -tags $(FYNE_TAGS) -count=1 -v -exec 'bash $(CURDIR)/scripts/windows-test-exec.sh' $(WINDOWS_TEST_PACKAGES)

test-darwin-compile:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	go build -tags $(FYNE_TAGS) ./internal/fileinfo ./internal/jobs ./internal/watcher
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
	go build -tags $(FYNE_TAGS) ./internal/fileinfo ./internal/jobs ./internal/watcher

# Prints the effective environment passed through Codex/project config.
debug-env:
	env || true

clean:
	rm -rf $(DIST)
