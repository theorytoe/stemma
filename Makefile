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

.PHONY: all build install test test-shim vet check lint-wiki site extract skills check-skills bench tidy fmt clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/stemma

# Put the binary on PATH with the same version stamp the build uses. `go
# install` writes to $(go env GOBIN), or to $(go env GOPATH)/bin when GOBIN is
# unset. For a system location instead, address the built binary directly:
#   make build && install -m 0755 bin/stemma /usr/local/bin/stemma
install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/stemma

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
check: build vet test test-shim lint-wiki site extract check-skills

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

# A scoped extract is a KB root, so it has to lint clean and build like one. This
# is the definition of done end to end: take the entry document out of the
# example wiki, then check what came out as a KB in its own right. The
# exhaustive half -- every extract of every page, at both depths, plus the round
# trip -- is internal/export's gate test, which `test` runs.
EXTRACT = .stemma/extract/gate

# The agent skill suite: one directory per skill, following the open Agent Skills
# standard. The umbrella skill's two references are generated, not committed
# (P8): the CLI reference comes from the tool's own command table, and the format
# specification is a copy of FORMAT.md until docs.md moves it into the wiki. Run
# this before copying skills/ into a harness, because a fresh clone has neither.
SKILLS ?= skills

skills: build
	@mkdir -p $(SKILLS)/stemma/references
	$(BIN) help --markdown > $(SKILLS)/stemma/references/cli.md
	cp FORMAT.md $(SKILLS)/stemma/references/format.md

# The format rules are a gate; an agent's behaviour is not. This checks every
# SKILL.md against the standard: name, description, and body limits. The
# non-deterministic half of the delegate -- running the skills against a real KB
# and reading where they mislead an agent -- is recorded in the delegate, not
# asserted here.
check-skills: skills
	$(GO) test ./internal/skills/
	test -s $(SKILLS)/stemma/references/cli.md
	test -s $(SKILLS)/stemma/references/format.md

extract: build
	$(BIN) export page --kb $(WIKI) pages/index.md --depth all --out $(EXTRACT)
	$(BIN) lint --kb $(WIKI)/$(EXTRACT) --strict
	$(BIN) build --kb $(WIKI)/$(EXTRACT) --out site

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
