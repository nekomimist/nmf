APP := nmf
APP_NAME := NMF
APP_ID := io.github.nekomimist.nmf
DIST := dist
WINDOWS_CC := x86_64-w64-mingw32-gcc
WINDOWS_OBJCOPY := x86_64-w64-mingw32-objcopy
FYNE_TAGS := migrated_fynedo

.PHONY: build build-linux build-windows test test-all test-race test-windows-compile test-darwin-compile debug-env clean

build: build-linux

build-linux:
	mkdir -p $(DIST)
	go build -tags $(FYNE_TAGS) -o $(DIST)/$(APP) .

build-windows:
	mkdir -p $(DIST)
	CC="$(WINDOWS_CC)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	fyne package --target windows --icon nmf-icon.png --app-id $(APP_ID) --name $(APP_NAME) --release
	mv $(APP_NAME).exe $(DIST)/$(APP).exe
	$(WINDOWS_OBJCOPY) --subsystem windows:6.0 $(DIST)/$(APP).exe

test:
	go test -tags $(FYNE_TAGS) ./internal/...

test-all:
	go test -tags $(FYNE_TAGS) ./...

test-race:
	go test -race -tags $(FYNE_TAGS) ./...

test-windows-compile:
	CC="$(WINDOWS_CC)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	go test -tags $(FYNE_TAGS) -exec=true ./...

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
