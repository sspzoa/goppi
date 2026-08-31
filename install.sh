#!/usr/bin/env bash
set -euo pipefail

# Install goppi. Prefers a GitHub release binary (checksum-verified).
# Falls back to `go install` when no release matches this machine.
#   curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash
#
# GOPPI_INSTALL_DIR     install prefix (default: ~/.local/bin)
# GOPPI_INSTALL_FROM    "go" to skip the release download
# GOPPI_RELEASE_BASE    override release URL (tests / mirrors).
#                       Non-GitHub bases require a signature unless GOPPI_SKIP_COSIGN=1
# GOPPI_SKIP_COSIGN     1 to skip Sigstore even if a bundle is present
# GOPPI_REQUIRE_COSIGN  1 to fail when the signature bundle is missing

REPO="sspzoa/goppi"
PREFIX="${GOPPI_INSTALL_DIR:-$HOME/.local/bin}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required" >&2
    exit 1
  fi
}

go_major_minor() {
  local ver major minor
  ver="$(go env GOVERSION 2>/dev/null || true)"
  major="$(printf '%s' "$ver" | sed -E 's/^go([0-9]+)\.([0-9]+).*/\1/')"
  minor="$(printf '%s' "$ver" | sed -E 's/^go([0-9]+)\.([0-9]+).*/\2/')"
  printf '%s %s' "$major" "$minor"
}

install_from_go() {
  if ! command -v go >/dev/null 2>&1; then
    echo "no GitHub release binary for this machine, and go is not installed: https://go.dev/dl/" >&2
    exit 1
  fi
  # shellcheck disable=SC2046
  set -- $(go_major_minor)
  if [ -z "${1:-}" ] || [ -z "${2:-}" ] || [ "$1" -lt 1 ] || { [ "$1" -eq 1 ] && [ "$2" -lt 27 ]; }; then
    echo "Go 1.27+ required, found $(go env GOVERSION 2>/dev/null || echo unknown)" >&2
    exit 1
  fi
  go install github.com/sspzoa/goppi/cmd/goppi@latest
  echo "installed: $(command -v goppi || echo "$(go env GOPATH)/bin/goppi")"
  goppi version 2>/dev/null || true
}

file_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

if [ "${GOPPI_INSTALL_FROM:-}" = "go" ]; then
  install_from_go
  exit 0
fi

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "unsupported arch: $arch" >&2
    exit 1
    ;;
esac
case "$os" in
  darwin | linux) ;;
  *)
    echo "unsupported os: $os (release binaries are darwin/linux amd64/arm64)" >&2
    exit 1
    ;;
esac

need_cmd curl
need_cmd tar
need_cmd awk

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

base="${GOPPI_RELEASE_BASE:-https://github.com/${REPO}/releases/latest/download}"
base="${base%/}"
case "$base" in
  "https://github.com/${REPO}/releases/"*) ;;
  *)
    if [ "${GOPPI_SKIP_COSIGN:-}" != "1" ]; then
      GOPPI_REQUIRE_COSIGN=1
    fi
    ;;
esac
if ! curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"; then
  echo "no GitHub release found; falling back to go install" >&2
  install_from_go
  exit 0
fi

verify_sums() {
  if [ "${GOPPI_SKIP_COSIGN:-}" = "1" ]; then
    return 0
  fi
  if curl -fsSL "$base/SHA256SUMS.sigstore.json" -o "$tmp/SHA256SUMS.sigstore.json"; then
    if ! command -v cosign >/dev/null 2>&1; then
      echo "cosign is required to verify SHA256SUMS.sigstore.json" >&2
      echo "install cosign or set GOPPI_SKIP_COSIGN=1" >&2
      return 1
    fi
    cosign verify-blob --yes \
      --bundle "$tmp/SHA256SUMS.sigstore.json" \
      --certificate-identity-regexp 'https://github.com/sspzoa/goppi/' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com \
      "$tmp/SHA256SUMS"
    return $?
  fi
  if [ "${GOPPI_REQUIRE_COSIGN:-}" = "1" ]; then
    echo "SHA256SUMS.sigstore.json missing" >&2
    return 1
  fi
  echo "no signature bundle; SHA256 only" >&2
  return 0
}

if ! verify_sums; then
  echo "release signature check failed" >&2
  exit 1
fi

name="$(awk -v os="$os" -v arch="$arch" '
  $2 ~ ("goppi_.*_" os "_" arch "\\.tar\\.gz$") { print $2; exit }
' "$tmp/SHA256SUMS")"
if [ -z "$name" ]; then
  echo "SHA256SUMS has no goppi_*_${os}_${arch}.tar.gz; falling back to go install" >&2
  install_from_go
  exit 0
fi

if ! curl -fsSL "$base/$name" -o "$tmp/$name"; then
  echo "failed to download $name" >&2
  exit 1
fi

want="$(awk -v f="$name" '$2 == f { print $1; exit }' "$tmp/SHA256SUMS")"
got="$(file_sha256 "$tmp/$name")"
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "checksum mismatch for $name" >&2
  echo "  want $want" >&2
  echo "  got  $got" >&2
  exit 1
fi

tar -xzf "$tmp/$name" -C "$tmp"
bin="$tmp/${name%.tar.gz}"
if [ ! -f "$bin" ]; then
  echo "archive $name did not contain ${name%.tar.gz}" >&2
  exit 1
fi

mkdir -p "$PREFIX"
install -m 0755 "$bin" "$PREFIX/goppi"
echo "installed: $PREFIX/goppi"
if ! command -v goppi >/dev/null 2>&1; then
  echo "add $PREFIX to PATH" >&2
fi
"$PREFIX/goppi" version 2>/dev/null || true
