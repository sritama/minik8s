# Design Document — Kubernetes-like Orchestrator

**Author:** Assistant
**Date:** 2025-08-11

---

## Table of contents
1. Executive summary
2. Goals, scope, and non-goals
3. High-level architecture
4. Component designs
   - API Server
   - Datastore
   - Controller Manager
   - Scheduler
   - Node Agent (kubelet-like)
   - Container Runtime Integration (CRI)
   - Networking (CNI)
   - Storage (CSI)
   - Client / CLI / SDK
5. API & data model
   - Object definitions (v1alpha1)
   - Versioning & compatibility
   - Watch & event semantics
6. Control loop & reconciliation model
7. Scheduling model & constraints
8. Node lifecycle & failure handling
9. Security model
   - Authn/Authz
   - mTLS between components
   - Secrets handling & encryption
10. Observability, logging, and tracing
11. Testing strategy
12. CI/CD & release strategy
13. Deployment & operations (HA, upgrades, backups)
14. Roadmap & milestones (detailed)
15. Risks, unknowns, and mitigation
16. Appendices
   - Sequence diagrams
   - Example API schemas
   - Minimal e2e test plan

---

## 1. Executive summary

This design document describes a pragmatic plan to build a Kubernetes-like orchestration system intended for learning, experimentation, and small-to-medium production use. The system follows the proven declarative API + control-loop pattern and deliberately reuses industry standards (OCI image format, CRI, CNI, CSI, Raft-based datastore). The document lays out component responsibilities, API surface, data model, reconciliation semantics, scheduling constraints, security, operational procedures, testing plans, and a phased roadmap with milestones and acceptance criteria.

The initial objective (MVP) is a system that can accept Pod-like objects, schedule them onto nodes, run containers via containerd/CRI on each node, and provide basic networking (pod-to-pod), persistent volumes (basic CSI or hostPath), and simple controller primitives (ReplicaSet / Deployment). The design emphasizes modularity to allow incremental implementation.

---

## 2. Goals, scope, and non-goals

### Goals (MVP)
- Declarative API server for creating Pod-like resources.
- Strongly-consistent persisted cluster state using a Raft-backed key-value store.
- Node agent that runs containers via CRI (containerd/CRI-O) and reports status.
- A simple scheduler capable of placing pods on nodes based on resources and constraints.
- Controller manager implementing at least ReplicaSet/Deployment reconciliation.
- Pod networking enabling cross-node pod-to-pod connectivity using a CNI plugin.
- Basic persistent volume support (hostPath for v0; CSI integration as phase 2).
- TLS-secured component communication and basic RBAC.
- Observability: Prometheus metrics + health endpoints.

### Non-goals (initial)
- Full feature parity with Kubernetes (no admission webhooks, advanced scheduler plugins, custom controllers, etc.).
- Native service mesh, network policy enforcement, or complex multi-tenancy primitives.
- Extensive multi-cluster federation or cloud-provider-specific features.

---

## 3. High-level architecture

Components:
- **API Server**: Exposes REST/gRPC endpoints, validates requests, enforces authz, persists objects to the datastore, supports watches.
- **Datastore**: Raft-backed key-value store providing strong consistency and watch semantics (etcd recommended for MVP).
- **Controller Manager**: Runs controllers (ReplicaSet/Deployment, Node lifecycle controller, PV controller).
- **Scheduler**: Watches unscheduled Pods and assigns nodes.
- **Node Agent**: Runs on each node, reconciles node-local pods by invoking CRI, sets up networking with CNI, mounts volumes via CSI or local mounts.
- **CNI Plugin(s)**: External plugin providing pod networking (can be existing project like Flannel or a minimal custom bridge for v0).
- **CSI Driver(s)**: External drivers for persistent volumes; initially stubbed/hostPath.
- **CLI / SDK**: `kubectl`-like CLI to submit manifests and query cluster state.

High-level interactions:
- Users → API Server → Datastore.
- Scheduler/Controllers ←→ API Server (watch & update).
- Node Agent ←→ API Server (watch assigned pods, update status) and communicates with CRI, CNI, CSI locally.

---

## 4. Component designs

