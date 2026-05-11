#!/bin/sh
# easyinfra installer
# Usage: sh install.sh [--version vX.Y.Z] [--install-dir <path>] [--dry-run] [--help]
#
# Environment variables:
#   EASYINFRA_VERSION  override version (default: latest)
#   INSTALL_DIR        override install directory

set -eu

REPO="guneet/easyinfra"
BINARY="easyinfra"
VERSION="${EASYINFRA_VERSION:-latest}"
INSTALL_DIR_OVERRIDE="${INSTALL_DIR:-}"
DRY_RUN=0

usage() {
    cat <<EOF
Usage: sh install.sh [OPTIONS]

Install the easyinfra CLI.

Options:
  --version vX.Y.Z      Install a specific version (default: latest)
  --install-dir PATH    Install to PATH (default: /usr/local/bin or ~/.local/bin)
  --dry-run             Print actions without performing them
  --help                Show this help message

Environment variables:
  EASYINFRA_VERSION     Same as --version
  INSTALL_DIR           Same as --install-dir

Examples:
  sh install.sh
  sh install.sh --version v1.2.3
  EASYINFRA_VERSION=v1.2.3 sh install.sh
  sh install.sh --install-dir \$HOME/bin
EOF
}

log() {
    printf '%s\n' "$*"
}

err() {
    printf 'error: %s\n' "$*" >&2
}

die() {
    err "$*"
    exit 1
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --version)
                [ $# -ge 2 ] || die "--version requires an argument"
                VERSION="$2"
                shift 2
                ;;
            --version=*)
                VERSION="${1#--version=}"
                shift
                ;;
            --install-dir)
                [ $# -ge 2 ] || die "--install-dir requires an argument"
                INSTALL_DIR_OVERRIDE="$2"
                shift 2
                ;;
            --install-dir=*)
                INSTALL_DIR_OVERRIDE="${1#--install-dir=}"
                shift
                ;;
            --dry-run)
                DRY_RUN=1
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                err "unknown argument: $1"
                usage >&2
                exit 1
                ;;
        esac
    done
}

detect_os() {
    os_raw=$(uname -s)
    case "$os_raw" in
        Linux)
            OS="Linux"
            ;;
        Darwin)
            OS="Darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*|Windows_NT)
            err "Windows is not supported by this installer."
            err "Please download the Windows release manually from:"
            err "  https://github.com/${REPO}/releases"
            exit 1
            ;;
        *)
            die "unsupported operating system: $os_raw"
            ;;
    esac
}

detect_arch() {
    arch_raw=$(uname -m)
    case "$arch_raw" in
        x86_64|amd64)
            ARCH="x86_64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            die "unsupported architecture: $arch_raw"
            ;;
    esac
}

detect_downloader() {
    if command -v curl >/dev/null 2>&1; then
        DOWNLOADER="curl"
    elif command -v wget >/dev/null 2>&1; then
        DOWNLOADER="wget"
    else
        die "neither curl nor wget is available; please install one"
    fi
}

detect_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        SHA256="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        SHA256="shasum -a 256"
    else
        die "neither sha256sum nor shasum is available; please install one"
    fi
}

determine_install_dir() {
    if [ -n "$INSTALL_DIR_OVERRIDE" ]; then
        TARGET_DIR="$INSTALL_DIR_OVERRIDE"
        return
    fi
    if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
        TARGET_DIR="/usr/local/bin"
    else
        TARGET_DIR="$HOME/.local/bin"
    fi
}

download() {
    url="$1"
    out="$2"
    case "$DOWNLOADER" in
        curl)
            curl -fsSL "$url" -o "$out"
            ;;
        wget)
            wget -q -O "$out" "$url"
            ;;
    esac
}

main() {
    parse_args "$@"
    detect_os
    detect_arch
    detect_downloader
    detect_sha256
    determine_install_dir

    ASSET="${BINARY}_${OS}_${ARCH}.tar.gz"

    if [ "$VERSION" = "latest" ]; then
        BASE_URL="https://github.com/${REPO}/releases/latest/download"
    else
        BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
    fi

    ASSET_URL="${BASE_URL}/${ASSET}"
    CHECKSUMS_URL="${BASE_URL}/checksums.txt"

    log "Detected platform: ${OS}/${ARCH}"
    log "Asset:             ${ASSET}"
    log "Version:           ${VERSION}"
    log "Downloader:        ${DOWNLOADER}"
    log "Install directory: ${TARGET_DIR}"

    if [ "$DRY_RUN" -eq 1 ]; then
        log "[dry-run] would download: ${ASSET_URL}"
        log "[dry-run] would download: ${CHECKSUMS_URL}"
        log "[dry-run] would verify sha256 of ${ASSET}"
        log "[dry-run] would extract ${ASSET}"
        log "[dry-run] would install ${BINARY} to ${TARGET_DIR}/${BINARY}"
        return 0
    fi

    tmpdir=$(mktemp -d 2>/dev/null || mktemp -d -t easyinfra)
    [ -n "$tmpdir" ] && [ -d "$tmpdir" ] || die "failed to create temp directory"
    # shellcheck disable=SC2064
    trap "rm -rf \"$tmpdir\"" EXIT INT TERM HUP

    log "Downloading ${ASSET}..."
    download "$ASSET_URL" "${tmpdir}/${ASSET}" || die "failed to download ${ASSET_URL}"

    log "Downloading checksums..."
    download "$CHECKSUMS_URL" "${tmpdir}/checksums.txt" || die "failed to download checksums.txt"

    log "Verifying sha256..."
    expected=$(grep " ${ASSET}\$" "${tmpdir}/checksums.txt" | awk '{print $1}')
    [ -n "$expected" ] || die "checksum for ${ASSET} not found in checksums.txt"
    actual=$(cd "$tmpdir" && $SHA256 "$ASSET" | awk '{print $1}')
    [ "$expected" = "$actual" ] || die "checksum mismatch: expected $expected, got $actual"

    log "Extracting..."
    (cd "$tmpdir" && tar -xzf "$ASSET") || die "failed to extract ${ASSET}"
    [ -f "${tmpdir}/${BINARY}" ] || die "binary ${BINARY} not found in archive"

    if [ ! -d "$TARGET_DIR" ]; then
        mkdir -p "$TARGET_DIR" || die "failed to create ${TARGET_DIR}"
    fi

    if [ ! -w "$TARGET_DIR" ]; then
        die "install directory not writable: ${TARGET_DIR}
re-run with --install-dir set to a writable directory, e.g.
  sh install.sh --install-dir \$HOME/.local/bin"
    fi

    mv "${tmpdir}/${BINARY}" "${TARGET_DIR}/${BINARY}" || die "failed to install binary"
    chmod +x "${TARGET_DIR}/${BINARY}"

    installed_version=$("${TARGET_DIR}/${BINARY}" version 2>/dev/null || echo "$VERSION")
    log "${BINARY} ${installed_version} installed to ${TARGET_DIR}/${BINARY}"

    case ":$PATH:" in
        *":${TARGET_DIR}:"*)
            ;;
        *)
            log ""
            log "note: ${TARGET_DIR} is not in your PATH."
            log "      add it to your shell rc, e.g.:"
            log "        export PATH=\"${TARGET_DIR}:\$PATH\""
            ;;
    esac
}

main "$@"
