#!/bin/bash
set -e
export COPYFILE_DISABLE=1

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"
FNOS_DIR="$SCRIPT_DIR/fnos"
FNPACK="$BUILD_DIR/fnpack"

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

[ -x "$FNPACK" ] || error "fnpack 未找到: $FNPACK — 请下载: https://static2.fnnas.com/fnpack/"
[ -d "$FNOS_DIR" ] || error "fnos/ 目录不存在"

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

# ── Step 3: Build fpk for each platform using fnpack ────────────────────────
for PLATFORM in x86 arm; do
    info "打包 ${PLATFORM} fpk..."

    if [ "$PLATFORM" = "x86" ]; then
        BINARY="$BUILD_DIR/store-server-x86"
    else
        BINARY="$BUILD_DIR/store-server-arm"
    fi

    STAGING="$BUILD_DIR/tmp/fnos-${PLATFORM}"
    rm -rf "$STAGING"
    cp -a "$FNOS_DIR" "$STAGING"

    cp "$BINARY" "$STAGING/app/store-server"
    if [ -d "$SCRIPT_DIR/web" ]; then
        cp -r "$SCRIPT_DIR/web" "$STAGING/app/web"
    fi

    if [ "$PLATFORM" = "arm" ]; then
        sed -i.tmp "s/^platform.*/platform              = arm/" "$STAGING/manifest"
        rm -f "$STAGING/manifest.tmp"
    fi

    cd "$SCRIPT_DIR"
    "$FNPACK" build --directory "$STAGING"

    APPNAME=$(grep "^appname" "$STAGING/manifest" | awk -F'=' '{print $2}' | tr -d ' ')
    VERSION=$(grep "^version" "$STAGING/manifest" | awk -F'=' '{print $2}' | tr -d ' ')
    FPK_NAME="${APPNAME}_${VERSION}_${PLATFORM}.fpk"
    mv "${APPNAME}.fpk" "$FPK_NAME" 2>/dev/null || true
done

info "构建完成！"
ls -lh "$SCRIPT_DIR"/*_*.fpk 2>/dev/null || true