### API Server
**Responsibilities**:
- Authenticate & authorize API requests.
- Validate and default incoming objects.
- Persist objects to datastore and maintain resource versions.
- Provide watch/streaming semantics for controllers and node agents.
- Serve health and metrics endpoints.

**Design notes**:
- Start with HTTP+JSON REST API (v1alpha1). Add protobuf/gRPC for performance later.
- Implement optimistic concurrency using `resourceVersion` for updates.
- Implement a watch API based on long-polling/HTTP SSE initially; later support streaming gRPC.
- Provide admission hooks (simple validations) as a pluggable interface.

**Scaling/HA**:
- API Servers are stateless; run multiple replicas behind a load-balancer. All servers read/write to the single Raft-backed store (etcd cluster).

### Datastore
**Responsibilities**:
- Strongly consistent key-value store for all cluster state.
- Provide watch semantics and compacted TTL/GC for old revisions.

**Design notes**:
- Use etcd for MVP to avoid reimplementing Raft + snapshots + compaction.
- Plan snapshot/backup (etcd snapshot), compaction strategy, and monitoring for disk/IO.

### Controller Manager
**Responsibilities**:
- Run controllers in the control plane; ensure controllers are leader-elected (single active leader) to avoid duplicate processing.
- Provide workqueue, rate-limiting, and retry semantics.

**Controllers (initial)**:
- ReplicaSet/Deployment controller.
- Node controller (mark nodes NotReady, evict pods after grace period).
- PV controller (bind/unbind basic volumes).

**Design notes**:
- Controllers follow reconcile loops: on event, compute desired vs actual and make updates.
- Use leader election (via datastore lease keys) for HA.

### Scheduler
**Responsibilities**:
- Assign unscheduled Pods to Nodes based on resources and constraints.

**Algorithm (MVP)**:
- Filter: remove nodes that do not satisfy resource requests or taints/tolerations.
- Score: simple scoring (least loaded, binpack, or round-robin).
- Bind: atomically write `pod.spec.nodeName` via API server.

**Design notes**:
- Implement scheduling as a controller that watches Pods in Pending state.
- Support topology/affinity later.

### Node Agent
**Responsibilities**:
- Watch for Pods assigned to this node and reconcile local state (create/start/stop containers).
- Report status and readiness of pods and node resources.
- Interact with CRI (containerd), CNI for networking, and CSI (node-side) for volumes.

**Design notes**:
- Node agent should be able to run in a container and communicate with the host container runtime socket.
- Implement an execution model that stores pod runtime state locally (checkpointing for restarts).
- Provide a garbage collection loop for old containers.

### Container Runtime Integration (CRI)
**Responsibilities**:
- Use CRI plugin interface to manage images and containers.

**Design notes**:
- Integrate with containerd (via its Go client) or have a CRI shim.
- Avoid using Docker Engine directly.

### Networking (CNI)
**Responsibilities**:
- Provide pod IP addressing and L3 connectivity across nodes.

**Design notes**:
- Implement CNI plugin hooks in node agent (NET_ADD, NET_DEL) and allow administrators to install a CNI plugin (e.g., Flannel, Calico) or use a default simple bridge for dev.
- For service discovery, implement a lightweight kube-proxy equivalent using iptables or userspace NAT for ClusterIP.

### Storage (CSI)
**Responsibilities**:
- Provide dynamic PV provisioning, attach/detach, and mount lifecycle.

**Design notes**:
- For MVP: support hostPath-style PVs and a simple static PV binder.
- Phase 2: implement CSI controller-side and node-side plumbing and integrate existing CSI drivers.

### Client / CLI / SDK
**Responsibilities**:
- YAML/JSON manifests submission; simple `kubectl`-like commands to create/get/delete resources.

**Design notes**:
- Provide a small Go-based CLI (`ykctl` / `kminictl`) that calls the API server.
- Use kube-style manifest files for developer familiarity.

---

## 5. API & data model

### Minimal objects (v1alpha1)
- **Pod**: metadata, spec (containers[], resources), status (phase, conditions, containerStatuses).
- **Node**: metadata, status (addresses, capacity, allocatable, conditions).
- **Service**: metadata, spec (selector, ports, type=ClusterIP), status (clusterIP).
- **PersistentVolume** & **PersistentVolumeClaim**: basic binding fields.
- **Deployment / ReplicaSet**: metadata, spec (replicas, selector, template).

