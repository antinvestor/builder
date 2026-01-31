# builder Security Model

## Overview

This document defines the security architecture for builder, covering authentication, authorization, credential management, sandbox isolation, data protection, and audit capabilities.

---

## Security Zones

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        SECURITY ZONES                                            │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │ ZONE 0: PUBLIC EDGE (Untrusted)                                          │    │
│  │                                                                          │    │
│  │  Controls:                                                               │    │
│  │  • TLS 1.3 termination (no TLS 1.2 or below)                            │    │
│  │  • DDoS protection (rate limiting, connection limits)                   │    │
│  │  • Request validation (size limits, schema validation)                  │    │
│  │  • Authentication verification (JWT validation)                         │    │
│  │  • WAF rules (SQL injection, XSS prevention)                           │    │
│  │                                                                          │    │
│  │  Trust Level: None                                                       │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                     │                                            │
│                              mTLS   │                                            │
│                                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │ ZONE 1: APPLICATION SERVICES (Internal)                                  │    │
│  │                                                                          │    │
│  │  Controls:                                                               │    │
│  │  • Service mesh with mTLS (SPIFFE/SPIRE identities)                     │    │
│  │  • RBAC enforcement per service                                         │    │
│  │  • Network policies (namespace isolation)                               │    │
│  │  • Audit logging (all API calls logged)                                 │    │
│  │  • No direct external access                                            │    │
│  │                                                                          │    │
│  │  Trust Level: Verified service identity                                 │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                     │                                            │
│                          Encrypted  │                                            │
│                                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │ ZONE 2: SECRETS MANAGEMENT (Highly Restricted)                           │    │
│  │                                                                          │    │
│  │  Controls:                                                               │    │
│  │  • Hardware Security Module (HSM) for master keys                       │    │
│  │  • Short-lived credential leases (15 min default)                       │    │
│  │  • Per-feature credential scoping                                       │    │
│  │  • Automatic key rotation                                               │    │
│  │  • Comprehensive audit trail                                            │    │
│  │  • Break-glass procedures for emergency access                          │    │
│  │                                                                          │    │
│  │  Trust Level: Service identity + explicit policy                        │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                     │                                            │
│                                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │ ZONE 3: EXECUTION SANDBOX (Isolated)                                     │    │
│  │                                                                          │    │
│  │  Controls:                                                               │    │
│  │  • Container namespace isolation (PID, mount, network, user)            │    │
│  │  • Seccomp syscall filtering                                            │    │
│  │  • Read-only root filesystem                                            │    │
│  │  • No privileged operations                                             │    │
│  │  • Network egress whitelist only                                        │    │
│  │  • Resource limits (CPU, memory, disk, time)                            │    │
│  │  • Ephemeral (destroyed after execution)                                │    │
│  │                                                                          │    │
│  │  Trust Level: Untrusted code execution environment                      │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Authentication

### External Client Authentication

```
┌─────────────────────────────────────────────────────────────────┐
│                 CLIENT AUTHENTICATION FLOW                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Client                    API Gateway              OIDC Provider│
│     │                          │                         │       │
│     │  1. Request + JWT        │                         │       │
│     │─────────────────────────▶│                         │       │
│     │                          │                         │       │
│     │                          │  2. Validate signature  │       │
│     │                          │  (cached JWKS)          │       │
│     │                          │                         │       │
│     │                          │  3. If expired/unknown: │       │
│     │                          │     Fetch JWKS          │       │
│     │                          │────────────────────────▶│       │
│     │                          │                         │       │
│     │                          │◀────────────────────────│       │
│     │                          │     JWKS response       │       │
│     │                          │                         │       │
│     │                          │  4. Validate claims:    │       │
│     │                          │  • iss (issuer)         │       │
│     │                          │  • aud (audience)       │       │
│     │                          │  • exp (expiration)     │       │
│     │                          │  • tenant_id            │       │
│     │                          │                         │       │
│     │  5. Authorized request   │                         │       │
│     │◀─────────────────────────│                         │       │
│     │                          │                         │       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### JWT Claims Structure

```go
type JWTClaims struct {
    jwt.RegisteredClaims

    // Custom claims
    TenantID    string   `json:"tenant_id"`
    UserID      string   `json:"user_id"`
    Email       string   `json:"email,omitempty"`
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
}
```

### Service-to-Service Authentication

Internal services authenticate using mTLS with SPIFFE identities:

```
SPIFFE ID Format: spiffe://feature.example.com/{service-name}

