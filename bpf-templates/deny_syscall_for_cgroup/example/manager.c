/* SPDX-License-Identifier: GPL-2.0 */
/* Copyright (c) 2024 AegisFlux */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/resource.h>
#include <bpf/bpf.h>
#include <bpf/libbpf.h>
#include "../include/deny_syscall_for_cgroup.h"

/* Example management program for the deny_syscall_for_cgroup BPF template */

static int map_fd_deny_configs = -1;
static int map_fd_cgroup_rules = -1;
static int map_fd_stats = -1;

static int open_maps(void)
{
    /* Open BPF maps */
    map_fd_deny_configs = bpf_obj_get("/sys/fs/bpf/deny_configs");
    if (map_fd_deny_configs < 0) {
        fprintf(stderr, "Failed to open deny_configs map: %s\n", strerror(errno));
        return -1;
    }

    map_fd_cgroup_rules = bpf_obj_get("/sys/fs/bpf/cgroup_rules");
    if (map_fd_cgroup_rules < 0) {
        fprintf(stderr, "Failed to open cgroup_rules map: %s\n", strerror(errno));
        return -1;
    }

    map_fd_stats = bpf_obj_get("/sys/fs/bpf/stats");
    if (map_fd_stats < 0) {
        fprintf(stderr, "Failed to open stats map: %s\n", strerror(errno));
        return -1;
    }

    return 0;
}

static void close_maps(void)
{
    if (map_fd_deny_configs >= 0) close(map_fd_deny_configs);
    if (map_fd_cgroup_rules >= 0) close(map_fd_cgroup_rules);
    if (map_fd_stats >= 0) close(map_fd_stats);
}

int add_deny_rule(__u32 rule_id, __u64 cgroup_id, __u32 syscall, 
                  const char *syscall_name, __u32 ttl)
{
    struct deny_config config = {
        .cgroup_id = cgroup_id,
        .syscall = syscall,
        .ttl = ttl,
        .created_at = bpf_ktime_get_ns()
    };

    /* Set syscall name */
    if (syscall_name) {
        strncpy(config.syscall_name, syscall_name, sizeof(config.syscall_name) - 1);
        config.syscall_name[sizeof(config.syscall_name) - 1] = '\0';
    } else {
        strncpy(config.syscall_name, SYSCALL_NUM_TO_NAME(syscall), sizeof(config.syscall_name) - 1);
        config.syscall_name[sizeof(config.syscall_name) - 1] = '\0';
    }

    /* Add rule to deny_configs map */
    if (bpf_map_update_elem(map_fd_deny_configs, &rule_id, &config, BPF_ANY) < 0) {
        fprintf(stderr, "Failed to add rule to deny_configs: %s\n", strerror(errno));
        return DENY_ERROR_MAP_NOT_FOUND;
    }

    /* Map cgroup to rule */
    if (bpf_map_update_elem(map_fd_cgroup_rules, &cgroup_id, &rule_id, BPF_ANY) < 0) {
        fprintf(stderr, "Failed to map cgroup to rule: %s\n", strerror(errno));
        /* Clean up the rule from deny_configs */
        bpf_map_delete_elem(map_fd_deny_configs, &rule_id);
        return DENY_ERROR_MAP_NOT_FOUND;
    }

    printf("Added deny rule: ID=%u, cgroup=%llu, syscall=%s (%u), ttl=%us\n",
           rule_id, cgroup_id, config.syscall_name, syscall, ttl);

    return DENY_SUCCESS;
}

int add_deny_rule_by_name(__u32 rule_id, __u64 cgroup_id, 
                          const char *syscall_name, __u32 ttl)
{
    __u32 syscall_num = SYSCALL_NAME_TO_NUM(syscall_name);
    if (syscall_num == -1) {
        fprintf(stderr, "Invalid syscall name: %s\n", syscall_name);
        return DENY_ERROR_INVALID_SYSCALL;
    }

    return add_deny_rule(rule_id, cgroup_id, syscall_num, syscall_name, ttl);
}

