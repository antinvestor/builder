# Service Feature

**Autonomous Feature Building Platform** - A composable, event-driven microservices platform that automatically generates, tests, reviews, and delivers code features using AI agents.

## Architecture

The platform consists of four independently deployable services:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Service Feature Platform                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌──────────────┐                                                          │
│   │   Gateway    │  HTTP API entry point                                    │
│   │  (stateless) │  • Receives feature requests                             │
│   └──────┬───────┘  • Publishes to NATS queue                               │
│          │          • Scale: Horizontal (unlimited)                         │
│          ▼                                                                   │
│   ┌──────────────┐                                                          │
│   │   Worker     │  Main event processor                                    │
│   │  (stateful)  │  • Repository checkout                                   │
│   └──────┬───────┘  • Patch generation (LLM)                                │
│          │          • Scale: Based on queue depth                           │
│          │                                                                   │
│   ┌──────┴───────┐                                                          │
│   │              │                                                          │
│   ▼              ▼                                                          │
│ ┌────────────┐ ┌────────────┐                                               │
│ │  Reviewer  │ │  Executor  │                                               │
│ │ (security) │ │ (sandbox)  │                                               │
│ └────────────┘ └────────────┘                                               │
│   • Security     • Test execution                                           │
│   • Architecture • Docker sandbox                                           │
│   • Risk scoring • Needs Docker socket                                      │
│   • Decisions    • Scale: Carefully                                         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Services

| Service | Purpose | Scaling | Resources |
|---------|---------|---------|-----------|
| **gateway** | HTTP API entry point | Horizontal (stateless) | 128MB, 0.5 CPU |
| **worker** | Event processing, LLM calls | Queue depth | 512MB, 1.0 CPU |
| **reviewer** | Security & architecture review | Independent | 256MB, 0.5 CPU |
| **executor** | Sandboxed test execution | Careful (Docker) | 512MB, 2.0 CPU |

## Quick Start

### Prerequisites

- Docker & Docker Compose v2.0+
- Git
- Make (optional)

### 1. Start Services

```bash
cd service-feature

# Start all core services (no observability)
make up

# Or start minimal set
make up-minimal

# Or with full observability stack
make up-full
```

### 2. Verify

```bash
# Check health
curl http://localhost:8080/health
# {"status":"healthy","service":"gateway"}

# Or use make
make health
```

### 3. Run Demo

```bash
make demo
```

## Project Structure

```
service-feature/
├── apps/                          # Independent services
│   ├── gateway/                   # HTTP API entry point
│   │   ├── cmd/main.go
│   │   ├── config/config.go
│   │   ├── Dockerfile
│   │   └── service/
│   ├── worker/                    # Main event processor
│   │   ├── cmd/main.go
│   │   ├── config/config.go
│   │   ├── Dockerfile
│   │   └── service/
│   ├── reviewer/                  # Review & control agent
│   │   ├── cmd/main.go
│   │   ├── config/config.go
│   │   ├── Dockerfile
│   │   └── service/
│   └── executor/                  # Sandbox execution
│       ├── cmd/main.go
│       ├── config/config.go
│       ├── Dockerfile
│       └── service/
├── internal/                      # Shared code
│   ├── events/                    # Event types & payloads
│   ├── models/                    # Shared data models
│   └── utils/                     # Shared utilities
├── configs/                       # Configuration files
├── scripts/                       # Utility scripts
├── examples/                      # Example requests
├── docker-compose.yml             # Multi-service deployment
├── Makefile                       # Build & operations
└── go.mod                         # Go module
```

## Service Configuration

### Gateway

```bash
# Environment variables
SERVICE_NAME=feature_gateway
SERVER_ADDRESS=:8080
QUEUE_FEATURE_REQUEST_URI=nats://nats:4222/feature.requests
```

### Worker

```bash
# Environment variables
SERVICE_NAME=feature_worker
DATABASE_URL=postgres://feature:feature@postgres:5432/feature
ANTHROPIC_API_KEY=your-key
WORKSPACE_BASE_PATH=/var/lib/feature-service/workspaces
```

### Reviewer

```bash
# Environment variables
SERVICE_NAME=feature_reviewer
MAX_RISK_SCORE=50
MAX_CRITICAL_ISSUES=0
BLOCK_ON_SECRETS=true
```

