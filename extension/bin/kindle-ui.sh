#!/bin/sh
# Kindle 全屏：暂停系统 UI（参考 KOReader KUAL 启动路径）。
export KIAGE_AWESOME_STOPPED=no
export KIAGE_PILLOW_DISABLED=no
export KIAGE_FB_DUMP=""

kiage_ui_enter() {
	if [ -r /dev/fb0 ]; then
		KIAGE_FB_DUMP="/var/tmp/kiage-fb.dump"
		cat /dev/fb0 >"$KIAGE_FB_DUMP" 2>/dev/null || KIAGE_FB_DUMP=""
	fi
	if lipc-set-prop com.lab126.pillow disableEnablePillow disable 2>/dev/null; then
		KIAGE_PILLOW_DISABLED=yes
	fi
	if killall -0 awesome 2>/dev/null; then
		killall -STOP awesome 2>/dev/null && export KIAGE_AWESOME_STOPPED=yes
	fi
	# 等待 KUAL / WM 收尾，避免系统界面叠在应用上
	sleep 2
}

kiage_ui_leave() {
	if [ "$KIAGE_AWESOME_STOPPED" = yes ]; then
		killall -CONT awesome 2>/dev/null
		KIAGE_AWESOME_STOPPED=no
	fi
	if [ "$KIAGE_PILLOW_DISABLED" = yes ]; then
		if [ -n "$KIAGE_FB_DUMP" ] && [ -f "$KIAGE_FB_DUMP" ]; then
			cat "$KIAGE_FB_DUMP" >/dev/fb0 2>/dev/null
			rm -f "$KIAGE_FB_DUMP"
		fi
		lipc-set-prop com.lab126.pillow disableEnablePillow enable 2>/dev/null
		KIAGE_PILLOW_DISABLED=no
	fi
}
