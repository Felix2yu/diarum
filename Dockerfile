# syntax=docker/dockerfile:1.17

# ---- Frontend build stage ----
FROM node:22-alpine AS frontend-builder

WORKDIR /app/site

# Install only dependencies needed for build (omit devDependencies that aren't required)
COPY site/package*.json ./

# Cache npm downloads across builds
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

COPY site/ ./

# Cache vite build cache
RUN --mount=type=cache,target=/app/site/.svelte-kit \
    --mount=type=cache,target=/app/site/node_modules/.vite \
    npm run build

# ---- Backend build stage ----
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app

# Install git for version detection (cached apk cache)
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git

COPY go.mod go.sum ./

# Cache Go module downloads
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

# Bring in the built frontend (embedded via //go:embed)
COPY --from=frontend-builder /app/site/build ./internal/static/build

ARG VERSION
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ -z "$VERSION" ]; then \
      VERSION=$(git describe --dirty --always --tags --abbrev=7 2>/dev/null || echo "docker"); \
    fi && \
    echo "Building version: $VERSION" && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.Version=$VERSION" \
      -o diarum .

# ---- Final (runtime) stage ----
FROM alpine:3.24

WORKDIR /app

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache ca-certificates tzdata && \
    adduser -D -H -u 10001 diarum

COPY --from=backend-builder /app/diarum /app/diarum

RUN mkdir -p /app/data && \
    chown -R diarum:diarum /app

ENV DIARUM_DATA_PATH=/app/data

USER diarum

EXPOSE 8090

CMD ["/app/diarum", "serve", "--http=0.0.0.0:8090"]
