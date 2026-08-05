#!/data/data/com.termux/files/usr/bin/sh
# EncomDB — one-command Termux installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/11fsociety/encomdb/main/scripts/install-termux.sh | sh
#
# What it does:
#   1. Installs pkg deps (git, golang, openssh, termux-api).
#   2. Clones (or updates) the repo into ~/encomdb.
#   3. Builds the encomdb binary from source.
#   4. Writes a Termux:Boot supervisor so it survives reboots.
#   5. Starts encomdb + the serveo.net tunnel.
#   6. Prints the public URL when serveo reports it.
set -eu

REPO_URL="${ENCOMDB_REPO:-https://github.com/11fsociety/encomdb.git}"
INSTALL_DIR="${ENCOMDB_HOME:-$HOME/encomdb}"

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

# Termux:Boot supervisor.
if [ -d "$HOME/.termux/boot" ] || command -v termux-wake-lock >/dev/null 2>&1; then
  mkdir -p "$HOME/.termux/boot"
  cat > "$HOME/.termux/boot/start-encomdb" <<EOF
#!/data/data/com.termux/files/usr/bin/sh
termux-wake-lock
cd $INSTALL_DIR
exec ./bin/encomdb serve --http=0.0.0.0:8090 >> $INSTALL_DIR/encomdb.log 2>&1
EOF
  chmod +x "$HOME/.termux/boot/start-encomdb"
  echo "[encomdb] Termux:Boot script installed."
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
