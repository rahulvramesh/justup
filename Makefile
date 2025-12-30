# Justup Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_NAME=justup
BINARY_DIR=bin

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X github.com/rahulvramesh/justup/internal/cli.Version=$(VERSION) \
	-X github.com/rahulvramesh/justup/internal/cli.GitCommit=$(GIT_COMMIT) \
	-X github.com/rahulvramesh/justup/internal/cli.BuildDate=$(BUILD_DATE)"

# Docker
DOCKER_REGISTRY ?= justup
DOCKER_TAG ?= latest

.PHONY: all build clean test deps docker-build docker-push install help

all: deps build

## Build

build: ## Build the CLI binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/justup

build-sshproxy: ## Build the SSH proxy binary
	@echo "Building SSH proxy..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/sshproxy ./cmd/sshproxy

build-all-binaries: build build-sshproxy ## Build all binaries

build-linux: ## Build for Linux (amd64)
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/justup

build-darwin: ## Build for macOS (amd64 and arm64)
	@echo "Building $(BINARY_NAME) for macOS..."
	@mkdir -p $(BINARY_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/justup
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/justup

build-all: build-linux build-darwin ## Build for all platforms

## Dependencies

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## Testing

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Docker

docker-build-devcontainer: ## Build dev container image
	@echo "Building devcontainer image..."
	docker build -t $(DOCKER_REGISTRY)/devcontainer:$(DOCKER_TAG) ./docker/devcontainer

docker-build-sshproxy: ## Build SSH proxy image
	@echo "Building sshproxy image..."
	docker build -t $(DOCKER_REGISTRY)/sshproxy:$(DOCKER_TAG) -f docker/sshproxy/Dockerfile .

docker-push-devcontainer: ## Push dev container image
	@echo "Pushing devcontainer image..."
	docker push $(DOCKER_REGISTRY)/devcontainer:$(DOCKER_TAG)

docker-push-sshproxy: ## Push SSH proxy image
	@echo "Pushing sshproxy image..."
	docker push $(DOCKER_REGISTRY)/sshproxy:$(DOCKER_TAG)

docker-build: docker-build-devcontainer docker-build-sshproxy ## Build all Docker images

docker-push: docker-push-devcontainer docker-push-sshproxy ## Push all Docker images

## Installation

install: build ## Install justup to /usr/local/bin
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BINARY_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Done! Run 'justup version' to verify."

install-user: build ## Install justup to ~/.local/bin
	@echo "Installing $(BINARY_NAME) to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	cp $(BINARY_DIR)/$(BINARY_NAME) ~/.local/bin/$(BINARY_NAME)
	@echo "Done! Make sure ~/.local/bin is in your PATH."

## Kubernetes

k8s-setup: ## Apply Kubernetes manifests (namespaces + RBAC)
	@echo "Setting up Kubernetes resources..."
	kubectl apply -f deploy/namespace.yaml
	kubectl apply -f deploy/rbac.yaml
	@echo "Done! Namespaces and RBAC configured."

k8s-deploy-proxy: ## Deploy SSH proxy to Kubernetes
	@echo "Deploying SSH proxy..."
	kubectl apply -f deploy/sshproxy.yaml
	@echo "Done! SSH proxy deployed."
	@echo "Get the proxy address with: kubectl get svc -n justup-system justup-sshproxy"

k8s-deploy: k8s-setup k8s-deploy-proxy ## Full Kubernetes deployment

k8s-clean: ## Remove Kubernetes resources
	@echo "Removing Kubernetes resources..."
	kubectl delete namespace justup-workspaces --ignore-not-found
	kubectl delete namespace justup-system --ignore-not-found
	kubectl delete clusterrole justup-controller --ignore-not-found
	kubectl delete clusterrolebinding justup-controller --ignore-not-found

## Cleanup

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

## Help

help: ## Show this help
	@echo "Justup - Kubernetes Dev Environment CLI"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
