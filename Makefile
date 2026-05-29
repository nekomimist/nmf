APP := nmf
APP_NAME := NMF
APP_ID := io.github.nekomimist.nmf
DIST := dist
WINDOWS_CC := x86_64-w64-mingw32-gcc
WINDOWS_OBJCOPY := x86_64-w64-mingw32-objcopy

.PHONY: build build-linux build-windows test test-windows-compile debug-env clean

build: build-linux

build-linux:
	mkdir -p $(DIST)
	go build -o $(DIST)/$(APP) .

build-windows:
	mkdir -p $(DIST)
	CC="$(WINDOWS_CC)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	fyne package --target windows --icon nmf-icon.png --app-id $(APP_ID) --name $(APP_NAME) --release
	mv $(APP_NAME).exe $(DIST)/$(APP).exe
	$(WINDOWS_OBJCOPY) --subsystem windows:6.0 $(DIST)/$(APP).exe

test:
	go test ./internal/...

test-windows-compile:
	CC="$(WINDOWS_CC)" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	go test -exec=true ./internal/...

# Prints the effective environment passed through Codex/project config.
debug-env:
	env || true

clean:
	rm -rf $(DIST)
