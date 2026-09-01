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

## Public repository

This repo is public. No secrets, real customer data, or production dumps —
see docs/TZ.md §1.1 (R5) and §11.
