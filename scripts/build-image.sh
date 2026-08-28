#!/bin/bash
# Build the backend Docker image and deploy it to kind cluster
# Usage: ./scripts/build-image.sh [image-tag]

set -e

IMAGE_TAG=${1:-"kube-chaos-sim:latest"}
CLUSTER_NAME="chaos-sim"

echo "=== Building Docker image: $IMAGE_TAG ==="
docker build -t "$IMAGE_TAG" .

echo ""
echo "=== Loading image into kind cluster: $CLUSTER_NAME ==="
kind load docker-image "$IMAGE_TAG" --name "$CLUSTER_NAME"

echo ""
echo "=== Deploying backend with Helm ==="
helm upgrade --install kube-chaos-sim helm/kube-chaos-sim

echo ""
echo "=== Waiting for backend to be ready ==="
kubectl rollout status deployment/kube-chaos-sim --timeout=120s

echo ""
echo "=== Backend status ==="
kubectl get pods -l app.kubernetes.io/name=kube-chaos-sim

echo ""
echo "✅ Image built, loaded, and deployed!"
echo ""
echo "Dashboard available at:"
echo "  http://localhost:8080"