### Executor

```bash
# Environment variables
SERVICE_NAME=feature_executor
SANDBOX_ENABLED=true
SANDBOX_TYPE=docker
SANDBOX_MEMORY_LIMIT_MB=2048
MAX_CONCURRENT_EXECUTIONS=5
```

## Common Commands

### Service Management

```bash
make up              # Start all services
make up-minimal      # Start gateway + worker only
make up-full         # Include observability stack
make down            # Stop all services
make restart         # Restart all
make restart-worker  # Restart specific service
make ps              # Show status
```

### Scaling

```bash
make scale-gateway N=3   # Scale gateway to 3 instances
make scale-worker N=5    # Scale workers to 5 instances
make scale-reviewer N=2  # Scale reviewers
```

### Building

```bash
make build           # Build all services
make build-gateway   # Build specific service
make docker-build    # Build all Docker images
make docker-build-worker  # Build specific image
```

### Development

```bash
make run-gateway     # Run gateway locally
make run-worker      # Run worker locally
make test            # Run all tests
make test-worker     # Test specific service
make fmt             # Format code
make lint            # Run linter
```

### Logs & Debugging

```bash
make logs            # All service logs
make logs-worker     # Specific service logs
make nats-monitor    # Watch NATS events
make db-shell        # PostgreSQL shell
```

## Deployment Scenarios

### Minimal (Development)

```bash
make up-minimal
```

Starts: gateway, worker, postgres, nats

### Standard (Production-like)

```bash
make up
```

Starts: All 4 services + postgres + nats

### Full Stack (With Observability)

```bash
make up-full
```

Includes: Grafana, Prometheus, Jaeger, Redis

## Scaling Guidelines

| Service | When to Scale | Considerations |
|---------|--------------|----------------|
| gateway | High request rate | Stateless, safe to scale horizontally |
| worker | Queue backlog | Each needs workspace storage |
| reviewer | Review backlog | Stateless, safe to scale |
| executor | Test backlog | Needs Docker socket access |

### Example: Scale for High Load

```bash
# Scale gateway for high request rate
make scale-gateway N=5

# Scale workers for processing backlog
make scale-worker N=10

# Scale reviewers
make scale-reviewer N=3
```

## Service Endpoints

| Service | Port | Endpoints |
|---------|------|-----------|
| gateway | 8080 | `/health`, `/ready`, `/api/v1/features` |
| worker | 8080 | `/health`, `/ready` |
| reviewer | 8080 | `/health`, `/ready`, `/api/v1/killswitch/status` |
| executor | 8080 | `/health`, `/ready`, `/api/v1/executions/active` |

## Infrastructure

| Service | Port | Purpose |
|---------|------|---------|
| PostgreSQL | 5432 | Primary database |
| NATS | 4222, 8222 | Message queue |
| Redis | 6379 | Cache (optional) |
| Grafana | 3000 | Dashboards (optional) |
| Prometheus | 9090 | Metrics (optional) |
| Jaeger | 16686 | Tracing (optional) |

## Example Feature Request

```json
{
  "execution_id": "demo000000001",
  "repository_url": "https://github.com/example/repo.git",
  "branch": "main",
  "specification": {
    "title": "Add greeting function",
    "description": "Create a greeting function",
    "requirements": [
      "Accept name parameter",
      "Return 'Hello, {name}!'",
      "Add unit tests"
    ],
    "target_files": ["src/greeting.go"],
    "language": "go"
  }
}
```

### Submit via NATS

```bash
# Enter NATS container
docker exec -it feature-nats sh

# Publish request
nats pub feature.requests "$(cat /examples/simple-feature.json)"
```

## Troubleshooting

### Services Won't Start

```bash
make logs-all        # Check all logs
make ps              # Check status
make down-clean      # Reset everything
make up
```

### Database Issues

```bash
make db-shell        # Access PostgreSQL
make db-reset        # Reset database
```

### Queue Issues

```bash
curl http://localhost:8222/varz  # NATS status
make nats-monitor                # Watch events
```

## Documentation

- [Agents.md](Agents.md) - Detailed agent architecture and implementation
- [examples/](examples/) - Example feature requests

## License

[License information here]
