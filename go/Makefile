MODULE     := github.com/monang404/luna-go
BINARY     := luna
CMD        := ./cmd/luna
DIST       := dist
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -X main.version=$(VERSION)

.PHONY: build build-termux test lint vet clean

## build: compile for the host platform (dev machine)
build:
	mkdir -p $(DIST)
	go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) $(CMD)

## build-termux: cross-compile a static linux/arm64 binary for Termux
build-termux:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64 $(CMD)

## test: run the full test suite
test:
	go test ./...

## vet: run go vet across the module
vet:
	go vet ./...

## lint: vet + gofmt check (fails if any file is not gofmt-clean)
lint: vet
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## clean: remove build artifacts
clean:
	rm -rf $(DIST)
