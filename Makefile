SHELL := /bin/bash
COMPOSE := docker compose -f deploy/docker-compose.yml

PG_USER := qrmenu
PG_PASS := qrmenu
PG_HOST := 127.0.0.1:5437

.PHONY: help
help:
	@echo "make infra-up          start Postgres/Redis/Kafka/MinIO/etcd for local dev"
	@echo "make infra-down        stop and remove local dev infra (keeps volumes)"
	@echo "make infra-reset       stop local dev infra and delete its volumes"
	@echo "make proto             regenerate pb/grpc stubs for all services (buf lint first)"
	@echo "make build             go build ./..."
	@echo "make test              go test ./..."
	@echo "make migrate-up        apply pending migrations for all 3 databases"
	@echo "make migrate-down      roll back the last migration for all 3 databases"
	@echo "make migrate-new svc=catalog name=add_menu_items   create a new migration file"
	@echo "make run-identity      run the identity rpc service"
	@echo "make run-catalog       run the catalog rpc service"
	@echo "make run-order         run the order rpc service"
	@echo "make run-gateway       run the gateway rest service"
	@echo "make docker-build      build local images for all 4 services (tag :local)"

# ---- Infra (docker-compose, local dev only — see deploy/docker-compose.yml) ----

.PHONY: infra-up infra-down infra-reset
infra-up:
	$(COMPOSE) up -d

infra-down:
	$(COMPOSE) down

infra-reset:
	$(COMPOSE) down -v

# ---- Protobuf ----

.PHONY: proto
proto: buf-lint
	@for svc in catalog order identity; do \
		echo "==> $$svc" ; \
		( cd proto/$$svc/v1 && protoc $$svc.proto \
			--go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative ) || exit 1 ; \
	done

.PHONY: buf-lint buf-breaking
buf-lint:
	buf lint

buf-breaking:
	buf breaking --against '.git#branch=main'

# ---- Build / test ----

.PHONY: build test vet fmt
build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# ---- Migrations (golang-migrate, one database per service — docs/TZ.md §6) ----

.PHONY: migrate-up migrate-down migrate-new
migrate-up:
	@for svc in catalog order identity; do \
		echo "==> $$svc" ; \
		migrate -path migrations/$$svc -database "postgres://$(PG_USER):$(PG_PASS)@$(PG_HOST)/$${svc}_db?sslmode=disable" up || exit 1 ; \
	done

migrate-down:
	@for svc in catalog order identity; do \
		echo "==> $$svc" ; \
		migrate -path migrations/$$svc -database "postgres://$(PG_USER):$(PG_PASS)@$(PG_HOST)/$${svc}_db?sslmode=disable" down 1 || exit 1 ; \
	done

migrate-new:
	@test -n "$(svc)" || (echo "usage: make migrate-new svc=<catalog|order|identity> name=<description>" && exit 1)
	@test -n "$(name)" || (echo "usage: make migrate-new svc=<catalog|order|identity> name=<description>" && exit 1)
	migrate create -ext sql -dir migrations/$(svc) -seq $(name)

# ---- Run services (local dev; expects infra-up + migrate-up already done) ----

.PHONY: run-identity run-catalog run-order run-gateway
run-identity:
	go run ./services/identity -f services/identity/etc/identity.v1.yaml

run-catalog:
	go run ./services/catalog -f services/catalog/etc/catalog.v1.yaml

run-order:
	go run ./services/order -f services/order/etc/order.v1.yaml

run-gateway:
	go run ./services/gateway -f services/gateway/etc/gateway-api.yaml

# ---- Docker images (see services/*/Dockerfile; CI builds/pushes these to
# GHCR from .github/workflows/docker.yml on push to main / a v* tag) ----

.PHONY: docker-build
docker-build:
	@for svc in gateway catalog order identity; do \
		echo "==> $$svc" ; \
		docker build -f services/$$svc/Dockerfile -t qrmenu-$$svc:local . || exit 1 ; \
	done
