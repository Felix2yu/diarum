# syntax=docker/dockerfile:1.17

# Final image assembled from CI-prebuilt artifacts:
#   - diarum-amd64 / diarum-arm64 -> ./bin/diarum
#     (Go binary built by the CI `build` job, frontend embedded via go:embed)
#
# This Dockerfile only assembles the runtime image — no Node toolchain,
# no Go compilation happens here. Base image ghcr.io/felix2yu/base:alpine-3.24
# supplies ca-certificates, tzdata, su-exec and the PUID/PGID entrypoint.

FROM ghcr.io/felix2yu/base:alpine-3.24

WORKDIR /app

RUN mkdir -p /app/data

# 本仓库的容器布局：二进制在 /app，数据在 /app/data（见 Felix2yu/.github docs/docker.md）。
# /app 不在基镜的 PATH 上，所以启动命令与外部备份工具都用绝对路径：
# 外部备份工具在容器里跑 `docker exec <容器> sh -c "/app/diarum backup"`，
# 裸名（含工具按镜像名推导出的 `diarum backup`）都解析不到，备份就接不上。
COPY --chmod=755 bin/diarum /app/diarum

ENV TZ=Asia/Shanghai \
    DIARUM_DATA_PATH=/app/data \
    APP_DATA_DIR=/app/data

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8090/healthz || exit 1

# ENTRYPOINT 由基镜提供（PUID/PGID 降权），这里只给服务命令；/app 不在 PATH 上，用绝对路径
CMD ["/app/diarum", "serve", "--http=0.0.0.0:8090"]
