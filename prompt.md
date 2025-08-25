# Prompt: Build a production-ready Helm chart for a “cluster-reflector” service (with CRDs)

You are a senior Kubernetes/Helm engineer. Create a **complete Helm chart** that deploys a small HTTP service which reports cluster metadata and app versions at `GET /cluster-info` and `GET /healthz`. The chart must include **all CRDs**, RBAC, and best-practice defaults. Produce everything necessary to `helm install` in a clean cluster.

---

## Functional Overview (context for the chart)

The app (container image will be supplied via `values.yaml`) watches:

* **Nodes**: expose per-node `{name, ip=InternalIP, role, version=kubeletVersion}`.
* **App versions**:

  1. Prefer **custom CRD** `AppVersion` (`cluster.grid.sce.com/v1alpha1`) with `spec.name` and `spec.version`.
  2. Fallback: derive from workloads (Deployments/StatefulSets) using `app.kubernetes.io/name` and `app.kubernetes.io/version`; if absent, parse first container image tag.

The pod maintains an in-memory snapshot and serves JSON:

```json
{
  "apiVersion": "reflector.grid.sce.com/v1",
  "timestamp": "RFC3339",
  "nodes": [ { "name": "...", "ip": "...", "role": "control-plane|worker", "version": "v1.xx.y" } ],
  "apps": [ { "name": "app1", "version": "0.1", "variants": ["0.1","0.2"] } ]
}
```

Health endpoint: `GET /healthz` returns 200 when caches are synced.

---

## Deliverables

1. **Chart layout** (generate files):

```
cluster-reflector/
  Chart.yaml
  values.yaml
  values.schema.json
  README.md
  templates/
    _helpers.tpl
    serviceaccount.yaml
    clusterrole.yaml
    clusterrolebinding.yaml
    role.yaml
    rolebinding.yaml
    deployment.yaml
    service.yaml
    ingress.yaml
    configmap-env.yaml
    networkpolicy.yaml
    pdb.yaml
    hpa.yaml
    servicemonitor.yaml       # gated by values for Prometheus Operator
    notes.txt
  crds/
    appversions.cluster.grid.sce.com.yaml
  tests/
    test-release-connection.yaml  # helm test hook
```

2. **CRD**: `crds/appversions.cluster.grid.sce.com.yaml`

* Group: `cluster.grid.sce.com`
* Kind: `AppVersion`
* Version: `v1alpha1`
* Scope: Namespaced
* Schema:

  * `spec.name: string` (required)
  * `spec.version: string` (required)
  * `status.observedAt: string(date-time)` (optional)
* Add `preserveUnknownFields: false` and structural schema.
* Include printer columns for `spec.version` and `age`.
* Use `apiextensions.k8s.io/v1`.

3. **RBAC**

* **ClusterRole** (list/watch/get):

  * `nodes` (core)
  * `deployments`, `statefulsets` (apps)
  * CRD: `appversions.cluster.grid.sce.com` (list/watch/get)
* **Role** (namespaced) only if namespaced discovery is enabled via values (e.g., for leader election configmaps or optional artifacts).
* Bindings for the ServiceAccount.
* Make rules **additive** based on values toggles (e.g., disable workload introspection if `values.appDiscovery.enabled=false`).

4. **Deployment**

* Single container; image and tag from values.
* Args/Env from values:

  * `--listen=:8080`
  * `--cache-ttl={{ .Values.cache.ttl }}`
  * `--namespace-selector={{ .Values.appDiscovery.namespaceSelector }}` (empty = all)
  * `--prefer-crd={{ .Values.appDiscovery.preferCRD }}`
  * `--fallback-workloads={{ .Values.appDiscovery.fallbackWorkloads }}`
  * `--log-level={{ .Values.logLevel }}`
* Probes:

  * Readiness/Liveness: HTTP `/healthz`.
* Security:

  * `runAsNonRoot: true`
  * `runAsUser: 10001`
  * `readOnlyRootFilesystem: true`
  * Drop all capabilities.
* Resources with sane defaults; allow override.
* Pod anti-affinity (soft), topology spread (soft).
* Optional PodDisruptionBudget and HPA.

5. **Service**

* ClusterIP on port 80 → targetPort 8080.
* Labels/annotations templated.
* Optional Ingress (v1) with TLS; supports NGINX and generic class via values.

6. **NetworkPolicy** (optional via values)

* Allow ingress to the pod from namespace selectors/labels passed in values.
* Allow egress to Kubernetes API if your policy model requires it (document assumption).

7. **Observability**

* `ServiceMonitor` (disabled by default) with scrape path `/metrics` if the app exposes it later; gate behind `.Values.serviceMonitor.enabled`.
* Annotations for scraping may also be toggled.

8. **Values**

