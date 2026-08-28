GO      ?= go
BIN     ?= bin/gv
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE  := gitverse.ru/veryviolet/gv
LDFLAGS := -s -w \
	-X $(MODULE)/internal/build.Version=$(VERSION) \
	-X $(MODULE)/internal/build.Date=$(DATE)

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build install test lint fmt vet generate clean cross tidy

all: build

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/gv

install:
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' ./cmd/gv

test:
	$(GO) test ./...

lint: vet
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "run 'make fmt'" && exit 1)

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

generate:
	$(GO) run ./internal/gen -spec spec/openapi-1.8.json -out internal/api/types_gen.go

tidy:
	$(GO) mod tidy

cross:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "building dist/gv_$${os}_$${arch}$$ext"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/gv_$${os}_$${arch}$$ext ./cmd/gv || exit 1; \
	done

clean:
	rm -rf bin dist