Examples:
- spiffe://feature.example.com/feature-service
- spiffe://feature.example.com/feature-worker
- spiffe://feature.example.com/git-service
- spiffe://feature.example.com/llm-orchestrator
```

---

## Authorization

### RBAC Model

```go
// Role definitions
type Role string

const (
    RoleAdmin       Role = "admin"        // Full access
    RoleOperator    Role = "operator"     // Manage repositories, view features
    RoleDeveloper   Role = "developer"    // Create features, view own features
    RoleViewer      Role = "viewer"       // Read-only access
)

// Permission definitions
type Permission string

const (
    // Feature permissions
    PermFeatureCreate   Permission = "feature:create"
    PermFeatureRead     Permission = "feature:read"
    PermFeatureCancel   Permission = "feature:cancel"
    PermFeatureRetry    Permission = "feature:retry"
    PermFeatureReadAll  Permission = "feature:read:all"  // View all features

    // Repository permissions
    PermRepoCreate      Permission = "repository:create"
    PermRepoRead        Permission = "repository:read"
    PermRepoUpdate      Permission = "repository:update"
    PermRepoDelete      Permission = "repository:delete"
    PermRepoCredentials Permission = "repository:credentials"

    // Admin permissions
    PermAdminAll        Permission = "admin:*"
)
```

### Role-Permission Mapping

| Role | Permissions |
|------|-------------|
| `admin` | `admin:*` (all permissions) |
| `operator` | `repository:*`, `feature:read:all`, `feature:cancel` |
| `developer` | `feature:create`, `feature:read`, `feature:cancel`, `feature:retry`, `repository:read` |
| `viewer` | `feature:read`, `repository:read` |

### Authorization Enforcement

```go
// AuthorizationMiddleware enforces RBAC
func AuthorizationMiddleware(required Permission) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            claims := auth.ClaimsFromContext(ctx)
            if claims == nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing authentication"))
            }

            if !hasPermission(claims, required) {
                return nil, connect.NewError(connect.CodePermissionDenied,
                    fmt.Errorf("permission %s required", required))
            }

            return next(ctx, req)
        }
    }
}
```

### Tenant Isolation

All data access is scoped to the authenticated tenant:

```go
// TenantScope ensures all queries are tenant-scoped
type TenantScope struct {
    TenantID string
}

func (s *TenantScope) Apply(db *gorm.DB) *gorm.DB {
    return db.Where("tenant_id = ?", s.TenantID)
}

// Repository query with tenant scope
func (r *featureRepository) Get(ctx context.Context, id string) (*models.FeatureExecution, error) {
    scope := auth.TenantScopeFromContext(ctx)

    var feature models.FeatureExecution
    if err := r.db.WithContext(ctx).
        Scopes(scope.Apply).
        Where("id = ?", id).
        First(&feature).Error; err != nil {
        return nil, err
    }

    return &feature, nil
}
```

---

## Credential Management

### Credential Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                CREDENTIAL LIFECYCLE                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  REGISTRATION                                                    │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                          │    │
│  │  1. User submits credential via API                     │    │
│  │  2. Credential validated (test git access)              │    │
│  │  3. Credential encrypted with tenant DEK                │    │
│  │  4. Encrypted credential stored in Vault                │    │
│  │  5. Reference ID returned (not the credential)          │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  FEATURE EXECUTION                                               │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                          │    │
│  │  1. Worker requests credential lease for repo           │    │
│  │  2. Vault validates worker identity + feature binding   │    │
│  │  3. Short-lived lease issued (15 min TTL)               │    │
│  │  4. Credential decrypted and returned to worker         │    │
│  │  5. Worker injects into git environment (tmpfs)         │    │
│  │  6. Lease auto-revoked on expiry                        │    │
│  │  7. Worker clears credential from memory on completion  │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ROTATION                                                        │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                          │    │
│  │  1. User updates credential via API                     │    │
│  │  2. New credential validated                            │    │
│  │  3. Old credential marked for deletion                  │    │
│  │  4. In-flight features continue with old lease          │    │
│  │  5. Old credential deleted after grace period           │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Credential Types

| Type | Storage | Injection Method |
|------|---------|------------------|
| SSH Key | Vault (encrypted) | `SSH_AUTH_SOCK` or key file on tmpfs |
| Token | Vault (encrypted) | `GIT_ASKPASS` helper or `.git-credentials` |
| OAuth | Vault (encrypted) | Token refresh + `GIT_ASKPASS` helper |
| Basic Auth | Vault (encrypted) | Credential helper |

### Credential Provider Implementation

```go
// CredentialProvider retrieves credentials from Vault
type CredentialProvider struct {
    vault      *vault.Client
    encryption *encryption.Service
}

