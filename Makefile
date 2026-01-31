# Makefile

BINARY = par2cron
SRC_DIR = ./cmd/par2cron

VERSION := $(shell \
  tag=$$(git describe --tags --exact-match 2>/dev/null); \
  if [ -n "$$tag" ]; then echo $$tag | sed 's/^v//'; \
  else git rev-parse --short=7 HEAD; fi)

.PHONY: all $(BINARY) benchmark check clean debug help info lint test test-fuzz-quick test-fuzz-long test-coverage vendor

all: vendor $(BINARY) ## Runs the entire build chain for the application

$(BINARY): ## Builds the application
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go build -ldflags="-w -s -X github.com/desertwitch/par2cron/internal/schema.ProgramVersion=$(VERSION) -buildid=" -trimpath -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

benchmark: ## Runs the benchmark script
	@$(MAKE) $(BINARY)
	@/usr/bin/env bash ./benchmark.sh

check: ## Runs all static analysis and tests on the application code
	@$(MAKE) lint
	@$(MAKE) test

clean: ## Returns the application build stage to its original state (deleting files)
	@rm -vf $(BINARY) || true

debug: ## Builds the application in debug mode (with symbols, race checks, ...)
	CGO_ENABLED=1 GOFLAGS="-mod=vendor" go build -ldflags="-X github.com/desertwitch/par2cron/internal/schema.ProgramVersion=$(VERSION)-DBG" -trimpath -race -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

help: ## Shows all build related commands of the Makefile
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

info: ## Shows information about the application binaries that were built
	@ldd $(BINARY) || true
	@file $(BINARY)

lint: ## Runs the linter on the application code
	@golangci-lint cache clean
	@golangci-lint run

test: ## Runs all written tests for and on the application code
	@go test -failfast -race -covermode=atomic ./...

test-fuzz-quick: ## Runs fuzz-related unit tests followed by 3min of fuzzing
	@go test -failfast ./internal/par2
	@go test -fuzz=FuzzParse -fuzztime=3m ./internal/par2

test-fuzz-long: ## Runs fuzz-related unit tests followed by 60min of fuzzing
	@go test -failfast ./internal/par2
	@go test -fuzz=FuzzParse -fuzztime=60m ./internal/par2

test-coverage: ## Runs all coverage tests for and on the application code
	@go test -failfast -race -covermode=atomic -coverpkg=./... -coverprofile=coverage.tmp ./... && \
	grep -v "mock_" coverage.tmp > coverage.txt && \
	rm coverage.tmp

vendor: ## Pulls the (remote) dependencies into the local vendor folder
	@go mod tidy
	@go mod vendor
