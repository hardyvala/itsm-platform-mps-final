2#!/bin/bash
# =============================================================================
# Code Generator Script
# Generates Go service code from DSL definitions
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DSL_DIR="$SCRIPT_DIR/dsl/apps"
OUTPUT_DIR="$SCRIPT_DIR/generated"
CODEGEN="$SCRIPT_DIR/bin/codegen"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== ITSM Platform Code Generator ===${NC}"

# Build codegen if not exists
if [ ! -f "$CODEGEN" ]; then
    echo "Building code generator..."
    mkdir -p "$SCRIPT_DIR/bin"
    go build -o "$CODEGEN" "$SCRIPT_DIR/cmd/codegen"
fi

# Generate specified service or all
if [ -n "$1" ]; then
    # Generate single service
    SERVICE="$1"
    DSL_PATH="$DSL_DIR/$SERVICE/service.json"
    
    if [ ! -f "$DSL_PATH" ]; then
        echo "Error: DSL not found at $DSL_PATH"
        exit 1
    fi
    
    echo "Generating $SERVICE-service..."
    "$CODEGEN" "$DSL_PATH" "$OUTPUT_DIR/$SERVICE-service"
    echo -e "${GREEN}✓ Generated $SERVICE-service${NC}"
else
    # Generate all services
    echo "Generating all services..."
    
    for DSL_PATH in "$DSL_DIR"/*/service.json; do
        SERVICE=$(basename "$(dirname "$DSL_PATH")")
        echo "  Generating $SERVICE-service..."
        "$CODEGEN" "$DSL_PATH" "$OUTPUT_DIR/$SERVICE-service"
    done
    
    echo -e "${GREEN}✓ All services generated${NC}"
fi

echo ""
echo "Generated files:"
find "$OUTPUT_DIR" -name "*.go" -type f | sort
