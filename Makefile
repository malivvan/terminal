# Makefile for github.com/malivvan/terminal
GO       ?= go
GOFLAGS  ?=
PKG      ?= ./...
GOTESTSUM ?= $(shell which gotestsum 2>/dev/null || echo "/home/malivvan/go/bin/gotestsum")

# Detect gotestsum presence; fall back to plain go test if missing
ifneq ($(shell command -v $(GOTESTSUM) 2>/dev/null),)
  TEST_RUNNER   ?= $(GOTESTSUM)
  TEST_FLAGS    ?= --format testdox --format-icons hivis --
else
  TEST_RUNNER   ?= $(GO) test
  TEST_FLAGS    ?=
endif

COVERAGE_OUT ?= coverage.out

.PHONY: all build vet lint test test-race test-verbose cover cover-html bench fuzz fmt tidy demo clean help

all: build

build: ## Compile all packages
	$(GO) build $(GOFLAGS) $(PKG)

vet: ## Run go vet
	$(GO) vet $(GOFLAGS) $(PKG)

lint: ## Run linters (go vet + gofmt)
	$(GO) vet $(GOFLAGS) $(PKG)
	@unformatted=$$(gofmt -s -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

test: ## Run unit tests (with gotestsum if available)
	$(TEST_RUNNER) $(TEST_FLAGS) $(GOFLAGS) $(PKG)

test-race: ## Run unit tests with race detector
	$(TEST_RUNNER) $(TEST_FLAGS) -race $(GOFLAGS) $(PKG)

test-verbose: ## Run unit tests with verbose output
	$(TEST_RUNNER) $(TEST_FLAGS) -v $(GOFLAGS) $(PKG)

cover: ## Run tests with coverage report
	$(TEST_RUNNER) $(TEST_FLAGS) -covermode=atomic -coverprofile=$(COVERAGE_OUT) $(GOFLAGS) $(PKG)
	$(GO) tool cover -func=$(COVERAGE_OUT)

cover-html: cover ## Open the HTML coverage report
	$(GO) tool cover -html=$(COVERAGE_OUT)

bench: ## Run benchmarks
	$(GO) test -run=^$$ -bench=. -benchmem $(GOFLAGS) $(PKG)

fuzz: ## Run parser fuzz tests briefly
	$(GO) test -run=^$$ -fuzz=. -fuzztime=30s ./...

fmt: ## Format Go sources
	$(GO) fmt $(PKG)
	gofmt -s -w .

tidy: ## Tidy go.mod
	$(GO) mod tidy

# Mirrored by the "Build demos" step of .github/workflows/ci.yml, which drives
# `go` directly: the Windows CI runner image has no make.
demo: ## Build every demo that applies to this platform
	@for d in $$($(GO) list -f '{{.ImportPath}}' ./demos/...); do \
		echo "==> building $$d"; \
		$(GO) build -o /dev/null $$d || exit 1; \
	done
	@echo "==> building github.com/malivvan/terminal/demos/31_js_wasm_pty_bridge (GOOS=js GOARCH=wasm)"
	@GOOS=js GOARCH=wasm $(GO) build -o /dev/null ./demos/31_js_wasm_pty_bridge
	@echo "==> building github.com/malivvan/terminal/demos/30_windows_conpty_shell (GOOS=windows)"
	@GOOS=windows $(GO) build -o /dev/null ./demos/30_windows_conpty_shell

clean: ## Remove build artifacts
	rm -f $(COVERAGE_OUT)
	$(GO) clean -cache -testcache

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
