#!/usr/bin/env bash
# Cross-compile encomdb for Android ARM64 (Termux) from any host with Go installed.
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p bin

echo "Building encomdb-android-arm64…"
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build \
  -trimpath -ldflags="-s -w" \
  -o bin/encomdb-android-arm64 ./cmd/encomdb

ls -lh bin/encomdb-android-arm64
