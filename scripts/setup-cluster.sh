#!/bin/bash
# Complete cluster setup: create kind cluster + install metrics-server + deploy podinfo
# Run this once to prepare the environment

set -e

echo "=== Creating kind cluster with accelerated HPA settings ==="
kind create cluster --name chaos-sim --config kind-config.yaml

echo ""
echo "=== Installing metrics-server ==="
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

echo ""
echo "=== Waiting for metrics-server deployment to start ==="
sleep 10

echo ""
echo "=== Patching metrics-server for kind compatibility ==="
# kind doesn't have proper TLS certificates, so we need to skip verification
# Also speed up resolution to 15s (must be > kubelet-request-timeout which is 10s)
kubectl patch deployment metrics-server -n kube-system --type='json' -p='[
  {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"},
  {"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--metric-resolution=5s"}
]'

echo ""
echo "=== Waiting for metrics-server to be ready ==="
kubectl rollout status deployment/metrics-server -n kube-system --timeout=180s

echo ""
echo "=== Installing podinfo (target microservice) ==="
helm repo add podinfo https://stefanprodan.github.io/podinfo
helm repo update
helm upgrade --install podinfo podinfo/podinfo -f helm/podinfo-values.yaml

echo ""
echo "=== Waiting for podinfo to be ready ==="
kubectl rollout status deployment/podinfo --timeout=120s

echo ""
echo "=== Verifying setup ==="
echo "Metrics-server:"
kubectl get pods -n kube-system | grep metrics-server
echo ""
echo "Podinfo pods:"
kubectl get pods -l app.kubernetes.io/name=podinfo

echo ""
echo "✅ Cluster setup complete!"
echo ""
echo "HPA timings:"
echo "  - sync-period: 5s (default 15s)"
echo "  - downscale-stabilization: 20s (default 5m)"
echo "  - metrics resolution: 15s (default 60s)"
echo ""
echo "Expected HPA reaction times:"
echo "  - Upscaling: 5-10 seconds"
echo "  - Downscaling: 20-30 seconds"
echo ""
echo "Next step:"
echo "  Run backend: go run ./cmd/server"
echo "  Open http://localhost:8080"