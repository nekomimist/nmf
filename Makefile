APP := nmf
APP_NAME := NMF
APP_ID := io.github.nekomimist.nmf
DIST := dist

.PHONY: build build-linux build-windows build-windows-tmp-cache test clean

build: build-linux

build-linux:
	mkdir -p $(DIST)
	go build -o $(DIST)/$(APP) .

build-windows:
	mkdir -p $(DIST)
	CC="zig cc -target x86_64-windows-gnu" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	fyne package --target windows --icon nmf-icon.png --app-id $(APP_ID) --name $(APP_NAME) --release
	mv $(APP_NAME).exe $(DIST)/$(APP).exe
	objcopy --subsystem windows:6.0 $(DIST)/$(APP).exe

build-windows-tmp-cache:
	GOCACHE=/tmp/nmf-go-build-cache \
	ZIG_LOCAL_CACHE_DIR=/tmp/zig-local-cache \
	ZIG_GLOBAL_CACHE_DIR=/tmp/zig-global-cache \
	$(MAKE) build-windows

test:
	go test ./internal/...

clean:
	rm -rf $(DIST)
