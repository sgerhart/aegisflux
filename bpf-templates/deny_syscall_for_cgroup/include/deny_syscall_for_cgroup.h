/* SPDX-License-Identifier: GPL-2.0 */
/* Copyright (c) 2024 AegisFlux */

#ifndef DENY_SYSCALL_FOR_CGROUP_H
#define DENY_SYSCALL_FOR_CGROUP_H

#include <linux/types.h>
#include <stdint.h>

/* Maximum number of concurrent deny rules */
#define MAX_DENY_RULES 1024

/* Maximum rule ID */
#define MAX_RULE_ID 0xFFFFFFFF

/* Default TTL for rules (in seconds) */
#define DEFAULT_RULE_TTL 3600

/* Syscall numbers (x86_64) */
#define SYSCALL_EXECVE 59
#define SYSCALL_EXECVEAT 322
#define SYSCALL_PTRACE 101

/* Syscall names */
#define SYSCALL_NAME_EXECVE "execve"
#define SYSCALL_NAME_EXECVEAT "execveat"
#define SYSCALL_NAME_PTRACE "ptrace"

/* Configuration structure for the deny rule */
struct deny_config {
    __u64 cgroup_id;        /* Cgroup ID to match */
    __u32 syscall;          /* Syscall number to deny */
    __u32 ttl;              /* Time-to-live in seconds */
    __u64 created_at;       /* Timestamp when rule was created */
    char syscall_name[16];  /* Human-readable syscall name */
} __attribute__((packed));

/* Statistics structure */
struct deny_stats {
    __u64 syscalls_blocked;  /* Number of syscalls blocked */
    __u64 syscalls_processed; /* Number of syscalls processed */
    __u64 execve_blocked;    /* Number of execve calls blocked */
    __u64 ptrace_blocked;    /* Number of ptrace calls blocked */
} __attribute__((packed));

/* Map names (must match BPF program) */
#define DENY_CONFIGS_MAP_NAME "deny_configs"
#define CGROUP_RULES_MAP_NAME "cgroup_rules"
#define STATS_MAP_NAME "stats"

/* Program sections */
#define LSM_PROG_SEC "lsm/bprm_check_security"
#define KPROBE_PTRACE_SEC "kprobe/__x64_sys_ptrace"
#define KPROBE_EXECVE_SEC "kprobe/__x64_sys_execve"

/* Return codes */
#define DENY_SUCCESS 0
#define DENY_ERROR_INVALID_PARAMS -1
#define DENY_ERROR_MAP_NOT_FOUND -2
#define DENY_ERROR_RULE_EXISTS -3
#define DENY_ERROR_RULE_NOT_FOUND -4
#define DENY_ERROR_MEMORY_ALLOC -5
#define DENY_ERROR_INVALID_SYSCALL -6

/* Helper macros */
#define SYSCALL_NAME_TO_NUM(name) \
    ((strcmp(name, SYSCALL_NAME_EXECVE) == 0) ? SYSCALL_EXECVE : \
     (strcmp(name, SYSCALL_NAME_EXECVEAT) == 0) ? SYSCALL_EXECVEAT : \
     (strcmp(name, SYSCALL_NAME_PTRACE) == 0) ? SYSCALL_PTRACE : -1)

#define SYSCALL_NUM_TO_NAME(num) \
    ((num) == SYSCALL_EXECVE ? SYSCALL_NAME_EXECVE : \
     (num) == SYSCALL_EXECVEAT ? SYSCALL_NAME_EXECVEAT : \
     (num) == SYSCALL_PTRACE ? SYSCALL_NAME_PTRACE : "unknown")

/* Function prototypes for user space management */
#ifdef __cplusplus
extern "C" {
#endif

/**
 * Add a new deny rule for a cgroup
 * @param rule_id: Unique identifier for the rule
 * @param cgroup_id: Cgroup ID to match
 * @param syscall: Syscall number to deny (or -1 for syscall name)
 * @param syscall_name: Syscall name (if syscall is -1)
 * @param ttl: Time-to-live in seconds
 * @return: DENY_SUCCESS on success, negative error code on failure
 */
int add_deny_rule(__u32 rule_id, __u64 cgroup_id, __u32 syscall, 
                  const char *syscall_name, __u32 ttl);

/**
 * Add a new deny rule using syscall name
 * @param rule_id: Unique identifier for the rule
 * @param cgroup_id: Cgroup ID to match
 * @param syscall_name: Name of syscall to deny ("execve", "execveat", "ptrace")
 * @param ttl: Time-to-live in seconds
 * @return: DENY_SUCCESS on success, negative error code on failure
 */
int add_deny_rule_by_name(__u32 rule_id, __u64 cgroup_id, 
                          const char *syscall_name, __u32 ttl);

/**
 * Remove a deny rule
 * @param rule_id: Rule ID to remove
 * @return: DENY_SUCCESS on success, negative error code on failure
 */
int remove_deny_rule(__u32 rule_id);

/**
 * Get statistics for the deny program
 * @param stats: Pointer to stats structure to fill
 * @return: DENY_SUCCESS on success, negative error code on failure
 */
int get_deny_stats(struct deny_stats *stats);

/**
 * List all active deny rules
 * @param rules: Array to store rule information
 * @param max_rules: Maximum number of rules to retrieve
 * @return: Number of rules retrieved, negative error code on failure
 */
int list_deny_rules(struct deny_config *rules, int max_rules);

/**
 * Check if a rule exists and is active
 * @param rule_id: Rule ID to check
 * @return: 1 if rule exists and is active, 0 if not found or expired, negative on error
 */
int is_rule_active(__u32 rule_id);

/**
 * Get cgroup ID for a given PID
 * @param pid: Process ID
 * @return: Cgroup ID on success, 0 on failure
 */
__u64 get_cgroup_id_for_pid(pid_t pid);

/**
 * Get cgroup ID for current process
 * @return: Cgroup ID on success, 0 on failure
 */
__u64 get_current_cgroup_id(void);

/**
 * Validate syscall name
 * @param syscall_name: Name to validate
 * @return: 1 if valid, 0 if invalid
 */
int is_valid_syscall_name(const char *syscall_name);

#ifdef __cplusplus
}
#endif

#endif /* DENY_SYSCALL_FOR_CGROUP_H */
