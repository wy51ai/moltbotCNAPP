#!/bin/bash

# Build script for ClawdBot Bridge
# Compiles binaries for multiple platforms

set -e

VERSION=${VERSION:-"0.1.0"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BINARY_NAME="clawdbot-bridge"
DIST_DIR="dist"
SRC_DIR="cmd/bridge"

# Build flags
LDFLAGS="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# Platforms to build
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

echo "Building ClawdBot Bridge v${VERSION}"
echo "Git Commit: ${GIT_COMMIT}"
echo "Build Time: ${BUILD_TIME}"
echo ""

# Clean dist directory
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    IFS="/" read -r GOOS GOARCH <<< "${PLATFORM}"

    OUTPUT_NAME="${BINARY_NAME}-${GOOS}-${GOARCH}"
    if [ "${GOOS}" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    OUTPUT_PATH="${DIST_DIR}/${OUTPUT_NAME}"

    echo "Building for ${GOOS}/${GOARCH}..."

    GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags="${LDFLAGS}" \
        -o "${OUTPUT_PATH}" \
        "./${SRC_DIR}/"

    if [ $? -eq 0 ]; then
        echo "✓ Built: ${OUTPUT_PATH}"

        # Calculate file size
        if [ "$(uname)" = "Darwin" ]; then
            SIZE=$(stat -f%z "${OUTPUT_PATH}")
        else
            SIZE=$(stat -c%s "${OUTPUT_PATH}" 2>/dev/null || echo "unknown")
        fi

        if [ "${SIZE}" != "unknown" ]; then
            SIZE_MB=$(echo "scale=2; ${SIZE}/1024/1024" | bc)
            echo "  Size: ${SIZE_MB} MB"
        fi
    else
        echo "✗ Failed to build for ${GOOS}/${GOARCH}"
        exit 1
    fi

    echo ""
done

echo "Build complete! Binaries are in ${DIST_DIR}/"
echo ""
echo "Created binaries:"
ls -lh "${DIST_DIR}"
