# Multi-target image build. One Go module, component-specific binaries and runtime layers:
#   apiserver   — HTTP + Web UI (no git/helm)
#   controller  — reconciler (no git/helm/UI)
#   reposerver  — gRPC renderer (git + helm)
#   all-in-one  — dev/MVP: all roles + UI + git + helm
#
# Build examples:
#   docker build --target apiserver -t orin-apiserver:dev .
#   docker build --target reposerver -t orin-reposerver:dev .

# ----- frontend (apiserver + all-in-one only) -----
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ .
RUN npm run build

# ----- Go binaries -----
FROM golang:1.26-alpine AS go
WORKDIR /src
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG LDFLAGS="-s -w -X github.com/orin/orin/internal/config.Version=${VERSION}"

RUN CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o /out/orin ./cmd/orin
RUN CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o /out/orin-apiserver ./cmd/orin-apiserver
RUN CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o /out/orin-controller ./cmd/orin-controller
RUN CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o /out/orin-reposerver ./cmd/orin-reposerver

# ----- slim runtime base (no git/helm) -----
FROM alpine:3.20 AS runtime-base
RUN apk add --no-cache ca-certificates \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -S -G nonroot -h /tmp nonroot
WORKDIR /app
USER 65532:65532

# ----- reposerver runtime (git + helm for render) -----
FROM alpine:3.20 AS runtime-reposerver
ARG HELM_VERSION=v3.16.4
RUN apk add --no-cache git ca-certificates curl \
    && ARCH=$(uname -m) \
    && case "$ARCH" in x86_64) H=amd64 ;; aarch64) H=arm64 ;; *) H=amd64 ;; esac \
    && curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-${H}.tar.gz" | tar xz \
    && install -m0755 "linux-${H}/helm" /usr/local/bin/helm \
    && rm -rf "linux-${H}" \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -S -G nonroot -h /tmp nonroot
WORKDIR /app
USER 65532:65532

FROM runtime-base AS apiserver
COPY --from=go /out/orin-apiserver /app/orin-apiserver
COPY --from=web /web/dist /app/web
ENV WEB_ASSETS_DIR=/app/web HTTP_ADDR=:8080 IN_CLUSTER=true
EXPOSE 8080
ENTRYPOINT ["/app/orin-apiserver"]

FROM runtime-base AS controller
COPY --from=go /out/orin-controller /app/orin-controller
ENV IN_CLUSTER=true REPO_CACHE_DIR=/tmp/orin-repos
ENTRYPOINT ["/app/orin-controller"]

FROM runtime-reposerver AS reposerver
COPY --from=go /out/orin-reposerver /app/orin-reposerver
ENV REPO_SERVER_ADDR=:50051 REPO_CACHE_DIR=/tmp/orin-repos
EXPOSE 50051
ENTRYPOINT ["/app/orin-reposerver"]

FROM runtime-reposerver AS all-in-one
COPY --from=go /out/orin /app/orin
COPY --from=web /web/dist /app/web
ENV WEB_ASSETS_DIR=/app/web \
    HTTP_ADDR=:8080 \
    REPO_CACHE_DIR=/tmp/orin-repos \
    IN_CLUSTER=true
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/orin"]
CMD ["all-in-one"]
