#!/usr/bin/env bash

set -euo pipefail

PACKAGE_NAME="cordium"
VERSION=""
ARCH=""
BINARY_PATH=""
OUTPUT_DIR="packaging"
LICENSE_FILE="LICENSE"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --package-name)
      PACKAGE_NAME="$2"
      shift 2
      ;;
    --version)
      VERSION="$2"
      shift 2
      ;;
    --arch)
      ARCH="$2"
      shift 2
      ;;
    --binary)
      BINARY_PATH="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --license-file)
      LICENSE_FILE="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

if [[ -z "$VERSION" || -z "$ARCH" ]]; then
  echo "Usage: $0 --version <version> --arch <amd64|arm64> [--binary <path>] [--output-dir <dir>] [--license-file <path>]"
  exit 1
fi

if [[ "$PACKAGE_NAME" != "cordium" ]]; then
  echo "This packaging script only supports the cordium CLI package"
  exit 1
fi

case "$ARCH" in
  amd64)
    PKG_ARCH="x86_64"
    ;;
  arm64)
    PKG_ARCH="arm64"
    ;;
  *)
    echo "Unknown architecture: $ARCH"
    exit 1
    ;;
esac

if [[ -z "$BINARY_PATH" ]]; then
  if [[ -f "bin/cordium" ]]; then
    BINARY_PATH="bin/cordium"
  elif [[ -f "dist/cordium-darwin-${ARCH}/cordium" ]]; then
    BINARY_PATH="dist/cordium-darwin-${ARCH}/cordium"
  elif [[ -f "cordium" ]]; then
    BINARY_PATH="cordium"
  else
    echo "Could not find cordium binary."
    echo "Expected one of:"
    echo "  bin/cordium"
    echo "  dist/cordium-darwin-${ARCH}/cordium"
    echo "  cordium"
    echo
    echo "Existing cordium-like files:"
    find . -maxdepth 5 -type f -name 'cordium*' -print || true
    exit 1
  fi
fi

if [[ ! -f "$BINARY_PATH" ]]; then
  echo "Binary does not exist: $BINARY_PATH"
  exit 1
fi

if [[ ! -f "$LICENSE_FILE" ]]; then
  echo "License file does not exist: $LICENSE_FILE"
  exit 1
fi

PACKAGE_ID="com.octelium.cordium"
DESCRIPTION="Cordium - open-source sandbox platform with identity-based, secretless infrastructure access"

WORK_DIR="packaging/macos-${ARCH}"
PKG_ROOT="${WORK_DIR}/pkg-root"
SCRIPTS_DIR="${WORK_DIR}/pkg-scripts"
RESOURCES_DIR="${WORK_DIR}/resources"
COMPONENT_PKG="${WORK_DIR}/cordium-component.pkg"
DIST_XML="${WORK_DIR}/distribution.xml"
FINAL_PKG="${OUTPUT_DIR}/cordium-${VERSION}-${ARCH}.pkg"

echo "Building macOS PKG for cordium version ${VERSION} (${ARCH})"
echo "Using binary: ${BINARY_PATH}"

rm -rf "$WORK_DIR"
mkdir -p "$PKG_ROOT/usr/local/bin"
mkdir -p "$SCRIPTS_DIR"
mkdir -p "$RESOURCES_DIR"
mkdir -p "$OUTPUT_DIR"

cp "$BINARY_PATH" "$PKG_ROOT/usr/local/bin/cordium"
chmod 0755 "$PKG_ROOT/usr/local/bin/cordium"

cat > "$SCRIPTS_DIR/postinstall" <<'EOF'
#!/bin/bash
set -e

if [ ! -d /usr/local/bin ]; then
  mkdir -p /usr/local/bin
fi

exit 0
EOF

chmod 0755 "$SCRIPTS_DIR/postinstall"

cat > "$DIST_XML" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
    <title>Cordium</title>
    <organization>com.octelium</organization>
    <domains enable_localSystem="true"/>
    <options customize="never" require-scripts="true" hostArchitectures="${PKG_ARCH}"/>

    <welcome file="welcome.html" mime-type="text/html"/>
    <license file="$(basename "$LICENSE_FILE")" mime-type="text/plain"/>
    <conclusion file="conclusion.html" mime-type="text/html"/>

    <choices-outline>
        <line choice="default">
            <line choice="${PACKAGE_ID}"/>
        </line>
    </choices-outline>

    <choice id="default"/>
    <choice id="${PACKAGE_ID}" visible="false">
        <pkg-ref id="${PACKAGE_ID}"/>
    </choice>

    <pkg-ref id="${PACKAGE_ID}" version="${VERSION}" onConclusion="none">
        cordium-component.pkg
    </pkg-ref>
</installer-gui-script>
EOF

cat > "$RESOURCES_DIR/welcome.html" <<EOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, sans-serif;
            line-height: 1.45;
        }
        h1 {
            color: #111;
        }
        code {
            background: #f2f2f2;
            padding: 2px 6px;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <h1>Welcome to the Cordium Installer</h1>
    <p>${DESCRIPTION}.</p>
    <p>This installer will install Cordium version ${VERSION} on your system.</p>
    <p>The <code>cordium</code> binary will be installed to <code>/usr/local/bin</code>.</p>
</body>
</html>
EOF

cat > "$RESOURCES_DIR/conclusion.html" <<EOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, sans-serif;
            line-height: 1.45;
        }
        h1 {
            color: #111;
        }
        code {
            background: #f2f2f2;
            padding: 2px 6px;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <h1>Installation Complete</h1>
    <p>Cordium has been successfully installed.</p>
    <p>You can now use it by running <code>cordium</code> from your terminal.</p>
    <p>If your shell cannot find the command, make sure <code>/usr/local/bin</code> is present in your <code>PATH</code>.</p>
</body>
</html>
EOF

cp "$LICENSE_FILE" "$RESOURCES_DIR/$(basename "$LICENSE_FILE")"

echo "Building component package..."
pkgbuild \
  --root "$PKG_ROOT" \
  --identifier "$PACKAGE_ID" \
  --version "$VERSION" \
  --scripts "$SCRIPTS_DIR" \
  --install-location / \
  "$COMPONENT_PKG"

echo "Building product package..."
productbuild \
  --distribution "$DIST_XML" \
  --resources "$RESOURCES_DIR" \
  --package-path "$WORK_DIR" \
  --version "$VERSION" \
  "$FINAL_PKG"

echo "Successfully created: $FINAL_PKG"