* Provide a **comprehensive** `values.yaml` with comments and secure defaults:

  * `image.repository`, `image.tag`, `image.pullPolicy`
  * `serviceAccount.create`, `serviceAccount.name`
  * `rbac.create`
  * `service` (type, port)
  * `ingress` (enabled, className, hosts, TLS)
  * `resources`, `nodeSelector`, `tolerations`, `affinity`, `topologySpreadConstraints`
  * `pdb.enabled`, `hpa.enabled` (CPU/memory targets)
  * `networkPolicy.enabled`, selectors
  * `cache.ttl: 10s`
  * `logLevel: info`
  * `appDiscovery`:

    * `enabled: true`
    * `preferCRD: true`
    * `fallbackWorkloads: true`
    * `namespaceSelector: ""` (comma-separated list or label selector string)
    * `workloadKinds: ["Deployment","StatefulSet"]`
  * `crds.install: true`
  * `serviceMonitor.enabled: false`
* Provide a **values.schema.json** enforcing types, enums, and basic constraints.

9. **Helpers**

* `_helpers.tpl`:

  * Standard name, fullname, chart, labels.
  * Common labels include `app.kubernetes.io/name`, `instance`, `version`, `managed-by`, `part-of`.
  * Template for selector labels.
  * Helper to render `commonAnnotations` and `commonLabels`.

10. **README.md**

* Clear install/upgrade/uninstall instructions.
* Example `helm install` with values.
* CRD management note (Helm installs from `crds/`).
* Security context rationale.
* How to **publish an AppVersion**:

  ```yaml
  apiVersion: cluster.grid.sce.com/v1alpha1
  kind: AppVersion
  metadata:
    name: app1
    namespace: apps
  spec:
    name: app1
    version: "0.1"
  ```
* How the fallback discovery works (labels and image tag parsing).
* Example curl against the Service or port-forward:

  ```
  kubectl -n <ns> port-forward svc/cluster-reflector 8080:80
  curl http://localhost:8080/cluster-info
  ```

11. **Helm tests**

* A simple `helm.sh/hook: test` Pod that `wget -qO- http://cluster-reflector:80/healthz` and asserts 200.

12. **NOTES.txt**

* Print Service DNS, port-forward snippet, and a sample `curl` to `/cluster-info`.

---

## Implementation Requirements

* **Compatibility**: Kubernetes v1.25+ (use networking.k8s.io/v1 Ingress, policy/v1 PDB).
* **Conditional rendering**: every optional component guarded by values.
* **Idempotent CRDs**: placed under `crds/` so `helm install/upgrade` handles them; do not template names/versions in CRDs.
* **RBAC least privilege**: split rules—if `.Values.appDiscovery.enabled=false`, omit workload rules.
* **Role detection hint** (doc only): control-plane if node has label `node-role.kubernetes.io/control-plane` or taints matching `node-role.kubernetes.io/(control-plane|master):NoSchedule`; otherwise `worker`.

---

## Concrete content to generate

* Populate **all files** listed in the tree with working templates.
* Provide a **realistic default** `values.yaml` (example image like `ghcr.io/yourorg/cluster-reflector:0.1.0`).
* Provide **values.schema.json** validating the shape (booleans, strings, enums for service type, ingress booleans, arrays for workloadKinds).
* Ensure `helm template` on an empty cluster yields valid YAML (no missing required fields).
* Ensure probes, securityContext, resources, and labels render correctly.

---

## Example Values (include in `values.yaml`)

```yaml
image:
  repository: ghcr.io/yourorg/cluster-reflector
  tag: "0.1.0"
  pullPolicy: IfNotPresent

serviceAccount:
  create: true
  name: ""

rbac:
  create: true

service:
  type: ClusterIP
  port: 80
  annotations: {}

ingress:
  enabled: false
  className: ""
  annotations: {}
  hosts:
    - host: reflector.example.com
      paths:
        - path: /
          pathType: Prefix
  tls: [] # - secretName, hosts

resources:
  requests: { cpu: "50m", memory: "64Mi" }
  limits:   { cpu: "200m", memory: "256Mi" }

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 10001
  fsGroup: 10001

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities: { drop: ["ALL"] }

cache:
  ttl: 10s

logLevel: info

appDiscovery:
  enabled: true
  preferCRD: true
  fallbackWorkloads: true
  namespaceSelector: ""        # "", label selector, or comma-separated namespaces
  workloadKinds:
    - Deployment
    - StatefulSet

crds:
  install: true

pdb:
  enabled: true
  minAvailable: 1

hpa:
  enabled: false
  minReplicas: 1
  maxReplicas: 3
  targetCPUUtilizationPercentage: 70

networkPolicy:
  enabled: false
  allowedNamespaces: []        # label selectors or ns names
  allowKubeAPI: false

serviceMonitor:
  enabled: false
  interval: 30s
  scrapeTimeout: 10s
```

---

## Output format

* Provide the **entire chart** as a code block hierarchy with file headers (e.g., `# templates/deployment.yaml`) and their contents.
* Ensure the chart can be copied to disk and installed with:

  ```
  helm install cluster-reflector ./cluster-reflector -n cluster-reflector --create-namespace
  ```
* Do not include placeholder TODOs—fill in working, validated Helm templates.
