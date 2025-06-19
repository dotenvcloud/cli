#!/bin/bash
set -e

echo "Running DotEnv CLI Tests"
echo "========================"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Change to CLI directory
CLI_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$CLI_DIR"

# Parse arguments
RUN_ALL=false
RUN_UNIT=false
RUN_INTEGRATION=false
RUN_COMPATIBILITY=false
RUN_COVERAGE=false
RUN_BENCHMARKS=false

if [ $# -eq 0 ]; then
    RUN_ALL=true
else
    for arg in "$@"; do
        case $arg in
            unit)
                RUN_UNIT=true
                ;;
            integration)
                RUN_INTEGRATION=true
                ;;
            compatibility)
                RUN_COMPATIBILITY=true
                ;;
            coverage)
                RUN_COVERAGE=true
                ;;
            bench|benchmark)
                RUN_BENCHMARKS=true
                ;;
            all)
                RUN_ALL=true
                ;;
            *)
                echo "Unknown test type: $arg"
                echo "Usage: $0 [unit|integration|compatibility|coverage|bench|all]"
                exit 1
                ;;
        esac
    done
fi

# Function to run tests and check result
run_test() {
    local test_name=$1
    local test_cmd=$2
    
    echo -e "\n${GREEN}Running $test_name...${NC}"
    if eval "$test_cmd"; then
        echo -e "${GREEN}✓ $test_name passed${NC}"
        return 0
    else
        echo -e "${RED}✗ $test_name failed${NC}"
        return 1
    fi
}

# Track failures
FAILURES=0

# Run unit tests
if [ "$RUN_ALL" = true ] || [ "$RUN_UNIT" = true ]; then
    if ! run_test "unit tests" "go test -v -timeout=10m ./cmd/... ./internal/... -tags='!integration,!compatibility'"; then
        ((FAILURES++))
    fi
fi

# Run integration tests
if [ "$RUN_ALL" = true ] || [ "$RUN_INTEGRATION" = true ]; then
    if ! run_test "integration tests" "go test -v -timeout=10m ./test/integration/... -tags=integration"; then
        ((FAILURES++))
    fi
fi

# Run compatibility tests if PHP/Node available
if [ "$RUN_ALL" = true ] || [ "$RUN_COMPATIBILITY" = true ]; then
    if command -v php &> /dev/null && command -v node &> /dev/null; then
        if ! run_test "compatibility tests" "go test -v -timeout=10m ./test/compatibility/... -tags=compatibility"; then
            ((FAILURES++))
        fi
    else
        echo -e "${YELLOW}⚠ Skipping compatibility tests (PHP/Node not found)${NC}"
    fi
fi

# Run with coverage
if [ "$RUN_COVERAGE" = true ]; then
    echo -e "\n${GREEN}Generating coverage report...${NC}"
    go test -coverprofile=coverage.out -covermode=atomic ./cmd/... ./internal/...
    go tool cover -html=coverage.out -o coverage.html
    
    # Check coverage threshold
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    THRESHOLD=80
    
    echo -e "\nTotal coverage: ${COVERAGE}%"
    
    if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
        echo -e "${RED}Coverage is below ${THRESHOLD}%${NC}"
        ((FAILURES++))
    else
        echo -e "${GREEN}Coverage meets threshold${NC}"
    fi
fi

# Run benchmarks
if [ "$RUN_BENCHMARKS" = true ]; then
    if ! run_test "benchmarks" "go test -bench=. -benchmem ./internal/crypto/... ./internal/formats/..."; then
        ((FAILURES++))
    fi
fi

# Summary
echo -e "\n========================"
if [ $FAILURES -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}$FAILURES test suite(s) failed${NC}"
    exit 1
fi