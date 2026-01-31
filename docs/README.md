# builder

Autonomous Feature-Building Platform - An event-driven system that autonomously builds features into existing applications using LLM-powered code generation.

## Overview

builder is a git-agnostic, event-driven platform that:
- Builds features into existing applications autonomously
- Works with any Git-compatible repository (GitHub, GitLab, Bitbucket, self-hosted)
- Uses internal event-driven instructions for workflow orchestration
- Supports many concurrent feature executions with strict per-feature ordering
- Integrates with LLMs via BAML for intelligent code generation

## Architecture

The platform follows a layered, event-sourced architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                      API Gateway Layer                           │
│  (Feature Submission, Repository Registration, Observability)    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Event Bus (Partitioned Log)                   │
│              Partition key: feature_execution_id                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Feature Worker Pool                          │
│        (Stateless workers with partition affinity)               │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  Git Operations │ │ LLM Orchestrator│ │  Code Analysis  │
│     Service     │ │  (BAML Runtime) │ │     Service     │
└─────────────────┘ └─────────────────┘ └─────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Sandbox Execution Environment                  │
│          (Isolated builds, tests, code execution)                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Persistence Layer                           │
│        (Event Store, State Store, Artifact Store)                │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
github.com/antinvestor/builder/
├── apps/
│   ├── default/                    # Main feature execution service
│   │   ├── cmd/
│   │   │   └── main.go            # Service entry point
│   │   ├── config/
│   │   │   └── config.go          # Configuration structs
│   │   ├── migrations/            # SQL migration files
│   │   ├── service/
│   │   │   ├── handlers/          # gRPC/Connect handlers
│   │   │   ├── business/          # Business logic layer
│   │   │   ├── repository/        # Data access layer
│   │   │   ├── models/            # Domain models
│   │   │   ├── events/            # Event handlers
│   │   │   └── queue/             # Queue processors
│   │   ├── tests/                 # Test suites
│   │   └── Dockerfile
│   ├── worker/                     # Feature execution workers
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── config/
│   │   ├── executor/              # Feature execution engine
│   │   ├── git/                   # Git operations abstraction
│   │   ├── llm/                   # BAML integration
│   │   ├── sandbox/               # Sandbox management
│   │   └── Dockerfile
│   └── gateway/                    # API gateway service
│       ├── cmd/
│       │   └── main.go
│       ├── config/
│       └── Dockerfile
├── internal/
│   ├── events/                    # Shared event definitions
│   ├── gitutil/                   # Git utilities
│   ├── cryptoutil/                # Encryption utilities
│   ├── sandboxutil/               # Sandbox utilities
│   └── bamlutil/                  # BAML utilities
├── baml/                          # BAML function definitions
│   ├── analyze.baml
│   ├── plan.baml
│   ├── generate.baml
│   └── validate.baml
├── .github/workflows/             # CI/CD pipelines
├── Makefile                       # Build targets
├── go.mod
├── go.sum
└── README.md
```

## Configuration

### Environment Variables

```bash
# Service Core
SERVICE_NAME=builder
SERVICE_PORT=80

# Database Configuration
DATABASE_URL=postgres://user:pass@host:5432/feature_db
DO_DATABASE_MIGRATE=true

# Event Bus Configuration
EVENT_BUS_BROKERS=broker1:9092,broker2:9092
EVENT_BUS_TOPIC_PREFIX=feature
EVENT_BUS_CONSUMER_GROUP=feature-workers
EVENT_BUS_PARTITION_COUNT=64

# Git Operations
GIT_WORKSPACE_BASE_PATH=/var/feature/workspaces
GIT_CLONE_TIMEOUT_SECONDS=300
GIT_OPERATION_TIMEOUT_SECONDS=60

# LLM Configuration (BAML)
BAML_CLIENT_PROVIDER=anthropic
BAML_CLIENT_MODEL=claude-sonnet-4-20250514
BAML_CLIENT_API_KEY=${ANTHROPIC_API_KEY}
BAML_MAX_TOKENS=8192
BAML_TEMPERATURE=0.1

# Sandbox Configuration
SANDBOX_RUNTIME=firecracker
SANDBOX_IMAGE_REGISTRY=registry.example.com
SANDBOX_CPU_LIMIT=4
SANDBOX_MEMORY_LIMIT_MB=8192
SANDBOX_TIMEOUT_SECONDS=600
SANDBOX_NETWORK_EGRESS_WHITELIST=github.com,gitlab.com,npmjs.org

# Secrets Management
VAULT_ADDR=https://vault.example.com
VAULT_TOKEN=${VAULT_TOKEN}
VAULT_CREDENTIAL_PATH=secret/data/feature/credentials

# Security
DEK_ACTIVE_KEY_ID=key-2024-01
DEK_ACTIVE_ENCRYPTION_TOKEN=${ENCRYPTION_KEY}
DEK_LOOKUP_TOKEN=${LOOKUP_HMAC_KEY}

# OIDC Configuration
OIDC_ISSUER_URL=https://auth.example.com
OIDC_CLIENT_ID=builder
OIDC_CLIENT_SECRET=${OIDC_SECRET}

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
LOG_LEVEL=info
LOG_FORMAT=json
```

## Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 15+
- Kafka/Redpanda (Event Bus)
- Docker (for sandboxing)
- Vault (for secrets management)

### Local Development

```bash
# Start dependencies
make docker-setup

# Wait for postgres
make pg_wait

# Run migrations
DO_DATABASE_MIGRATE=true go run ./apps/default/cmd/main.go

# Start the service
go run ./apps/default/cmd/main.go

# In another terminal, start workers
go run ./apps/worker/cmd/main.go
```

### Running Tests

```bash
# Run all tests
make tests

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## API Documentation

See [API Reference](./api-reference.md) for detailed API documentation.

Key endpoints:
- `FeatureService.Create` - Submit a new feature request
- `FeatureService.Get` - Get feature execution status
- `FeatureService.Search` - Search feature executions
- `FeatureService.Cancel` - Cancel a feature execution
- `RepositoryService.Register` - Register a repository
- `RepositoryService.UpdateCredentials` - Update repository credentials

## Event System

The platform uses event sourcing for all state management. See [Event Reference](./event-reference.md) for the complete event catalog.

Key event flows:
1. Feature Lifecycle: `FeatureRequested` → `AnalysisStarted` → `PlanGenerated` → `ExecutionCompleted` → `FeatureDelivered`
2. Git Operations: `BranchCreated` → `CommitCreated` → `PushCompleted`
3. Verification: `VerificationStarted` → `BuildExecuted` → `TestsExecuted` → `VerificationPassed`

## Security

- All credentials stored in Vault with short-lived leases
- Git credentials scoped to specific repositories
- Sandbox isolation using Firecracker/gVisor
- mTLS between all internal services
- Audit logging for all security-relevant operations

See [Security Model](./security-model.md) for details.

## Deployment

See [Deployment Guide](./deployment-guide.md) for production deployment instructions.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests (`make tests`)
5. Submit a pull request

## License

Copyright (c) 2024-2025 Antinvestor. All rights reserved.
