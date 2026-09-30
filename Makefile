GO      ?= go
BIN     ?= bin/gv
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE  := gitverse.ru/veryviolet/gv
LDFLAGS := -s -w \
	-X $(MODULE)/internal/build.Version=$(VERSION) \
	-X $(MODULE)/internal/build.Date=$(DATE)

# Package version: Debian and RPM both reject a leading "v".
PKGVERSION := $(patsubst v%,%,$(VERSION))

PLATFORMS  := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
PKGARCHES  := amd64 arm64
PACKAGERS  := deb rpm apk

.PHONY: all build install test lint fmt vet generate docs site clean cross packages tarballs checksums dist-clean release publish mirror tidy

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

# docs regenerates the man pages and completion scripts that the packages ship.
docs:
	rm -rf share
	$(GO) run ./internal/gen/docs -out share -version $(VERSION)
	gzip -9 -n share/man/man1/*.1

# site regenerates the static documentation published to GitVerse Pages.
site:
	rm -rf site
	$(GO) run ./internal/gen/docs -site site -version $(VERSION)
	@ls -1 site

cross:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "building dist/gv_$${os}_$${arch}$$ext"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/gv_$${os}_$${arch}$$ext ./cmd/gv || exit 1; \
	done

# packages builds deb, rpm and apk for every packaged architecture. nfpm writes
# the archives itself, so neither dpkg-dev nor rpmbuild is needed.
packages: cross docs
	@command -v nfpm >/dev/null || { echo "nfpm not found: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest"; exit 1; }
	@mkdir -p dist
	@for arch in $(PKGARCHES); do \
		cp dist/gv_linux_$$arch dist/gv; \
		for pkg in $(PACKAGERS); do \
			echo "packaging $$pkg/$$arch"; \
			ARCH=$$arch VERSION=$(PKGVERSION) nfpm package -p $$pkg -f nfpm.yaml -t dist/ || exit 1; \
		done; \
	done
	@rm -f dist/gv

# tarballs bundles the binary with its man pages, completions and licence.
tarballs: cross docs
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		stage=$$(mktemp -d); \
		root=$$stage/gv_$(PKGVERSION)_$${os}_$${arch}; \
		mkdir -p $$root/bin $$root/share; \
		cp dist/gv_$${os}_$${arch}$$ext $$root/bin/gv$$ext; \
		cp -r share/man share/completions $$root/share/; \
		cp LICENSE README.md $$root/; \
		if [ "$$os" = "windows" ]; then \
			(cd $$stage && zip -qr - gv_$(PKGVERSION)_$${os}_$${arch}) > dist/gv_$(PKGVERSION)_$${os}_$${arch}.zip; \
		else \
			tar -czf dist/gv_$(PKGVERSION)_$${os}_$${arch}.tar.gz -C $$stage gv_$(PKGVERSION)_$${os}_$${arch}; \
		fi; \
		rm -rf $$stage; \
		echo "packed dist/gv_$(PKGVERSION)_$${os}_$${arch}"; \
	done

# checksums covers the distributable archives only; the bare cross-compiled
# binaries stay in dist/ as build intermediates.
checksums:
	@cd dist && rm -f SHA256SUMS && \
		sha256sum *.deb *.rpm *.apk *.tar.gz *.zip > SHA256SUMS 2>/dev/null && \
		echo "wrote dist/SHA256SUMS ($$(wc -l < SHA256SUMS) files)"

# publish stages the artefacts in the form GitVerse accepts as release
# attachments. The API sniffs the content of an upload and takes archives only:
# .apk is already gzip and goes up untouched, while .deb and .rpm have to be
# gzipped, and a plain-text SHA256SUMS is rejected outright.
publish: release
	@rm -rf dist/publish && mkdir -p dist/publish
	@cp dist/*.apk dist/*.tar.gz dist/*.zip dist/publish/
	@for f in dist/*.deb dist/*.rpm; do gzip -9 -c "$$f" > dist/publish/$$(basename $$f).gz; done
	@cp dist/SHA256SUMS dist/publish/SHA256SUMS && gzip -9 -f dist/publish/SHA256SUMS
	@ls -1 dist/publish

# release builds everything that gets attached to a GitVerse release.
release: dist-clean packages tarballs checksums
	@ls -1 dist

# mirror keeps the GitHub copy in step with GitVerse. GitVerse stays the
# origin; the mirror exists so the project is findable where people search.
mirror:
	git push origin main --tags
	git push github main --tags
	git push origin pages
	git push github pages

dist-clean:
	rm -rf dist

clean: dist-clean
	rm -rf bin share site
