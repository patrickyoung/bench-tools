.DEFAULT_GOAL := help

GO ?= go
VERSION ?= 0.1.0-dev

.PHONY: help build test check integration clean

help:
	@echo 'make build    Build the standalone Hire UI into bin/hire-ui'
	@echo 'make test     Run offline HTTP and process tests'
	@echo 'make integration Run selected real public tools (see README)'
	@echo 'make check    Run tests, race tests and vet'
	@echo 'make clean    Remove this application build output'

build:
	@mkdir -p bin
	GOWORK=off $(GO) build -trimpath -ldflags '-X main.version=$(VERSION)' -o bin/hire-ui .

test:
	GOWORK=off $(GO) test ./...

integration:
	@test -n "$(BENCH_UI_INTEGRATION_SOURCE)" -a -n "$(BENCH_UI_INTEGRATION_BIN)" || (echo 'Select BENCH_UI_INTEGRATION_SOURCE and BENCH_UI_INTEGRATION_BIN; see README.' >&2; exit 2)
	GOWORK=off $(GO) test -count=1 -run '^TestPublicExecutableIntegration$$' -v .

check:
	GOWORK=off $(GO) test ./...
	GOWORK=off $(GO) test -race ./...
	GOWORK=off $(GO) vet ./...

clean:
	rm -f bin/hire-ui
