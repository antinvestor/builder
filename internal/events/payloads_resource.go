package events

import "time"

// ===== RESOURCE ACQUISITION =====

// ResourcesAcquiredPayload is the payload for ResourcesAcquired.
type ResourcesAcquiredPayload struct {
	// Resources are the acquired resources.
	Resources []AcquiredResource `json:"resources"`

	// TotalWaitMS is total wait time to acquire all resources.
	TotalWaitMS int64 `json:"total_wait_ms"`

	// AcquiredAt is when resources were acquired.
	AcquiredAt time.Time `json:"acquired_at"`
}

// AcquiredResource describes an acquired resource.
type AcquiredResource struct {
	// Type is the resource type.
	Type ResourceType `json:"type"`

	// Identifier uniquely identifies the resource.
	Identifier string `json:"identifier"`

	// Details provides type-specific details.
	Details map[string]string `json:"details,omitempty"`

	// ExpiresAt is when the resource lock expires.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// ResourceType categorizes resource types.
type ResourceType string

const (
	ResourceTypeLock        ResourceType = "lock"        // Distributed lock
	ResourceTypeCredential  ResourceType = "credential"  // Git credential
	ResourceTypeWorkspace   ResourceType = "workspace"   // Workspace directory
	ResourceTypeQuota       ResourceType = "quota"       // Resource quota allocation
	ResourceTypeSandbox     ResourceType = "sandbox"     // Execution sandbox
)

// ResourcesReleasedPayload is the payload for ResourcesReleased.
type ResourcesReleasedPayload struct {
	// Resources are the released resources.
	Resources []ReleasedResource `json:"resources"`

	// ReleasedAt is when resources were released.
	ReleasedAt time.Time `json:"released_at"`
}

// ReleasedResource describes a released resource.
type ReleasedResource struct {
	// Type is the resource type.
	Type ResourceType `json:"type"`

	// Identifier uniquely identifies the resource.
	Identifier string `json:"identifier"`

	// HeldDurationMS is how long the resource was held.
	HeldDurationMS int64 `json:"held_duration_ms"`

	// ReleaseReason indicates why the resource was released.
	ReleaseReason ReleaseReason `json:"release_reason"`
}

// ReleaseReason indicates why a resource was released.
type ReleaseReason string

const (
	ReleaseReasonCompleted ReleaseReason = "completed" // Normal completion
	ReleaseReasonFailed    ReleaseReason = "failed"    // Execution failed
	ReleaseReasonAborted   ReleaseReason = "aborted"   // User aborted
	ReleaseReasonTimeout   ReleaseReason = "timeout"   // Lease expired
	ReleaseReasonCleanup   ReleaseReason = "cleanup"   // System cleanup
)

// ===== SANDBOX EVENTS =====

// SandboxCreatedPayload is the payload for SandboxCreated.
type SandboxCreatedPayload struct {
	// SandboxID uniquely identifies the sandbox.
	SandboxID string `json:"sandbox_id"`

	// Type is the sandbox type.
	Type SandboxType `json:"type"`

	// Config contains sandbox configuration.
	Config SandboxConfig `json:"config"`

	// NetworkConfig contains network settings.
	NetworkConfig SandboxNetworkConfig `json:"network_config"`

	// CreatedAt is when sandbox was created.
	CreatedAt time.Time `json:"created_at"`
}

// SandboxType identifies sandbox implementation.
type SandboxType string

const (
	SandboxTypeContainer  SandboxType = "container"   // Docker/containerd
	SandboxTypeFirecracker SandboxType = "firecracker" // Firecracker microVM
	SandboxTypeGVisor     SandboxType = "gvisor"      // gVisor
	SandboxTypeNone       SandboxType = "none"        // No sandbox (testing)
)

// SandboxConfig contains sandbox configuration.
type SandboxConfig struct {
	// Image is the container/VM image.
	Image string `json:"image"`

	// CPULimit is CPU limit (e.g., "2" for 2 cores).
	CPULimit string `json:"cpu_limit"`

	// MemoryLimitMB is memory limit in MB.
	MemoryLimitMB int `json:"memory_limit_mb"`

	// DiskLimitMB is disk limit in MB.
	DiskLimitMB int `json:"disk_limit_mb"`

	// TimeoutSeconds is execution timeout.
	TimeoutSeconds int `json:"timeout_seconds"`

	// Environment are environment variables.
	Environment map[string]string `json:"environment,omitempty"`

	// Mounts are volume mounts.
	Mounts []SandboxMount `json:"mounts,omitempty"`
}

// SandboxMount describes a volume mount.
type SandboxMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

// SandboxNetworkConfig contains network configuration.
type SandboxNetworkConfig struct {
	// Mode is the network mode.
	Mode NetworkMode `json:"mode"`

	// AllowedHosts are hosts the sandbox can reach.
	AllowedHosts []string `json:"allowed_hosts,omitempty"`

	// AllowedPorts are ports the sandbox can connect to.
	AllowedPorts []int `json:"allowed_ports,omitempty"`

	// DNSServers are custom DNS servers.
	DNSServers []string `json:"dns_servers,omitempty"`
}

// NetworkMode indicates sandbox network mode.
type NetworkMode string

const (
	NetworkModeNone     NetworkMode = "none"     // No network access
	NetworkModeHost     NetworkMode = "host"     // Host network (not recommended)
	NetworkModeBridge   NetworkMode = "bridge"   // Bridge network (default)
	NetworkModeRestricted NetworkMode = "restricted" // Limited outbound only
)

// SandboxDestroyedPayload is the payload for SandboxDestroyed.
type SandboxDestroyedPayload struct {
	// SandboxID is the destroyed sandbox ID.
	SandboxID string `json:"sandbox_id"`

	// Reason is why the sandbox was destroyed.
	Reason SandboxDestroyReason `json:"reason"`

	// LifetimeMS is how long the sandbox existed.
	LifetimeMS int64 `json:"lifetime_ms"`

	// ResourceUsage contains resource usage statistics.
	ResourceUsage SandboxResourceUsage `json:"resource_usage"`

	// DestroyedAt is when sandbox was destroyed.
	DestroyedAt time.Time `json:"destroyed_at"`
}

// SandboxDestroyReason indicates why sandbox was destroyed.
type SandboxDestroyReason string

const (
	SandboxDestroyReasonCompleted SandboxDestroyReason = "completed"
	SandboxDestroyReasonFailed    SandboxDestroyReason = "failed"
	SandboxDestroyReasonTimeout   SandboxDestroyReason = "timeout"
	SandboxDestroyReasonOOM       SandboxDestroyReason = "oom"     // Out of memory
	SandboxDestroyReasonAborted   SandboxDestroyReason = "aborted"
	SandboxDestroyReasonCleanup   SandboxDestroyReason = "cleanup" // System cleanup
)

// SandboxResourceUsage contains resource usage statistics.
type SandboxResourceUsage struct {
	// CPUSecondsUsed is CPU time used.
	CPUSecondsUsed float64 `json:"cpu_seconds_used"`

	// PeakMemoryMB is peak memory usage.
	PeakMemoryMB int `json:"peak_memory_mb"`

	// DiskReadMB is disk read in MB.
	DiskReadMB int `json:"disk_read_mb"`

	// DiskWriteMB is disk write in MB.
	DiskWriteMB int `json:"disk_write_mb"`

	// NetworkIngressMB is network ingress in MB.
	NetworkIngressMB int `json:"network_ingress_mb"`

	// NetworkEgressMB is network egress in MB.
	NetworkEgressMB int `json:"network_egress_mb"`
}

// ===== LOCKING HELPERS =====

// LockInfo describes a distributed lock.
type LockInfo struct {
	// Key is the lock key.
	Key string `json:"key"`

	// Owner is the lock owner (execution ID).
	Owner string `json:"owner"`

	// AcquiredAt is when the lock was acquired.
	AcquiredAt time.Time `json:"acquired_at"`

	// ExpiresAt is when the lock expires.
	ExpiresAt time.Time `json:"expires_at"`

	// Metadata contains lock metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}