int remove_deny_rule(__u32 rule_id)
{
    struct deny_config config;

    /* Get rule to find cgroup_id */
    if (bpf_map_lookup_elem(map_fd_deny_configs, &rule_id, &config) < 0) {
        fprintf(stderr, "Rule %u not found\n", rule_id);
        return DENY_ERROR_RULE_NOT_FOUND;
    }

    /* Remove cgroup mapping */
    bpf_map_delete_elem(map_fd_cgroup_rules, &config.cgroup_id);

    /* Remove rule */
    if (bpf_map_delete_elem(map_fd_deny_configs, &rule_id) < 0) {
        fprintf(stderr, "Failed to remove rule %u: %s\n", rule_id, strerror(errno));
        return DENY_ERROR_RULE_NOT_FOUND;
    }

    printf("Removed deny rule: ID=%u\n", rule_id);
    return DENY_SUCCESS;
}

int get_deny_stats(struct deny_stats *stats)
{
    __u32 key = 0;

    if (bpf_map_lookup_elem(map_fd_stats, &key, stats) < 0) {
        fprintf(stderr, "Failed to get stats: %s\n", strerror(errno));
        return DENY_ERROR_MAP_NOT_FOUND;
    }

    return DENY_SUCCESS;
}

int list_deny_rules(struct deny_config *rules, int max_rules)
{
    __u32 rule_id = 0;
    struct deny_config config;
    int count = 0;

    /* Iterate through all rules */
    while (bpf_map_get_next_key(map_fd_deny_configs, &rule_id, &rule_id) == 0) {
        if (count >= max_rules) break;

        if (bpf_map_lookup_elem(map_fd_deny_configs, &rule_id, &config) == 0) {
            rules[count] = config;
            count++;
        }
    }

    return count;
}

int is_rule_active(__u32 rule_id)
{
    struct deny_config config;

    if (bpf_map_lookup_elem(map_fd_deny_configs, &rule_id, &config) < 0) {
        return 0; /* Rule not found */
    }

    /* Check if rule has expired */
    __u64 current_time = bpf_ktime_get_ns();
    __u64 ttl_ns = (__u64)config.ttl * 1000000000ULL;

    if ((current_time - config.created_at) > ttl_ns) {
        /* Rule expired, remove it */
        remove_deny_rule(rule_id);
        return 0;
    }

    return 1; /* Rule is active */
}

__u64 get_cgroup_id_for_pid(pid_t pid)
{
    char path[256];
    char cgroup_path[512];
    FILE *f;
    __u64 cgroup_id = 0;

    /* Read cgroup path from /proc/<pid>/cgroup */
    snprintf(path, sizeof(path), "/proc/%d/cgroup", pid);
    f = fopen(path, "r");
    if (!f) {
        fprintf(stderr, "Failed to open %s: %s\n", path, strerror(errno));
        return 0;
    }

    /* Look for unified cgroup (cgroup v2) */
    while (fgets(cgroup_path, sizeof(cgroup_path), f)) {
        if (strstr(cgroup_path, ":") == NULL) {
            /* This is a cgroup v2 path */
            char *line = strtok(cgroup_path, "\n");
            if (line && line[0] == '/') {
                /* Convert cgroup path to ID (simplified) */
                cgroup_id = bpf_get_prandom_u32(); /* Placeholder - real implementation would hash the path */
                break;
            }
        }
    }

    fclose(f);
    return cgroup_id;
}

__u64 get_current_cgroup_id(void)
{
    return get_cgroup_id_for_pid(getpid());
}

int is_valid_syscall_name(const char *syscall_name)
{
    return (strcmp(syscall_name, SYSCALL_NAME_EXECVE) == 0 ||
            strcmp(syscall_name, SYSCALL_NAME_EXECVEAT) == 0 ||
            strcmp(syscall_name, SYSCALL_NAME_PTRACE) == 0);
}

