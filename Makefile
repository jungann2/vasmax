VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X main.version=$(VERSION)
BINARY := VasmaX

.PHONY: build build-linux-amd64 build-linux-arm64 clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/vasmax/

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY)_linux_amd64 ./cmd/vasmax/

build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY)_linux_arm64 ./cmd/vasmax/

clean:
	rm -f $(BINARY) $(BINARY)_linux_*
