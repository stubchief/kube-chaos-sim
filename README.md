# kube-chaos-sim

A Kubernetes chaos engineering dashboard: click buttons to run real chaos
actions (kill pod, inject latency, memory spike) against a real cluster,
watch the pod grid and HPA/PDB react in real time.

## Stack

- **Cluster:** kind (local, disposable)
- **Backend:** Go + client-go (Informers, in-memory cache)
- **Realtime:** SSE via Datastar (no WebSocket, no JSON API)
- **Frontend:** plain HTML + Datastar `data-*` attributes (no React, no npm)
- **Metrics:** synthetic generator in Go (no Prometheus required for demo)

## Quick start

### Prerequisites

Install tools:
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [helm](https://helm.sh/docs/intro/install/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Step 1: Create cluster and install infrastructure

```bash
# Create kind cluster, install metrics-server, deploy podinfo
./scripts/setup-cluster.sh
```

This script:
- Creates kind cluster with 3 emulated zones (ru-central1-a/b/d) and extraPortMappings (localhost:8080 → NodePort:30080)
- Installs metrics-server with 15s resolution (minimum supported, default 60s)
- Deploys podinfo as target microservice
- Applies accelerated HPA settings (sync-period=5s, downscale-stabilization=20s)

### Step 2: Build and deploy the backend

```bash
# Build Docker image, load into kind, deploy with Helm
./scripts/build-image.sh
```

This script:
- Builds backend Docker image (multi-stage build with Go binary + kubectl)
- Loads image into kind cluster (all nodes)
- Deploys backend with Helm (includes RBAC for client-go operations)
- Waits for deployment to be ready

Dashboard is available at http://localhost:8080 automatically (via kind's `extraPortMappings` + NodePort Service). No `kubectl port-forward` needed.

### Iterative development

After making changes to the backend code, rebuild and redeploy:

```bash
./scripts/build-image.sh
```

This will rebuild the image, load it into kind, and redeploy the backend.

### Alternative: run backend locally (for development)

```bash
go build -o server ./cmd/server/
./server
```

This uses your local kubeconfig (~/.kube/config) instead of in-cluster ServiceAccount.
Access at http://localhost:8080 (port-forward not needed — the binary listens on :8080).


## Project structure

```
cmd/server/       — entrypoint
internal/k8s/     — client-go setup, Informers, in-memory cache
internal/chaos/   — Chaos Controller (kill pod, inject latency, etc.)
internal/metrics/ — synthetic metrics generator
internal/sse/     — SSE broadcaster
web/              — static HTML + Datastar attributes
helm/             — Helm charts
  kube-chaos-sim/ — backend deployment (RBAC, ServiceAccount, Deployment, Service)
  podinfo-values.yaml — target microservice values
scripts/          — setup and build scripts
Dockerfile        — multi-stage build for backend
```

## RBAC permissions

The backend runs as a pod in the cluster and uses a dedicated ServiceAccount with explicit RBAC permissions for all client-go operations:

| API Group | Resource | Verbs | Why |
|---|---|---|---|
| `""` (core) | pods | get, list, watch, delete | informer + kill-pod |
| `""` (core) | pods/exec | create | kubectl exec (latency, memory, cpu) |
| `""` (core) | nodes | get, list, watch | informer (zone display) |
| `apps/v1` | deployments | get, list, watch, patch | rollout restart |
| `autoscaling/v2` | horizontalpodautoscalers | get, list, watch, create, update, patch | HPA management |
| `policy/v1` | poddisruptionbudgets | get, list, watch, create, update, patch | PDB management |
| `metrics.k8s.io` | pods | get, list | CPU metrics collection |

See `helm/kube-chaos-sim/templates/clusterrole.yaml` for the full RBAC configuration.


## What I learned

- Go and client-go from scratch
- Kubernetes Informers and in-memory caching patterns
- SSE-based realtime UI without a JavaScript framework
- Chaos engineering concepts (HPA, PDB, pod disruption)
- Docker multi-stage builds and loading images into kind
- Helm chart templating (Deployment, Service, RBAC)
- Kubernetes RBAC: ServiceAccount, ClusterRole, ClusterRoleBinding
- In-cluster config vs kubeconfig (how client-go switches automatically)
