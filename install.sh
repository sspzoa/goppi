#!/usr/bin/env bash
set -euo pipefail

# Install goppi with Go. Requires Go 1.27+.
#   curl -fsSL https://raw.githubusercontent.com/sspzoa/goppi/main/install.sh | bash

if ! command -v go >/dev/null 2>&1; then
  echo "go is required: https://go.dev/dl/" >&2
  exit 1
fi

go install github.com/sspzoa/goppi/cmd/goppi@latest
echo "installed: $(command -v goppi || echo "$GOPATH/bin/goppi or ~/go/bin/goppi")"
goppi version 2>/dev/null || true
