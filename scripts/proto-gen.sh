#!/bin/bash
# Protobuf Generation Script
# Author: online-game team
# Description: Generate Go code from Protocol Buffer definitions

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROTO_DIR="./proto"
PKG_DIR="./pkg"
API_DIR="./api"
PROTO_INCLUDE_DIR="./third_party/proto"

# Check if protoc is installed
check_protoc() {
    if ! command -v protoc &> /dev/null; then
        echo -e "${RED}Error: protoc is not installed${NC}"
        echo "Please install Protocol Buffers compiler:"
        echo "  macOS:   brew install protobuf"
        echo "  Ubuntu:  apt-get install protobuf-compiler"
        echo "  Or visit: https://grpc.io/docs/protoc-installation/"
        exit 1
    fi

    local protoc_version=$(protoc --version | awk '{print $2}')
    echo -e "${GREEN}✓${NC} protoc version: $protoc_version"
}

# Check required plugins
check_plugins() {
    local plugins=(
        "protoc-gen-go"
        "protoc-gen-go-grpc"
        "protoc-gen-grpc-gateway"
        "protoc-gen-openapiv2"
        "protoc-gen-validate"
    )

    local missing_plugins=()

    for plugin in "${plugins[@]}"; do
        if command -v "$plugin" &> /dev/null; then
            echo -e "${GREEN}✓${NC} $plugin"
        else
            echo -e "${YELLOW}✗${NC} $plugin (not found)"
            missing_plugins+=("$plugin")
        fi
    done

    if [ ${#missing_plugins[@]} -gt 0 ]; then
        echo -e "\n${YELLOW}Installing missing plugins...${NC}"
        for plugin in "${missing_plugins[@]}"; do
            case $plugin in
                "protoc-gen-go")
                    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
                    ;;
                "protoc-gen-go-grpc")
                    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
                    ;;
                "protoc-gen-grpc-gateway")
                    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
                    ;;
                "protoc-gen-openapiv2")
                    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
                    ;;
                "protoc-gen-validate")
                    go install github.com/envoyproxy/protoc-gen-validate@latest
                    ;;
            esac
        done
    fi
}

# Create output directories
create_directories() {
    echo -e "${BLUE}Creating output directories...${NC}"
    mkdir -p "$PKG_DIR"/{api,proto,grpc}
    mkdir -p "$API_DIR"/openapi
    mkdir -p "$API_DIR"/swagger
}

