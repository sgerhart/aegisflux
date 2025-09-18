// SPDX-License-Identifier: GPL-2.0
/* Copyright (c) 2024 AegisFlux */

#include <linux/bpf.h>
#include <linux/ptrace.h>
#include <linux/sched.h>
#include <linux/cgroup.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Syscall numbers (from arch/x86/include/generated/uapi/asm/unistd_64.h)
#define __NR_execve 59
#define __NR_execveat 322
#define __NR_ptrace 101

// Configuration structure for the deny rule
struct deny_config {
    __u64 cgroup_id;    // Cgroup ID to match
    __u32 syscall;      // Syscall number to deny
    __u32 ttl;          // Time-to-live in seconds
    __u64 created_at;   // Timestamp when rule was created
    char syscall_name[16]; // Human-readable syscall name
};

// Map to store deny configurations
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);  // Rule ID
    __type(value, struct deny_config);
} deny_configs SEC(".maps");

// Map to store cgroup information
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u64);  // Cgroup ID
    __type(value, __u32); // Rule ID
} cgroup_rules SEC(".maps");

// Per-CPU map for statistics
struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, struct {
        __u64 syscalls_blocked;
        __u64 syscalls_processed;
        __u64 execve_blocked;
        __u64 ptrace_blocked;
    });
} stats SEC(".maps");

// Helper function to get current time in nanoseconds
static __always_inline __u64 get_current_time_ns(void)
{
    return bpf_ktime_get_ns();
}

// Helper function to check if a rule has expired
static __always_inline bool is_rule_expired(struct deny_config *config)
{
    __u64 current_time = get_current_time_ns();
    __u64 ttl_ns = (__u64)config->ttl * 1000000000ULL; // Convert seconds to nanoseconds
    
    return (current_time - config->created_at) > ttl_ns;
}

// Helper function to get cgroup ID from task
static __always_inline __u64 get_task_cgroup_id(struct task_struct *task)
{
    struct css_set *cgroups;
    struct cgroup *cgrp;
    __u64 cgroup_id = 0;

    if (!task)
        return 0;

    // Get the css_set from the task
    cgroups = READ_KERNEL(task->cgroups);
    if (!cgroups)
        return 0;

    // Get the unified cgroup (cgroup v2)
    cgrp = READ_KERNEL(cgroups->dfl_cgrp);
    if (!cgrp)
        return 0;

    // Get cgroup ID
    cgroup_id = READ_KERNEL(cgrp->kn->id);
    
    return cgroup_id;
}

// LSM hook for execve syscall
SEC("lsm/bprm_check_security")
int deny_execve_for_cgroup(struct linux_binprm *bprm)
{
    struct task_struct *current_task = bpf_get_current_task_btf();
    __u64 cgroup_id = get_task_cgroup_id(current_task);
    
    if (cgroup_id == 0) {
        return 0; // Allow if we can't determine cgroup
    }

    // Get statistics counter
    __u32 stats_key = 0;
    struct {
        __u64 syscalls_blocked;
        __u64 syscalls_processed;
        __u64 execve_blocked;
        __u64 ptrace_blocked;
    } *stats_val = bpf_map_lookup_elem(&stats, &stats_key);
    if (!stats_val) {
        return 0;
    }

    // Increment processed syscalls
    __sync_fetch_and_add(&stats_val->syscalls_processed, 1);

    // Look up rule for this cgroup
    __u32 *rule_id = bpf_map_lookup_elem(&cgroup_rules, &cgroup_id);
    if (!rule_id) {
        return 0; // No rule for this cgroup, allow
    }

    // Get deny configuration for this rule
    struct deny_config *config = bpf_map_lookup_elem(&deny_configs, rule_id);
    if (!config) {
        return 0; // No config found, allow
    }

    // Check if rule has expired
    if (is_rule_expired(config)) {
        // Remove expired rule
        bpf_map_delete_elem(&deny_configs, rule_id);
        bpf_map_delete_elem(&cgroup_rules, &cgroup_id);
        return 0;
    }

    // Check if this rule applies to execve
    if (config->syscall == __NR_execve || config->syscall == __NR_execveat) {
        // Block the execve
        __sync_fetch_and_add(&stats_val->syscalls_blocked, 1);
        __sync_fetch_and_add(&stats_val->execve_blocked, 1);
        
        bpf_printk("Blocked execve for cgroup %llu (rule %u)", cgroup_id, *rule_id);
        return -EPERM;
    }

    return 0; // Allow other syscalls
}

