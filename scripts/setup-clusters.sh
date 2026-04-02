#!/usr/bin/env bash
# Creates two kind clusters: cluster-a and cluster-b.
# Both run on the same Docker "kind" network so pods in cluster-a
# can reach NodePort services in cluster-b via the node container IP.
set -euo pipefail

command -v kind  >/dev/null || { echo "ERROR: 'kind' not found. Install: https://kind.sigs.k8s.io/"; exit 1; }
command -v kubectl >/dev/null || { echo "ERROR: 'kubectl' not found."; exit 1; }

echo "==> Creating cluster-a..."
kind create cluster --name cluster-a || echo "cluster-a already exists, skipping"

echo "==> Creating cluster-b..."
kind create cluster --name cluster-b || echo "cluster-b already exists, skipping"

echo ""
echo "Clusters ready:"
kind get clusters
echo ""
echo "Contexts:"
echo "  kubectl --context kind-cluster-a ..."
echo "  kubectl --context kind-cluster-b ..."
