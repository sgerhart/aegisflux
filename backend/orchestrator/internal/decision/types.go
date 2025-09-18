package decision

import (
	"time"
)

// Plan represents a decision plan from the decision engine
type Plan struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Priority    int            `json:"priority"`
	Status      string         `json:"status"` // "pending", "active", "paused", "completed", "failed"
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Controls    []Control      `json:"controls"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Control represents a control action within a plan
type Control struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "ebpf_drop_egress", "ebpf_deny_syscall", etc.
	Target      string                 `json:"target"`
	Parameters  map[string]interface{} `json:"parameters"`
	ArtifactID  string                 `json:"artifact_id,omitempty"`
	Status      string                 `json:"status"` // "pending", "deployed", "active", "failed", "rollback"
	DeployedAt  *time.Time             `json:"deployed_at,omitempty"`
	ActivatedAt *time.Time             `json:"activated_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PlanUpdateRequest represents a request to update a plan
type PlanUpdateRequest struct {
	PlanID      string                 `json:"plan_id"`
	Controls    []Control              `json:"controls"`
	UpdateType  string                 `json:"update_type"` // "add", "update", "remove"
	Reason      string                 `json:"reason"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PlanUpdateResponse represents the response from a plan update
type PlanUpdateResponse struct {
	PlanID        string            `json:"plan_id"`
	Status        string            `json:"status"`
	UpdatedControls []Control       `json:"updated_controls"`
	ArtifactIDs   []string          `json:"artifact_ids"`
	DeploymentIDs []string          `json:"deployment_ids"`
	UpdatedAt     time.Time         `json:"updated_at"`
	Error         string            `json:"error,omitempty"`
}

// DeploymentRequest represents a request to deploy plan controls
type DeploymentRequest struct {
	PlanID       string   `json:"plan_id"`
	ControlIDs   []string `json:"control_ids,omitempty"` // If empty, deploy all controls
	DeploymentType string `json:"deployment_type"` // "canary", "enforce", "rollback"
	Targets      []string `json:"targets,omitempty"`
	TTL          int      `json:"ttl,omitempty"`
	Reason       string   `json:"reason"`
}

// DeploymentResponse represents the response from a deployment request
type DeploymentResponse struct {
	PlanID        string            `json:"plan_id"`
	DeploymentID  string            `json:"deployment_id"`
	Status        string            `json:"status"`
	DeployedControls []string       `json:"deployed_controls"`
	Targets       []string          `json:"targets"`
	DeployedAt    time.Time         `json:"deployed_at"`
	Error         string            `json:"error,omitempty"`
}

// ControlStatus represents the status of a control deployment
type ControlStatus struct {
	ControlID     string    `json:"control_id"`
	Status        string    `json:"status"`
	ArtifactID    string    `json:"artifact_id,omitempty"`
	DeploymentID  string    `json:"deployment_id,omitempty"`
	LastUpdate    time.Time `json:"last_update"`
	Error         string    `json:"error,omitempty"`
	Telemetry     map[string]interface{} `json:"telemetry,omitempty"`
}

// PlanStatus represents the overall status of a plan
type PlanStatus struct {
	PlanID           string          `json:"plan_id"`
	Status           string          `json:"status"`
	TotalControls    int             `json:"total_controls"`
	ActiveControls   int             `json:"active_controls"`
	FailedControls   int             `json:"failed_controls"`
	RollbackControls int             `json:"rollback_controls"`
	Controls         []ControlStatus `json:"controls"`
	LastUpdate       time.Time       `json:"last_update"`
}

// EBFPControlTypes defines the supported eBPF control types
var EBFPControlTypes = map[string]bool{
	"ebpf_drop_egress_by_cgroup": true,
	"ebpf_deny_syscall_for_cgroup": true,
	"ebpf_drop_egress_by_process": true,
	"ebpf_deny_syscall_for_process": true,
	"ebpf_rate_limit": true,
	"ebpf_traffic_monitor": true,
}

// IsEBPFControl checks if a control type is an eBPF control
func IsEBPFControl(controlType string) bool {
	_, exists := EBFPControlTypes[controlType]
	return exists
}

// EBFPControlTemplate defines the template mapping for eBPF controls
var EBFPControlTemplate = map[string]string{
	"ebpf_drop_egress_by_cgroup": "drop_egress_by_cgroup",
	"ebpf_deny_syscall_for_cgroup": "deny_syscall_for_cgroup",
	"ebpf_drop_egress_by_process": "drop_egress_by_cgroup", // Could be a variant
	"ebpf_deny_syscall_for_process": "deny_syscall_for_cgroup", // Could be a variant
	"ebpf_rate_limit": "rate_limit",
	"ebpf_traffic_monitor": "traffic_monitor",
}

// GetEBPFTemplate returns the template name for an eBPF control type
func GetEBPFTemplate(controlType string) string {
	if template, exists := EBFPControlTemplate[controlType]; exists {
		return template
	}
	return ""
}

// RollbackRequest represents a request to rollback a deployment
type RollbackRequest struct {
	PlanID       string                 `json:"plan_id"`
	ControlID    string                 `json:"control_id,omitempty"`
	Reason       string                 `json:"reason"`
	Strategy     RollbackStrategy       `json:"strategy"`
	Force        bool                   `json:"force,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// RollbackStrategy defines the rollback strategy
type RollbackStrategy string

const (
	RollbackStrategyImmediate RollbackStrategy = "immediate"
	RollbackStrategyGradual   RollbackStrategy = "gradual"
	RollbackStrategyCanary    RollbackStrategy = "canary"
)
