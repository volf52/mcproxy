#!/bin/bash

# Release build script for mcproxy
# Builds an optimized binary for production deployment

set -e

# Configuration
APP_NAME="mcproxy"
MAIN_PACKAGE="./cmd/${APP_NAME}"
OUTPUT_DIR="dist"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'unknown')}"
BUILD_TIME="$(date -u '+%Y-%m-%d_%H:%M:%S')"
GO_VERSION=$(go version | awk '{print $3}')

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building ${APP_NAME} release version${NC}"
echo "Version: ${VERSION}"
echo "Build time: ${BUILD_TIME}"
echo "Go version: ${GO_VERSION}"
echo

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Determine target architecture
TARGET_OS="${TARGET_OS:-$(go env GOOS)}"
TARGET_ARCH="${TARGET_ARCH:-$(go env GOARCH)}"
OUTPUT_NAME="${APP_NAME}-${TARGET_OS}-${TARGET_ARCH}"

if [ "${TARGET_OS}" = "windows" ]; then
    OUTPUT_NAME="${OUTPUT_NAME}.exe"
fi

# Build flags for optimization
LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X main.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X main.BuildTime=${BUILD_TIME}"
LDFLAGS="${LDFLAGS} -X main.GoVersion=${GO_VERSION}"

echo -e "${YELLOW}Building for ${TARGET_OS}/${TARGET_ARCH}${NC}"
echo "Output: ${OUTPUT_DIR}/${OUTPUT_NAME}"
echo

# Build the binary with optimizations
CGO_ENABLED=0 GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" go build \
    -ldflags "${LDFLAGS}" \
    -trimpath \
    -o "${OUTPUT_DIR}/${OUTPUT_NAME}" \
    "${MAIN_PACKAGE}"

# Verify the binary was created
if [ -f "${OUTPUT_DIR}/${OUTPUT_NAME}" ]; then
    echo -e "${GREEN}✓ Build successful!${NC}"

    # Show binary size
    BINARY_SIZE=$(ls -lh "${OUTPUT_DIR}/${OUTPUT_NAME}" | awk '{print $5}')
    echo "Binary size: ${BINARY_SIZE}"

    # Show build info
    echo
    echo "Build artifacts:"
    ls -la "${OUTPUT_DIR}/"

    # Create a checksum for verification
    if command -v sha256sum >/dev/null 2>&1; then
        echo
        echo -e "${YELLOW}Creating SHA256 checksum...${NC}"
        sha256sum "${OUTPUT_DIR}/${OUTPUT_NAME}" > "${OUTPUT_DIR}/${OUTPUT_NAME}.sha256"
        echo "Checksum: $(cat "${OUTPUT_DIR}/${OUTPUT_NAME}.sha256")"
    fi
else
    echo -e "${RED}✗ Build failed!${NC}" >&2
    exit 1
fi

echo
echo -e "${GREEN}Release build complete!${NC}"
echo "Binary: ${OUTPUT_DIR}/${OUTPUT_NAME}"
echo "To run: ./${OUTPUT_DIR}/${OUTPUT_NAME}"