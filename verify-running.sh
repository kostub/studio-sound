#!/bin/bash
# Quick verification that Studio Sound is running correctly.
#
# Scope: development builds only. Process detection matches the dev binary name
# `studio-sound-app` (not the bundled `Studio Sound App` release), so this is a
# dev-loop liveness + regression check, not a packaged-app health check.
#
# The full IPC ping/pong is driven from the app's Diagnostics UI (see Next Steps
# below); there is no CLI to drive it, so this script does not automate it.
set -uo pipefail

FAILED=0

echo "🔍 Checking Studio Sound App Status..."
echo ""

# ps | grep | awk can return several matching PIDs (e.g. release + dev build,
# helper processes), newline-separated. Take the first so downstream `ps -fp`
# gets a single well-formed PID rather than a multiline value.
APP_PID=$(ps aux | grep "studio-sound-app" | grep -v grep | awk '{print $2}' | head -1)
if [ -n "$APP_PID" ]; then
    echo "✅ Tauri App Running (PID: $APP_PID)"
else
    echo "❌ Tauri App NOT running"
    FAILED=1
fi

SIDECAR_PID=$(ps aux | grep "studio-sidecar serve" | grep -v grep | awk '{print $2}' | head -1)
if [ -n "$SIDECAR_PID" ]; then
    echo "✅ Go Sidecar Running (PID: $SIDECAR_PID)"
else
    echo "❌ Go Sidecar NOT running"
    FAILED=1
fi

VITE_PID=$(ps aux | grep "vite.*1420" | grep -v grep | awk '{print $2}' | head -1)
if [ -n "$VITE_PID" ]; then
    echo "✅ Vite Dev Server Running (PID: $VITE_PID)"
else
    echo "⚠️  Vite Dev Server NOT running (might be normal if using production build)"
fi

echo ""
echo "📋 Process Tree:"
PIDS=$(printf '%s\n' "$APP_PID" "$SIDECAR_PID" | grep -v '^$' | paste -sd, -)
if [ -n "$PIDS" ]; then
    pstree -p "$APP_PID" 2>/dev/null || ps -fp "$PIDS"
fi

echo ""
echo "📁 Binaries Check:"
OS=$(uname -s)
ARCH=$(uname -m)

if [ "$OS" = "Darwin" ]; then
    if [ "$ARCH" = "arm64" ]; then
        SUFFIX="aarch64-apple-darwin"
    else
        SUFFIX="x86_64-apple-darwin"
    fi
    LOG_DIR="$HOME/Library/Logs/com.studiosound.app"
elif [ "$OS" = "Linux" ]; then
    SUFFIX="x86_64-unknown-linux-gnu"
    # Matches Tauri v2 app_log_dir() on Linux (used by logging.rs / supervisor.rs).
    LOG_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/com.studiosound.app/logs"
else
    SUFFIX="x86_64-pc-windows-msvc.exe"
    LOG_DIR="${LOCALAPPDATA:-${APPDATA:-}}/com.studiosound.app/logs"
fi

if [ -f "app/src-tauri/binaries/ffprobe-$SUFFIX" ]; then
    echo "✅ ffprobe binary exists"
else
    echo "❌ ffprobe binary missing (expected ffprobe-$SUFFIX)"
    FAILED=1
fi

if [ -f "app/src-tauri/binaries/studio-sidecar-$SUFFIX" ]; then
    echo "✅ sidecar binary exists"
else
    echo "❌ sidecar binary missing (expected studio-sidecar-$SUFFIX)"
    FAILED=1
fi

echo ""
echo "📝 Recent Logs:"
if [ -d "$LOG_DIR" ]; then
    echo "✅ Log directory exists at $LOG_DIR"
    # The Rust app log rolls daily as `tauri.log.YYYY-MM-DD` (tracing-appender),
    # so a bare *.log glob misses it; include tauri.log* explicitly.
    LATEST_LOG=$(ls -t "$LOG_DIR"/tauri.log* "$LOG_DIR"/*.log 2>/dev/null | head -1)
    if [ -n "$LATEST_LOG" ]; then
        echo "   Latest log: $(basename "$LATEST_LOG")"
        echo "   Last 3 lines:"
        tail -3 "$LATEST_LOG" | sed 's/^/   /'
    fi
    # Regression guard for the exact bug this script accompanies: a reactor-less
    # panic at startup. Fail non-zero if any current log carries the signature.
    if grep -rIl -e "there is no reactor running" -e "panicked" "$LOG_DIR" >/dev/null 2>&1; then
        echo "❌ Panic / 'no reactor running' found in logs:"
        grep -rIn -e "there is no reactor running" -e "panicked" "$LOG_DIR" | sed 's/^/   /'
        FAILED=1
    else
        echo "✅ No panic / reactor errors in logs"
    fi
else
    echo "⚠️  Log directory not found (might be created after first IPC call)"
fi

echo ""
if [ "$FAILED" -ne 0 ]; then
    echo "❌ Studio Sound verification FAILED (see ❌ items above)"
    exit 1
fi

echo "🎉 Studio Sound is running correctly!"
echo ""
echo "📱 Next Steps (manual IPC ping/pong check):"
echo "   1. Focus the Studio Sound app window"
echo "   2. Press Cmd+Shift+D to open Diagnostics"
echo "   3. Click 'Ping Sidecar' to test IPC"
echo "   4. Try probing a video file"
echo ""
echo "📖 See TEST_GUIDE.md for full testing instructions"
exit 0