# Generate protobuf files
generate_proto() {
    echo -e "\n${BLUE}Generating protobuf files...${NC}"

    # Find all proto files
    local proto_files=()
    while IFS= read -r -d '' file; do
        proto_files+=("$file")
    done < <(find "$PROTO_DIR" -name "*.proto" -print0)

    if [ ${#proto_files[@]} -eq 0 ]; then
        echo -e "${YELLOW}No .proto files found in $PROTO_DIR${NC}"
        return
    fi

    echo -e "${GREEN}Found ${#proto_files[@]} proto file(s)${NC}"

    # Generate for each proto file
    for proto_file in "${proto_files[@]}"; do
        echo -e "${GREEN}Generating: $proto_file${NC}"

        protoc \
            --proto_path="$PROTO_DIR" \
            --proto_path="$PROTO_INCLUDE_DIR" \
            --go_out="$PKG_DIR" \
            --go_opt=paths=source_relative \
            --go-grpc_out="$PKG_DIR" \
            --go-grpc_opt=paths=source_relative \
            --grpc-gateway_out="$PKG_DIR" \
            --grpc-gateway_opt=paths=source_relative \
            --grpc-gateway_opt=generate_unbound_methods=true \
            --openapiv2_out="$API_DIR/openapi" \
            --openapiv2_opt=allow_merge=true,merge_file_name=api \
            --validate_out="lang=go:$PKG_DIR" \
            --validate_opt=paths=source_relative \
            "$proto_file"

        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓${NC} Generated: $(basename "$proto_file")"
        else
            echo -e "${RED}✗${NC} Failed: $(basename "$proto_file")"
            exit 1
        fi
    done
}

# Format generated files
format_generated() {
    echo -e "\n${BLUE}Formatting generated files...${NC}"

    # Find all generated Go files
    find "$PKG_DIR" -name "*.pb.go" -exec gofmt -w {} \;
    find "$PKG_DIR" -name "*.pb.validate.go" -exec gofmt -w {} \;
    find "$PKG_DIR" -name "*.gw.go" -exec gofmt -w {} \;

    echo -e "${GREEN}✓${NC} Formatted generated files"
}

# Run goimports on generated files
imports_generated() {
    if command -v goimports &> /dev/null; then
        echo -e "\n${BLUE}Running goimports...${NC}"

        find "$PKG_DIR" -name "*.pb.go" -exec goimports -w {} \;
        find "$PKG_DIR" -name "*.pb.validate.go" -exec goimports -w {} \;
        find "$PKG_DIR" -name "*.gw.go" -exec goimports -w {} \;

        echo -e "${GREEN}✓${NC} Imports organized"
    else
        echo -e "${YELLOW}goimports not found, skipping...${NC}"
    fi
}

# Generate summary
generate_summary() {
    echo -e "\n${BLUE}Generation Summary:${NC}"
    echo -e "  Proto files:     $(find "$PROTO_DIR" -name "*.proto" | wc -l)"
    echo -e "  Generated files: $(find "$PKG_DIR" -name "*.pb.go" | wc -l)"
    echo -e "  gRPC files:      $(find "$PKG_DIR" -name "*_grpc.pb.go" | wc -l)"
    echo -e "  Gateway files:   $(find "$PKG_DIR" -name "*.gw.go" | wc -l)"
    echo -e "  Validate files:  $(find "$PKG_DIR" -name "*.pb.validate.go" | wc -l)"
    echo -e "  API specs:       $(find "$API_DIR/openapi" -name "*.json" | wc -l)"
}

# Watch mode (optional)
watch_mode() {
    if [ "$1" == "--watch" ] || [ "$1" == "-w" ]; then
        echo -e "${BLUE}Watching for changes...${NC}"
        echo -e "Press Ctrl+C to stop\n"

        if command -v fswatch &> /dev/null; then
            fswatch -o "$PROTO_DIR" | while read -r; do
                echo -e "\n${YELLOW}Change detected, regenerating...${NC}"
                generate_proto
                format_generated
                imports_generated
            done
        elif command -v inotifywait &> /dev/null; then
            while inotifywait -r -e modify,create,delete "$PROTO_DIR"; do
                echo -e "\n${YELLOW}Change detected, regenerating...${NC}"
                generate_proto
                format_generated
                imports_generated
            done
        else
            echo -e "${RED}No file watcher found. Install fswatch or inotify-tools.${NC}"
            exit 1
        fi
    fi
}

# Clean generated files
clean_generated() {
    if [ "$1" == "--clean" ] || [ "$1" == "-c" ]; then
        echo -e "${YELLOW}Cleaning generated files...${NC}"
        find "$PKG_DIR" -name "*.pb.go" -delete
        find "$PKG_DIR" -name "*.pb.validate.go" -delete
        find "$PKG_DIR" -name "*.gw.go" -delete
        rm -rf "$API_DIR/openapi"/*
        echo -e "${GREEN}✓${NC} Cleaned generated files"
        exit 0
    fi
}

# Main execution
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Protobuf Code Generation${NC}"
    echo -e "${BLUE}========================================${NC}\n"

    # Handle flags
    clean_generated "$1"

    # Check dependencies
    check_protoc
    check_plugins

    # Create directories
    create_directories

    # Generate code
    generate_proto

    # Post-processing
    format_generated
    imports_generated

    # Summary
    generate_summary

    # Watch mode
    watch_mode "$1"

    echo -e "\n${GREEN}✓${NC} Code generation complete!"
}

# Run main function
main "$@"
