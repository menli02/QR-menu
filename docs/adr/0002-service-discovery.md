# ADR-0002 — etcd for service discovery

**Status:** Accepted · 2026-09-01 (supersedes the draft decision to use Kubernetes Services)

## Context
go-zero supports etcd-based discovery and direct targets. Kubernetes DNS looks like the
cheaper option — one fewer stateful dependency — but a gRPC client that resolves a
`ClusterIP` Service opens one long-lived connection and pins every request to whichever pod
it landed on. Getting real per-request balancing out of DNS needs headless Services plus a
resolver configured correctly in every client, which is the same amount of moving parts with
less visibility.

## Decision
Use etcd for discovery in local development and in the cluster alike
(`etcd.qrmenu.svc.cluster.local:2379`, keys `catalog.rpc`, `order.rpc`, `identity.rpc`),
with `NonBlock: true` on every client so a slow or absent dependency cannot stall startup.

## Consequences
- Client-side load balancing works out of the box, and the same configuration shape runs on a
  laptop and in the cluster — local development actually exercises the production path.
- One more stateful dependency to run, back up and monitor. etcd is in
  `deploy/docker-compose.yml` and must be in the cluster manifests before any service starts.
- `NonBlock: true` means a service can come up with a dependency missing; readiness
  (`/readyz`) is what keeps it out of the load balancer until the dependency resolves.
- Moving to Kubernetes Services later is a config change, not a code change.
