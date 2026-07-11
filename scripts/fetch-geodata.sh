#!/bin/sh
# 下载 Mihomo geo 数据库到 internal/geodata/，供编译嵌入与发布打包使用。
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/internal/geodata"

GEOIP_URL="${GEOIP_URL:-https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb}"
GEOSITE_URL="${GEOSITE_URL:-https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat}"

MIN_GEOIP_SIZE=1048576    # 1 MiB
MIN_GEOSITE_SIZE=1048576  # 1 MiB

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "error: missing command: $1" >&2
		exit 1
	}
}

fetch_if_needed() {
	url="$1"
	file="$2"
	min_size="$3"

	if [ -f "$file" ]; then
		size="$(wc -c < "$file" | tr -d ' ')"
		if [ "$size" -ge "$min_size" ]; then
			echo ">> skip $(basename "$file") (already present, ${size} bytes)"
			return 0
		fi
		echo ">> re-download $(basename "$file") (existing file too small: ${size} bytes)"
	fi

	tmp="${file}.download"
	echo ">> download $(basename "$file") ..."
	curl -fsSL --retry 3 --retry-delay 2 -o "$tmp" "$url"
	size="$(wc -c < "$tmp" | tr -d ' ')"
	if [ "$size" -lt "$min_size" ]; then
		rm -f "$tmp"
		echo "error: $(basename "$file") too small after download (${size} bytes)" >&2
		exit 1
	fi
	mv "$tmp" "$file"
	echo ">> saved $(basename "$file") (${size} bytes)"
}

need_cmd curl
need_cmd wc

mkdir -p "$DEST"
fetch_if_needed "$GEOIP_URL" "$DEST/geoip.metadb" "$MIN_GEOIP_SIZE"
fetch_if_needed "$GEOSITE_URL" "$DEST/geosite.dat" "$MIN_GEOSITE_SIZE"
echo ">> geodata ready in $DEST"
