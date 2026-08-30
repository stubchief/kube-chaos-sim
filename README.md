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
  values-cloud.yaml — cloud-specific overrides (LoadBalancer, YCR image)
terraform/        — Yandex Cloud infrastructure (VPC, k8s, registry)
  bootstrap/      — S3 bucket for Terraform state (one-time setup)
scripts/          — setup and build scripts
.github/workflows/ — CI/CD (deploy.yml, destroy.yml)
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
- Terraform for infrastructure-as-code (Yandex Cloud provider)
- GitHub Actions CI/CD with manual approval (GitHub Environments)
- Remote Terraform state management (S3 backend via Yandex Object Storage)
- Cross-zone Kubernetes deployment (1 node per availability zone)
- Budget optimization: preemptible VMs, core_fraction, platform selection


## Cloud deployment (Yandex Cloud)

The project supports deployment to Yandex Cloud for testing cross-zone behavior and real managed Kubernetes.

### Quick start (cloud)

**Prerequisites:**
- Yandex Cloud account with billing enabled
- `yc` CLI installed and configured
- GitHub repository with secrets configured (see below)

**Step 1: Bootstrap Terraform state bucket (one-time)**

```bash
cd terraform/bootstrap
export YC_KEY_JSON=$(cat /path/to/key.json)
terraform init
terraform apply
cd ../..
```

**Step 1b: Create CI service account and grant permissions (one-time)**

Terraform cannot assign IAM roles without already having IAM admin permissions (chicken-and-egg problem). Create the CI service account and grant it `resource-manager.admin` manually:

```bash
# Create CI service account
yc iam service-account create --name kube-chaos-sim-ci

# Get the service account ID
SA_ID=$(yc iam service-account get kube-chaos-sim-ci --format json | jq -r '.id')

# Grant resource-manager.admin role (allows managing IAM bindings, VPC, k8s, etc.)
yc resource-manager folder add-access-binding <FOLDER_ID> \
  --role resource-manager.admin \
  --subject serviceAccount:$SA_ID

# Create IAM key for the service account (used by GitHub Actions)
yc iam key create --service-account-name kube-chaos-sim-ci --output key.json
```

Save `key.json` — you'll need it for GitHub Secrets in the next step.

**Step 2: Configure GitHub Secrets**

Add these secrets to your GitHub repository:
- `YC_KEY_JSON` — IAM key for service account (JSON)
- `YC_FOLDER_ID` — Yandex Cloud folder ID
- `AWS_ACCESS_KEY_ID` — Static key for S3 backend
- `AWS_SECRET_ACCESS_KEY` — Secret part of static key
- `TF_STATE_BUCKET` — S3 bucket name for Terraform state (e.g., `chaos-sim-tf-state`)

**Step 3: Deploy**

Push to `main` branch or trigger `deploy.yml` workflow manually. GitHub Actions will:
1. Build Docker image and push to Yandex Container Registry
2. Run `terraform apply` to create infrastructure (VPC, k8s cluster, node group, registry)
3. Deploy applications (podinfo, kube-chaos-sim) via Helm

**Step 4: Access dashboard**

After deployment, get the LoadBalancer IP:
```bash
kubectl get svc kube-chaos-sim -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

Dashboard available at `http://<LB_IP>:8080`

**Step 5: Destroy (when done)**

Trigger `destroy.yml` workflow manually. This will:
1. Remove Helm releases (podinfo, kube-chaos-sim)
2. Run `terraform destroy` to delete all infrastructure

### Cloud architecture

- **3 nodes** (1 per zone: ru-central1-a/b/d) — demonstrates cross-zone scheduling
- **Budget-optimized**: preemptible VMs, 20% CPU guarantee, 2GB RAM, 5GB disk
- **Platform**: Intel Cascade Lake (standard-v2) — cheaper than Ice Lake
- **LoadBalancer** for public access (no auth — cluster is short-lived)
