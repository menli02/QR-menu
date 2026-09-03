# syntax=docker/dockerfile:1

# gateway — go-zero REST+WS edge. The only publicly exposed component
# (docs/TZ.md §7.1). Build context is the repo root, because all services
# share one go.mod:
#
#   docker build -f services/gateway/Dockerfile -t qrmenu-gateway:local .
#
# The baked-in etc/gateway-api.yaml is the *local dev* config (see that
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
    go build -trimpath -ldflags="-s -w" -o /out/gateway ./services/gateway

# Distroless static: no shell, no package manager, runs as non-root by
# default — matches a CGO_ENABLED=0 pure-Go binary with no libc dependency.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/gateway ./gateway
COPY services/gateway/etc/gateway-api.yaml ./etc/gateway-api.yaml

USER nonroot:nonroot
EXPOSE 8888
ENTRYPOINT ["./gateway"]
CMD ["-f", "etc/gateway-api.yaml"]
