APP := nmf
APP_NAME := NMF
APP_ID := io.github.nekomimist.nmf
DIST := dist
WINDOWS_ZIG ?= zig
WINDOWS_OBJCOPY ?= llvm-objcopy
WINDOWS_CC_FLAGS := -Wdeprecated-non-prototype -Wl,--subsystem,windows
FYNE_TAGS := migrated_fynedo

.PHONY: build build-linux build-windows build-windows-arm64 test test-all test-race test-windows-compile test-windows-compile-arm64 test-darwin-compile debug-env clean
.NOTPARALLEL: build-windows build-windows-arm64

build: build-linux

build-linux:
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
	$(call build-windows-target,x86_64,amd64,$(APP))

build-windows-arm64:
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
	$(call test-windows-target,x86_64,amd64)

test-windows-compile-arm64:
	$(call test-windows-target,aarch64,arm64)

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
