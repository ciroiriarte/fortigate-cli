# fortigate-cli Makefile. The binary is `fgt`.
GO        ?= go
BINARY    := fgt
PKG       := github.com/ciroiriarte/fortigate-cli
CMD       := ./cmd/fgt
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.Date=$(DATE)

.PHONY: all build test vet fmt fmtcheck check tidy clean run docs

all: build

# Run the same gates as CI in one shot — use before committing/pushing.
check: fmtcheck vet test build

build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) $(CMD)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Fail if any non-vendored file isn't gofmt-clean (mirrors the CI gofmt gate).
fmtcheck:
	@unformatted=$$(gofmt -l . | grep -v '^vendor/' || true); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed (run 'make fmt'):"; echo "$$unformatted"; exit 1; \
	fi

tidy:
	$(GO) mod tidy

# Regenerate man pages (docs/man/) and shell completions (contrib/completions/)
# from the cobra command tree. Committed, so packagers need no Go toolchain.
docs:
	$(GO) run ./tools/gen-docs

clean:
	rm -f $(BINARY)
	rm -rf dist

run: build
	./$(BINARY) $(ARGS)
