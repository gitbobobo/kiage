#!/usr/bin/env bash
# 交叉编译并同步 extension/ 到已挂载的 Kindle KUAL 扩展目录。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/extension"

detect_kindle_extensions() {
	if [[ -n "${KINDLE_EXT:-}" ]]; then
		echo "$KINDLE_EXT"
		return 0
	fi
	if [[ -d /Volumes/Kindle/extensions ]]; then
		echo /Volumes/Kindle/extensions
		return 0
	fi
	local vol
	for vol in /Volumes/*/extensions; do
		if [[ -d "$vol" ]]; then
			echo "$vol"
			return 0
		fi
	done
	return 1
}

EXT_ROOT="$(detect_kindle_extensions)" || {
	echo "error: 未找到 Kindle 扩展目录。请 USB 连接设备并确保已挂载，或设置 KINDLE_EXT=/path/to/extensions" >&2
	exit 1
}

DEST="$EXT_ROOT/kiage"
echo "==> build kindle binary"
make -C "$ROOT" build-kindle

echo "==> deploy to $DEST"
mkdir -p "$DEST"

rsync -a \
	--exclude '.DS_Store' \
	--exclude '._*' \
	--exclude 'cache/' \
	--exclude 'etc/config.json' \
	--exclude 'etc/import/' \
	"$SRC/" "$DEST/"

if [[ -f "$SRC/etc/config.json" && "${KIAGE_DEPLOY_CONFIG:-}" == "1" ]]; then
	cp "$SRC/etc/config.json" "$DEST/etc/config.json"
	echo "    synced etc/config.json from local (KIAGE_DEPLOY_CONFIG=1)"
elif [[ ! -f "$DEST/etc/config.json" ]]; then
	if [[ -f "$SRC/etc/config.json" ]]; then
		cp "$SRC/etc/config.json" "$DEST/etc/config.json"
		echo "    copied local etc/config.json"
	else
		cp "$DEST/etc/config.json.example" "$DEST/etc/config.json"
		echo "    created etc/config.json from example (请配置 token)"
	fi
fi

mkdir -p "$DEST/cache" "$DEST/etc/import"
chmod +x "$DEST/bin/kiage" "$DEST/bin/"*.sh
if [[ -x "$DEST/bin/fbink" ]]; then
	echo "    fbink: $DEST/bin/fbink"
else
	echo "warn: 未捆绑 fbink，设备需有 /mnt/us/libkh/bin/fbink 或 KOReader" >&2
fi

if [[ ! -f "$DEST/fonts/NotoSansSC-Regular.otf" && ! -f "$DEST/fonts/NotoSansSC-Regular.ttf" ]]; then
	echo "warn: 未找到中文字体，Kindle 上中文可能显示异常。本地执行: make fonts" >&2
fi

echo "==> done"
echo "    KUAL: 点击 Kiage 启动看板"
echo "    日志: $DEST/cache/kiage.log"
echo "    配置: $DEST/etc/config.json 或 USB 写入 etc/import/token"
