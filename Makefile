# Developer shortcuts. CI runs the same commands (.github/workflows/ci.yml);
# builds and packaging are documented in CONTRIBUTING.md and not wrapped here.
#
# Fyne needs cgo and a C compiler; see CONTRIBUTING.md.

PKGS ?= ./...
COVER_PROFILE ?= coverage.txt
COVER_HTML ?= dist/coverage.html

.DEFAULT_GOAL := help

.PHONY: help lint fmt test cover cover-html check

help: ## list the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

lint: ## golangci-lint run
	golangci-lint run $(PKGS)

fmt: ## fix formatting and imports
	golangci-lint fmt $(PKGS)

test: ## go test (PKGS=./ui/... to narrow, ARGS='-run Name' for go test flags)
	go test $(ARGS) $(PKGS)

cover: ## tests with coverage: writes coverage.txt, prints the total
	go test -coverprofile=$(COVER_PROFILE) $(ARGS) $(PKGS)
	@go tool cover -func=$(COVER_PROFILE) | tail -n 1

cover-html: cover ## coverage as an HTML report in dist/coverage.html
	@mkdir -p $(dir $(COVER_HTML))
	go tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)
	@echo "wrote $(COVER_HTML)"

check: lint test ## lint, then test
