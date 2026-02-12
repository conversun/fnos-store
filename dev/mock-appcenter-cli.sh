#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOCK_APPS_DIR="${MOCK_APPS_DIR:-$SCRIPT_DIR/mock-apps}"
MOCK_STATE_DIR="/tmp/fnos-store-dev/state"

mkdir -p "$MOCK_STATE_DIR"

cmd_list() {
    printf "%-24s %-16s %-10s\n" "APPNAME" "VERSION" "STATUS"
    for manifest in "$MOCK_APPS_DIR"/*/manifest; do
        [ -f "$manifest" ] || continue
        appname=$(grep '^appname' "$manifest" | cut -d= -f2 | tr -d ' ')
        version=$(grep '^version' "$manifest" | cut -d= -f2 | tr -d ' ')
        state_file="$MOCK_STATE_DIR/$appname"
        if [ -f "$state_file" ]; then
            status=$(cat "$state_file")
        else
            status="running"
            echo "$status" > "$state_file"
        fi
        printf "%-24s %-16s %-10s\n" "$appname" "$version" "$status"
    done
}

cmd_check() {
    local appname="$1"
    for manifest in "$MOCK_APPS_DIR"/*/manifest; do
        [ -f "$manifest" ] || continue
        found=$(grep '^appname' "$manifest" | cut -d= -f2 | tr -d ' ')
        if [ "$found" = "$appname" ]; then
            echo "Installed"
            return 0
        fi
    done
    echo "Not installed"
}

cmd_status() {
    local appname="$1"
    state_file="$MOCK_STATE_DIR/$appname"
    if [ -f "$state_file" ]; then
        cat "$state_file"
    else
        for manifest in "$MOCK_APPS_DIR"/*/manifest; do
            [ -f "$manifest" ] || continue
            found=$(grep '^appname' "$manifest" | cut -d= -f2 | tr -d ' ')
            if [ "$found" = "$appname" ]; then
                echo "running"
                echo "running" > "$state_file"
                return 0
            fi
        done
        echo "unknown"
        return 1
    fi
}

cmd_install_fpk() {
    local fpk_path="$1"
    shift
    local volume=1
    while [ $# -gt 0 ]; do
        case "$1" in
            -v|--volume) volume="$2"; shift 2 ;;
            *) shift ;;
        esac
    done
    local basename
    basename=$(basename "$fpk_path" .fpk)
    local app_dir="$MOCK_APPS_DIR/$basename"
    mkdir -p "$app_dir"
    if [ ! -f "$app_dir/manifest" ]; then
        cat > "$app_dir/manifest" << EOF
appname         = $basename
version         = 1.0.0
display_name    = $basename
platform        = x86
service_port    = 0
desc            = Installed via mock fpk
source          = thirdparty
EOF
    fi
    echo "running" > "$MOCK_STATE_DIR/$basename"
    echo "Installed $basename to volume $volume"
}

cmd_uninstall() {
    local appname="$1"
    rm -f "$MOCK_STATE_DIR/$appname"
    echo "Uninstalled $appname"
}

cmd_start() {
    local appname="$1"
    echo "running" > "$MOCK_STATE_DIR/$appname"
    echo "Started $appname"
}

cmd_stop() {
    local appname="$1"
    echo "stopped" > "$MOCK_STATE_DIR/$appname"
    echo "Stopped $appname"
}

cmd_default_volume() {
    echo "1"
}

case "${1:-}" in
    list)           cmd_list ;;
    check)          cmd_check "${2:?appname required}" ;;
    status)         cmd_status "${2:?appname required}" ;;
    install-fpk)    shift; cmd_install_fpk "$@" ;;
    uninstall)      cmd_uninstall "${2:?appname required}" ;;
    start)          cmd_start "${2:?appname required}" ;;
    stop)           cmd_stop "${2:?appname required}" ;;
    default-volume) cmd_default_volume ;;
    *)
        echo "Usage: mock-appcenter-cli.sh <command> [args]"
        echo "Commands: list, check, status, install-fpk, uninstall, start, stop, default-volume"
        exit 1
        ;;
esac
