package tools

import (
	"log/slog"
	"math/rand"
	"strings"
	"time"
)

// EBPFConfig contains configuration for the eBPF tool
type EBPFConfig struct {
	TemplateRegistry string
	APIKey           string
	Timeout          time.Duration
	MockMode         bool // For testing without real eBPF service
}

// EBPFTool provides interface to eBPF template suggestion services
type EBPFTool struct {
	config EBPFConfig
	logger *slog.Logger
}

// NewEBPFTool creates a new eBPF tool instance
func NewEBPFTool(config EBPFConfig, logger *slog.Logger) *EBPFTool {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &EBPFTool{
		config: config,
		logger: logger,
	}
}

// EBPFTemplate represents an eBPF template
type EBPFTemplate struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Template    string            `json:"template"`
	Parameters  map[string]any    `json:"parameters"`
	Metadata    map[string]any    `json:"metadata"`
}

// Suggest suggests eBPF templates based on context
func (e *EBPFTool) Suggest(templateHint string, context map[string]any) (map[string]any, error) {
	e.logger.Debug("Suggesting eBPF templates", "hint", templateHint, "context", context)

	if e.config.MockMode {
		return e.mockSuggest(templateHint, context), nil
	}

	// TODO: Implement real eBPF template suggestion API integration
	// This would typically involve:
	// 1. Querying eBPF template registry
	// 2. Analyzing context for relevant templates
	// 3. Ranking templates by relevance
	// 4. Returning template with parameter suggestions
	
	return e.mockSuggest(templateHint, context), nil
}

// mockSuggest provides mock eBPF template suggestions
func (e *EBPFTool) mockSuggest(templateHint string, context map[string]any) map[string]any {
	rand.Seed(time.Now().UnixNano())
	
	// Determine template category based on hint and context
	category := e.determineCategory(templateHint, context)
	
	// Generate template based on category
	template := e.generateTemplate(category, context)
	
	result := map[string]any{
		"template": template,
		"params":   e.generateParameters(template, context),
		"metadata": map[string]any{
			"suggested_at": time.Now().Format("2006-01-02T15:04:05Z"),
			"hint": templateHint,
			"category": category,
			"confidence": 0.7 + rand.Float64()*0.3, // 0.7-1.0 confidence
		},
	}
	
	e.logger.Info("eBPF template suggestion completed", 
		"hint", templateHint, 
		"category", category,
		"template", template["name"])
	
	return result
}

// determineCategory determines the appropriate eBPF template category
func (e *EBPFTool) determineCategory(templateHint string, context map[string]any) string {
	hint := strings.ToLower(templateHint)
	
	// Check context for clues
	eventType, _ := context["event_type"].(string)
	severity, _ := context["severity"].(string)
	
	// Determine category based on hints and context
	if strings.Contains(hint, "network") || strings.Contains(hint, "connect") {
		return "network_monitoring"
	} else if strings.Contains(hint, "exec") || strings.Contains(hint, "process") {
		return "process_monitoring"
	} else if strings.Contains(hint, "file") || strings.Contains(hint, "access") {
		return "file_monitoring"
	} else if strings.Contains(hint, "security") || severity == "critical" || severity == "high" {
		return "security_monitoring"
	} else if strings.Contains(hint, "performance") || strings.Contains(hint, "trace") {
		return "performance_monitoring"
	} else if eventType == "connect" {
		return "network_monitoring"
	} else if eventType == "exec" {
		return "process_monitoring"
	} else {
		// Default to general monitoring
		return "general_monitoring"
	}
}

