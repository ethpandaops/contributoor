#!/bin/bash
# Test script for verifying contributoor can connect to all beacon nodes

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
ENCLAVE_NAME="${ENCLAVE_NAME:-contributoor-test}"
NETWORK_NAME="kt-${ENCLAVE_NAME}"
TEST_DURATION="${TEST_DURATION:-30}"
OUTPUT_SERVER="${OUTPUT_SERVER:-localhost:8080}"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_separator() {
    echo "======================================"
}

# Get beacon node endpoints from Kurtosis
get_beacon_nodes() {
    log_info "Fetching beacon node endpoints from Kurtosis enclave..."
    
    local beacon_nodes=$(kurtosis enclave inspect "$ENCLAVE_NAME" | \
        grep -E 'cl-(teku|prysm|lighthouse|nimbus|lodestar)' | \
        grep http | \
        awk '{ print $6 }' | \
        grep -v validator)
    
    if [ -z "$beacon_nodes" ]; then
        log_error "No beacon nodes found in Kurtosis enclave!"
        return 1
    fi
    
    echo "$beacon_nodes"
}

# Test contributoor connection to a single beacon node
test_single_beacon() {
    local beacon_url="$1"
    local node_name=$(echo "$beacon_url" | cut -d'/' -f3 | cut -d':' -f1)
    
    print_separator
    log_info "Testing contributoor against $node_name"
    log_info "Endpoint: $beacon_url"
    print_separator
    
    # Check if beacon node is healthy first
    if ! check_beacon_health "$beacon_url"; then
        log_warning "Beacon node $node_name is not healthy yet, waiting..."
        sleep 10
        if ! check_beacon_health "$beacon_url"; then
            log_error "Beacon node $node_name failed health check"
            return 1
        fi
    fi
    
    # Run contributoor with timeout
    local container_name="contributoor-test-${node_name}"
    local exit_code
    
    timeout "$TEST_DURATION" docker run --rm \
        --name "$container_name" \
        --network "$NETWORK_NAME" \
        ethpandaops/contributoor:local \
        --beacon-node-address="$beacon_url" \
        --output-server.address="$OUTPUT_SERVER" \
        --log-level=debug \
        --network=kurtosis \
        || exit_code=$?
    
    # Check results
    if [ "${exit_code:-0}" -eq 124 ]; then
        log_info "✓ Contributoor successfully connected to $node_name"
        return 0
    else
        log_error "✗ Contributoor failed to connect to $node_name (exit code: ${exit_code:-0})"
        # Try to get logs if container still exists
        docker logs "$container_name" 2>&1 | tail -20 || true
        return 1
    fi
}

# Check beacon node health
check_beacon_health() {
    local beacon_url="$1"
    local health_url="${beacon_url}/eth/v1/node/health"
    
    # Try to reach the health endpoint
    if curl -s -f -m 5 "$health_url" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Main test execution
main() {
    log_info "Starting contributoor beacon node integration tests"
    log_info "Configuration:"
    log_info "  - Enclave: $ENCLAVE_NAME"
    log_info "  - Network: $NETWORK_NAME"
    log_info "  - Test Duration: ${TEST_DURATION}s"
    echo ""
    
    # Get beacon nodes
    local beacon_nodes
    if ! beacon_nodes=$(get_beacon_nodes); then
        exit 1
    fi
    
    log_info "Found beacon nodes:"
    echo "$beacon_nodes"
    echo ""
    
    # Test each beacon node
    local failed_nodes=()
    local successful_nodes=()
    
    while IFS= read -r beacon; do
        if test_single_beacon "$beacon"; then
            successful_nodes+=("$beacon")
        else
            failed_nodes+=("$beacon")
        fi
        echo ""
    done <<< "$beacon_nodes"
    
    # Summary
    print_separator
    log_info "Test Summary:"
    log_info "  - Successful: ${#successful_nodes[@]}"
    log_info "  - Failed: ${#failed_nodes[@]}"
    
    if [ ${#failed_nodes[@]} -eq 0 ]; then
        log_info "✓ All beacon node tests passed!"
        print_separator
        exit 0
    else
        log_error "✗ Some beacon node tests failed:"
        for node in "${failed_nodes[@]}"; do
            log_error "  - $node"
        done
        print_separator
        exit 1
    fi
}

# Run main function
main "$@"