// GetCredential retrieves a credential with a lease
func (p *CredentialProvider) GetCredential(ctx context.Context, repoID, featureID string) (*Credential, *CredentialLease, error) {
    // Verify feature is authorized for this repo
    if err := p.validateFeatureRepoBinding(ctx, featureID, repoID); err != nil {
        return nil, nil, fmt.Errorf("unauthorized: %w", err)
    }

    // Read from Vault
    secret, err := p.vault.KVv2("credentials").Get(ctx, fmt.Sprintf("repos/%s", repoID))
    if err != nil {
        return nil, nil, fmt.Errorf("vault read: %w", err)
    }

    // Decrypt credential
    encryptedCred := secret.Data["credential"].(string)
    credBytes, err := p.encryption.Decrypt(ctx, encryptedCred)
    if err != nil {
        return nil, nil, fmt.Errorf("decrypt: %w", err)
    }

    var cred Credential
    if err := json.Unmarshal(credBytes, &cred); err != nil {
        return nil, nil, fmt.Errorf("unmarshal: %w", err)
    }

    // Create lease
    lease := &CredentialLease{
        LeaseID:   uuid.NewString(),
        RepoID:    repoID,
        FeatureID: featureID,
        ExpiresAt: time.Now().Add(15 * time.Minute),
        Renewable: true,
    }

    // Register lease
    if err := p.registerLease(ctx, lease); err != nil {
        return nil, nil, fmt.Errorf("register lease: %w", err)
    }

    return &cred, lease, nil
}
```

### Credential Injection

```go
// InjectCredential injects credentials into the git environment
func InjectCredential(cred *Credential, workspace *Workspace) (func(), error) {
    var cleanup func()

    switch cred.Type {
    case CredentialTypeSSHKey:
        // Write key to tmpfs
        keyPath := filepath.Join("/dev/shm", workspace.ID, "id_rsa")
        if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
            return nil, err
        }
        if err := os.WriteFile(keyPath, []byte(cred.SSHKey), 0600); err != nil {
            return nil, err
        }

        // Set environment
        os.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=accept-new", keyPath))

        cleanup = func() {
            os.Remove(keyPath)
            os.Unsetenv("GIT_SSH_COMMAND")
        }

    case CredentialTypeToken:
        // Create askpass helper
        helper := fmt.Sprintf("#!/bin/sh\necho '%s'", cred.Token)
        helperPath := filepath.Join("/dev/shm", workspace.ID, "git-askpass")
        if err := os.WriteFile(helperPath, []byte(helper), 0700); err != nil {
            return nil, err
        }

        os.Setenv("GIT_ASKPASS", helperPath)

        cleanup = func() {
            os.Remove(helperPath)
            os.Unsetenv("GIT_ASKPASS")
        }
    }

    return cleanup, nil
}
```

---

## Sandbox Security

### Isolation Model

```
┌─────────────────────────────────────────────────────────────────┐
│                 SANDBOX ISOLATION LAYERS                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ LAYER 1: NAMESPACE ISOLATION                             │    │
│  │                                                          │    │
│  │  PID Namespace:                                          │    │
│  │  • Sandbox processes isolated from host PID tree        │    │
│  │  • Init process (PID 1) manages sandbox lifecycle       │    │
│  │                                                          │    │
│  │  Mount Namespace:                                        │    │
│  │  • Isolated filesystem view                             │    │
│  │  • Overlay FS with ephemeral upper layer               │    │
│  │  • Read-only root filesystem                            │    │
│  │                                                          │    │
│  │  Network Namespace:                                      │    │
│  │  • Isolated network stack                               │    │
│  │  • Only loopback + controlled egress                    │    │
│  │                                                          │    │
│  │  User Namespace:                                         │    │
│  │  • UID/GID remapping (root in sandbox → nobody on host) │    │
│  │  • No actual root privileges                            │    │
│  │                                                          │    │
│  │  IPC Namespace:                                          │    │
│  │  • Isolated IPC primitives                              │    │
│  │  • No shared memory with host                           │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ LAYER 2: RESOURCE LIMITS (cgroups v2)                    │    │
│  │                                                          │    │
│  │  CPU:                                                    │    │
│  │  • cpu.max = "400000 100000" (4 cores max)              │    │
│  │  • cpu.weight = 100 (fair scheduling)                   │    │
│  │                                                          │    │
│  │  Memory:                                                 │    │
│  │  • memory.max = 8589934592 (8GB)                        │    │
│  │  • memory.swap.max = 0 (no swap)                        │    │
│  │  • memory.oom.group = 1                                 │    │
│  │                                                          │    │
│  │  I/O:                                                    │    │
│  │  • io.max = "8:0 rbps=104857600 wbps=104857600"        │    │
│  │  • (100MB/s read/write limit)                           │    │
│  │                                                          │    │
│  │  PIDs:                                                   │    │
│  │  • pids.max = 1024                                      │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ LAYER 3: SYSCALL FILTERING (Seccomp)                     │    │
│  │                                                          │    │
│  │  Default: SCMP_ACT_ERRNO                                │    │
│  │                                                          │    │
│  │  Allowed syscalls:                                       │    │
│  │  • File operations: read, write, open, close, stat...   │    │
│  │  • Process: fork, exec, exit, wait...                   │    │
│  │  • Network: socket, connect, send, recv (filtered)      │    │
│  │  • Memory: mmap, munmap, brk...                         │    │
│  │                                                          │    │
│  │  Blocked syscalls:                                       │    │
│  │  • mount, umount (filesystem modification)              │    │
│  │  • ptrace (debugging/injection)                         │    │
│  │  • reboot, kexec_load (system modification)             │    │
│  │  • module_* (kernel module loading)                     │    │
│  │  • *xattr (extended attributes)                         │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ LAYER 4: NETWORK POLICY                                  │    │
│  │                                                          │    │
│  │  Default Policy: DENY ALL                               │    │
│  │                                                          │    │
│  │  Egress Whitelist:                                       │    │
│  │  • DNS: 10.0.0.10:53 (internal DNS)                     │    │
│  │  • Package registries:                                   │    │
│  │    - registry.npmjs.org:443                             │    │
│  │    - pypi.org:443                                       │    │
│  │    - proxy.golang.org:443                               │    │
│  │    - crates.io:443                                      │    │
│  │  • Git remotes (per-feature dynamic whitelist)          │    │
│  │  • Internal services (LLM orchestrator, state store)    │    │
│  │                                                          │    │
│  │  Ingress: DENY ALL                                      │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Sandbox Configuration

