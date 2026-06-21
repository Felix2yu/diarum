# syntax=docker/dockerfile:1.17

# ---- Frontend build stage ----
FROM node:24-alpine AS frontend-builder

WORKDIR /app/site

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache zstd

COPY site/package*.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund --loglevel=error

COPY site/ ./

RUN --mount=type=cache,target=/app/site/.svelte-kit \
    --mount=type=cache,target=/app/site/node_modules/.vite \
    npm run build

# ---- Backend build stage ----
FROM golang:1.26-alpine AS backend-builder

WORKDIR /app

RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache git

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY --from=frontend-builder /app/site/build ./internal/static/build

COPY . .

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
    adduser -D -H -u 1000 diarum && \
    mkdir -p /app/data && \
    chown -R diarum:diarum /app

COPY --from=backend-builder /app/diarum /app/diarum

ENV TZ=Asia/Shanghai
ENV DIARUM_DATA_PATH=/app/data

USER diarum

EXPOSE 8090

CMD ["/app/diarum", "serve", "--http=0.0.0.0:8090"]
