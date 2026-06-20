#!/bin/sh
cd "$(dirname "$0")/.." || exit 1
PID=$(pgrep -f './bin/kiage run' | head -1)
if [ -n "$PID" ]; then
  kill -TERM "$PID" 2>/dev/null
  sleep 2
  kill -KILL "$PID" 2>/dev/null
fi
lipc-set-prop com.lab126.powerd preventScreenSaver 0 2>/dev/null
