# Build and test entry points. CI calls these targets, so a local run and a CI
# run cannot drift apart.

GO     ?= go
BIN    ?= bin/stemma

# The example wiki: the project's own documentation, written in the format the
# tool defines and checked by the tool itself.
WIKI   ?= wiki

.PHONY: all build test vet check lint-wiki tidy fmt clean

all: build

build:
	$(GO) build -o $(BIN) ./cmd/stemma

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

# What CI runs.
check: build vet test lint-wiki

# The documentation is a KB, so it has to lint clean under --strict. If the
# project's own documentation cannot pass its own checks, the release is not
# shippable. Raising the findings is not enough: this has to fail the build.
lint-wiki: build
	$(BIN) lint --kb $(WIKI) --strict

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

clean:
	rm -rf bin