Include `metadata.resourceVersion`, `metadata.uid`, `metadata.generation` for concurrency and rolling updates.

### Versioning
- Start `v1alpha1`. Follow a semantic migration path: `v1alpha1` → `v1beta1` → `v1`.
- Keep API compatibility: servers should reject unknown fields or store them under `metadata.raw` until a stable API is agreed upon.

### Watch semantics
- Watches are anchored to a resourceVersion and deliver a stream of `ADDED`, `MODIFIED`, `DELETED` events.
- Provide retryable watch endpoints with reconnection and resume using the last seen `resourceVersion`.

---

## 6. Control loop & reconciliation model

- Controllers implement idempotent reconcile functions.
- Maintain a local workqueue with exponential backoff on errors.
- Use eventual consistency with read-after-write guarantees from the datastore.
- Ensure controllers have leader-election to avoid split-brain processing.
- Design reconciler decisions to be deterministic and safe to run multiple times.

---

## 7. Scheduling model & constraints

**Inputs**: Pod resource requests, node capacities, taints/tolerations, node labels, affinity/anti-affinity.

**Steps**:
1. Filter nodes that do not satisfy resource and constraint predicates.
2. Score remaining nodes using a scoring function (e.g., least-allocated, binpack).
3. Select the highest scoring node and write the binding.

**Atomicity**:
- Binding should be done through the API server: a scheduler writes a `binding` subresource or updates `pod.spec.nodeName` using optimistic concurrency to avoid races.

---

## 8. Node lifecycle & failure handling

- Health heartbeat: node agent posts regular heartbeats / leases to API server.
- Node controller marks node `NotReady` if heartbeats stop for configurable threshold.
- Eviction: a grace period before evicting pods; pods can be gracefully terminated (SIGTERM) with configurable grace period.
- On node rejoin, node agent reconciles local containers and reports status; controllers recreate pods that were evicted.

---

## 9. Security model

**Authentication**:
- Support x509 client certs and token-based authentication for users and components.

**Authorization**:
- Role-Based Access Control (RBAC) with rules for verbs on resources.

**Transport security**:
- mTLS for all control-plane and data-plane component communication (API server <-> node agent, controllers, scheduler).

**Secrets**:
- Store secrets encrypted at rest in datastore; support KMS-based envelope encryption integration.
- Limit secret exposure to node agents and containers via ephemeral mounts.

**Attestation**:
- Optionally support node attestation (e.g., node certs signed by a central CA after bootstrap) for stronger trust.

---

## 10. Observability, logging, and tracing

- **Metrics**: expose Prometheus `metrics` on API server, controllers, scheduler, and node agent.
- **Logs**: structured JSON logs with correlation IDs for requests.
- **Tracing**: optional distributed tracing (OpenTelemetry) for expensive request flows.
- **Health**: readiness and liveness endpoints for each component.

---

## 11. Testing strategy

**Unit tests**:
- Controller reconcile logic, scheduler scoring, API server validation.

**Integration tests**:
- API server + datastore, watch semantics, controller interactions.

**E2E tests**:
- Start control plane + 1–3 node agents (local or CI VMs / containers).
- Tests: create Pod -> scheduled -> container running -> network connectivity -> delete.
- PV tests: create PVC -> PV bound -> pod mounts storage -> write/read -> cleanup.

**Chaos tests**:
- etcd leader failover, node agent restart, network partition between API server and node.

**Test harness**:
- Provide scripts to spin local clusters (docker-compose, Kind-like harness, or nested containers).

---

## 12. CI/CD & release strategy

- Use GitHub Actions / Jenkins / GitLab CI to run unit, integration, and e2e on PRs.
- Build multi-arch container images and push to registry on release tags.
- Provide an automated release checklist: run full e2e, snapshot etcd, publish artifacts.

---

## 13. Deployment & operations

