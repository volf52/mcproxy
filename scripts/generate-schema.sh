#!/bin/bash

# MCProxy JSON Schema Generation Script
# This script generates a JSON Schema from the Go config structs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default output file
OUTPUT_FILE="config.schema.json"

# Parse command line arguments
if [ $# -gt 0 ]; then
    OUTPUT_FILE="$1"
fi

# Change to project root
cd "$PROJECT_ROOT"

echo -e "${YELLOW}Generating JSON Schema...${NC}"

# Run the schema generator (suppress its output since we'll show our own)
go run cmd/generate-schema/main.go "$OUTPUT_FILE" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ JSON Schema generated successfully: $OUTPUT_FILE${NC}"

    # Show file info
    echo -e "${YELLOW}Schema file details:${NC}"
    ls -la "$OUTPUT_FILE"

    echo -e "\n${YELLOW}You can now use this schema for:${NC}"
    echo "  - IDE autocomplete and validation"
    echo "  - JSON schema validation in CI/CD"
    echo "  - Documentation generation"
    echo "  - Configuration file validation"
else
    echo -e "${RED}✗ Failed to generate JSON Schema${NC}"
    exit 1
fi