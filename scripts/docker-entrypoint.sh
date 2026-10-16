#!/bin/sh
set -e

DATA_DIR="${DIARUM_DATA_PATH:-/app/data}"

mkdir -p "$DATA_DIR"
chown -R diarum:diarum "$DATA_DIR"

exec su-exec diarum "$@"
