#!/bin/bash
# Quick verification that Studio Sound is running correctly

echo "🔍 Checking Studio Sound App Status..."
echo ""

# Check if Tauri app is running
APP_PID=$(ps aux | grep "studio-sound-app" | grep -v grep | awk '{print $2}')
if [ -n "$APP_PID" ]; then
    echo "✅ Tauri App Running (PID: $APP_PID)"
else
    echo "❌ Tauri App NOT running"
    exit 1
fi

# Check if Go sidecar is running
SIDECAR_PID=$(ps aux | grep "studio-sidecar serve" | grep -v grep | awk '{print $2}')
if [ -n "$SIDECAR_PID" ]; then
    echo "✅ Go Sidecar Running (PID: $SIDECAR_PID)"
else
    echo "❌ Go Sidecar NOT running"
    exit 1
fi

# Check if Vite dev server is running
VITE_PID=$(ps aux | grep "vite.*1420" | grep -v grep | awk '{print $2}')
if [ -n "$VITE_PID" ]; then
    echo "✅ Vite Dev Server Running (PID: $VITE_PID)"
else
    echo "⚠️  Vite Dev Server NOT running (might be normal if using production build)"
fi

echo ""
echo "📋 Process Tree:"
pstree -p $APP_PID 2>/dev/null || ps -f $APP_PID $SIDECAR_PID

echo ""
echo "📁 Binaries Check:"
if [ -f "app/src-tauri/binaries/ffprobe-aarch64-apple-darwin" ]; then
    echo "✅ ffprobe binary exists"
else
    echo "❌ ffprobe binary missing"
fi

if [ -f "app/src-tauri/binaries/studio-sidecar-aarch64-apple-darwin" ]; then
    echo "✅ sidecar binary exists"
else
    echo "❌ sidecar binary missing"
fi

echo ""
echo "📝 Recent Logs:"
LOG_DIR="$HOME/Library/Logs/com.studiosound.app"
if [ -d "$LOG_DIR" ]; then
    echo "✅ Log directory exists at $LOG_DIR"
    LATEST_LOG=$(ls -t "$LOG_DIR"/*.log 2>/dev/null | head -1)
    if [ -n "$LATEST_LOG" ]; then
        echo "   Latest log: $(basename $LATEST_LOG)"
        echo "   Last 3 lines:"
        tail -3 "$LATEST_LOG" | sed 's/^/   /'
    fi
else
    echo "⚠️  Log directory not found (might be created after first IPC call)"
fi

echo ""
echo "🎉 Studio Sound is running correctly!"
echo ""
echo "📱 Next Steps:"
echo "   1. Focus the Studio Sound app window"
echo "   2. Press Cmd+Shift+D to open Diagnostics"
echo "   3. Click 'Ping Sidecar' to test IPC"
echo "   4. Try probing a video file"
echo ""
echo "📖 See TEST_GUIDE.md for full testing instructions"
