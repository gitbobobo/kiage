#!/bin/sh
# Kindle 全屏：暂停系统 UI（参考 KOReader koreader.sh KUAL 路径）。
export KIAGE_AWESOME_STOPPED=no
export KIAGE_PILLOW_DISABLED=no
export KIAGE_FB_DUMP=""
export KIAGE_KEYS_LOCKED=no

KIAGE_UI_STATE="/var/tmp/kiage-ui.state"

kiage_ui_log() {
	[ -n "${KIAGE_UI_LOG:-}" ] && echo "$(date '+%F %T') [kindle-ui] $*" >>"$KIAGE_UI_LOG"
}

kiage_keys_unlock() {
	[ -e /proc/keypad ] && echo unlock >/proc/keypad 2>/dev/null
	[ -e /proc/fiveway ] && echo unlock >/proc/fiveway 2>/dev/null
}

kiage_keys_lock() {
	if [ -e /proc/keypad ] && echo lock >/proc/keypad 2>/dev/null; then
		KIAGE_KEYS_LOCKED=yes
	fi
	if [ -e /proc/fiveway ] && echo lock >/proc/fiveway 2>/dev/null; then
		KIAGE_KEYS_LOCKED=yes
	fi
}

kiage_fsr_keypad_enable() {
	lipc-set-prop com.lab126.deviced fsrkeypadEnable 1 2>/dev/null
	lipc-set-prop com.lab126.deviced fsrkeypadPrevEnable 1 2>/dev/null
	lipc-set-prop com.lab126.deviced fsrkeypadNextEnable 1 2>/dev/null
}

kiage_ui_save_state() {
	{
		echo "KIAGE_PILLOW_DISABLED=$KIAGE_PILLOW_DISABLED"
		echo "KIAGE_AWESOME_STOPPED=$KIAGE_AWESOME_STOPPED"
		echo "KIAGE_KEYS_LOCKED=$KIAGE_KEYS_LOCKED"
		echo "KIAGE_FB_DUMP=$KIAGE_FB_DUMP"
		echo "KIAGE_FBINK=${KIAGE_FBINK:-}"
	} >"$KIAGE_UI_STATE"
}

kiage_ui_load_state() {
	if [ -f "$KIAGE_UI_STATE" ]; then
		# shellcheck disable=SC1090
		. "$KIAGE_UI_STATE"
		rm -f "$KIAGE_UI_STATE"
		return 0
	fi
	# start.sh 同 shell 内可能仍保留 enter 时的变量
	[ "$KIAGE_PILLOW_DISABLED" = yes ] || [ "$KIAGE_AWESOME_STOPPED" = yes ]
}

kiage_fb_refresh() {
	fbink_bin="${1:-${KIAGE_FBINK:-}}"
	[ -n "$fbink_bin" ] && [ -x "$fbink_bin" ] || return 0
	"$fbink_bin" -q -f -W GC16 2>/dev/null
}

kiage_keypad_redraw() {
	[ -e /proc/keypad ] && echo send 139 >/proc/keypad 2>/dev/null
	[ -e /proc/keypad ] && echo send 139 >/proc/keypad 2>/dev/null
}

kiage_ui_enter() {
	kiage_keys_unlock
	if [ -r /dev/fb0 ]; then
		KIAGE_FB_DUMP="/var/tmp/kiage-fb.dump"
		cat /dev/fb0 >"$KIAGE_FB_DUMP" 2>/dev/null || KIAGE_FB_DUMP=""
	fi
	if lipc-set-prop com.lab126.pillow disableEnablePillow disable 2>/dev/null; then
		export KIAGE_PILLOW_DISABLED=yes
	fi
	if killall -0 awesome 2>/dev/null; then
		killall -STOP awesome 2>/dev/null && export KIAGE_AWESOME_STOPPED=yes
	fi
	# EVIOCGRAB 在部分机型上失败，改由 proc 拦截系统按键路由。
	kiage_keys_lock
	kiage_ui_save_state
	kiage_ui_log "enter pillow=$KIAGE_PILLOW_DISABLED awesome=$KIAGE_AWESOME_STOPPED keys_locked=$KIAGE_KEYS_LOCKED fb_dump=${KIAGE_FB_DUMP:-none}"
	# 等待 KUAL / WM 收尾，避免系统界面叠在应用上
	sleep 2
}

kiage_ui_leave() {
	if ! kiage_ui_load_state; then
		kiage_ui_log "leave skipped (no state)"
		return 0
	fi

	kiage_ui_log "leave begin pillow=$KIAGE_PILLOW_DISABLED awesome=$KIAGE_AWESOME_STOPPED keys_locked=$KIAGE_KEYS_LOCKED"

	kiage_keys_unlock

	if [ "$KIAGE_AWESOME_STOPPED" = yes ]; then
		killall -CONT awesome 2>/dev/null
		kiage_ui_log "awesome resumed"
		KIAGE_AWESOME_STOPPED=no
	fi

	if [ "$KIAGE_PILLOW_DISABLED" = yes ]; then
		# 恢复进入前的主页 framebuffer（含状态栏），再全刷到 e-ink；启动方向改由加速度计探测，不依赖 currentRota。
		if [ -n "$KIAGE_FB_DUMP" ] && [ -f "$KIAGE_FB_DUMP" ]; then
			cat "$KIAGE_FB_DUMP" >/dev/fb0 2>/dev/null
			rm -f "$KIAGE_FB_DUMP"
			kiage_ui_log "fb dump restored"
		fi
		if kiage_fb_refresh "$KIAGE_FBINK"; then
			kiage_ui_log "fbink GC16 refresh ok"
		else
			kiage_ui_log "fbink GC16 refresh skipped or failed"
		fi
		lipc-set-prop com.lab126.pillow disableEnablePillow enable 2>/dev/null
		kiage_ui_log "pillow enabled"
		kiage_fsr_keypad_enable
		lipc-set-prop com.lab126.appmgrd start app://com.lab126.booklet.home 2>/dev/null
		kiage_ui_log "home booklet started"
		usleep 750000 2>/dev/null || sleep 1
		kiage_fb_refresh "$KIAGE_FBINK"
		kiage_keypad_redraw
		kiage_ui_log "leave done"
		KIAGE_PILLOW_DISABLED=no
	fi
}
