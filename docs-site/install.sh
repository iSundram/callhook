#!/usr/bin/env bash
# callhook installer — https://callhook.github.io/install.sh
#
#   curl -fsSL https://callhook.github.io/install.sh | bash
#
# Downloads the latest callhook release for your platform and puts the
# single binary (with the embedded web app) in ./callhook/.
set -euo pipefail

REPO="iSundram/callhook"
INSTALL_DIR="${1:-callhook}"

# --- detect platform ---
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "unsupported OS: $(uname -s) (linux and darwin only)" >&2; exit 1 ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported architecture: $ARCH (amd64 and arm64 only)" >&2; exit 1 ;;
esac

# --- find latest release asset ---
ASSET="callhook_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

echo "callhook — event-driven AI phone calls"
echo "downloading ${ASSET} ..."

mkdir -p "$INSTALL_DIR"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" | tar -xz -C "$INSTALL_DIR"
else
  wget -qO- "$URL" | tar -xz -C "$INSTALL_DIR"
fi
BIN="${INSTALL_DIR}/callhook_${OS}_${ARCH}/callhook"
if [ ! -f "$BIN" ]; then
  echo "download failed — asset not found: $URL" >&2
  exit 1
fi
chmod +x "$BIN" "${INSTALL_DIR}/callhook_${OS}_${ARCH}/callhookctl"

cat <<EOF

installed: ${BIN}

run it (dry-run, no API key needed):

  ./${INSTALL_DIR}/callhook_${OS}_${ARCH}/callhook

then open the war room:

  http://localhost:8080

docs: https://callhook.github.io
EOF
