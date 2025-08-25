# cluster-reflector

A Helm chart for deploying cluster-reflector, a service that reports Kubernetes cluster metadata and application versions.

## Overview

cluster-reflector is a lightweight HTTP service that provides real-time information about your Kubernetes cluster, including:

- **Node Information**: Names, IPs, roles, and Kubernetes versions
- **Application Versions**: Either from custom `AppVersion` CRDs or derived from workload metadata

The service exposes two main endpoints:
- `GET /cluster-info` - Returns cluster metadata and application versions in JSON format
- `GET /healthz` - Health check endpoint

## Prerequisites

- Kubernetes 1.25+
- Helm 3.2.0+

## Installation

### Quick Start

```bash
# Add the Helm repository (if published)
helm repo add yourorg https://charts.yourorg.com
helm repo update

# Install the chart
helm install cluster-reflector yourorg/cluster-reflector \
  --namespace cluster-reflector \
  --create-namespace
```

### From Source

```bash
# Clone the repository
git clone https://github.com/yourorg/cluster-reflector
cd cluster-reflector

# Install the chart
helm install cluster-reflector ./helm/cluster-reflector \
  --namespace cluster-reflector \
  --create-namespace
```

### With Custom Values

```bash
# Create a values file
cat > my-values.yaml <<EOF
image:
  repository: ghcr.io/yourorg/cluster-reflector
  tag: "0.2.0"

ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: reflector.example.com
      paths:
        - path: /
          pathType: Prefix

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
EOF

# Install with custom values
helm install cluster-reflector ./cluster-reflector \
  --namespace cluster-reflector \
  --create-namespace \
  --values my-values.yaml
```

## Configuration

### Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `image.repository` | string | `"ghcr.io/yourorg/cluster-reflector"` | Container image repository |
| `image.tag` | string | `"0.1.0"` | Container image tag |
| `image.pullPolicy` | string | `"IfNotPresent"` | Image pull policy |
| `replicaCount` | int | `1` | Number of replicas |
| `serviceAccount.create` | bool | `true` | Create service account |
| `serviceAccount.name` | string | `""` | Service account name (auto-generated if empty) |
| `rbac.create` | bool | `true` | Create RBAC resources |
| `service.type` | string | `"ClusterIP"` | Service type |
| `service.port` | int | `80` | Service port |
| `ingress.enabled` | bool | `false` | Enable ingress |
| `ingress.className` | string | `""` | Ingress class name |
| `resources.limits.cpu` | string | `"200m"` | CPU limit |
| `resources.limits.memory` | string | `"256Mi"` | Memory limit |
| `resources.requests.cpu` | string | `"50m"` | CPU request |
| `resources.requests.memory` | string | `"64Mi"` | Memory request |
| `cache.ttl` | string | `"10s"` | Cache TTL for cluster data |
| `logLevel` | string | `"info"` | Log level (debug, info, warn, error) |
| `appDiscovery.enabled` | bool | `true` | Enable application discovery |
| `appDiscovery.preferCRD` | bool | `true` | Prefer AppVersion CRDs over workload discovery |
| `appDiscovery.fallbackWorkloads` | bool | `true` | Enable workload fallback discovery |
| `appDiscovery.crdOnly` | bool | `false` | Only discover from AppVersion CRDs, ignore workloads |
| `appDiscovery.namespaceSelector` | string | `""` | Namespace selector for discovery |
| `appDiscovery.workloadKinds` | list | `["Deployment","StatefulSet"]` | Workload types to discover |
| `pdb.enabled` | bool | `true` | Enable PodDisruptionBudget |
| `hpa.enabled` | bool | `false` | Enable HorizontalPodAutoscaler |
| `networkPolicy.enabled` | bool | `false` | Enable NetworkPolicy |
| `serviceMonitor.enabled` | bool | `false` | Enable ServiceMonitor for Prometheus |

### AppVersion Custom Resource

The chart installs a custom resource definition for `AppVersion`:

```yaml
apiVersion: cluster.grid.sce.com/v1alpha1
kind: AppVersion
metadata:
  name: my-app
  namespace: default
spec:
  name: my-app
  version: "1.0.0"
```

### Security Context

The deployment runs with a strict security context:

