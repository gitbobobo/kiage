#!/bin/sh
# KUAL 入口：启动 Kiage 全屏看板（单实例）。
ROOT="$(CDPATH= cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

export KIAGE_ROOT="$ROOT"
export KIAGE_PORTRAIT=1
export KIAGE_KINDLE_UI=1
export FBINK_NO_SW_ROTA=1

# shellcheck source=/dev/null
. "$ROOT/bin/kindle-ui.sh"

if [ -d /mnt/us/libkh/lib ]; then
	export LD_LIBRARY_PATH="/mnt/us/libkh/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
fi
if [ -d /var/local/kmc/lib ]; then
	export LD_LIBRARY_PATH="/var/local/kmc/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
fi

if [ -z "${KIAGE_FBINK:-}" ]; then
	for p in \
		/mnt/us/libkh/bin/fbink \
		/var/local/kmc/bin/fbink \
		/mnt/usr/usbnet/bin/fbink \
		/usr/bin/fbink \
		/usr/local/bin/fbink \
		"$ROOT/bin/fbink" \
		/mnt/us/koreader/fbink
	do
		if [ -x "$p" ] && ! strings "$p" 2>/dev/null | grep -q 'Image support is disabled in this FBInk build'; then
			export KIAGE_FBINK="$p"
			break
		fi
	done
fi

mkdir -p "$ROOT/cache"
LOG="$ROOT/cache/kiage.log"
export KIAGE_UI_LOG="$LOG"

log_line() {
	echo "$1" >>"$LOG"
}

# 可选：etc/rtc_sec 写入一个秒数即可覆盖 RTC 周期唤醒间隔（用于快速验证休眠联网；
# 删除该文件恢复默认 3600）。
if [ -f "$ROOT/etc/rtc_sec" ]; then
	RTC_SEC="$(tr -dc '0-9' <"$ROOT/etc/rtc_sec")"
	if [ -n "$RTC_SEC" ]; then
		export KIAGE_RTC_SEC="$RTC_SEC"
	fi
fi

if pgrep -f "$ROOT/bin/kiage run" >/dev/null 2>&1; then
	log_line "$(date '+%F %T') [start.sh] already running, skip"
	exit 0
fi

cleanup() {
	log_line "[start.sh] cleanup begin"
	kiage_ui_leave
	lipc-set-prop com.lab126.powerd preventScreenSaver 0 2>/dev/null
	log_line "[start.sh] cleanup done"
}
trap cleanup EXIT INT TERM

if [ "${KIAGE_KEEP_AWAKE:-}" = "1" ]; then
	lipc-set-prop com.lab126.powerd preventScreenSaver 1 2>/dev/null
fi

log_line "=== kiage start $(date) ==="
log_line "ROOT=$ROOT"
log_line "PWD=$(pwd)"
log_line "KIAGE_FBINK=${KIAGE_FBINK:-not set}"
log_line "KIAGE_PORTRAIT=1"
log_line "FBINK_NO_SW_ROTA=1"
log_line "LD_LIBRARY_PATH=${LD_LIBRARY_PATH:-}"

kiage_ui_enter
log_line "[start.sh] kindle ui entered pillow=$KIAGE_PILLOW_DISABLED awesome=$KIAGE_AWESOME_STOPPED"

./bin/kiage run >>"$LOG" 2>&1
EC=$?
log_line "=== kiage exit $EC $(date) ==="
exit "$EC"
