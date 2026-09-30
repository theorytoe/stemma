# Build and test entry points. CI calls these targets, so a local run and a CI
# run cannot drift apart.

GO     ?= go
BIN    ?= bin/stemma

# The version the build reports and sends in its user agent. A release
# overrides it; a working tree gets the commit it was built from.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X github.com/theorytoe/stemma/internal/version.Version=$(VERSION)

# The example wiki: the project's own documentation, written in the format the
# tool defines and checked by the tool itself.
WIKI   ?= wiki

.PHONY: all build test test-shim vet check lint-wiki site bench tidy fmt clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/stemma

test:
	$(GO) test ./...

# The extraction script's own tests, run with plain python3: no framework, and
# skipped rather than failed when there is no interpreter, because the core has
# to build and test without Python.
test-shim:
	@if command -v python3 >/dev/null 2>&1; then \
		python3 internal/extract/extract_test.py; \
	else \
		echo "test-shim: no python3, skipped"; \
	fi

vet:
	$(GO) vet ./...

# What CI runs. Further gates (the static site, the export) belong in this
# target rather than in the workflow, so that a local run and a CI run check the
# same things.
check: build vet test test-shim lint-wiki site

# The documentation is a KB, so it has to lint clean under --strict. If the
# project's own documentation cannot pass its own checks, the release is not
# shippable. Raising the findings is not enough: this has to fail the build.
lint-wiki: build
	$(BIN) lint --kb $(WIKI) --strict

# The example wiki's static site, written by the binary CI just built. Parsing,
# rendering and writing are the end-to-end path, so this is part of check rather
# than something someone remembers to run. What the output must contain is
# asserted by internal/build's conformance test, which `test` runs; this target
# is here so the shipped binary has to produce the site the wiki describes.
site: build
	$(BIN) build --kb $(WIKI)

# The scale benchmark behind the tier finding in FORMAT.md. It measures, it
# does not pass or fail, so it is not part of check. One iteration per size
# keeps it a wall-clock measurement rather than a long average.
bench:
	$(GO) test ./internal/index/ -run '^$$' -bench BenchmarkTiers -benchtime 1x -benchmem

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

clean:
	rm -rf bin
