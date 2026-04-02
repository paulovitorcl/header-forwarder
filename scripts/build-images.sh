#!/usr/bin/env bash
# Builds all Docker images and loads them into both kind clusters.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

build_and_load() {
  local name="$1"
  local context="$2"
  echo "==> Building $name..."
  docker build -t "${name}:latest" "$context"
  echo "==> Loading $name into cluster-a..."
  kind load docker-image "${name}:latest" --name cluster-a
  if [[ "$name" == "service3" ]]; then
    echo "==> Loading $name into cluster-b..."
    kind load docker-image "${name}:latest" --name cluster-b
  fi
}

build_and_load service1       "$ROOT/services/service1"
build_and_load service2       "$ROOT/services/service2"
build_and_load service3       "$ROOT/services/service3"
build_and_load header-forwarder "$ROOT/sidecar"

echo ""
echo "All images built and loaded."