```go
// SandboxConfig defines sandbox security parameters
type SandboxConfig struct {
    // Resource limits
    CPUCores        int           `yaml:"cpu_cores"`
    MemoryMB        int           `yaml:"memory_mb"`
    DiskMB          int           `yaml:"disk_mb"`
    MaxPIDs         int           `yaml:"max_pids"`
    IOBytesPerSec   int64         `yaml:"io_bytes_per_sec"`

    // Time limits
    ExecutionTimeout time.Duration `yaml:"execution_timeout"`

    // Network policy
    NetworkPolicy   NetworkPolicy `yaml:"network_policy"`

    // Security profile
    SeccompProfile  string        `yaml:"seccomp_profile"`
    AppArmorProfile string        `yaml:"apparmor_profile"`
    ReadOnlyRoot    bool          `yaml:"read_only_root"`
    NoNewPrivileges bool          `yaml:"no_new_privileges"`
    DropCapabilities []string     `yaml:"drop_capabilities"`
}

// Default sandbox configuration
var DefaultSandboxConfig = SandboxConfig{
    CPUCores:         4,
    MemoryMB:         8192,
    DiskMB:           20480,
    MaxPIDs:          1024,
    IOBytesPerSec:    104857600, // 100MB/s
    ExecutionTimeout: 10 * time.Minute,
    SeccompProfile:   "runtime/default",
    ReadOnlyRoot:     true,
    NoNewPrivileges:  true,
    DropCapabilities: []string{"ALL"},
}
```

