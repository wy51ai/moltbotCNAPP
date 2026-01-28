#!/bin/bash

# Quick build script for current platform only

set -e

VERSION=${VERSION:-"dev"}
BINARY_NAME="clawdbot-bridge"
SRC_DIR="cmd/bridge"

echo "Building ClawdBot Bridge for current platform..."

go build -o "${BINARY_NAME}" "${SRC_DIR}/main.go"

echo "✓ Built: ${BINARY_NAME}"
echo ""
echo "Run with: ./${BINARY_NAME}"
