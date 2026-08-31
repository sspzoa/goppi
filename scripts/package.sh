#!/usr/bin/env bash
set -euo pipefail

# Build signed-release artifacts (same layout as .github/workflows/release.yml).
#   scripts/package.sh [version] [outdir]
# Produces: goppi_<ver>_{darwin,linux}_{amd64,arm64}.tar.gz and SHA256SUMS

root="$(cd "$(dirname "$0")/.." && pwd)"
ver="${1:-${VERSION:-dev}}"
outdir="${2:-$root/dist}"
only_os="${3:-}"
only_arch="${4:-}"

mkdir -p "$outdir"
# bare version, no leading v
ver="${ver#v}"

built=()
for os in darwin linux; do
  if [ -n "$only_os" ] && [ "$os" != "$only_os" ]; then
    continue
  fi
  for arch in amd64 arm64; do
    if [ -n "$only_arch" ] && [ "$arch" != "$only_arch" ]; then
      continue
    fi
    name="goppi_${ver}_${os}_${arch}"
    rm -f "$outdir/${name}.tar.gz" "$outdir/${name}"
    out="$outdir/$name"
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build \
      -C "$root" \
      -ldflags "-s -w -X github.com/sspzoa/goppi/internal/config.Version=${ver}" \
      -o "$out" ./cmd/goppi
    tar -C "$outdir" -czf "${out}.tar.gz" "$name"
    rm -f "$out"
    built+=("${name}.tar.gz")
  done
done

if [ ${#built[@]} -eq 0 ]; then
  echo "no release artifacts built" >&2
  exit 1
fi

rm -f "$outdir/SHA256SUMS" "$outdir/SHA256SUMS.sigstore.json"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$outdir" && sha256sum "${built[@]}" > SHA256SUMS)
else
  (cd "$outdir" && shasum -a 256 "${built[@]}" > SHA256SUMS)
fi

echo "$outdir"
ls -1 "$outdir"
