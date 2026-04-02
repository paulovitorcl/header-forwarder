#!/usr/bin/env bash
# End-to-end test — demonstrates header propagation.
# Run AFTER deploy.sh. Requires kubectl port-forward (started below).
set -euo pipefail

KA="kubectl --context kind-cluster-a"

echo "==> Starting port-forward to Service 1..."
$KA port-forward svc/service1 8080:80 -n header-forwarder &
PF_PID=$!
trap "kill $PF_PID 2>/dev/null" EXIT
sleep 2  # wait for port-forward to be ready

BASE="http://localhost:8080"

echo ""
echo "==========================================================="
echo "  TEST 1: /whoAreYou (basic smoke test)"
echo "==========================================================="
curl -s "$BASE/whoAreYou" | python3 -m json.tool 2>/dev/null || curl -s "$BASE/whoAreYou"

echo ""
echo "==========================================================="
echo "  TEST 2: /startTest WITHOUT x-journey-id"
echo "  Expected: x-journey-id is absent in service2 and service3"
echo "==========================================================="
curl -s "$BASE/startTest" | python3 -m json.tool 2>/dev/null || curl -s "$BASE/startTest"

echo ""
echo "==========================================================="
echo "  TEST 3: /startTest WITH x-journey-id: trilha-xpto"
echo "  Expected: x-journey-id propagated to service2 AND service3"
echo "  WITHOUT any application-level forwarding"
echo "==========================================================="
curl -s -H "x-journey-id: trilha-xpto" "$BASE/startTest" | python3 -m json.tool 2>/dev/null || \
  curl -s -H "x-journey-id: trilha-xpto" "$BASE/startTest"

echo ""
echo "==========================================================="
echo "  SIDECAR LOGS — shows what the header-forwarder intercepted"
echo "==========================================================="
POD=$($KA get pod -n header-forwarder -l app=service1 -o jsonpath='{.items[0].metadata.name}')
echo "Pod: $POD"
$KA logs "$POD" -n header-forwarder -c header-forwarder --tail=30

echo ""
echo "==========================================================="
echo "  SERVICE 2 LOGS — confirms x-journey-id was received"
echo "==========================================================="
POD2=$($KA get pod -n header-forwarder -l app=service2 -o jsonpath='{.items[0].metadata.name}')
$KA logs "$POD2" -n header-forwarder --tail=10

echo ""
echo "Test complete."
