#!/usr/bin/env bash
# Deploys all services and wires cross-cluster connectivity.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KA="kubectl --context kind-cluster-a"
KB="kubectl --context kind-cluster-b"

echo "==> Deploying Service 3 to cluster-b..."
$KB apply -f "$ROOT/k8s/cluster-b/service3.yaml"
$KB rollout status deployment/service3 -n header-forwarder --timeout=120s

# Discover how to reach Service 3 from cluster-a pods.
# Kind nodes are Docker containers — they share the "kind" Docker bridge network,
# so all pods in cluster-a can reach cluster-b nodes via their Docker container IP.
echo "==> Discovering Cluster B node IP..."
CLUSTER_B_NODE_IP=$(docker inspect cluster-b-control-plane \
  --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' 2>/dev/null || \
  $KB get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')

SERVICE3_NODEPORT=30083
SERVICE3_URL="http://${CLUSTER_B_NODE_IP}:${SERVICE3_NODEPORT}"
echo "    Service 3 reachable at: $SERVICE3_URL"

echo "==> Deploying Service 2 to cluster-a..."
$KA apply -f "$ROOT/k8s/cluster-a/service2.yaml"

echo "==> Deploying Service 1 to cluster-a (with header-forwarder sidecar)..."
$KA apply -f "$ROOT/k8s/cluster-a/service1.yaml"

# Patch the ConfigMap so Service 1 knows where Service 3 is
echo "==> Patching service1-config with SERVICE3_URL=$SERVICE3_URL..."
$KA patch configmap service1-config -n header-forwarder \
  --type merge \
  -p "{\"data\":{\"SERVICE3_URL\":\"${SERVICE3_URL}\"}}"

# Restart Service 1 to pick up the new ConfigMap value
$KA rollout restart deployment/service1 -n header-forwarder

echo "==> Waiting for rollouts..."
$KA rollout status deployment/service2 -n header-forwarder --timeout=120s
$KA rollout status deployment/service1 -n header-forwarder --timeout=120s

echo ""
echo "All services deployed!"
echo ""
echo "==> Getting Service 1 NodePort..."
SVC1_NODEPORT=$($KA get svc service1 -n header-forwarder -o jsonpath='{.spec.ports[0].nodePort}')
CLUSTER_A_NODE_IP=$(docker inspect cluster-a-control-plane \
  --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' 2>/dev/null)

echo ""
echo "Service 1 is accessible at:"
echo "  From host (via kubectl port-forward):"
echo "    kubectl --context kind-cluster-a port-forward svc/service1 8080:80 -n header-forwarder"
echo "    curl http://localhost:8080/whoAreYou"
echo ""
echo "  From within cluster-a pods:"
echo "    http://service1.header-forwarder.svc.cluster.local/startTest"