**Single-site HA**:
- Deploy an odd-numbered etcd cluster (3 nodes) for quorum.
- Run multiple API server replicas behind load balancer.
- Controller Manager and Scheduler run with leader election (multiple replicas; one active).

**Backups**:
- Automated etcd snapshots; test restore procedures monthly.

**Upgrades**:
- Control plane: upgrade API servers one at a time (drain traffic from old instance), then controllers, then node agents with rolling upgrade.

**Monitoring & alerts**:
- Alert on etcd lag, API server error rates, node NotReady counts, controller queue latency.

---

## 14. Roadmap & milestones (detailed)

**Phase 0 (Weeks 0–2)**: Project scaffolding
- Repo skeleton, tech choices, API definitions (v1alpha1), developer runbook.
- Acceptance: can run API server + in-memory store and create GET/POST pod.

**Phase 1 (Weeks 2–6)**: Datastore & API
- Integrate etcd, implement CRUD + watch for Pod and Node.
- Acceptance: create Pod -> watch events delivered to client.

**Phase 2 (Weeks 6–12)**: Node agent + CRI
- Implement node agent with containerd integration; basic `run`, `stop`, `status`.
- Acceptance: API -> schedule (manual) -> node agent pulls image and runs container.

**Phase 3 (Weeks 12–16)**: Scheduler + Controller
- Implement scheduler; implement ReplicaSet controller.
- Acceptance: create Deployment -> ReplicaSet created -> desired count of pods running.

**Phase 4 (Weeks 16–22)**: Networking & Services
- CNI integration (simple bridge or Flannel), kube-proxy-like Service implementation.
- Acceptance: two pods on different nodes can reach each other; ClusterIP service routes traffic.

**Phase 5 (Weeks 22–28)**: Storage & PV
- Add hostPath PVs and PVC controller; plan CSI integration.
- Acceptance: workloads can mount volumes and persist data.

**Phase 6 (Weeks 28–40)**: Hardening & HA
- TLS, RBAC, etcd snapshots, leader election, observability, CI e2e.
- Acceptance: cluster survives control-plane restart; rolling upgrade tested.

---

## 15. Risks, unknowns, and mitigation

- **Datastore scaling & compaction**: Mitigate by using etcd and operational playbooks (snapshots, compaction).
- **Networking complexity**: Mitigate by supporting pluggable CNI and using established plugins for production.
- **Container runtime differences**: Stick to CRI abstraction and test against containerd/CRI-O.
- **Security mistakes**: Strong default TLS + RBAC, rotate certs, integrate KMS for secrets.

---

## 16. Appendices

### Appendix A — Example sequence: Pod creation & run
1. User POST /api/v1alpha1/pods → API server validates and stores Pod (phase Pending).
2. Scheduler reads Pending Pod via watch, selects Node `node-1` and updates Pod with `spec.nodeName=node-1`.
3. Node agent on `node-1` sees new Pod via watch, pulls image via containerd, executes container using CRI.
4. Node agent reports Pod `status.phase=Running` and container statuses back to API server.
5. Controller manager ensures ReplicaSet desired count; if a pod dies, it re-creates it.

### Appendix B — Minimal Pod JSON (v1alpha1)
```json
{
  "apiVersion": "v1alpha1",
  "kind": "Pod",
  "metadata": { "name": "hello-1", "namespace": "default" },
  "spec": {
    "containers": [
      { "name": "app", "image": "nginx:1.25", "resources": { "cpu": "100m", "memory": "128Mi" } }
    ],
    "nodeName": "",
    "restartPolicy": "Always"
  },
  "status": { "phase": "Pending" }
}
```

### Appendix C — Minimal e2e test checklist
- Create a Deployment (replicas=3). Verify 3 pods reach Running.
- Verify pods on different nodes can ping each other (pod IPs).
- Write a file to a hostPath-backed PV from pod A; read it from pod B.
- Kill etcd leader; verify cluster recovers and controllers continue.

---

## Next recommended actions
1. Approve this design and pick the language/runtime (Go recommended).
2. I will generate a starter repository scaffold (API server + in-memory store + simple node agent poller) if you want — ready-to-run locally.
3. Assign first sprint: implement CRUD + watch for Pod and Node and add unit tests.


---

*End of document.*

