# Multi-stage build that produces a single image carrying the Go binary +
# pre-built frontend assets. The binary's all-in-one subcommand serves both
# the API and the SPA from /app/web.

# ----- frontend -----
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ .
RUN npm run build

# ----- backend -----
FROM golang:1.26-alpine AS go
WORKDIR /src
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/k8s-ui/k8s-ui/internal/config.Version=${VERSION}" -o /out/k8s-ui ./cmd/k8s-ui

# ----- runtime -----
# go-git invokes the real `git` binary when cloning from a local bare repo
# (checkout for render). Distroless has no git — use Alpine + git + CA bundle.
FROM alpine:3.20
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
COPY --from=go  /out/k8s-ui /app/k8s-ui
COPY --from=web /web/dist /app/web
ENV WEB_ASSETS_DIR=/app/web \
    HTTP_ADDR=:8080 \
    REPO_CACHE_DIR=/tmp/k8s-ui-repos \
    IN_CLUSTER=true
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/k8s-ui"]
CMD ["all-in-one"]