// generateTemplate generates an eBPF template based on category
func (e *EBPFTool) generateTemplate(category string, context map[string]any) map[string]any {
	templates := map[string]map[string]any{
		"network_monitoring": {
			"name": "network_connection_monitor",
			"description": "Monitors network connections and traffic patterns",
			"category": "network_monitoring",
			"template": `#include <uapi/linux/bpf.h>
#include <linux/net.h>
#include <linux/socket.h>

SEC("kprobe/tcp_connect")
int trace_tcp_connect(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    struct sockaddr_in *addr = (struct sockaddr_in *)PT_REGS_PARM2(ctx);
    
    // Extract connection details
    u32 dst_ip = addr->sin_addr.s_addr;
    u16 dst_port = addr->sin_port;
    
    // Log connection attempt
    bpf_trace_printk("TCP connect to %pI4:%d\\n", &dst_ip, dst_port);
    
    return 0;
}`,
		},
		"process_monitoring": {
			"name": "process_execution_monitor",
			"description": "Monitors process execution and command line arguments",
			"category": "process_monitoring",
			"template": `#include <uapi/linux/bpf.h>
#include <linux/sched.h>

SEC("kprobe/execve")
int trace_execve(struct pt_regs *ctx) {
    char filename[256];
    char argv[512];
    
    // Extract filename and arguments
    bpf_probe_read_user_str(filename, sizeof(filename), (char *)PT_REGS_PARM1(ctx));
    bpf_probe_read_user_str(argv, sizeof(argv), (char **)PT_REGS_PARM2(ctx)[0]);
    
    // Log process execution
    bpf_trace_printk("Process exec: %s %s\\n", filename, argv);
    
    return 0;
}`,
		},
		"file_monitoring": {
			"name": "file_access_monitor",
			"description": "Monitors file access patterns and suspicious activities",
			"category": "file_monitoring",
			"template": `#include <uapi/linux/bpf.h>
#include <linux/fs.h>

SEC("kprobe/vfs_open")
int trace_file_open(struct pt_regs *ctx) {
    struct path *path = (struct path *)PT_REGS_PARM1(ctx);
    struct file *file = (struct file *)PT_REGS_PARM2(ctx);
    char filename[256];
    
    // Extract filename
    bpf_probe_read_str(filename, sizeof(filename), path->dentry->d_name.name);
    
    // Log file access
    bpf_trace_printk("File access: %s\\n", filename);
    
    return 0;
}`,
		},
		"security_monitoring": {
			"name": "security_event_monitor",
			"description": "Monitors security-relevant events and anomalies",
			"category": "security_monitoring",
			"template": `#include <uapi/linux/bpf.h>
#include <linux/security.h>

SEC("kprobe/security_inode_create")
int trace_file_create(struct pt_regs *ctx) {
    struct inode *dir = (struct inode *)PT_REGS_PARM1(ctx);
    struct dentry *dentry = (struct dentry *)PT_REGS_PARM2(ctx);
    char filename[256];
    
    // Extract filename
    bpf_probe_read_str(filename, sizeof(filename), dentry->d_name.name);
    
    // Check for suspicious patterns
    if (strstr(filename, ".exe") || strstr(filename, ".bat")) {
        bpf_trace_printk("Suspicious file creation: %s\\n", filename);
    }
    
    return 0;
}`,
		},
		"performance_monitoring": {
			"name": "performance_profiler",
			"description": "Monitors system performance and resource usage",
			"category": "performance_monitoring",
			"template": `#include <uapi/linux/bpf.h>
#include <linux/sched.h>

SEC("kprobe/finish_task_switch")
int trace_context_switch(struct pt_regs *ctx) {
    struct task_struct *prev = (struct task_struct *)PT_REGS_PARM1(ctx);
    struct task_struct *next = (struct task_struct *)PT_REGS_PARM2(ctx);
    
    // Log context switches for performance analysis
    bpf_trace_printk("Context switch: %d -> %d\\n", 
                     prev->pid, next->pid);
    
    return 0;
}`,
		},
		"general_monitoring": {
			"name": "general_system_monitor",
			"description": "General system monitoring and event tracking",
			"category": "general_monitoring",
			"template": `#include <uapi/linux/bpf.h>
#include <linux/sched.h>

SEC("kprobe/sys_enter")
int trace_syscall_enter(struct pt_regs *ctx) {
    u64 syscall_id = PT_REGS_PARM1(ctx);
    u64 pid = bpf_get_current_pid_tgid() >> 32;
    
    // Log system call entry
    bpf_trace_printk("Syscall %d from PID %d\\n", syscall_id, pid);
    
    return 0;
}`,
		},
	}
	
	template, exists := templates[category]
	if !exists {
		template = templates["general_monitoring"]
	}
	
	// Add metadata
	template["metadata"] = map[string]any{
		"created_at": time.Now().Format("2006-01-02T15:04:05Z"),
		"version": "1.0.0",
		"author": "AegisFlux eBPF Generator",
		"target_kernel": ">= 4.15",
	}
	
	return template
}