---

## Data Protection

### Encryption at Rest

| Data Type | Encryption | Key Management |
|-----------|------------|----------------|
| Credentials | AES-256-GCM | Vault (tenant DEK) |
| Source Code | Encrypted volumes | Kubernetes secrets |
| Events | Database encryption | PostgreSQL TDE |
| Artifacts | Server-side encryption | S3 SSE-KMS |
| Logs | Encrypted | Logging service |

### Encryption Implementation

```go
// EncryptionService handles data encryption
type EncryptionService struct {
    activeKeyID string
    keys        map[string][]byte
    hmacKey     []byte
}

// Encrypt encrypts data using AES-256-GCM
func (s *EncryptionService) Encrypt(ctx context.Context, plaintext []byte) (string, error) {
    key := s.keys[s.activeKeyID]

    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

    // Format: keyID:base64(nonce+ciphertext)
    return fmt.Sprintf("%s:%s", s.activeKeyID, base64.StdEncoding.EncodeToString(ciphertext)), nil
}

// Decrypt decrypts AES-256-GCM encrypted data
func (s *EncryptionService) Decrypt(ctx context.Context, encrypted string) ([]byte, error) {
    parts := strings.SplitN(encrypted, ":", 2)
    if len(parts) != 2 {
        return nil, errors.New("invalid encrypted format")
    }

    keyID, ciphertextB64 := parts[0], parts[1]
    key, ok := s.keys[keyID]
    if !ok {
        return nil, fmt.Errorf("unknown key ID: %s", keyID)
    }

    ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
    if err != nil {
        return nil, err
    }

    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}

// CreateLookupToken creates an HMAC token for lookups
func (s *EncryptionService) CreateLookupToken(ctx context.Context, data []byte) ([]byte, error) {
    h := hmac.New(sha256.New, s.hmacKey)
    h.Write(data)
    return h.Sum(nil), nil
}
```

### Encryption in Transit

All internal communication uses mTLS:

```yaml
# Service mesh configuration
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: feature-system
spec:
  mtls:
    mode: STRICT

---
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: feature-worker-policy
  namespace: feature-system
spec:
  selector:
    matchLabels:
      app: feature-worker
  action: ALLOW
  rules:
  - from:
    - source:
        principals:
        - "cluster.local/ns/feature-system/sa/feature-service"
        - "cluster.local/ns/feature-system/sa/git-service"
        - "cluster.local/ns/feature-system/sa/llm-orchestrator"
```

---

## Audit Logging

### Audit Event Structure

```go
// AuditEvent represents a security-relevant action
type AuditEvent struct {
    EventID     string            `json:"event_id"`
    Timestamp   time.Time         `json:"timestamp"`
    Actor       AuditActor        `json:"actor"`
    Action      string            `json:"action"`
    Resource    AuditResource     `json:"resource"`
    Outcome     string            `json:"outcome"` // success, failure, denied
    Context     AuditContext      `json:"context"`
    Details     map[string]any    `json:"details,omitempty"`
}

type AuditActor struct {
    Type        string `json:"type"`     // user, service, feature
    ID          string `json:"id"`
    TenantID    string `json:"tenant_id"`
    Email       string `json:"email,omitempty"`
    ServiceName string `json:"service_name,omitempty"`
}

type AuditResource struct {
    Type string `json:"type"` // repository, feature, credential
    ID   string `json:"id"`
    Name string `json:"name,omitempty"`
}

type AuditContext struct {
    SourceIP      string `json:"source_ip"`
    UserAgent     string `json:"user_agent"`
    CorrelationID string `json:"correlation_id"`
    RequestID     string `json:"request_id"`
}
```

