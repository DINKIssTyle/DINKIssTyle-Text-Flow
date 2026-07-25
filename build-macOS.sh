#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="DKST Text Flow"
APP_PATH="$ROOT_DIR/bin/$APP_NAME.app"
DEFAULT_IDENTITY="Apple Development: dinki@me.com (48Z2CKZS59)"

usage() {
  cat <<USAGE
Usage: ./build-macOS.sh [--run] [--identity "CODE SIGN IDENTITY"]

Builds DKST Text Flow and signs the .app with a stable codesigning identity.

Options:
  --run             Restart the built app after signing.
  --identity VALUE  Codesign identity. Defaults to:
                    $DEFAULT_IDENTITY

Environment:
  DKST_CODESIGN_IDENTITY  Alternative way to set the signing identity.
USAGE
}

RUN_AFTER_BUILD=0
IDENTITY="${DKST_CODESIGN_IDENTITY:-$DEFAULT_IDENTITY}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bindings|--skip-bindings)
      echo "$1 is deprecated; Wails now keeps generated bindings up to date automatically." >&2
      shift
      ;;
    --run)
      RUN_AFTER_BUILD=1
      shift
      ;;
    --identity)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "--identity requires a value." >&2
        usage >&2
        exit 2
      fi
      IDENTITY="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "$ROOT_DIR"

for tool in go npm security codesign xattr; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Required tool not found: $tool" >&2
    exit 1
  fi
done

if command -v wails3 >/dev/null 2>&1; then
  WAILS_BIN="$(command -v wails3)"
else
  WAILS_BIN="$(go env GOPATH)/bin/wails3"
  if [[ ! -x "$WAILS_BIN" ]]; then
    echo "wails3 was not found in PATH or GOPATH/bin." >&2
    exit 1
  fi
fi

if [[ "$IDENTITY" == "-" ]]; then
  echo "Using ad-hoc codesigning (-)."
elif ! security find-identity -v -p codesigning | grep -Fq "\"$IDENTITY\""; then
  echo "Codesign identity not found: $IDENTITY" >&2
  echo "Available identities:" >&2
  security find-identity -v -p codesigning >&2 || true
  echo "Falling back to ad-hoc codesigning (-)." >&2
  IDENTITY="-"
fi

go test ./...

"$WAILS_BIN" package

if [[ ! -d "$APP_PATH" ]]; then
  echo "Build output not found: $APP_PATH" >&2
  exit 1
fi

RESOURCE_DIR="$APP_PATH/Contents/Resources"
mkdir -p "$RESOURCE_DIR"

if [[ -f "$ROOT_DIR/build/menu_icon.png" ]]; then
  cp "$ROOT_DIR/build/menu_icon.png" "$RESOURCE_DIR/menu_icon.png"
fi

xattr -cr "$APP_PATH"

codesign \
  --force \
  --deep \
  --timestamp=none \
  --sign "$IDENTITY" \
  "$APP_PATH"

codesign --verify --deep --strict --verbose=2 "$APP_PATH"

# Remove the standalone raw binary, leaving only the signed .app bundle
rm -f "$ROOT_DIR/bin/$APP_NAME"

# Update modification time to force macOS Finder to reload the app icon
touch "$APP_PATH"

echo
echo "Signed app:"
codesign -dv --verbose=4 "$APP_PATH" 2>&1 | sed -n '1,28p'

echo
echo "Accessibility target:"
echo "$APP_PATH"

if [[ "$RUN_AFTER_BUILD" -eq 1 ]]; then
  killall "$APP_NAME" 2>/dev/null || true
  open -n "$APP_PATH"
fi
