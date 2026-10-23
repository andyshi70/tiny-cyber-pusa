#!/usr/bin/env bash
# Build the macOS .app bundle.
#
#   tools/build-macos.sh                 # version from git describe / gateway.Version
#   tools/build-macos.sh 1.3.0           # explicit version
#
# Output: dist/Zen Gate.app (plus a zip for distribution) and dist/zen-gate-darwin,
# the bare binary the auto-updater expects as the release asset name.
set -euo pipefail

cd "$(dirname "$0")/.."

BUNDLE_ID="com.lagcomcom.zen-gate"
APP="Zen Gate"
VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  VERSION="$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
  VERSION="${VERSION:-dev}"
fi

DIST="dist"
APP_DIR="$DIST/$APP.app"
echo "==> building $APP $VERSION"

rm -rf "$APP_DIR"
mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

# ---- binary ------------------------------------------------------------------
# One Mach-O carrying both slices: the app is installed by hand rather than
# through the App Store, so a Mac that cannot run the build host's slice must not
# be a dead end. cgo cross-compiles between the two because the macOS SDK carries
# both.
#
# -H=windowsgui has no macOS counterpart; the bundle has no console window at
# all, and -no-tray still logs to stdout for `zen-gate -no-tray` debugging.
build_universal() { # build_universal <out> <pkg> <ldflags>
  local out="$1" pkg="$2" ldflags="$3"
  local slices=()
  for arch in arm64 amd64; do
    CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" \
      go build -trimpath -ldflags "$ldflags" -o "$out.$arch" "$pkg"
    slices+=("$out.$arch")
  done
  lipo -create "${slices[@]}" -output "$out"
  rm -f "${slices[@]}"
  lipo -info "$out"
}

build_universal "$APP_DIR/Contents/MacOS/zen-gate" ./cmd/zen-gate \
  "-s -w -X zen-gate/internal/gateway.Version=$VERSION -X zen-gate/internal/update.Current=$VERSION"
build_universal "$DIST/zenstats" ./cmd/zenstats "-s -w"

# ---- icon -------------------------------------------------------------------
# iconutil wants the ten sizes in an .iconset; the repo has 16…256 px PNGs, so
# the 512 px pair is upscaled from icon-src.png (1024 px).
echo "==> building icon"
ICONSET="$(mktemp -d)/zen-gate.iconset"
mkdir -p "$ICONSET"
resize() { sips -z "$2" "$2" "$1" --out "$ICONSET/$3" >/dev/null; }
resize assets/icon-16.png  16  icon_16x16.png
resize assets/icon-16.png  32  icon_16x16@2x.png
resize assets/icon-32.png  32  icon_32x32.png
resize assets/icon-32.png  64  icon_32x32@2x.png
resize assets/icon-128.png 128 icon_128x128.png
resize assets/icon-256.png 256 icon_128x128@2x.png
resize assets/icon-256.png 256 icon_256x256.png
resize assets/icon-src.png 512 icon_256x256@2x.png
resize assets/icon-src.png 512 icon_512x512.png
resize assets/icon-src.png 1024 icon_512x512@2x.png
iconutil -c icns "$ICONSET" -o "$APP_DIR/Contents/Resources/zen-gate.icns"

# ---- Info.plist -------------------------------------------------------------
# CFBundleVersion and CFBundleShortVersionString must be 1–3 dot-separated
# integers. A branch name ("master", from workflow_dispatch) or the "dev"
# fallback is not one, and a bundle LaunchServices refuses is worse than a
# generic number — so the plist gets a sanitised version while VERSION stays
# verbatim everywhere else.
BUNDLE_VERSION="$(printf '%s' "$VERSION" | sed 's/[^0-9.]//g')"
BUNDLE_VERSION="${BUNDLE_VERSION:-0.0.0}"

cat > "$APP_DIR/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key><string>$APP</string>
    <key>CFBundleDisplayName</key><string>$APP</string>
    <key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
    <key>CFBundleVersion</key><string>$BUNDLE_VERSION</string>
    <key>CFBundleShortVersionString</key><string>$BUNDLE_VERSION</string>
    <key>CFBundleExecutable</key><string>zen-gate</string>
    <key>CFBundleIconFile</key><string>zen-gate</string>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>LSMinimumSystemVersion</key><string>13.0</string>
    <key>NSHighResolutionCapable</key><true/>
    <key>NSHumanReadableCopyright</key><string>MIT</string>
</dict>
</plist>
PLIST

# An ad-hoc signature is enough to launch locally and keeps the app out of the
# "unidentified developer" gate on every rebuild. Sign after the payload is in
# place, or the bundle hash is invalidated.
codesign --force --sign - --timestamp=none "$APP_DIR" >/dev/null 2>&1 || \
  echo "==> codesign skipped (ad-hoc signing unavailable)"

# ---- dist -------------------------------------------------------------------
# The bare binary the auto-updater expects as the release asset name, copied out
# of the signed bundle so it carries the same signature.
cp -f "$APP_DIR/Contents/MacOS/zen-gate" "$DIST/zen-gate-darwin"

ditto -c -k --keepParent "$APP_DIR" "$DIST/zen-gate-$VERSION-darwin.zip"

echo "==> built:"
echo "    $APP_DIR"
echo "    $DIST/zen-gate-$VERSION-darwin.zip"
echo "    $DIST/zen-gate-darwin"
echo
echo "run it with:  open \"$APP_DIR\""
echo "or install:   cp -R \"$APP_DIR\" /Applications/"