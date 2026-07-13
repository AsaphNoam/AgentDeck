# AgentDeck — convenience build targets.
#
# The Go server embeds the built UI via //go:embed at internal/server/ui/dist.
# Because go:embed paths are relative to the source file, the UI build output
# (repo-root ui/dist) is copied into internal/server/ui/dist before `go build`.

BINARY      := agentdeck
PKG         := github.com/agentdeck/agentdeck
VERSION_PKG := $(PKG)/internal/version

VERSION ?= 0.1.0
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) \
           -X $(VERSION_PKG).Commit=$(COMMIT) \
           -X $(VERSION_PKG).Date=$(DATE)

EMBED_DIR := internal/server/ui/dist

# Build tags. sqlite_fts5 compiles FTS5 into the SQLite driver; the archive
# search path (MATCH/snippet/bm25) requires it, so every shipped binary must
# carry it. Untagged builds degrade to a plain sessions_fts table on which
# MATCH errors at runtime — fine for the no-tag test checkpoint, not for release.
TAGS := sqlite_fts5

.PHONY: all build ui embed dist run check-specs test vet clean

all: build

## ui: build the React/Vite UI into ui/dist
ui:
	cd ui && npm ci && npm run build

## embed: copy the built UI into the Go embed location
embed: ui
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	cp -R ui/dist/. $(EMBED_DIR)/

## build: build the agentdeck binary (assumes embed dir is populated)
build:
	go build -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/agentdeck

## dist: full release build — UI + embed + binary
dist: embed build

## run: start the dashboard in the foreground
run: build
	./bin/$(BINARY) dashboard start

## check-specs: validate the authoritative specification set
check-specs:
	@scripts/check-specs.sh

## test: lint specs, then run the Go suite (no-tag fallback + sqlite_fts5 FTS)
test: check-specs
	go test ./...
	go test -tags sqlite_fts5 ./...

## vet: static analysis
vet:
	go vet ./...

## clean: remove build artifacts
clean:
	rm -rf bin ui/dist
	git checkout -- $(EMBED_DIR)/index.html 2>/dev/null || true
