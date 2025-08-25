#!/bin/bash
# Build script for cluster-reflector

set -e

# Default values
TARGET="build"
VERSION="v0.1.0"
REGISTRY="ghcr.io/yourorg"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --target)
            TARGET="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --registry)
            REGISTRY="$2"
            shift 2
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option $1"
            show_help
            exit 1
            ;;
    esac
done

IMAGE_NAME="$REGISTRY/cluster-reflector"
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

function show_help() {
    echo "cluster-reflector build script"
    echo
    echo "Usage: $0 [--target <target>] [--version <version>] [--registry <registry>]"
    echo
    echo "Targets:"
    echo "  build      - Build Docker image (default)"
    echo "  test       - Run tests in Docker"
    echo "  push       - Push Docker image to registry"
    echo "  helm       - Install/upgrade Helm chart"
    echo "  all        - Build, test, and install"
    echo "  help       - Show this help"
    echo
    echo "Examples:"
    echo "  $0                                           # Build image"
    echo "  $0 --target test                             # Run tests"
    echo "  $0 --target all --version v0.2.0            # Full build and deploy"
    echo "  $0 --target push --registry myregistry       # Push to custom registry"
}

function show_status() {
    echo "🔧 Build Configuration:"
    echo "  Version:    $VERSION"
    echo "  Registry:   $REGISTRY"
    echo "  Image:      $IMAGE_NAME"
    echo "  Git Commit: $GIT_COMMIT"
    echo "  Build Date: $BUILD_DATE"
    echo
}

function build_docker() {
    echo "🏗️  Building Docker image..."
    docker build -t "$IMAGE_NAME:$VERSION" -t "$IMAGE_NAME:latest" .
    echo "✅ Docker image built successfully"
    echo "📦 Images: $IMAGE_NAME:$VERSION, $IMAGE_NAME:latest"
}

function test_docker() {
    echo "🧪 Running tests in Docker..."
    docker run --rm -v "$(pwd):/workspace" -w /workspace golang:1.21-alpine go test -v ./...
    echo "✅ Tests passed"
}

function push_docker() {
    echo "📤 Pushing Docker image..."
    docker push "$IMAGE_NAME:$VERSION"
    docker push "$IMAGE_NAME:latest"
    echo "✅ Docker image pushed successfully"
}

function install_helm() {
    echo "⚓ Installing/Upgrading Helm chart..."
    helm upgrade --install cluster-reflector ./helm/cluster-reflector \
        --namespace cluster-reflector \
        --create-namespace \
        --set image.repository="$IMAGE_NAME" \
        --set image.tag="$VERSION"
    
    echo "✅ Helm chart installed successfully"
    echo
    echo "🚀 Access the service:"
    echo "  kubectl port-forward -n cluster-reflector svc/cluster-reflector 8080:80"
    echo "  curl http://localhost:8080/cluster-info"
}

# Main execution
show_status

case $TARGET in
    build)
        build_docker
        ;;
    test)
        test_docker
        ;;
    push)
        push_docker
        ;;
    helm)
        install_helm
        ;;
    all)
        build_docker
        test_docker
        install_helm
        ;;
    help)
        show_help
        ;;
    *)
        echo "❌ Unknown target: $TARGET"
        show_help
        exit 1
        ;;
esac

echo "🎉 Operation completed successfully!"
