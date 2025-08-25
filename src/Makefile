# Makefile for cluster-reflector

# Build variables
BINARY_NAME = cluster-reflector
DOCKER_IMAGE = docker.shorebreak.dev/cluster-reflector
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "v0.1.0")
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Go build flags
LDFLAGS = -X main.Version=$(VERSION) \
          -X main.GitCommit=$(GIT_COMMIT) \
          -X main.BuildDate=$(BUILD_DATE) \
          -w -s

# Default target
.PHONY: all
all: test build

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	docker rmi $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest 2>/dev/null || true

# Download dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Run tests
.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run linting
.PHONY: lint
lint:
	golangci-lint run

# Build binary
.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./app/cmd/cluster-reflector

# Build for Linux (useful for cross-compilation)
.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-linux ./app/cmd/cluster-reflector

# Build Docker image
.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

# Push Docker image
.PHONY: docker-push
docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

# Run locally (requires kubeconfig)
.PHONY: run
run: build
	./$(BINARY_NAME) --log-level=debug

# Generate go.sum
.PHONY: go-sum
go-sum:
	go mod tidy

# Install development tools
.PHONY: dev-tools
dev-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run development server with live reload (requires air)
.PHONY: dev
dev:
	air

# Helm template test
.PHONY: helm-template
helm-template:
	helm template cluster-reflector ./helm/cluster-reflector --values ./helm/cluster-reflector/values.yaml

# Helm install
.PHONY: helm-install
helm-install: docker-build
	helm upgrade --install cluster-reflector ./helm/cluster-reflector \
		--namespace cluster-reflector \
		--create-namespace \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(VERSION)

# Helm uninstall
.PHONY: helm-uninstall
helm-uninstall:
	helm uninstall cluster-reflector --namespace cluster-reflector

# Build and push everything
.PHONY: release
release: clean test build docker-build docker-push

# Print version
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all           - Run tests and build binary"
	@echo "  build         - Build binary"
	@echo "  build-linux   - Build binary for Linux"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download dependencies"
	@echo "  dev           - Run development server with live reload"
	@echo "  dev-tools     - Install development tools"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-push   - Push Docker image"
	@echo "  go-sum        - Generate go.sum"
	@echo "  helm-install  - Install Helm chart"
	@echo "  helm-template - Template Helm chart"
	@echo "  helm-uninstall - Uninstall Helm chart"
	@echo "  lint          - Run linting"
	@echo "  release       - Build and push everything"
	@echo "  run           - Run locally"
	@echo "  test          - Run tests"
	@echo "  version       - Print version information"
