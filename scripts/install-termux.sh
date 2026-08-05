#!/data/data/com.termux/files/usr/bin/sh
# EncomDB — one-command Termux installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/11fsociety/encomdb/main/scripts/install-termux.sh | sh
#
# Optional env inputs (set before piping to sh):
#   ENCOMDB_TUNNEL_SUBDOMAIN=asmitdb          request a fixed serveo subdomain
#   ENCOMDB_TUNNEL_REGISTRY_URL=<url>         defaults to encomportal.vercel.app
#   ENCOMDB_TUNNEL_REGISTRY_TOKEN=<hex>       optional shared secret
#
# What it does:
#   1. Installs pkg deps (git, golang, openssh, termux-api).
#   2. Clones (or updates) the repo into ~/encomdb.
#   3. Builds the encomdb binary from source.
#   4. Writes a Termux:Boot supervisor that survives reboots (with env baked in).
#   5. Starts encomdb + the serveo.net tunnel.
set -eu

REPO_URL="${ENCOMDB_REPO:-https://github.com/11fsociety/encomdb.git}"
INSTALL_DIR="${ENCOMDB_HOME:-$HOME/encomdb}"

# Default the tunnel registry to the public EncomPortal deployment so a
# fresh phone auto-registers its serveo URL with zero config.
: "${ENCOMDB_TUNNEL_REGISTRY_URL:=https://encomportal.vercel.app/api/tunnel}"

echo "[encomdb] installing pkg deps…"
yes | pkg update >/dev/null 2>&1 || true
yes | pkg install -y git golang openssh termux-api >/dev/null

if [ -d "$INSTALL_DIR/.git" ]; then
  echo "[encomdb] updating repo at $INSTALL_DIR…"
  cd "$INSTALL_DIR"
  git pull --ff-only
else
  echo "[encomdb] cloning repo to $INSTALL_DIR…"
  git clone --depth=1 "$REPO_URL" "$INSTALL_DIR"
  cd "$INSTALL_DIR"
fi

echo "[encomdb] building binary (2-5 min on-phone)…"
GOFLAGS="-trimpath" go build -ldflags="-s -w" -o "$INSTALL_DIR/bin/encomdb" ./cmd/encomdb

# Build the env prelude that goes into every boot script.
ENV_PRELUDE=""
if [ "${ENCOMDB_TUNNEL_SUBDOMAIN:-}" != "" ]; then
  ENV_PRELUDE="${ENV_PRELUDE}export ENCOMDB_TUNNEL_SUBDOMAIN='${ENCOMDB_TUNNEL_SUBDOMAIN}'\n"
fi
if [ "${ENCOMDB_TUNNEL_REGISTRY_URL:-}" != "" ]; then
  ENV_PRELUDE="${ENV_PRELUDE}export ENCOMDB_TUNNEL_REGISTRY_URL='${ENCOMDB_TUNNEL_REGISTRY_URL}'\n"
fi
if [ "${ENCOMDB_TUNNEL_REGISTRY_TOKEN:-}" != "" ]; then
  ENV_PRELUDE="${ENV_PRELUDE}export ENCOMDB_TUNNEL_REGISTRY_TOKEN='${ENCOMDB_TUNNEL_REGISTRY_TOKEN}'\n"
fi

# Termux:Boot supervisor.
if [ -d "$HOME/.termux/boot" ] || command -v termux-wake-lock >/dev/null 2>&1; then
  mkdir -p "$HOME/.termux/boot"
  {
    echo '#!/data/data/com.termux/files/usr/bin/sh'
    echo 'termux-wake-lock'
    printf "%b" "$ENV_PRELUDE"
    echo "cd $INSTALL_DIR"
    echo "exec ./bin/encomdb serve --http=0.0.0.0:8090 >> $INSTALL_DIR/encomdb.log 2>&1"
  } > "$HOME/.termux/boot/start-encomdb"
  chmod +x "$HOME/.termux/boot/start-encomdb"
  echo "[encomdb] Termux:Boot script installed at ~/.termux/boot/start-encomdb"
fi

# Kill any previous instance.
pkill -f 'bin/encomdb serve' 2>/dev/null || true

termux-wake-lock 2>/dev/null || true

echo ""
echo "[encomdb] starting server…"
echo "[encomdb] the serveo.net PUBLIC URL will appear below within ~10 seconds."
echo "[encomdb] press Ctrl+C to stop."
echo ""

cd "$INSTALL_DIR"
exec ./bin/encomdb serve --http=0.0.0.0:8090
