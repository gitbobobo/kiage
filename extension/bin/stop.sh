#!/bin/sh
ROOT="$(CDPATH= cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

export KIAGE_UI_LOG="$ROOT/cache/kiage.log"

# shellcheck source=/dev/null
. "$ROOT/bin/kindle-ui.sh"

PID=$(pgrep -f "$ROOT/bin/kiage run" | head -1)
if [ -n "$PID" ]; then
	kill -TERM "$PID" 2>/dev/null
	sleep 2
	kill -KILL "$PID" 2>/dev/null
fi
kiage_ui_leave
lipc-set-prop com.lab126.powerd preventScreenSaver 0 2>/dev/null
