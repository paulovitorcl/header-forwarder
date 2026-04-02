# Header Forwarder PoC

Demonstrates **transparent HTTP header propagation across microservices and Kubernetes clusters** — with **zero changes to application code**.

---

## The Problem

When a request arrives at Service 1 carrying `x-journey-id: trilha-xpto`, that header must automatically reach Service 2 (same cluster) and Service 3 (different cluster) — even though none of the services contain any header-forwarding logic.

---

## Solution: Sidecar Proxy Pattern

The solution uses a custom **header-forwarder sidecar** deployed alongside Service 1. The application is completely unaware of it.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  CLUSTER A                                                              │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  Service 1 Pod                                               │       │
│  │                                                              │       │
│  │  ┌──────────────────────┐     ┌──────────────────────────┐  │       │
│  │  │  header-forwarder    │     │  service1 app            │  │       │
│  │  │  (sidecar)           │     │  (no header logic)       │  │       │
│  │  │                      │     │                          │  │       │
│  │  │  :80  inbound  ──────┼────▶│  :8080                   │  │       │
│  │  │  captures             │     │                          │  │       │
│  │  │  x-journey-id         │     │  HTTP_PROXY=             │  │       │
│  │  │                      │     │  http://localhost:9090   │  │       │
│  │  │  :9090 forward  ◀────┼─────│  (outbound calls routed  │  │       │
│  │  │  proxy                │     │   here automatically)    │  │       │
│  │  │  injects header       │     │                          │  │       │
│  │  └──────────┬───────────┘     └──────────────────────────┘  │       │
│  │             │                                                │       │
│  └─────────────┼──────────────────────────────────────────────-┘       │
│                │                                                        │
│                │  x-journey-id injected                                 │
│                ▼                                                        │
│  ┌─────────────────────┐                                               │
│  │  Service 2 Pod      │  ← receives x-journey-id automatically        │
│  │  service2 app       │                                               │
│  └─────────────────────┘                                               │
│                │ cross-cluster                                          │
└────────────────┼────────────────────────────────────────────────────────┘
                 │
                 ▼  (via NodePort on kind Docker network)
┌────────────────────────────────────────────────────────────────────────┐
│  CLUSTER B                                                             │
│                                                                        │
│  ┌─────────────────────┐                                               │
│  │  Service 3 Pod      │  ← receives x-journey-id automatically        │
│  │  service3 app       │                                               │
│  └─────────────────────┘                                               │
└────────────────────────────────────────────────────────────────────────┘
```

### How it works — step by step

1. **Inbound**: The Kubernetes Service for Service 1 points to **port 80** (the sidecar), not port 8080. Every incoming request first hits the sidecar, which reads and stores `x-journey-id` in memory, then reverse-proxies to the app on `localhost:8080`.

2. **Outbound**: The app container has `HTTP_PROXY=http://localhost:9090` in its environment. Go's `net/http` default client honours this env var — every `http.Get(...)` call is automatically routed to the sidecar's forward proxy **without any code change**.

3. **Injection**: Before forwarding each outbound request to its real destination, the sidecar reads the stored `x-journey-id` and sets it as a request header. The destination service (Service 2 or Service 3) receives the header as if the caller had always sent it.

4. **Absent header**: If no `x-journey-id` was in the inbound request, the sidecar stores an empty string and does **not** add the header to outbound requests.

### Why this approach

| Constraint | How it is met |
|---|---|
| No application code changes | `HTTP_PROXY` env var is set in the Kubernetes manifest (infrastructure config) |
| Intra-cluster propagation | Forward proxy routes through kube-dns to Service 2 |
| Inter-cluster propagation | Forward proxy connects to Cluster B node IP (same Docker bridge network) |
| Header absent → not injected | Sidecar tracks presence/absence separately |
| Observable | Sidecar logs every capture and injection; services echo received headers |

---

## Repository Layout

```
header-forwarder/
├── services/
│   ├── service1/     # GET /whoAreYou, GET /startTest
│   ├── service2/     # GET /whoAreYou  (logs + echoes headers)
│   └── service3/     # GET /whoAreYou  (logs + echoes headers)
├── sidecar/          # header-forwarder proxy (the infrastructure component)
├── k8s/
│   ├── cluster-a/    # Service 1 + Service 2 manifests
│   └── cluster-b/    # Service 3 manifest (NodePort)
└── scripts/
    ├── setup-clusters.sh
    ├── build-images.sh
    ├── deploy.sh
    └── test.sh
```

---

## Prerequisites

| Tool | Install |
|---|---|
| Docker | https://docs.docker.com/get-docker/ |
| kind | `brew install kind` or https://kind.sigs.k8s.io/ |
| kubectl | `brew install kubectl` |
| Go 1.22+ | `brew install go` (only to build locally; Docker builds work without it) |

---

## Step-by-Step Setup

### 1 — Create the clusters

```bash
./scripts/setup-clusters.sh
```