### Audited Actions

| Action | Description | Level |
|--------|-------------|-------|
| `credential.create` | Credential registered | HIGH |
| `credential.access` | Credential retrieved | HIGH |
| `credential.update` | Credential modified | HIGH |
| `credential.delete` | Credential deleted | HIGH |
| `repository.create` | Repository registered | MEDIUM |
| `repository.delete` | Repository removed | MEDIUM |
| `feature.create` | Feature execution started | LOW |
| `feature.cancel` | Feature execution cancelled | MEDIUM |
| `sandbox.create` | Sandbox provisioned | LOW |
| `auth.login` | User authenticated | MEDIUM |
| `auth.failed` | Authentication failed | HIGH |
| `permission.denied` | Authorization denied | HIGH |

### Audit Logger Implementation

```go
// AuditLogger logs security events
type AuditLogger struct {
    store  AuditStore
    logger *slog.Logger
}

// Log records an audit event
func (a *AuditLogger) Log(ctx context.Context, action string, resource AuditResource, outcome string, details map[string]any) {
    actor := auditActorFromContext(ctx)

    event := AuditEvent{
        EventID:   uuid.NewString(),
        Timestamp: time.Now().UTC(),
        Actor:     actor,
        Action:    action,
        Resource:  resource,
        Outcome:   outcome,
        Context: AuditContext{
            SourceIP:      sourceIPFromContext(ctx),
            UserAgent:     userAgentFromContext(ctx),
            CorrelationID: correlationIDFromContext(ctx),
            RequestID:     requestIDFromContext(ctx),
        },
        Details: details,
    }

    // Persist to audit store
    if err := a.store.Store(ctx, event); err != nil {
        a.logger.Error("failed to store audit event",
            "event_id", event.EventID,
            "action", action,
            "error", err,
        )
    }

    // Also log for real-time monitoring
    a.logger.Info("audit",
        "event_id", event.EventID,
        "action", action,
        "actor_type", actor.Type,
        "actor_id", actor.ID,
        "resource_type", resource.Type,
        "resource_id", resource.ID,
        "outcome", outcome,
    )
}
```

---

## Security Checklist

### Pre-Deployment

- [ ] TLS certificates provisioned and rotated
- [ ] OIDC provider configured
- [ ] Vault unsealed and policies configured
- [ ] Service mesh mTLS enabled
- [ ] Network policies deployed
- [ ] Seccomp profiles loaded
- [ ] Audit logging enabled
- [ ] Secrets encrypted at rest

### Runtime

- [ ] JWT validation enabled
- [ ] Rate limiting configured
- [ ] Credential leases expiring correctly
- [ ] Sandbox isolation verified
- [ ] Audit events flowing
- [ ] Alerts configured for security events

### Periodic

- [ ] Key rotation scheduled
- [ ] Credential rotation enforced
- [ ] Security patches applied
- [ ] Access reviews completed
- [ ] Audit log review
- [ ] Penetration testing

---

## Incident Response

### Security Event Classification

| Severity | Examples | Response Time |
|----------|----------|---------------|
| Critical | Credential exposure, unauthorized access | Immediate |
| High | Auth bypass attempt, sandbox escape | < 1 hour |
| Medium | Rate limit breach, suspicious activity | < 4 hours |
| Low | Failed auth attempts, policy violations | < 24 hours |

### Response Procedures

1. **Detection**: Alert triggered via monitoring
2. **Triage**: Assess severity and scope
3. **Containment**: Isolate affected resources
4. **Eradication**: Remove threat
5. **Recovery**: Restore normal operations
6. **Lessons Learned**: Post-incident review
