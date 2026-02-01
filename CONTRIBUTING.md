# Contributing to builder

Thank you for your interest in contributing to the builder platform. This document outlines the contribution process and requirements.

## Development Setup

### Prerequisites

- Go 1.23+
- Docker
- kubectl (for Kubernetes development)
- golangci-lint

### Getting Started

```bash
# Clone the repository
git clone https://github.com/antinvestor/builder.git
cd builder

# Install dependencies
go mod download

# Run tests
go test ./...

# Run linter
golangci-lint run
```

## Branch Protection Requirements

The `main` branch is protected with the following requirements. These must be configured in GitHub Settings > Branches > Branch protection rules.

### Required for Main Branch

| Setting | Value | Rationale |
|---------|-------|-----------|
| **Require a pull request before merging** | Enabled | All changes must go through code review |
| **Require approvals** | 1 minimum | At least one reviewer must approve |
| **Dismiss stale pull request approvals** | Enabled | Re-review required after new commits |
| **Require review from code owners** | Recommended | Critical paths reviewed by owners |
| **Require status checks to pass** | Enabled | CI must pass before merge |
| **Require branches to be up to date** | Enabled | Branch must be current with main |
| **Require conversation resolution** | Enabled | All review comments must be addressed |
| **Do not allow bypassing** | Enabled | Admins cannot bypass protections |

### Required Status Checks

The following CI checks must pass before a PR can be merged:

| Check | Description |
|-------|-------------|
| `lint` | golangci-lint must pass with no errors |
| `test` | All unit tests must pass |
| `build` | Go build must succeed |

### Configuring Branch Protection

To configure branch protection in GitHub:

1. Go to **Settings** > **Branches**
2. Click **Add rule** (or edit existing)
3. Set **Branch name pattern**: `main`
4. Enable the following:

```
[x] Require a pull request before merging
    [x] Require approvals: 1
    [x] Dismiss stale pull request approvals when new commits are pushed
    [x] Require review from code owners

[x] Require status checks to pass before merging
    [x] Require branches to be up to date before merging
    Status checks that are required:
    - lint
    - test
    - build

[x] Require conversation resolution before merging

[ ] Require signed commits (optional, recommended)

[x] Do not allow bypassing the above settings
```

## Pull Request Process

### Before Creating a PR

1. **Create an issue first** - Discuss changes before implementing
2. **Create a feature branch** - Branch from `main`
3. **Run tests locally** - Ensure all tests pass
4. **Run linter** - Fix all linting issues
5. **Keep changes focused** - One issue per PR

### PR Naming Convention

```
{type}: {short description}

Types:
- feat: New feature
- fix: Bug fix
- refactor: Code refactoring
- docs: Documentation
- test: Test additions/changes
- chore: Maintenance
```

Examples:
- `feat: add persistent workspace tracking`
- `fix: prevent rate limiting on LLM calls`
- `docs: update architecture documentation`

### PR Description Template

```markdown
## Summary

Brief description of changes.

Closes #{issue_number}

## Changes

- Change 1
- Change 2

## Testing

Describe testing performed.

## Checklist

- [ ] Tests pass locally
- [ ] Linter passes
- [ ] Documentation updated (if applicable)
- [ ] No breaking changes (or documented in PR)
```

### Review Process

1. **Automated checks run** - CI must pass
2. **Code review** - At least one approval required
3. **Address feedback** - Respond to all comments
4. **Merge** - Squash and merge preferred

## Code Style

### Go Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `goimports` for formatting
- Run `golangci-lint` before committing
- Write table-driven tests
- Use `util.Log(ctx)` for all logging
- Use `errors.Is()` for error comparison

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
{type}({scope}): {description}

{body}

{footer}
```

Example:

```
feat(llm): add rate limiting for provider clients

- Add token bucket rate limiter per provider
- Configure default RPS limits
- Add burst capacity configuration

Closes #34
```

## Testing Requirements

### Unit Tests

- All new code should have tests
- Aim for >80% coverage on new code
- Use table-driven tests for comprehensive coverage
- Use testcontainers for integration tests

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/llm/...

# Run with race detection
go test -race ./...
```

## CI/CD Pipeline

### Pull Request Checks

Every PR runs:

1. **Lint** - `golangci-lint run`
2. **Test** - `go test -race ./...`
3. **Build** - `go build ./...`

### Release Process

Releases are triggered by pushing a tag:

```bash
# Create and push a release tag
git tag v1.0.0
git push origin v1.0.0
```

This triggers:

1. **Build** - Multi-platform Docker images
2. **Push** - Images pushed to registry
3. **Release** - GitHub release created with changelog

## Security

### Reporting Vulnerabilities

Please report security vulnerabilities privately via GitHub Security Advisories rather than public issues.

### Security Requirements

- Never commit secrets or credentials
- Use environment variables for configuration
- Follow OWASP guidelines for input validation
- Use prepared statements for database queries

## Getting Help

- **Issues**: Use GitHub Issues for bugs and features
- **Discussions**: Use GitHub Discussions for questions
- **Documentation**: See `/docs` directory

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.
