# Makefile

BINARY = par2cron
SRC_DIR = ./cmd/par2cron

VERSION := $(shell \
  tag=$$(git describe --tags --exact-match 2>/dev/null); \
  if [ -n "$$tag" ]; then echo $$tag | sed 's/^v//'; \
  else git rev-parse --short=7 HEAD; fi)

A2X = a2x
A2X_FLAGS = -a version=$(VERSION)
DOCS_DIR = ./docs
MAN_DIR = $(DOCS_DIR)/man
MAN_ADOC = $(MAN_DIR)/par2cron.adoc
COMPLETIONS_DIR = $(DOCS_DIR)/completions

.PHONY: all $(BINARY) $(BINARY)-embed benchmark check check-slop clean debug docs docs-clean docs-man docs-markdown docs-pdf docs-text docs-completions generate help info is-clean lint test test-fuzz-quick test-fuzz-long test-coverage vendor

all: vendor $(BINARY) ## Runs the entire build chain for the application

$(BINARY): ## Builds the application
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go build -ldflags="-w -s -X github.com/desertwitch/par2cron/internal/schema.ProgramVersion=$(VERSION) -buildid=" -trimpath -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

$(BINARY)-embed: ## Builds the application (with embedded "par2")
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go build -tags embed_par2 -ldflags="-w -s -X github.com/desertwitch/par2cron/internal/schema.ProgramVersion=$(VERSION)-EMB -buildid=" -trimpath -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

benchmark: ## Runs the benchmark suite
	go test -bench=. -benchmem ./internal/bundle/
	go test -bench=. -benchmem ./internal/par2/
	go test -bench=. -benchmem ./internal/create/
	go test -bench=. -benchmem ./internal/verify/
	go test -bench=. -benchmem ./internal/repair/

check: ## Runs all static analysis and tests on the application code
	@$(MAKE) check-slop
	@$(MAKE) lint
	@$(MAKE) test

check-slop: ## Checks relevant text files for punctuation used by AI
	@grep -RInP \
		--exclude-dir=vendor \
		--include='*.go' \
		--include='*.txt' \
		--include='*.yaml' \
		--include='*.yml' \
		--include='*.md' \
		'[\x{2013}\x{2014}\x{2018}\x{2019}\x{201C}\x{201D}\x{2026}\x{00A0}]' . ; \
	rc=$$?; \
	if [ $$rc -eq 0 ]; then \
		exit 1; \
	elif [ $$rc -eq 1 ]; then \
		exit 0; \
	else \
		exit $$rc; \
	fi

clean: ## Returns the application build stage to its original state (deleting files)
	@$(MAKE) docs-clean
	@rm -vfr dist || true
	@rm -vf $(BINARY) || true

debug: ## Builds the application in debug mode (with symbols, race checks, ...)
	CGO_ENABLED=1 GOFLAGS="-mod=vendor" go build -ldflags="-X github.com/desertwitch/par2cron/internal/schema.ProgramVersion=$(VERSION)-DBG" -trimpath -race -o $(BINARY) $(SRC_DIR)
	@$(MAKE) info

docs: ## Builds all documentation (manpages, markdown, PDF, plain text)
	@$(MAKE) docs-man
	@$(MAKE) docs-pdf
	@$(MAKE) docs-text
	@$(MAKE) docs-completions
	@$(MAKE) docs-markdown

docs-clean: ## Removes generated documentation files
	@rm -vf $(MAN_DIR)/*.pdf $(MAN_DIR)/*.text $(MAN_DIR)/*.1 $(MAN_DIR)/*.8 $(MAN_DIR)/*.xml || true
	@rm -vf $(COMPLETIONS_DIR)/* || true

docs-completions: ## Generates shell completion scripts
	@mkdir -p $(COMPLETIONS_DIR)
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) completion bash > $(COMPLETIONS_DIR)/par2cron.bash
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) completion zsh > $(COMPLETIONS_DIR)/par2cron.zsh
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) completion fish > $(COMPLETIONS_DIR)/par2cron.fish

docs-man: ## Builds manpage documentation
	$(A2X) $(A2X_FLAGS) -f manpage $(MAN_ADOC)

docs-markdown: ## Generates the markdown documentation
	CGO_ENABLED=0 GOFLAGS="-mod=vendor" go run $(SRC_DIR) gen-markdown $(DOCS_DIR)

docs-pdf: ## Builds PDF documentation
	$(A2X) $(A2X_FLAGS) -f pdf $(MAN_ADOC)

docs-text: ## Builds plain text documentation
	$(A2X) $(A2X_FLAGS) -f text $(MAN_ADOC)

generate: ## Re-generate the static files that are used in tests
	@go generate ./...

help: ## Shows all build related commands of the Makefile
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

info: ## Shows information about the application binaries that were built
	@ldd $(BINARY) || true
	@file $(BINARY)
	@./$(BINARY) --version

is-clean: ## Checks if the git tree is clean (e.g. before release)
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "ERROR: tree is not clean - commit the changes"; \
		git status --short; \
		exit 1; \
	fi

lint: ## Runs the linter on the application code
	@golangci-lint cache clean
	@golangci-lint run

test: ## Runs all written tests for and on the application code
	@go test -failfast -race -covermode=atomic ./...

test-fuzz-quick: ## Runs fuzz-related unit tests followed by 3min of fuzzing
	go test -failfast ./internal/par2 ./internal/bundle
	./scripts/golang-fuzz.sh Fuzz_Parse ./internal/par2 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Open ./internal/bundle 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Scan ./internal/bundle 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Pack ./internal/bundle 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Manifest ./internal/bundle 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Unpack ./internal/bundle 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Update ./internal/bundle 3m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Validate ./internal/bundle 3m

test-fuzz-long: ## Runs fuzz-related unit tests followed by 60min of fuzzing
	go test -failfast ./internal/par2 ./internal/bundle
	./scripts/golang-fuzz.sh Fuzz_Parse ./internal/par2 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Open ./internal/bundle 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Scan ./internal/bundle 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Pack ./internal/bundle 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Manifest ./internal/bundle 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Unpack ./internal/bundle 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Update ./internal/bundle 60m
	./scripts/golang-fuzz.sh Fuzz_Bundle_Validate ./internal/bundle 60m

test-coverage: ## Runs all coverage tests for and on the application code
	@go test -failfast -race -covermode=atomic -coverpkg=./... -coverprofile=coverage.tmp ./... && \
	grep -v "mock_" coverage.tmp > coverage.txt && \
	rm coverage.tmp

vendor: ## Pulls the (remote) dependencies into the local vendor folder
	@go mod tidy
	@go mod vendor