This creates `kind-cluster-a` and `kind-cluster-b`. Both run as Docker containers on the `kind` Docker bridge network, which means pods in Cluster A can reach Cluster B nodes by IP.

### 2 — Build and load images

```bash
./scripts/build-images.sh
```

Builds four images locally and loads them into the relevant kind clusters:

| Image | Clusters |
|---|---|
| `service1:latest` | cluster-a |
| `service2:latest` | cluster-a |
| `service3:latest` | cluster-b |
| `header-forwarder:latest` | cluster-a |

### 3 — Deploy

```bash
./scripts/deploy.sh
```

This script:
1. Deploys Service 3 to Cluster B
2. Discovers Cluster B's node IP (Docker container IP on the `kind` network)
3. Deploys Service 1 + Service 2 to Cluster A
4. Patches the `service1-config` ConfigMap with `SERVICE3_URL=http://<cluster-b-node-ip>:30083`
5. Rolls out Service 1 to pick up the new URL

---

## Testing

### Quick automated test

```bash
./scripts/test.sh
```

### Manual tests

Start a port-forward in one terminal:

```bash
kubectl --context kind-cluster-a port-forward svc/service1 8080:80 -n header-forwarder
```

#### Smoke test

```bash
curl http://localhost:8080/whoAreYou
```

Expected:
```json
{"message":"I am 1","service":"1"}
```

---

#### Test WITHOUT `x-journey-id`

```bash
curl http://localhost:8080/startTest
```

Expected response (abbreviated):
```json
{
  "note": "Header propagation is done by the sidecar proxy, NOT by this application",
  "service2_response": {
    "message": "I am 2",
    "service": "2",
    "x-journey-id": "",
    "received_headers": { ... }
  },
  "service3_response": {
    "message": "I am 3",
    "service": "3",
    "x-journey-id": "",
    "received_headers": { ... }
  }
}
```

`x-journey-id` is **empty** in both responses — the header was not present and was not injected.

---

#### Test WITH `x-journey-id: trilha-xpto`

```bash
curl -H "x-journey-id: trilha-xpto" http://localhost:8080/startTest
```

Expected response (abbreviated):
```json
{
  "note": "Header propagation is done by the sidecar proxy, NOT by this application",
  "service2_response": {
    "message": "I am 2",
    "service": "2",
    "x-journey-id": "trilha-xpto",
    "received_headers": {
      "X-Journey-Id": "trilha-xpto",
      ...
    }
  },
  "service3_response": {
    "message": "I am 3",
    "service": "3",
    "x-journey-id": "trilha-xpto",
    "received_headers": {
      "X-Journey-Id": "trilha-xpto",
      ...
    }
  }
}
```

`x-journey-id: trilha-xpto` appears in **both** downstream services — injected by the sidecar, not the app.

---

## Observability

### Sidecar logs (header-forwarder)

```bash
POD=$(kubectl --context kind-cluster-a get pod -n header-forwarder -l app=service1 -o jsonpath='{.items[0].metadata.name}')
kubectl --context kind-cluster-a logs $POD -n header-forwarder -c header-forwarder -f
```

With `x-journey-id` present:
```
[inbound]  captured x-journey-id="trilha-xpto"  path=/startTest
[outbound] inject x-journey-id="trilha-xpto"  → http://service2.header-forwarder.svc.cluster.local/whoAreYou
[outbound] inject x-journey-id="trilha-xpto"  → http://<cluster-b-node>:30083/whoAreYou
```

Without `x-journey-id`:
```
[inbound]  no x-journey-id  path=/startTest
[outbound] no x-journey-id to inject  → http://service2.header-forwarder.svc.cluster.local/whoAreYou
[outbound] no x-journey-id to inject  → http://<cluster-b-node>:30083/whoAreYou
```

### Service logs

```bash
# Service 2
kubectl --context kind-cluster-a logs -l app=service2 -n header-forwarder -f

# Service 3
kubectl --context kind-cluster-b logs -l app=service3 -n header-forwarder -f
```

---

## Cleanup

```bash
kind delete cluster --name cluster-a
kind delete cluster --name cluster-b
```

---

## Production Considerations

This PoC uses an in-memory variable in the sidecar to store the last seen `x-journey-id`. This works correctly for the demo but has caveats in production:

| Concern | Production solution |
|---|---|
| Concurrent requests | Key the stored header by request ID (e.g., `x-request-id`) using a `sync.Map`; pass the request ID as an outbound header so the proxy can correlate |
| HTTPS upstream | Extend the sidecar to handle CONNECT tunnelling or use a service mesh (Istio/Linkerd) alongside this pattern |
| Multiple replicas | The sidecar is pod-local — each replica has its own state, which is correct |
| Header lifecycle | Clear stored header after the request completes (add response hook) |

For a full production-grade solution without application changes, consider **Istio's `ext_proc` filter** or **OpenTelemetry Collector as a sidecar** with custom processors.
