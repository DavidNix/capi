default: help

.PHONY: help
help: ## Print this help message
	@echo "Available make commands:"; grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: fmt
fmt: ## Run formatters
	go tool goimports -w ./googleads ./internal ./cmd

.PHONY: vet
vet: ## Run Go vet and linters
	golangci-lint run ./...

.PHONY: test
test: ## Run Go unit tests
	go test -mod=readonly -race -cover -short -timeout=60s ./...
