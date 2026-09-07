# QR-menu

Contactless QR menu, ordering and kitchen display for cafes and restaurants.

Architecture, domain model, contracts and rollout plan: see [docs/TZ.md](docs/TZ.md).

## Layout

```
proto/{catalog,order,identity}/v1/   gRPC contracts (source of truth) + generated pb/grpc code
services/gateway/                    go-zero REST+WS edge — the only public component
services/{catalog,order,identity}/   go-zero rpc services
migrations/{catalog,order,identity}/ per-service SQL migrations (golang-migrate)
deploy/docker-compose.yml            local dev infra: Postgres, Redis, Kafka, MinIO, etcd
deploy/k8s/                          Kubernetes manifests (target runtime; not yet written)
web/{admin,guest,kds}/               React frontends (not yet scaffolded)
```

Each service owns one Postgres database (`catalog_db`, `order_db`, `identity_db`) —
no cross-service foreign keys, no shared schema. `order` is the only service that
calls another over gRPC (`order` → `catalog.ResolveOrderItems`, to price and validate
items before committing an order).

Everything under `services/`, `internal/logic/**`, and `internal/middleware/**` is
currently a scaffold: it builds, boots, registers with etcd, and the gRPC/REST routes
exist end to end — but request handling is still `// todo` stubs. See docs/TZ.md for
what each endpoint is supposed to do.

## Prerequisites

- Go 1.25+
- `goctl`, `buf`, `protoc` + `protoc-gen-go`/`protoc-gen-go-grpc` (regenerating contracts)
- `golang-migrate` CLI (`migrate`)
- Docker + Docker Compose (local infra)
- `grpcurl` (optional, for poking rpc services directly)

## Running locally

```sh
make infra-up      # Postgres, Redis, Kafka, MinIO, etcd
make migrate-up     # apply migrations to catalog_db / order_db / identity_db

make run-identity   # in one shell
make run-catalog    # in another
make run-order      # in another
make run-gateway    # in another — REST+WS on :8888
```

`make help` lists every target, including `make proto` (regenerate contracts after
editing a `.proto` file) and `make migrate-new svc=<name> name=<description>`.

### Realtime (optional locally)

`KafkaBrokers` is empty in every `etc/*.yaml`, which turns the outbox relay and
the gateway's consumer off. Everything still works: services write outbox rows
you can inspect in Postgres, sockets connect, and clients fall back to polling.
Nothing tries to dial a broker that isn't there.

To exercise the full push path:

```sh
make kafka-topics   # creates the §8.3 topics with their planned partition counts
```

then set `KafkaBrokers: ["127.0.0.1:9094"]` in `services/{order,catalog}/etc/*.yaml`
and `services/gateway/etc/gateway-api.yaml`. (9094 is compose's host-reachable
listener; services running *inside* compose use `kafka:9092`.)

Topics are created explicitly rather than left to the broker's auto-creation:
an auto-created topic gets one partition, which silently discards the partition
plan that §8.3's per-key ordering guarantee depends on.

## CI and container images

Every push/PR runs `.github/workflows/ci.yml` (`gofmt`, `go vet`, `go build`,
`go test -race`, `golangci-lint`, `buf lint`/`buf breaking`) and
`.github/workflows/docker.yml` (builds all 4 service images; pushes to GHCR
only from `main` or a `v*` tag).

The test job runs a real Postgres service container and applies the migrations
before testing, so the model-layer integration suites actually execute rather
than skipping. Those tests are where the concurrency behaviour lives — partial
unique indexes, `ON CONFLICT` arbiters, compare-and-set updates,
`FOR UPDATE SKIP LOCKED` — and they have caught several bugs that the unit
tests passed straight over. A dedicated step fails the job if they skip, since
a silent skip would otherwise look identical to a pass.

Each service has its own multi-stage `Dockerfile` (distroless static
runtime, non-root, ~18MB images). Build locally with:

```sh
make docker-build   # builds qrmenu-{gateway,catalog,order,identity}:local
```

The config baked into each image is the local-dev `etc/*.yaml` — production
values come from a Kubernetes ConfigMap/Secret mounted at the same path
(`deploy/k8s`, not yet written; see docs/TZ.md §7.1).

## Public repository

This repo is public. No secrets, real customer data, or production dumps —
see docs/TZ.md §1.1 (R5) and §11.
