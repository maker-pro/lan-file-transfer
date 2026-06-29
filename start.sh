#!/usr/bin/env sh

# Cross-platform launcher:
# - macOS / Linux: run ./start.sh
# - Windows: run ./start.sh in Git Bash, MSYS2, Cygwin, or WSL
# - Native Windows users can run start.bat

set -eu

APP_NAME="lan-file-transfer"
PORT="${PORT:-0}"
UPLOAD_DIR="${UPLOAD_DIR:-uploads}"

# The first argument can be used as the port, for example: ./start.sh 9090
if [ "${1:-}" != "" ]; then
  PORT="$1"
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR"

OS_NAME=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH_NAME=$(uname -m | tr '[:upper:]' '[:lower:]')

case "$OS_NAME" in
  darwin*)
    GOOS_VALUE="darwin"
    EXE_SUFFIX=""
    ;;
  linux*)
    GOOS_VALUE="linux"
    EXE_SUFFIX=""
    ;;
  mingw*|msys*|cygwin*)
    GOOS_VALUE="windows"
    EXE_SUFFIX=".exe"
    ;;
  *)
    echo "Unsupported OS: $OS_NAME"
    exit 1
    ;;
esac

case "$ARCH_NAME" in
  x86_64|amd64)
    GOARCH_VALUE="amd64"
    ;;
  arm64|aarch64)
    GOARCH_VALUE="arm64"
    ;;
  *)
    echo "Unsupported CPU architecture: $ARCH_NAME"
    exit 1
    ;;
esac

BIN_DIR="$SCRIPT_DIR/bin"
BIN_PATH="$BIN_DIR/${APP_NAME}-${GOOS_VALUE}-${GOARCH_VALUE}${EXE_SUFFIX}"

echo "OS/ARCH: $GOOS_VALUE/$GOARCH_VALUE"
if [ "$PORT" = "0" ]; then
  echo "Port: random available port"
else
  echo "Port: $PORT"
fi
echo "Upload dir: $UPLOAD_DIR"

if [ ! -x "$BIN_PATH" ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Go was not found. Install Go, or put a compiled binary here:"
    echo "$BIN_PATH"
    exit 1
  fi

  echo "Binary not found. Building..."
  mkdir -p "$BIN_DIR"
  GOOS="$GOOS_VALUE" GOARCH="$GOARCH_VALUE" go build -buildvcs=false -o "$BIN_PATH" .
  chmod +x "$BIN_PATH" 2>/dev/null || true
fi

echo "Starting server..."
exec "$BIN_PATH" -port "$PORT" -dir "$UPLOAD_DIR"
