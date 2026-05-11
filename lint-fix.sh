#!/bin/bash

# lint-fix.sh - Comprehensive linting script with fix option for all Go modules
# Based on GitHub workflow structure

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to run linting for a specific module
lint_module() {
    local module_path="$1"
    local module_name="$2"

    print_status "Linting $module_name at $module_path"

    cd "$module_path"

    if golangci-lint run --fix --timeout 5m; then
        print_success "✓ $module_name completed"
    else
        print_error "✗ $module_name failed"
        return 1
    fi

    cd - > /dev/null
}

# Main execution
main() {
    print_status "Running golangci-lint --fix on all Go modules"
    echo ""

    # Store original directory
    ORIGINAL_DIR=$(pwd)

    # Define modules to lint (based on GitHub workflow)
    MODULES=(
        "./apps/anchor:anchor app"
        "./shared/fxmodules:shared fxmodules"
        "./shared/toolkit:shared toolkit"
        "./shared/middlewares:shared middlewares"
    )

    # Track results
    SUCCESS_COUNT=0
    FAILED_MODULES=()

    # Lint each module
    for module_info in "${MODULES[@]}"; do
        module_path="${module_info%:*}"
        module_name="${module_info#*:}"

        if lint_module "$module_path" "$module_name"; then
            ((SUCCESS_COUNT++))
        else
            FAILED_MODULES+=("$module_name")
        fi
        echo ""
    done

    # Return to original directory
    cd "$ORIGINAL_DIR"

    # Print summary
    if [ ${#FAILED_MODULES[@]} -eq 0 ]; then
        print_success "All modules passed! 🎉"
    else
        print_error "Failed modules: ${FAILED_MODULES[*]}"
        exit 1
    fi
}

main "$@"
