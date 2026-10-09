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

.PHONY: all build install install-skills test test-shim vet check lint-wiki site extract mcp-schema check-mcp mcp-probe skills check-skills man check-man install-man bench tidy fmt clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/stemma
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN)-mcp ./cmd/stemma-mcp

# Put the binary on PATH with the same version stamp the build uses. `go
# install` writes to $(go env GOBIN), or to $(go env GOPATH)/bin when GOBIN is
# unset. For a system location instead, address the built binary directly:
#   make build && install -m 0755 bin/stemma /usr/local/bin/stemma
install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/stemma
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/stemma-mcp

# Install the skill suite into the user's agent-skills directory, so every
# harness on this machine can load it. The umbrella's references are generated
# first, because they are not committed. Override the destination with
# SKILLS_DIR, for example `make install-skills SKILLS_DIR=~/.config/skills`.
# Each skill replaces the copy already there, so a stale reference file cannot
# survive an update.
install-skills: skills
	@mkdir -p "$(SKILLS_DIR)"
	@for d in $(SKILLS)/*/; do \
		name=$$(basename "$$d"); \
		rm -r "$(SKILLS_DIR)/$$name"; \
		cp -R "$$d" "$(SKILLS_DIR)/$$name"; \
		echo "installed $$name to $(SKILLS_DIR)"; \
	done

# The suite runs with STEMMA_KB unset: tests build their own KBs, and a KB
# exported in the caller's shell would otherwise answer the discovery some of
# them exercise. Running `go test ./...` by hand needs the same guard.
test:
	env -u STEMMA_KB $(GO) test ./...

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
check: build vet test test-shim lint-wiki site extract mcp-schema check-mcp check-skills check-man

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

# The curated MCP surface, written by the mcp package's generator from the
# registry the server will serve: each tool with its input schema and result
# shape, the envelope every result uses, and the commands deliberately left on
# the CLI. The MCP documentation renders its tool reference from this file, so
# a schema and its description cannot drift. Generated, so not committed (P8).
mcp-schema:
	$(GO) run ./internal/mcp/gen -out docs/mcp-tools.json

# The MCP server answers on the wire the way its tests say it does. This
# holds a two-turn conversation with the shipped binary — a discover, then a
# tools/list — and fails when the answers stop matching. The protocol tests
# and the parity work live in internal/mcp; this gate proves the binary a
# harness would launch is the one that can hold the conversation.
check-mcp: build
	@printf '%s\n' \
		'{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}' \
		| $(BIN)-mcp 2>/dev/null | grep -q supportedVersions
	@printf '%s\n' \
		'{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}' \
		| $(BIN)-mcp 2>/dev/null | grep -q '"name":"status"'
	@echo "mcp server held the conversation"

# The probe drives the shipped binary the way a harness does — one message at
# a time over stdio, a full turn of the curated set against its own scratch
# KB — without speaking for any real client. It is the repeatable half of the
# two-harness exercise the MCP plan asks for; the other half, running the
# server from a real client, stays out of CI. Run this before touching the
# surface.
mcp-probe: build
	$(GO) run ./internal/mcp/probe -bin $(BIN)-mcp

# The agent skill suite: one directory per skill, following the open Agent Skills
# standard. The umbrella skill's two references are generated, not committed
# (P8): the CLI reference comes from the tool's own command table, and the format
# specification is a copy of FORMAT.md, which stays the canonical, self-contained
# specification. Run this before copying skills/ into a harness, because a fresh
# clone has neither.
SKILLS ?= skills

# Where `make install-skills` puts the suite. The open Agent Skills standard does
# not fix a location; this is the per-user one the author's harnesses read.
SKILLS_DIR ?= $(HOME)/.agents/skills

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

# The manual pages: one roff page per command, generated by the binary from the
# command table, so a verb that is added to the table gets a page and `man
# stemma-lint` exists without anyone writing it. The set is a generated
# derivative, so it is not committed (P8); install it with install-man.
MAN       ?= man
MANPREFIX ?= /usr/local

man: build
	@$(BIN) help --man --out $(MAN) >/dev/null
	@echo "wrote $$(ls $(MAN)/*.1 | wc -l | tr -d ' ') manual pages to $(MAN)"

# A page that is empty, or written by something other than the shipped binary,
# is worse than no page at all, so the set is regenerated and read back. That
# every command has a page is asserted by the tests `test` runs, in
# internal/cli; this target proves the binary can write what it claims.
check-man: man
	test -s $(MAN)/stemma.1
	test -s $(MAN)/stemma-lint.1
	test -s $(MAN)/stemma-cite-add.1

# Put the pages where `man` looks for them. Section 1 is user commands, and the
# prefix is the usual one, overridable the way SKILLS_DIR is.
install-man: man
	@mkdir -p "$(MANPREFIX)/share/man/man1"
	install -m 0644 $(MAN)/*.1 "$(MANPREFIX)/share/man/man1"
	@echo "installed $$(ls $(MAN)/*.1 | wc -l | tr -d ' ') manual pages to $(MANPREFIX)/share/man/man1"

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
	rm -rf bin $(MAN)
