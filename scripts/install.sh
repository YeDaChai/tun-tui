#!/bin/sh
# tun-tui 一键安装：下载最新 Release 并安装到 PATH
#
#   curl -fsSL https://raw.githubusercontent.com/YeDaChai/tun-tui/main/scripts/install.sh | sh
#
# 可选环境变量:
#   INSTALL_DIR   安装目录，默认 /usr/local/bin（无权限时自动用 ~/.local/bin）
#   VERSION       指定版本，如 0.1.5（默认 latest）
#   GITHUB_REPO   仓库，默认 YeDaChai/tun-tui

set -eu

APP=tun-tui
GITHUB_REPO="${GITHUB_REPO:-YeDaChai/tun-tui}"
VERSION="${VERSION:-}"

usage() {
	cat <<EOF
用法: install.sh [选项]

选项:
  -d, --dir DIR     安装目录（默认 /usr/local/bin 或 ~/.local/bin）
  -v, --version VER 指定版本（默认 latest）
  -h, --help        显示帮助

示例:
  curl -fsSL https://raw.githubusercontent.com/YeDaChai/tun-tui/main/scripts/install.sh | sh
  curl -fsSL ... | sh -s -- -d ~/.local/bin
EOF
}

log() { printf '>> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"
}

detect_platform() {
	os="$(uname -s)"
	arch="$(uname -m)"

	case "$os" in
	Darwin) platform_os=macos ;;
	Linux) platform_os=linux ;;
	*) die "不支持的操作系统: $os（仅支持 macOS / Linux）" ;;
	esac

	case "$arch" in
	x86_64 | amd64)
		case "$platform_os" in
		macos) platform_label=intel-x86_64 ;;
		*) platform_label=x86_64 ;;
		esac
		;;
	arm64 | aarch64)
		case "$platform_os" in
		macos) platform_label=apple-silicon-arm64 ;;
		*) platform_label=arm64 ;;
		esac
		;;
	*) die "不支持的 CPU 架构: $arch" ;;
	esac

	printf '%s-%s' "$platform_os" "$platform_label"
}

resolve_install_dir() {
	if [ -n "${INSTALL_DIR:-}" ]; then
		printf '%s' "$INSTALL_DIR"
		return
	fi

	if [ -w /usr/local/bin ] 2>/dev/null || [ ! -e /usr/local/bin ]; then
		printf '/usr/local/bin'
	else
		printf '%s' "${HOME}/.local/bin"
	fi
}

fetch_latest_version() {
	need_cmd curl
	json=""
	if json="$(curl -fsSL --http1.1 \
		-H "Accept: application/vnd.github+json" \
		"https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null)"; then
		ver="$(printf '%s\n' "$json" \
			| sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
			| head -n1 \
			| sed 's/^v//')"
		if [ -n "$ver" ]; then
			printf '%s' "$ver"
			return
		fi
	fi

	# API 失败或解析失败时，跟随 releases/latest 重定向
	ver="$(curl -fsSIL --http1.1 "https://github.com/${GITHUB_REPO}/releases/latest" 2>/dev/null \
		| sed -n 's/^[Ll]ocation:.*\/tag\/v\{0,1\}//p' \
		| tr -d '\r\n' \
		| head -n1)"
	if [ -n "$ver" ]; then
		printf '%s' "$ver"
		return
	fi

	die "无法获取最新版本（请检查网络，或用 -v 指定版本）"
}

fetch_release_tag() {
	if [ -n "$VERSION" ]; then
		printf 'v%s' "$VERSION"
	else
		ver="$(fetch_latest_version)"
		printf 'v%s' "$ver"
	fi
}

version_from_tag() {
	printf '%s' "$1" | sed 's/^v//'
}

verify_checksum() {
	sum_file="$1"
	binary_name="$2"
	sum_dir="$(dirname "$sum_file")"
	sum_base="$(basename "$sum_file")"

	if command -v sha256sum >/dev/null 2>&1; then
		check_cmd="sha256sum -c"
	elif command -v shasum >/dev/null 2>&1; then
		check_cmd="shasum -a 256 -c"
	else
		die "缺少 sha256sum 或 shasum，无法校验"
	fi

	(
		cd "$sum_dir"
		grep " ${binary_name}\$" "$sum_base" | $check_cmd -
	) || die "SHA256 校验失败"
}

install_binary() {
	install_dir="$1"
	tag="$2"
	platform="$3"
	ver="$(version_from_tag "$tag")"
	archive="${APP}-${ver}-${platform}.tar.gz"
	base_url="https://github.com/${GITHUB_REPO}/releases/download/${tag}"

	need_cmd curl
	need_cmd tar

	tmpdir="$(mktemp -d)"
	trap 'rm -rf "$tmpdir"' EXIT INT TERM

	log "下载 ${archive} ..."
	curl -fsSL --http1.1 -o "${tmpdir}/${archive}" "${base_url}/${archive}"

	log "下载 SHA256SUMS ..."
	if curl -fsSL --http1.1 -o "${tmpdir}/SHA256SUMS" "${base_url}/SHA256SUMS" 2>/dev/null; then
		verify_checksum "${tmpdir}/SHA256SUMS" "$archive"
	else
		log "跳过校验（未找到 SHA256SUMS）"
	fi

	tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"
	binary="${tmpdir}/${APP}"
	[ -f "$binary" ] || die "压缩包内未找到 ${APP}"

	if [ "$(uname -s)" = Darwin ] && command -v xattr >/dev/null 2>&1; then
		xattr -d com.apple.quarantine "$binary" 2>/dev/null || true
	fi

	mkdir -p "$install_dir"
	target="${install_dir}/${APP}"

	if [ -w "$install_dir" ]; then
		install -m 755 "$binary" "$target"
	else
		need_cmd sudo
		sudo install -m 755 "$binary" "$target"
	fi

	log "已安装 -> ${target} (${ver}, ${platform})"

	case ":${PATH}:" in
	*":${install_dir}:"*) ;;
	*)
		log "请将 ${install_dir} 加入 PATH，例如在 ~/.zshrc 中添加:"
		printf '    export PATH="%s:$PATH"\n' "$install_dir"
		;;
	esac
}

# 解析参数
while [ $# -gt 0 ]; do
	case "$1" in
	-d | --dir)
		shift
		[ $# -gt 0 ] || die "缺少 --dir 参数"
		INSTALL_DIR="$1"
		;;
	-v | --version)
		shift
		[ $# -gt 0 ] || die "缺少 --version 参数"
		VERSION="$1"
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "未知参数: $1"
		;;
	esac
	shift
done

platform="$(detect_platform)"
install_dir="$(resolve_install_dir)"
tag="$(fetch_release_tag)"

log "平台: ${platform}"
log "安装目录: ${install_dir}"
install_binary "$install_dir" "$tag" "$platform"
