# Build and test entry points. CI calls these targets, so a local run and a CI
# run cannot drift apart.

GO     ?= go
BIN    ?= bin/stemma

.PHONY: all build test vet check tidy fmt clean

all: build

build:
	$(GO) build -o $(BIN) ./cmd/stemma

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

# What CI runs (foundation Task 1). The four gates from delegates/docs.md are
# added to this target, not to the workflow.
check: build vet test

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

clean:
	rm -rf bin