// generateParameters generates suggested parameters for the template
func (e *EBPFTool) generateParameters(template map[string]any, context map[string]any) map[string]any {
	category := template["category"].(string)
	
	baseParams := map[string]any{
		"buffer_size": 1024 * 1024, // 1MB buffer
		"sample_rate": 1,           // Sample every event
		"output_format": "json",
		"log_level": "info",
	}
	
	// Add category-specific parameters
	switch category {
	case "network_monitoring":
		baseParams["monitor_ports"] = []int{22, 80, 443, 8080}
		baseParams["filter_protocols"] = []string{"tcp", "udp"}
		baseParams["track_connections"] = true
		
	case "process_monitoring":
		baseParams["monitor_exec"] = true
		baseParams["monitor_fork"] = true
		baseParams["capture_args"] = true
		baseParams["max_arg_length"] = 512
		
	case "file_monitoring":
		baseParams["monitor_reads"] = true
		baseParams["monitor_writes"] = true
		baseParams["monitor_executes"] = true
		baseParams["filter_paths"] = []string{"/etc/", "/home/", "/tmp/"}
		
	case "security_monitoring":
		baseParams["alert_threshold"] = 5
		baseParams["monitor_privileged"] = true
		baseParams["detect_anomalies"] = true
		
	case "performance_monitoring":
		baseParams["profile_frequency"] = 100 // Hz
		baseParams["track_cpu"] = true
		baseParams["track_memory"] = true
		
	default:
		baseParams["monitor_all"] = true
	}
	
	// Add context-specific parameters
	if hostID, ok := context["host_id"].(string); ok {
		baseParams["target_host"] = hostID
	}
	
	if severity, ok := context["severity"].(string); ok {
		switch severity {
		case "critical":
			baseParams["alert_threshold"] = 1
			baseParams["sample_rate"] = 1
		case "high":
			baseParams["alert_threshold"] = 3
			baseParams["sample_rate"] = 1
		case "medium":
			baseParams["alert_threshold"] = 5
			baseParams["sample_rate"] = 2
		case "low":
			baseParams["alert_threshold"] = 10
			baseParams["sample_rate"] = 5
		}
	}
	
	return baseParams
}

// GetAvailableTemplates returns a list of available eBPF templates
func (e *EBPFTool) GetAvailableTemplates() []map[string]any {
	templates := []map[string]any{
		{
			"name": "network_connection_monitor",
			"category": "network_monitoring",
			"description": "Monitors network connections and traffic patterns",
		},
		{
			"name": "process_execution_monitor",
			"category": "process_monitoring",
			"description": "Monitors process execution and command line arguments",
		},
		{
			"name": "file_access_monitor",
			"category": "file_monitoring",
			"description": "Monitors file access patterns and suspicious activities",
		},
		{
			"name": "security_event_monitor",
			"category": "security_monitoring",
			"description": "Monitors security-relevant events and anomalies",
		},
		{
			"name": "performance_profiler",
			"category": "performance_monitoring",
			"description": "Monitors system performance and resource usage",
		},
		{
			"name": "general_system_monitor",
			"category": "general_monitoring",
			"description": "General system monitoring and event tracking",
		},
	}
	
	return templates
}

// ValidateTemplate validates an eBPF template
func (e *EBPFTool) ValidateTemplate(template map[string]any) (bool, []string, error) {
	var errors []string
	
	// Check required fields
	if _, ok := template["name"]; !ok {
		errors = append(errors, "template name is required")
	}
	
	if _, ok := template["template"]; !ok {
		errors = append(errors, "template code is required")
	}
	
	if _, ok := template["category"]; !ok {
		errors = append(errors, "template category is required")
	}
	
	// Validate template code (basic syntax check)
	if templateCode, ok := template["template"].(string); ok {
		if !strings.Contains(templateCode, "#include") {
			errors = append(errors, "template must include required headers")
		}
		if !strings.Contains(templateCode, "SEC(") {
			errors = append(errors, "template must define at least one eBPF section")
		}
	}
	
	return len(errors) == 0, errors, nil
}
