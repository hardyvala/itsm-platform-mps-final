#!/bin/bash
# =============================================================================
# DAL-Based Code Generator Script
# Generates Go service code from DSL definitions using DAL client
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/generated"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== ITSM Platform DAL-Based Code Generator ===${NC}"
echo -e "${YELLOW}Generates services that use DAL client (no manual DB logic)${NC}"
echo ""

# Function to generate service from DSL
generate_service() {
    local dsl_path="$1"
    local service_name="$2"
    
    if [ ! -f "$dsl_path" ]; then
        echo -e "${RED}Error: DSL not found at $dsl_path${NC}"
        return 1
    fi
    
    echo "🔧 Generating $service_name-service from DSL..."
    echo "   DSL: $dsl_path"
    echo "   Output: $OUTPUT_DIR/$service_name-service"
    
    # Use the new DAL client-based generator
    go run "$SCRIPT_DIR/cmd/codegen" -dsl "$dsl_path" -output "$OUTPUT_DIR"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Generated $service_name-service (uses DAL client)${NC}"
        echo ""
    else
        echo -e "${RED}✗ Failed to generate $service_name-service${NC}"
        return 1
    fi
}

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Generate specified service or all available services
if [ -n "$1" ]; then
    # Generate single service
    SERVICE="$1"
    
    # Check multiple possible DSL locations
    DSL_PATHS=(
        #"$SCRIPT_DIR/services/$SERVICE-service/dsl/service.json"
        #"$SCRIPT_DIR/services/$SERVICE-service/dsl/simple-service.json"
        "$SCRIPT_DIR/dsl/apps/$SERVICE/service.json"
        #"$SCRIPT_DIR/$SERVICE.json"
    )
    
    DSL_FOUND=""
    for dsl_path in "${DSL_PATHS[@]}"; do
        if [ -f "$dsl_path" ]; then
            DSL_FOUND="$dsl_path"
            break
        fi
    done
    
    if [ -z "$DSL_FOUND" ]; then
        echo -e "${RED}Error: DSL not found for service '$SERVICE'${NC}"
        echo "Checked locations:"
        for dsl_path in "${DSL_PATHS[@]}"; do
            echo "  - $dsl_path"
        done
        exit 1
    fi
    
    generate_service "$DSL_FOUND" "$SERVICE"
    
else
    # Generate all available services
    echo "🔍 Searching for available DSL files..."
    echo ""
    
    # Look for DSLs in services directory
    FOUND_SERVICES=()
    
    for service_dir in "$SCRIPT_DIR/services"/*-service; do
        if [ -d "$service_dir" ]; then
            service_name=$(basename "$service_dir" | sed 's/-service$//')
            
            # Check for DSL files
            for dsl_file in "$service_dir/dsl/service.json" "$service_dir/dsl/simple-service.json"; do
                if [ -f "$dsl_file" ]; then
                    FOUND_SERVICES+=("$service_name:$dsl_file")
                    break
                fi
            done
        fi
    done
    
    # Also check dsl/apps directory
    if [ -d "$SCRIPT_DIR/dsl/apps" ]; then
        for app_dir in "$SCRIPT_DIR/dsl/apps"/*; do
            if [ -d "$app_dir" ] && [ -f "$app_dir/service.json" ]; then
                service_name=$(basename "$app_dir")
                FOUND_SERVICES+=("$service_name:$app_dir/service.json")
            fi
        done
    fi
    
    if [ ${#FOUND_SERVICES[@]} -eq 0 ]; then
        echo -e "${YELLOW}No DSL files found. To generate a service:${NC}"
        echo "  $0 <service-name>"
        echo ""
        echo "Expected DSL locations:"
        echo "  - ./services/<service>-service/dsl/service.json"
        echo "  - ./dsl/apps/<service>/service.json"
        exit 0
    fi
    
    echo "📋 Found ${#FOUND_SERVICES[@]} service(s) with DSL:"
    for service_info in "${FOUND_SERVICES[@]}"; do
        service_name="${service_info%:*}"
        dsl_path="${service_info#*:}"
        echo "  - $service_name ($dsl_path)"
    done
    echo ""
    
    # Generate all found services
    for service_info in "${FOUND_SERVICES[@]}"; do
        service_name="${service_info%:*}"
        dsl_path="${service_info#*:}"
        generate_service "$dsl_path" "$service_name"
    done
fi

echo "📁 Generated files:"
if [ -d "$OUTPUT_DIR" ]; then
    find "$OUTPUT_DIR" -name "*.go" -type f | sort
    echo ""
    echo -e "${GREEN}🎉 Code generation completed!${NC}"
    echo -e "${BLUE}All generated services use DAL client (no manual DB logic)${NC}"
else
    echo "No files generated"
fi
