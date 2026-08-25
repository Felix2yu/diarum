# syntax=docker/dockerfile:1.17

# Final image assembled from CI-prebuilt artifacts:
#   - diarum-amd64 / diarum-arm64 -> ./bin/diarum
#     (Go binary built by the CI `build` job, frontend embedded via go:embed)
#
# This Dockerfile only assembles the runtime image — no Node toolchain,
# no Go compilation happens here.

FROM alpine:3.24

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata su-exec && \
    adduser -D -H -u 1000 diarum && \
    mkdir -p /app/data && \
    chown -R diarum:diarum /app

COPY bin/diarum /app/diarum
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /app/diarum /entrypoint.sh

ENV TZ=Asia/Shanghai
ENV DIARUM_DATA_PATH=/app/data

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8090/healthz || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/diarum", "serve", "--http=0.0.0.0:8090"]
