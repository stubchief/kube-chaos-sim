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

### Create cluster and deploy target microservice

```bash
# Create kind cluster with 3 emulated zones (ru-central1-a/b/d)
kind create cluster --name chaos-sim --config kind-config.yaml

# Deploy podinfo as target microservice
helm repo add podinfo https://stefanprodan.github.io/podinfo
helm upgrade --install podinfo podinfo/podinfo

# Verify pods are running
kubectl get pods
```

### Run the backend

```bash
go build -o server ./cmd/server/
./server
```

Open http://localhost:8080 in your browser.

## Project structure

```
cmd/server/       — entrypoint
internal/k8s/     — client-go setup, Informers, in-memory cache
internal/chaos/   — Chaos Controller (kill pod, inject latency, etc.)
internal/metrics/ — synthetic metrics generator
internal/sse/     — SSE broadcaster
web/              — static HTML + Datastar attributes
```

## What I learned

- Go and client-go from scratch
- Kubernetes Informers and in-memory caching patterns
- SSE-based realtime UI without a JavaScript framework
- Chaos engineering concepts (HPA, PDB, pod disruption)