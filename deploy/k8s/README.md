# Kubernetes manifests

Plain YAML plus a kustomize overlay for the four services (docs/TZ.md §7.1).
No Helm chart: there is one deployment target, and a chart's indirection buys
nothing until there are several.

## What is here — and what is not

| | |
|---|---|
| **Here** | namespace + ServiceAccount, per-service ConfigMaps, Deployments, Services, gateway Ingress + HPA, NetworkPolicies, PodDisruptionBudgets, migration Job |
| **Not here** | Postgres, Kafka, Redis, MinIO, etcd |

The stateful dependencies are deliberately absent. A hand-rolled Postgres
StatefulSet is a data-loss incident waiting to happen — no backups, no
point-in-time recovery, no tested failover, no connection pooling — and
writing one here would make it look supported. Run them from an operator
(CloudNativePG, Strimzi) or as managed services, and point the ConfigMaps at
them. The DNS names in `configmaps.yaml` assume they live in this namespace;
change them if they do not.

**Minimum cluster version: 1.29.** The gateway's `preStop.sleep` hook needs
it. Verified: the manifests fail schema validation against 1.28 for exactly
that field, and nothing else.

## Deploying

```sh
# 1. Secrets, once per environment. NEVER from secrets.example.yaml —
#    that file is placeholders, and it is excluded from the kustomization
#    so `apply -k` cannot overwrite real credentials with "<DB_PASSWORD>".
openssl genrsa -out signing-key.pem 2048
kubectl -n qrmenu create secret generic identity-signing-key \
  --from-file=signing-key.pem=signing-key.pem
shred -u signing-key.pem
#    ...plus the four DSN/password secrets; see secrets.example.yaml for
#    the exact keys each service expects.

# 2. Pin the image tags to the commit being deployed. Needs the standalone
#    kustomize binary — `kubectl kustomize` can render but not edit.
cd deploy/k8s
for svc in gateway catalog order identity migrate; do
  kustomize edit set image \
    ghcr.io/menli02/qr-menu-$svc=ghcr.io/menli02/qr-menu-$svc:sha-$GIT_SHA
done
#    Without that binary, edit the `images:` block in kustomization.yaml by
#    hand. Do not skip it: `:latest` is a rollback you cannot perform.

# 3. Migrate, then roll. In that order — every migration so far is
#    additive, so the new schema is safe for the running old code.
kubectl -n qrmenu delete job qrmenu-migrate --ignore-not-found
kubectl -n qrmenu apply -f migrate-job.yaml
kubectl -n qrmenu wait --for=condition=complete job/qrmenu-migrate --timeout=5m

# 4. Apply everything else.
kubectl apply -k .
kubectl -n qrmenu rollout status deploy/identity deploy/catalog deploy/order deploy/gateway
```

`checksum/config` in each Deployment is a literal `REPLACE_ON_APPLY`. Set it
from the rendered ConfigMap in the pipeline, or a ConfigMap edit will update
the mounted file while nothing restarts — leaving half the fleet on the old
settings until an unrelated rollout happens to pick them up.

## Decisions worth knowing before you change something

**Probes.** Three of them, and they are not interchangeable:

- `startupProbe` → `/readyz` on 6061. Dependency-aware: the pod takes no
  traffic until it has proved it can reach its database. This is what catches
  a wrong secret or an unmigrated schema on a rollout.
- `livenessProbe` → `/healthz` on 6060. Depends on nothing external, on
  purpose. Restarting a pod because Postgres is down does not fix Postgres.
- `readinessProbe` → also `/healthz`. **Not** `/readyz`: every replica shares
  one database, so a DB blip would make all of them unready at once, strip the
  Service of endpoints, and turn a degraded service into a total blackout with
  nowhere to shift traffic. `pkg/health`'s package comment has the full
  argument.

**The rpc Services are headless.** Discovery goes through etcd and clients
balance per request. A ClusterIP would put kube-proxy in front of long-lived
gRPC connections and pin each client to one backend for its lifetime — the
standard way to end up with one hot pod and the rest idle.

**The gateway's Kafka consumer group must stay per-pod.** `KafkaInstanceID`
is left unset so it defaults to `HOSTNAME`, i.e. the pod name. A WebSocket
lives on exactly one pod, so every pod needs every event (docs/TZ.md §7.3).
Pinning the group in the ConfigMap would give all replicas the same one and
split events between them — most clients would silently stop receiving
pushes, and nothing would look broken.

**identity replicas must share one signing key.** It is mounted read-only
from a Secret. The service generates a key only when the file is absent, so a
per-pod `emptyDir` would have each replica signing with a different key and
demoting the others in `signing_keys`. Tokens minted by one pod would fail
verification elsewhere.

**No CPU limits.** CPU is compressible; a limit on a latency-sensitive Go
service mostly buys throttling during the traffic spike you wanted headroom
for. Memory has limits because it is not compressible — one OOMKilled pod
beats a node evicting everything on it.

**NetworkPolicy needs a CNI that implements it.** On one that does not,
these objects apply cleanly and enforce nothing, which is the dangerous
failure mode. Verify with a deliberate connection attempt from a pod that
should be blocked, not by checking that `kubectl` accepted the manifest.

## Validating changes

Both of these run without a cluster:

```sh
kubectl kustomize deploy/k8s | kubeconform -strict -summary -kubernetes-version 1.30.0 -
kubeconform -strict -summary -kubernetes-version 1.30.0 deploy/k8s/migrate-job.yaml
```

## Known gaps

- `WSAllowedOrigins`, the Ingress host and the `cert-manager` issuer are
  example values. The origin list is a real security control — browsers do
  not apply the same-origin policy to WebSocket upgrades — so set it before
  exposing the ingress.
- No ServiceMonitor/PodMonitor. The pods carry `prometheus.io/*` annotations,
  which suits an annotation-scraping Prometheus; a Prometheus Operator
  install wants its own CRs instead, and which one this cluster runs is not
  something to guess at.
- No tracing backend. The services carry OpenTelemetry trace ids through
  logs, gRPC and outbox events, but nothing exports spans yet — go-zero needs
  a `Telemetry` config block pointed at a collector.
- Resource requests are estimates, not measurements. Revisit them against
  real traffic before capacity planning depends on them.
