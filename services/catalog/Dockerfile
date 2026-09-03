# syntax=docker/dockerfile:1

# catalog — go-zero rpc service: venues, halls, tables, categories, items,
# modifiers, availability (docs/TZ.md §7.1). Build context is the repo
# root, because all services share one go.mod:
#
#   docker build -f services/catalog/Dockerfile -t qrmenu-catalog:local .
#
# The baked-in etc/catalog.v1.yaml is the *local dev* config (see that
# file's own header comment) — a safe default for `docker compose` smoke
# testing only. In Kubernetes it is overridden by mounting a ConfigMap/
# Secret at the same path (deploy/k8s, not yet written).

FROM --platform=$BUILDPLATFORM golang:1.25.3-alpine AS build
WORKDIR /src

# Cache module downloads in their own layer, separate from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/catalog ./services/catalog

# Distroless static: no shell, no package manager, runs as non-root by
# default — matches a CGO_ENABLED=0 pure-Go binary with no libc dependency.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/catalog ./catalog
COPY services/catalog/etc/catalog.v1.yaml ./etc/catalog.v1.yaml

USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["./catalog"]
CMD ["-f", "etc/catalog.v1.yaml"]
