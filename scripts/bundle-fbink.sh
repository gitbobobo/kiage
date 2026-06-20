#!/usr/bin/env bash
# 从已挂载的 Kindle 复制支持图片的 fbink 到 extension/bin/。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/extension/bin/fbink"
KINDLE_ROOT="${KINDLE_ROOT:-/Volumes/Kindle}"

has_image_support() {
	local bin="$1"
	[[ -f "$bin" ]] || return 1
	! strings "$bin" 2>/dev/null | grep -q 'Image support is disabled in this FBInk build'
}

if [[ -x "$DEST" ]] && has_image_support "$DEST"; then
	echo "fbink ok (image support): $DEST"
	file "$DEST"
	exit 0
fi

sources=(
	"$KINDLE_ROOT/libkh/bin/fbink"
	"$KINDLE_ROOT/extensions/MRInstaller/bin/KHF/fbink"
	"$KINDLE_ROOT/koreader/fbink"
)

for src in "${sources[@]}"; do
	if [[ -f "$src" ]] && has_image_support "$src"; then
		mkdir -p "$(dirname "$DEST")"
		cp "$src" "$DEST"
		chmod +x "$DEST"
		echo "copied fbink: $src -> $DEST"
		file "$DEST"
		exit 0
	fi
done

echo "error: 未找到带 Image 支持的 fbink（请安装 Jailbreak Hotfix）" >&2
exit 1