- `runAsNonRoot: true`
- `runAsUser: 10001`
- `readOnlyRootFilesystem: true`
- All capabilities dropped
- No privilege escalation

This ensures the container follows security best practices.

## Usage

### Accessing the Service

#### Port Forward

```bash
kubectl -n cluster-reflector port-forward svc/cluster-reflector 8080:80
```

#### Test Endpoints

```bash
# Health check
curl http://localhost:8080/healthz

# Cluster information
curl http://localhost:8080/cluster-info | jq
```

### Example Response

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
    },
    {
      "name": "node-2",
      "ip": "10.0.1.101",
      "role": "worker",
      "version": "v1.28.4"
    }
  ],
  "apps": [
    {
      "name": "my-app",
      "version": "1.0.0",
      "variants": ["1.0.0"]
    },
    {
      "name": "another-app",
      "version": "2.1.0",
      "variants": ["2.1.0", "2.0.0"]
    }
  ]
}
```

### Publishing Application Versions

#### Using AppVersion CRD (Preferred)

```yaml
apiVersion: cluster.grid.sce.com/v1alpha1
kind: AppVersion
metadata:
  name: my-application
  namespace: production
spec:
  name: my-application
  version: "2.1.3"
```

#### Using Workload Labels (Fallback)

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

If labels are missing, the service will parse the first container image tag.

### CRD-Only Mode

For environments where you want to strictly control application version discovery through custom CRDs only, you can enable CRD-only mode:

```yaml
appDiscovery:
  crdOnly: true
  preferCRD: true
  fallbackWorkloads: false
```

When `crdOnly: true` is set:
- Only `AppVersion` CRDs will be discovered
- Workload-based discovery (Deployments, StatefulSets) is completely disabled
- The service will only report applications that have explicit `AppVersion` CRDs
- This ensures consistent, controlled application version management

**Note**: CRD-only mode requires `preferCRD: true` to be set.

### Role Detection

Node roles are determined by:
- **Control Plane**: Nodes with label `node-role.kubernetes.io/control-plane` or taints `node-role.kubernetes.io/(control-plane|master):NoSchedule`
- **Worker**: All other nodes

## Monitoring

### Prometheus Integration

Enable ServiceMonitor for Prometheus Operator:

```yaml
serviceMonitor:
  enabled: true
  interval: 30s
  scrapeTimeout: 10s
```

### Health Checks

The service includes liveness and readiness probes on `/healthz`:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: http
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /healthz
    port: http
  initialDelaySeconds: 5
  periodSeconds: 5
```

## Management

### Upgrading

```bash
helm upgrade cluster-reflector ./cluster-reflector \
  --namespace cluster-reflector \
  --values my-values.yaml
```

### Uninstalling

```bash
helm uninstall cluster-reflector --namespace cluster-reflector
```

**Note**: Custom Resource Definitions (CRDs) are not automatically removed. To remove them:

```bash
kubectl delete crd appversions.cluster.grid.sce.com
```

### Testing

Run Helm tests to verify the installation:

```bash
helm test cluster-reflector --namespace cluster-reflector
```

## CRD Management

The chart includes the `AppVersion` CRD in the `crds/` directory. Helm will:

- Install CRDs before installing the chart
- **Not** upgrade CRDs during chart upgrades
- **Not** delete CRDs when uninstalling the chart

To manually manage CRDs:

```bash
# Apply CRD updates manually
kubectl apply -f crds/appversions.cluster.grid.sce.com.yaml

# Delete CRDs manually
kubectl delete -f crds/appversions.cluster.grid.sce.com.yaml
```

## Troubleshooting

### Common Issues

1. **RBAC Permissions**: Ensure the service account has proper cluster permissions
2. **Image Pull**: Verify the image repository and tag are correct
3. **Network Policies**: Check if network policies are blocking communication
4. **Resource Limits**: Ensure sufficient CPU/memory resources are allocated

### Debugging

```bash
# Check pod status
kubectl -n cluster-reflector get pods

# View logs
kubectl -n cluster-reflector logs -l app.kubernetes.io/name=cluster-reflector

# Check service endpoints
kubectl -n cluster-reflector get endpoints

# Verify RBAC
kubectl auth can-i get nodes --as=system:serviceaccount:cluster-reflector:cluster-reflector
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test with `helm template` and `helm lint`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
