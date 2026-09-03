# syntax=docker/dockerfile:1

# identity — go-zero rpc service: staff accounts, roles, password hashing,
# token issue/refresh, guest token signing keys. Security-sensitive;
# smallest possible blast radius and audit surface (docs/TZ.md §7.1).
# Build context is the repo root, because all services share one go.mod:
#
#   docker build -f services/identity/Dockerfile -t qrmenu-identity:local .
#
# The baked-in etc/identity.v1.yaml is the *local dev* config (see that
# file's own header comment) — a safe default for `docker compose` smoke
# testing only. In Kubernetes it is overridden by mounting a ConfigMap/
# Secret at the same path (deploy/k8s, not yet written). Signing keys in
# particular must come from a Secret, never be baked into the image.

FROM --platform=$BUILDPLATFORM golang:1.25.3-alpine AS build
WORKDIR /src

# Cache module downloads in their own layer, separate from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/identity ./services/identity

# Distroless static: no shell, no package manager, runs as non-root by
# default — matches a CGO_ENABLED=0 pure-Go binary with no libc dependency.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/identity ./identity
COPY services/identity/etc/identity.v1.yaml ./etc/identity.v1.yaml

USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["./identity"]
CMD ["-f", "etc/identity.v1.yaml"]