// Kprobe for ptrace syscall
SEC("kprobe/__x64_sys_ptrace")
int deny_ptrace_for_cgroup(struct pt_regs *ctx)
{
    struct task_struct *current_task = bpf_get_current_task_btf();
    __u64 cgroup_id = get_task_cgroup_id(current_task);
    
    if (cgroup_id == 0) {
        return 0; // Allow if we can't determine cgroup
    }

    // Get statistics counter
    __u32 stats_key = 0;
    struct {
        __u64 syscalls_blocked;
        __u64 syscalls_processed;
        __u64 execve_blocked;
        __u64 ptrace_blocked;
    } *stats_val = bpf_map_lookup_elem(&stats, &stats_key);
    if (!stats_val) {
        return 0;
    }

    // Increment processed syscalls
    __sync_fetch_and_add(&stats_val->syscalls_processed, 1);

    // Look up rule for this cgroup
    __u32 *rule_id = bpf_map_lookup_elem(&cgroup_rules, &cgroup_id);
    if (!rule_id) {
        return 0; // No rule for this cgroup, allow
    }

    // Get deny configuration for this rule
    struct deny_config *config = bpf_map_lookup_elem(&deny_configs, rule_id);
    if (!config) {
        return 0; // No config found, allow
    }

    // Check if rule has expired
    if (is_rule_expired(config)) {
        // Remove expired rule
        bpf_map_delete_elem(&deny_configs, rule_id);
        bpf_map_delete_elem(&cgroup_rules, &cgroup_id);
        return 0;
    }

    // Check if this rule applies to ptrace
    if (config->syscall == __NR_ptrace) {
        // Block the ptrace
        __sync_fetch_and_add(&stats_val->syscalls_blocked, 1);
        __sync_fetch_and_add(&stats_val->ptrace_blocked, 1);
        
        bpf_printk("Blocked ptrace for cgroup %llu (rule %u)", cgroup_id, *rule_id);
        
        // Modify return value to -EPERM
        bpf_override_return(ctx, -EPERM);
    }

    return 0; // Allow other syscalls or let ptrace proceed if not blocked
}

// Kprobe for execve syscall (alternative to LSM)
SEC("kprobe/__x64_sys_execve")
int deny_execve_kprobe_for_cgroup(struct pt_regs *ctx)
{
    struct task_struct *current_task = bpf_get_current_task_btf();
    __u64 cgroup_id = get_task_cgroup_id(current_task);
    
    if (cgroup_id == 0) {
        return 0; // Allow if we can't determine cgroup
    }

    // Get statistics counter
    __u32 stats_key = 0;
    struct {
        __u64 syscalls_blocked;
        __u64 syscalls_processed;
        __u64 execve_blocked;
        __u64 ptrace_blocked;
    } *stats_val = bpf_map_lookup_elem(&stats, &stats_key);
    if (!stats_val) {
        return 0;
    }

    // Increment processed syscalls
    __sync_fetch_and_add(&stats_val->syscalls_processed, 1);

    // Look up rule for this cgroup
    __u32 *rule_id = bpf_map_lookup_elem(&cgroup_rules, &cgroup_id);
    if (!rule_id) {
        return 0; // No rule for this cgroup, allow
    }

    // Get deny configuration for this rule
    struct deny_config *config = bpf_map_lookup_elem(&deny_configs, rule_id);
    if (!config) {
        return 0; // No config found, allow
    }

    // Check if rule has expired
    if (is_rule_expired(config)) {
        // Remove expired rule
        bpf_map_delete_elem(&deny_configs, rule_id);
        bpf_map_delete_elem(&cgroup_rules, &cgroup_id);
        return 0;
    }

    // Check if this rule applies to execve
    if (config->syscall == __NR_execve) {
        // Block the execve
        __sync_fetch_and_add(&stats_val->syscalls_blocked, 1);
        __sync_fetch_and_add(&stats_val->execve_blocked, 1);
        
        bpf_printk("Blocked execve kprobe for cgroup %llu (rule %u)", cgroup_id, *rule_id);
        
        // Modify return value to -EPERM
        bpf_override_return(ctx, -EPERM);
    }

    return 0; // Allow other syscalls or let execve proceed if not blocked
}

char _license[] SEC("license") = "GPL";
