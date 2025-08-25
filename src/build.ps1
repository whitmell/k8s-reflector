# PowerShell build script for cluster-reflector

param(
    [string]$Target = "build",
    [string]$Version = "v0.1.0",
    [string]$Registry = "ghcr.io/yourorg"
)

$ImageName = "$Registry/cluster-reflector"
$GitCommit = try { git rev-parse --short HEAD 2>$null } catch { "unknown" }
$BuildDate = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"

function Build-Docker {
    Write-Host "Building Docker image..." -ForegroundColor Green
    docker build -t "${ImageName}:${Version}" -t "${ImageName}:latest" .
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Docker image built successfully" -ForegroundColor Green
        Write-Host "Images: ${ImageName}:${Version}, ${ImageName}:latest" -ForegroundColor Cyan
    } else {
        Write-Host "❌ Docker build failed" -ForegroundColor Red
        exit 1
    }
}

function Test-Docker {
    Write-Host "Running tests in Docker..." -ForegroundColor Green
    docker run --rm -v "${PWD}:/workspace" -w /workspace golang:1.21-alpine go test -v ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Tests passed" -ForegroundColor Green
    } else {
        Write-Host "❌ Tests failed" -ForegroundColor Red
        exit 1
    }
}

function Push-Docker {
    Write-Host "Pushing Docker image..." -ForegroundColor Green
    docker push "${ImageName}:${Version}"
    docker push "${ImageName}:latest"
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Docker image pushed successfully" -ForegroundColor Green
    } else {
        Write-Host "❌ Docker push failed" -ForegroundColor Red
        exit 1
    }
}

function Install-Helm {
    Write-Host "Installing/Upgrading Helm chart..." -ForegroundColor Green
    helm upgrade --install cluster-reflector ./helm/cluster-reflector `
        --namespace cluster-reflector `
        --create-namespace `
        --set image.repository=$ImageName `
        --set image.tag=$Version
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Helm chart installed successfully" -ForegroundColor Green
        Write-Host "Access the service:" -ForegroundColor Cyan
        Write-Host "  kubectl port-forward -n cluster-reflector svc/cluster-reflector 8080:80" -ForegroundColor Yellow
        Write-Host "  curl http://localhost:8080/cluster-info" -ForegroundColor Yellow
    } else {
        Write-Host "❌ Helm install failed" -ForegroundColor Red
        exit 1
    }
}

function Show-Help {
    Write-Host "cluster-reflector build script" -ForegroundColor Green
    Write-Host ""
    Write-Host "Usage: .\build.ps1 -Target <target> [-Version <version>] [-Registry <registry>]" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Targets:" -ForegroundColor Yellow
    Write-Host "  build      - Build Docker image (default)"
    Write-Host "  test       - Run tests in Docker"
    Write-Host "  push       - Push Docker image to registry"
    Write-Host "  helm       - Install/upgrade Helm chart"
    Write-Host "  all        - Build, test, and install"
    Write-Host "  help       - Show this help"
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor Yellow
    Write-Host "  .\build.ps1                                    # Build image"
    Write-Host "  .\build.ps1 -Target test                       # Run tests"
    Write-Host "  .\build.ps1 -Target all -Version v0.2.0       # Full build and deploy"
    Write-Host "  .\build.ps1 -Target push -Registry myregistry # Push to custom registry"
}

function Show-Status {
    Write-Host "Build Configuration:" -ForegroundColor Green
    Write-Host "  Version:    $Version" -ForegroundColor Cyan
    Write-Host "  Registry:   $Registry" -ForegroundColor Cyan
    Write-Host "  Image:      $ImageName" -ForegroundColor Cyan
    Write-Host "  Git Commit: $GitCommit" -ForegroundColor Cyan
    Write-Host "  Build Date: $BuildDate" -ForegroundColor Cyan
    Write-Host ""
}

# Main execution
Show-Status

switch ($Target.ToLower()) {
    "build" { Build-Docker }
    "test" { Test-Docker }
    "push" { Push-Docker }
    "helm" { Install-Helm }
    "all" { 
        Build-Docker
        Test-Docker
        Install-Helm
    }
    "help" { Show-Help }
    default { 
        Write-Host "❌ Unknown target: $Target" -ForegroundColor Red
        Show-Help
        exit 1
    }
}

Write-Host "🎉 Operation completed successfully!" -ForegroundColor Green
