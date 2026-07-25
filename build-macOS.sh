#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_NAME="DKST Text Flow"
BUILD_BIN_DIR="bin"
APP_PATH="$ROOT_DIR/$BUILD_BIN_DIR/$APP_NAME.app"
ENTITLEMENTS_PATH="$ROOT_DIR/build/darwin/entitlements.plist"
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

cleanup_incomplete_app() {
  local status=$?
  if [[ "$status" -ne 0 && -d "$APP_PATH" ]]; then
    echo "Removing incomplete app bundle after build failure: $APP_PATH" >&2
    rm -rf "$APP_PATH"
  fi
}

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
export GOOS=darwin
export CGO_ENABLED=1

for tool in go npm xcrun lipo security codesign xattr; do
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

if [[ ! -f "$ENTITLEMENTS_PATH" ]]; then
  echo "Entitlements file not found: $ENTITLEMENTS_PATH" >&2
  exit 1
fi

if [[ "$IDENTITY" == "-" ]]; then
  echo "Using ad-hoc codesigning (-)."
else
  # Resolve a certificate name to its SHA-1 hash. This avoids codesign's
  # "ambiguous identity" error when an expired certificate has the same name.
  RESOLVED_IDENTITY="$(
    security find-identity -v -p codesigning |
      sed -n 's/^[[:space:]]*[0-9][0-9]*) \([0-9A-Fa-f]\{40\}\) "\(.*\)"$/\1	\2/p' |
      while IFS=$'\t' read -r certificate_hash certificate_name; do
        if [[ "$IDENTITY" == "$certificate_hash" || "$IDENTITY" == "$certificate_name" ]]; then
          printf '%s' "$certificate_hash"
          break
        fi
      done
  )"

  if [[ -n "$RESOLVED_IDENTITY" ]]; then
    echo "Using codesigning identity: $IDENTITY ($RESOLVED_IDENTITY)"
    IDENTITY="$RESOLVED_IDENTITY"
  else
    echo "Valid codesign identity not found: $IDENTITY" >&2
    echo "Available identities:" >&2
    security find-identity -v -p codesigning >&2 || true
    echo "Falling back to ad-hoc codesigning (-)." >&2
    IDENTITY="-"
  fi
fi

go test ./...

trap cleanup_incomplete_app EXIT

"$WAILS_BIN" package BIN_DIR="$BUILD_BIN_DIR"

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
  --options runtime \
  --entitlements "$ENTITLEMENTS_PATH" \
  --timestamp=none \
  --sign "$IDENTITY" \
  "$APP_PATH"

codesign --verify --deep --strict --verbose=2 "$APP_PATH"

if ! codesign -d --entitlements - "$APP_PATH" 2>&1 |
  grep -Fq "com.apple.security.cs.disable-library-validation"; then
  echo "Required ONNX Runtime library-validation entitlement is missing." >&2
  exit 1
fi

# Remove the standalone raw binary, leaving only the signed .app bundle
rm -f "$ROOT_DIR/$BUILD_BIN_DIR/$APP_NAME"

# Update modification time to force macOS Finder to reload the app icon
touch "$APP_PATH"

# The app is now signed and verified. Keep it if an optional launch fails.
trap - EXIT

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