void print_usage(const char *prog_name)
{
    printf("Usage: %s <command> [args...]\n", prog_name);
    printf("\nCommands:\n");
    printf("  add <rule_id> <cgroup_id> <syscall_name> <ttl>\n");
    printf("  remove <rule_id>\n");
    printf("  stats\n");
    printf("  list\n");
    printf("  check <rule_id>\n");
    printf("  get-cgroup <pid>\n");
    printf("\nSyscall names:\n");
    printf("  execve, execveat, ptrace\n");
    printf("\nExamples:\n");
    printf("  %s add 1 12345 execve 3600\n", prog_name);
    printf("  %s add 2 67890 ptrace 1800\n", prog_name);
    printf("  %s remove 1\n", prog_name);
    printf("  %s stats\n", prog_name);
    printf("  %s get-cgroup 1234\n", prog_name);
}

int main(int argc, char *argv[])
{
    if (argc < 2) {
        print_usage(argv[0]);
        return 1;
    }

    if (open_maps() < 0) {
        fprintf(stderr, "Failed to open BPF maps. Make sure the BPF program is loaded.\n");
        return 1;
    }

    if (strcmp(argv[1], "add") == 0) {
        if (argc != 6) {
            fprintf(stderr, "Usage: %s add <rule_id> <cgroup_id> <syscall_name> <ttl>\n", argv[0]);
            return 1;
        }

        __u32 rule_id = atoi(argv[2]);
        __u64 cgroup_id = atoll(argv[3]);
        const char *syscall_name = argv[4];
        __u32 ttl = atoi(argv[5]);

        if (!is_valid_syscall_name(syscall_name)) {
            fprintf(stderr, "Invalid syscall name: %s\n", syscall_name);
            fprintf(stderr, "Valid syscalls: execve, execveat, ptrace\n");
            return 1;
        }

        if (add_deny_rule_by_name(rule_id, cgroup_id, syscall_name, ttl) != DENY_SUCCESS) {
            return 1;
        }

    } else if (strcmp(argv[1], "remove") == 0) {
        if (argc != 3) {
            fprintf(stderr, "Usage: %s remove <rule_id>\n", argv[0]);
            return 1;
        }

        __u32 rule_id = atoi(argv[2]);
        if (remove_deny_rule(rule_id) != DENY_SUCCESS) {
            return 1;
        }

    } else if (strcmp(argv[1], "stats") == 0) {
        struct deny_stats stats;
        if (get_deny_stats(&stats) != DENY_SUCCESS) {
            return 1;
        }

        printf("Statistics:\n");
        printf("  Syscalls processed: %llu\n", stats.syscalls_processed);
        printf("  Syscalls blocked: %llu\n", stats.syscalls_blocked);
        printf("  Execve blocked: %llu\n", stats.execve_blocked);
        printf("  Ptrace blocked: %llu\n", stats.ptrace_blocked);

    } else if (strcmp(argv[1], "list") == 0) {
        struct deny_config rules[MAX_DENY_RULES];
        int count = list_deny_rules(rules, MAX_DENY_RULES);

        printf("Active rules (%d):\n", count);
        for (int i = 0; i < count; i++) {
            printf("  Rule %u: cgroup=%llu, syscall=%s (%u), ttl=%us\n",
                   i, rules[i].cgroup_id, rules[i].syscall_name, 
                   rules[i].syscall, rules[i].ttl);
        }

    } else if (strcmp(argv[1], "check") == 0) {
        if (argc != 3) {
            fprintf(stderr, "Usage: %s check <rule_id>\n", argv[0]);
            return 1;
        }

        __u32 rule_id = atoi(argv[2]);
        int active = is_rule_active(rule_id);
        printf("Rule %u is %s\n", rule_id, active ? "active" : "inactive/expired");

    } else if (strcmp(argv[1], "get-cgroup") == 0) {
        if (argc != 3) {
            fprintf(stderr, "Usage: %s get-cgroup <pid>\n", argv[0]);
            return 1;
        }

        pid_t pid = atoi(argv[2]);
        __u64 cgroup_id = get_cgroup_id_for_pid(pid);
        printf("Cgroup ID for PID %d: %llu\n", pid, cgroup_id);

    } else {
        fprintf(stderr, "Unknown command: %s\n", argv[1]);
        print_usage(argv[0]);
        return 1;
    }

    close_maps();
    return 0;
}
