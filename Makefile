APP := nmf
DIST := dist

.PHONY: build build-linux build-windows test clean

build: build-linux

build-linux:
	mkdir -p $(DIST)
	go build -o $(DIST)/$(APP) .

build-windows:
	mkdir -p $(DIST)
	CC="zig cc -target x86_64-windows-gnu" \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
	go build -o $(DIST)/$(APP).exe .

test:
	go test ./internal/...

clean:
	rm -rf $(DIST)
