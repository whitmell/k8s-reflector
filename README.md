# cluster-reflector

A Kubernetes service that provides real-time cluster metadata and application version information through HTTP endpoints.

## Overview

cluster-reflector watches your Kubernetes cluster and serves information about:

- **Cluster Nodes**: Names, IPs, roles (control-plane/worker), and Kubernetes versions
- **Application Versions**: Discovered from custom AppVersion CRDs or workload metadata

## Features

- 🔍 **Multi-source Discovery**: Prefers custom AppVersion CRDs, falls back to workload metadata
- 🚀 **High Performance**: In-memory caching with configurable TTL
- 🔒 **Security First**: Runs as non-root with read-only filesystem
- 📊 **Observability**: Prometheus metrics, structured logging, health checks
- ⚙️ **Configurable**: Extensive configuration options via CLI flags
- 🏷️ **Smart Role Detection**: Automatic control-plane vs worker node classification

## API Endpoints

### GET /cluster-info

Returns cluster metadata in JSON format:

```json
{
  "apiVersion": "reflector.grid.sce.com/v1",
  "timestamp": "2024-01-15T10:30:00Z",
  "nodes": [
    {
      "name": "node-1",
      "ip": "10.0.1.100", 
      "role": "control-plane",
      "version": "v1.28.4"
    }
  ],
  "apps": [
    {
      "name": "my-app",
      "version": "1.0.0",
      "variants": ["1.0.0", "0.9.0"]
    }
  ]
}
```

### GET /healthz

Health check endpoint returning 200 OK when service is healthy.

### GET /metrics

Prometheus metrics endpoint (when enabled with `--metrics`).

## Quick Start

### Using Helm (Recommended)

```bash
# Install from the Helm chart
helm install cluster-reflector ./helm/cluster-reflector \
  --namespace cluster-reflector \
  --create-namespace

# Access the service
kubectl port-forward -n cluster-reflector svc/cluster-reflector 8080:80
curl http://localhost:8080/cluster-info
```

### Using Docker

```bash
# Build the image
make docker-build

# Run locally (requires kubeconfig)
docker run --rm -p 8080:8080 \
  -v ~/.kube/config:/kubeconfig:ro \
  -e KUBECONFIG=/kubeconfig \
  ghcr.io/yourorg/cluster-reflector:latest
```

### From Source

```bash
# Build and run
make build
./cluster-reflector --log-level=debug

# With custom configuration
./cluster-reflector \
  --listen=:8080 \
  --cache-ttl=30s \
  --namespace-selector="production,staging" \
  --prefer-crd=true \
  --fallback-workloads=true \
  --log-level=info
```

## Configuration

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--listen` | `:8080` | Address to listen on |
| `--cache-ttl` | `10s` | Cache TTL for cluster data |
| `--namespace-selector` | `""` | Namespace selector (empty = all) |
| `--prefer-crd` | `true` | Prefer AppVersion CRDs |
| `--fallback-workloads` | `true` | Enable workload discovery |
| `--log-level` | `info` | Log level (debug/info/warn/error) |
| `--workload-kinds` | `Deployment,StatefulSet` | Workload types to discover |
| `--metrics` | `false` | Enable metrics endpoint |

### Environment Variables

All flags can be set via environment variables by prefixing with `CLUSTER_REFLECTOR_` and converting to uppercase:

```bash
export CLUSTER_REFLECTOR_LOG_LEVEL=debug
export CLUSTER_REFLECTOR_CACHE_TTL=30s
```

## Application Discovery

### Method 1: AppVersion CRDs (Preferred)

Create custom resources to explicitly declare application versions:

```yaml
apiVersion: cluster.grid.sce.com/v1alpha1
kind: AppVersion
metadata:
  name: my-app
  namespace: production
spec:
  name: my-app
  version: "2.1.0"
```

### Method 2: Workload Labels (Fallback)

Use standard Kubernetes labels on your workloads:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app.kubernetes.io/name: my-app
    app.kubernetes.io/version: "1.0.0"
spec:
  # ... deployment spec
```

### Method 3: Image Tag Parsing (Last Resort)

If no labels are found, the service will parse the first container's image tag:

```yaml
spec:
  containers:
  - name: app
    image: my-app:v1.0.0  # Parsed as name="my-app", version="v1.0.0"
```

## Development

### Prerequisites

- Go 1.21+
- Docker
- Kubernetes cluster (for testing)
- Helm 3.2+ (for chart development)

### Building

```bash
# Install dependencies
make deps

# Run tests
make test

# Build binary
make build

# Build Docker image
make docker-build

# Run linting
make lint
```

### Development Workflow

```bash
# Install development tools
make dev-tools

# Run with live reload (requires air)
make dev

# Template Helm chart
make helm-template

# Install locally
make helm-install
```

### Testing

```bash
# Unit tests
make test

# Integration test with Helm
make helm-install
kubectl port-forward -n cluster-reflector svc/cluster-reflector 8080:80
curl http://localhost:8080/healthz
curl http://localhost:8080/cluster-info | jq

# Cleanup
make helm-uninstall
```

## Production Deployment

### RBAC Requirements

The service needs these Kubernetes permissions:

- **Cluster-wide**: `get`, `list`, `watch` on `nodes`
- **Cluster-wide**: `get`, `list`, `watch` on `appversions.cluster.grid.sce.com`
- **Apps API**: `get`, `list`, `watch` on `deployments`, `statefulsets` (if workload discovery enabled)

### Security Considerations

- Runs as non-root user (UID 10001)
- Read-only root filesystem
- All capabilities dropped
- No privilege escalation
- Minimal base image (scratch)

### Monitoring

Enable Prometheus metrics and ServiceMonitor:

```yaml
# values.yaml
serviceMonitor:
  enabled: true
  interval: 30s

metrics: true
```

Key metrics:
- `cluster_reflector_nodes_total`: Total nodes
- `cluster_reflector_apps_total`: Total applications  
- `cluster_reflector_control_plane_nodes`: Control plane nodes
- `cluster_reflector_worker_nodes`: Worker nodes

### Resource Requirements

Default resource requests/limits:

```yaml
resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi
```

## Troubleshooting

### Common Issues

1. **RBAC Permissions**
   ```bash
   kubectl auth can-i get nodes --as=system:serviceaccount:cluster-reflector:cluster-reflector
   ```

2. **CRD Not Found**
   ```bash
   kubectl get crd appversions.cluster.grid.sce.com
   ```

3. **Cache Issues**
   - Check logs for cache refresh errors
   - Verify `--cache-ttl` setting
   - Ensure cluster connectivity

### Debug Mode

Run with debug logging:

```bash
./cluster-reflector --log-level=debug
```

Or in Helm:

```yaml
logLevel: debug
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Add tests: `make test`
5. Run linting: `make lint`
6. Commit changes: `git commit -m 'Add amazing feature'`
7. Push to branch: `git push origin feature/amazing-feature`
8. Open a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
