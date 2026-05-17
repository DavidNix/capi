default: help

.PHONY: help
help: ## Print this help message
	@echo "Available make commands:"; grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: check-clean
check-clean: ## Check if git state is clean
	@git diff-index --quiet HEAD -- || { echo "Error: Git is dirty."; exit 1; }

DEFAULT_BRANCH ?= main
FEAT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
.PHONY: pr
pr: check-clean ## Mimic a local PR from a branch into main
	@if [ "$(FEAT_BRANCH)" = "$(DEFAULT_BRANCH)" ]; then \
		echo "Error: You are on the $(DEFAULT_BRANCH) branch."; \
		exit 1; \
	fi
	@git checkout $(DEFAULT_BRANCH)
	@git merge --squash $(FEAT_BRANCH)

.PHONY: fmt
fmt: ## Run formatters
	go tool goimports -w ./googleads ./internal ./cmd

.PHONY: vet
vet: ## Run Go vet and linters
	golangci-lint run ./...

.PHONY: test
test: ## Run Go unit tests
	go test -mod=readonly -race -cover -short -timeout=60s ./...
