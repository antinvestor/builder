#!/bin/bash
# =============================================================================
# Service Feature - Demo Script
# =============================================================================
# This script demonstrates the full feature pipeline by submitting a feature
# request and monitoring its progress through all agents.
#
# Usage:
#   ./demo.sh                    # Run full demo
#   ./demo.sh --quick            # Run quick smoke test
#   ./demo.sh --help             # Show help
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SERVICE_URL="${SERVICE_URL:-http://localhost:8080}"
NATS_URL="${NATS_URL:-nats://localhost:4222}"
TIMEOUT=300

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

wait_for_service() {
    local url=$1
    local max_attempts=30
    local attempt=1

    log_info "Waiting for service at $url..."

    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$url/health" > /dev/null 2>&1; then
            log_success "Service is healthy!"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done

    log_error "Service did not become healthy in time"
    return 1
}

generate_execution_id() {
    # Generate XID-like ID (simplified)
    echo "demo$(date +%s | cut -c5-10)$(shuf -i 1000-9999 -n 1)"
}

# =============================================================================
# Demo Functions
# =============================================================================

show_banner() {
    echo -e "${CYAN}"
    echo "╔═══════════════════════════════════════════════════════════════════════════╗"
    echo "║                                                                           ║"
    echo "║   ███████╗███████╗ █████╗ ████████╗██╗   ██╗██████╗ ███████╗             ║"
    echo "║   ██╔════╝██╔════╝██╔══██╗╚══██╔══╝██║   ██║██╔══██╗██╔════╝             ║"
    echo "║   █████╗  █████╗  ███████║   ██║   ██║   ██║██████╔╝█████╗               ║"
    echo "║   ██╔══╝  ██╔══╝  ██╔══██║   ██║   ██║   ██║██╔══██╗██╔══╝               ║"
    echo "║   ██║     ███████╗██║  ██║   ██║   ╚██████╔╝██║  ██║███████╗             ║"
    echo "║   ╚═╝     ╚══════╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝╚══════╝             ║"
    echo "║                                                                           ║"
    echo "║   Autonomous Feature Building Platform - Demo                             ║"
    echo "║                                                                           ║"
    echo "╚═══════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

check_health() {
    log_step "Step 1: Health Check"

    log_info "Checking service health..."
    local response=$(curl -s "$SERVICE_URL/health")
    echo "$response" | jq . 2>/dev/null || echo "$response"

    log_info "Checking service readiness..."
    local ready=$(curl -s "$SERVICE_URL/ready")
    echo "$ready" | jq . 2>/dev/null || echo "$ready"

    log_success "Service is up and running!"
}

submit_feature_request() {
    log_step "Step 2: Submit Feature Request"

    local execution_id=$(generate_execution_id)
    log_info "Generated execution ID: $execution_id"

    # Create feature request payload
    local payload=$(cat <<EOF
{
    "execution_id": "$execution_id",
    "repository_url": "https://github.com/example/demo-repo.git",
    "branch": "main",
    "specification": {
        "title": "Add greeting function",
        "description": "Create a simple greeting function that returns a personalized message",
        "requirements": [
            "Function should accept a name parameter",
            "Function should return 'Hello, {name}!' format",
            "Add unit tests for the function",
            "Handle empty name with default 'World'"
        ],
        "target_files": ["src/greeting.go"],
        "language": "go"
    },
    "context": {
        "iteration_number": 0,
        "max_iterations": 3
    },
    "metadata": {
        "requested_by": "demo-script",
        "demo": true
    }
}
EOF
)

    log_info "Submitting feature request..."
    echo "$payload" | jq .

    # Publish to NATS queue
    if command -v nats &> /dev/null; then
        echo "$payload" | nats pub feature.requests --server "$NATS_URL"
        log_success "Feature request published to NATS queue!"
    else
        log_warning "NATS CLI not available, using curl fallback"
        # Fallback: Use HTTP endpoint if available
        curl -s -X POST "$SERVICE_URL/api/v1/features" \
            -H "Content-Type: application/json" \
            -d "$payload" || log_warning "HTTP fallback not available"
    fi

    echo "$execution_id"
}

monitor_execution() {
    local execution_id=$1
    log_step "Step 3: Monitor Execution Pipeline"

    log_info "Monitoring execution: $execution_id"
    log_info "This demonstrates all agents in the pipeline:"
    echo ""
    echo "  1. Repository Agent     - Clone/checkout repository"
    echo "  2. Planning Agent       - Analyze and create plan"
    echo "  3. Patch Agent          - Generate code patches"
    echo "  4. Test Agent           - Generate and run tests"
    echo "  5. Review Agent         - Security and architecture review"
    echo "  6. Control Agent        - Decision making (iterate/complete/abort)"
    echo ""

    # Subscribe to events (demonstration)
    if command -v nats &> /dev/null; then
        log_info "Subscribing to feature events (5 seconds)..."
        timeout 5 nats sub "feature.>" --server "$NATS_URL" || true
    fi

    log_success "Demo execution submitted!"
}

show_observability() {
    log_step "Step 4: Observability Dashboard"

    echo "Access the following dashboards to monitor the system:"
    echo ""
    echo "  Grafana:     http://localhost:3000  (admin/admin)"
    echo "  Prometheus:  http://localhost:9090"
    echo "  Jaeger:      http://localhost:16686"
    echo "  NATS:        http://localhost:8222"
    echo ""

    log_info "Checking metrics endpoint..."
    curl -s "$SERVICE_URL/metrics" 2>/dev/null | head -20 || log_warning "Metrics endpoint not available"
}

demonstrate_agents() {
    log_step "Step 5: Agent Demonstrations"

    echo -e "${YELLOW}Repository Agent${NC}"
    echo "  - Clones repositories with SSH/HTTPS authentication"
    echo "  - Creates isolated workspaces per execution"
    echo "  - Handles branch checkout and workspace lifecycle"
    echo ""

    echo -e "${YELLOW}Planning Agent${NC}"
    echo "  - Normalizes feature specifications"
    echo "  - Performs impact analysis"
    echo "  - Generates step-by-step execution plan"
    echo ""

    echo -e "${YELLOW}Patch Generation Agent${NC}"
    echo "  - Uses BAML/LLM to generate code patches"
    echo "  - Validates syntax and structure"
    echo "  - Applies patches to workspace"
    echo ""

    echo -e "${YELLOW}Test Agent${NC}"
    echo "  - Generates tests using BAML/LLM"
    echo "  - Executes tests in isolated sandboxes"
    echo "  - Classifies failures (compilation, assertion, timeout, etc.)"
    echo "  - Compares pre/post feature test results"
    echo ""

    echo -e "${YELLOW}Review & Control Agent${NC}"
    echo "  - Security analysis (SQL injection, XSS, secrets, etc.)"
    echo "  - Architecture analysis (breaking changes, dependencies)"
    echo "  - Risk scoring with weighted dimensions"
    echo "  - Control decisions: APPROVE, ITERATE, ABORT, COMPLETE"
    echo ""

    echo -e "${YELLOW}Kill Switch System${NC}"
    echo "  - Global, Feature, Repository scopes"
    echo "  - Auto-triggers for error rates, failures, resources"
    echo "  - Safety guards for all operations"
    echo ""
}

run_quick_test() {
    log_step "Quick Smoke Test"

    log_info "Running quick health checks..."

    # Health check
    local health=$(curl -s -o /dev/null -w "%{http_code}" "$SERVICE_URL/health")
    if [ "$health" == "200" ]; then
        log_success "Health check: OK"
    else
        log_error "Health check: FAILED ($health)"
        return 1
    fi

    # Ready check
    local ready=$(curl -s -o /dev/null -w "%{http_code}" "$SERVICE_URL/ready")
    if [ "$ready" == "200" ]; then
        log_success "Ready check: OK"
    else
        log_error "Ready check: FAILED ($ready)"
        return 1
    fi

    log_success "All quick tests passed!"
}

show_help() {
    echo "Service Feature Demo Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --quick     Run quick smoke test only"
    echo "  --agents    Show agent demonstrations"
    echo "  --observe   Show observability information"
    echo "  --help      Show this help message"
    echo ""
    echo "Environment Variables:"
    echo "  SERVICE_URL   Service URL (default: http://localhost:8080)"
    echo "  NATS_URL      NATS URL (default: nats://localhost:4222)"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run full demo"
    echo "  $0 --quick            # Quick smoke test"
    echo "  SERVICE_URL=http://service:8080 $0"
    echo ""
}

# =============================================================================
# Main
# =============================================================================

main() {
    case "${1:-}" in
        --help|-h)
            show_help
            exit 0
            ;;
        --quick)
            show_banner
            wait_for_service "$SERVICE_URL"
            run_quick_test
            exit 0
            ;;
        --agents)
            show_banner
            demonstrate_agents
            exit 0
            ;;
        --observe)
            show_banner
            show_observability
            exit 0
            ;;
        *)
            show_banner
            wait_for_service "$SERVICE_URL"
            check_health
            demonstrate_agents
            local execution_id=$(submit_feature_request)
            monitor_execution "$execution_id"
            show_observability

            log_step "Demo Complete!"
            echo "The feature request has been submitted to the pipeline."
            echo "Use the observability dashboards to monitor progress."
            echo ""
            echo "Next steps:"
            echo "  1. Check Jaeger for distributed traces"
            echo "  2. Check Grafana for metrics dashboards"
            echo "  3. Monitor NATS for event flow"
            echo ""
            log_success "Thank you for trying Service Feature!"
            ;;
    esac
}

main "$@"
