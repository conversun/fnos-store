#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"
FNOS_APPS_DIR="$SCRIPT_DIR/../fnos-apps"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

cleanup() {
    rm -rf "$BUILD_DIR/tmp"
}
trap cleanup EXIT

# Verify fnos-apps repo exists (needed for build-fpk.sh and shared framework)
[ -f "$FNOS_APPS_DIR/scripts/build-fpk.sh" ] || error "fnos-apps repo not found at $FNOS_APPS_DIR"

mkdir -p "$BUILD_DIR/tmp"

# ── Step 1: Build frontend ──────────────────────────────────────────────────
info "构建前端..."
if [ -d "$SCRIPT_DIR/frontend" ] && [ -f "$SCRIPT_DIR/frontend/package.json" ]; then
    cd "$SCRIPT_DIR/frontend"
    npm install --silent
    npm run build
    cd "$SCRIPT_DIR"
    info "前端构建完成"
else
    warn "frontend/ 目录不存在或缺少 package.json，跳过前端构建"
fi

# ── Step 2: Build Go binaries ───────────────────────────────────────────────
info "构建 Go 二进制文件 (x86)..."
GOOS=linux GOARCH=amd64 go build -o "$BUILD_DIR/store-server-x86" ./cmd/server/

info "构建 Go 二进制文件 (arm)..."
GOOS=linux GOARCH=arm64 go build -o "$BUILD_DIR/store-server-arm" ./cmd/server/

info "Go 构建完成"

# ── Step 3: Create app.tgz and build fpk for each platform ─────────────────
for PLATFORM in x86 arm; do
    info "打包 ${PLATFORM} fpk..."

    if [ "$PLATFORM" = "x86" ]; then
        BINARY="$BUILD_DIR/store-server-x86"
    else
        BINARY="$BUILD_DIR/store-server-arm"
    fi

    # Create app.tgz: binary + web assets
    APP_TGZ="$BUILD_DIR/tmp/app-${PLATFORM}.tgz"
    STAGING="$BUILD_DIR/tmp/staging-${PLATFORM}"
    mkdir -p "$STAGING"

    cp "$BINARY" "$STAGING/store-server"
    if [ -d "$SCRIPT_DIR/web" ]; then
        cp -r "$SCRIPT_DIR/web" "$STAGING/web"
    fi

    tar -czf "$APP_TGZ" -C "$STAGING" .

    # Use build-fpk.sh from fnos-apps to produce the .fpk
    cd "$SCRIPT_DIR"
    "$FNOS_APPS_DIR/scripts/build-fpk.sh" "." "$APP_TGZ" "" "$PLATFORM"
done

info "构建完成！"
ls -lh "$SCRIPT_DIR"/fnos-apps-store_*.fpk 2>/dev/null || true
