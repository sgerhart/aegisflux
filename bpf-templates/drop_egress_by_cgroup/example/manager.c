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
#include "../include/drop_egress_by_cgroup.h"

/* Example management program for the drop_egress_by_cgroup BPF template */

static int map_fd_drop_configs = -1;
static int map_fd_cgroup_rules = -1;
static int map_fd_stats = -1;

static int open_maps(void)
{
    /* Open BPF maps */
    map_fd_drop_configs = bpf_obj_get("/sys/fs/bpf/drop_configs");
    if (map_fd_drop_configs < 0) {
        fprintf(stderr, "Failed to open drop_configs map: %s\n", strerror(errno));
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
    if (map_fd_drop_configs >= 0) close(map_fd_drop_configs);
    if (map_fd_cgroup_rules >= 0) close(map_fd_cgroup_rules);
    if (map_fd_stats >= 0) close(map_fd_stats);
}

int add_drop_rule(__u32 rule_id, __u32 dst_ip, __u16 dst_port, 
                  __u64 cgroup_id, __u32 ttl)
{
    struct drop_config config = {
        .dst_ip = dst_ip,
        .dst_port = dst_port,
        .cgroup_id = cgroup_id,
        .ttl = ttl,
        .created_at = bpf_ktime_get_ns()
    };

    /* Add rule to drop_configs map */
    if (bpf_map_update_elem(map_fd_drop_configs, &rule_id, &config, BPF_ANY) < 0) {
        fprintf(stderr, "Failed to add rule to drop_configs: %s\n", strerror(errno));
        return DROP_ERROR_MAP_NOT_FOUND;
    }

    /* Map cgroup to rule */
    if (bpf_map_update_elem(map_fd_cgroup_rules, &cgroup_id, &rule_id, BPF_ANY) < 0) {
        fprintf(stderr, "Failed to map cgroup to rule: %s\n", strerror(errno));
        /* Clean up the rule from drop_configs */
        bpf_map_delete_elem(map_fd_drop_configs, &rule_id);
        return DROP_ERROR_MAP_NOT_FOUND;
    }

    printf("Added drop rule: ID=%u, cgroup=%llu, dst=%d.%d.%d.%d:%u, ttl=%us\n",
           rule_id, cgroup_id,
           (dst_ip >> 24) & 0xFF, (dst_ip >> 16) & 0xFF,
           (dst_ip >> 8) & 0xFF, dst_ip & 0xFF,
           dst_port, ttl);

    return DROP_SUCCESS;
}

int remove_drop_rule(__u32 rule_id)
{
    struct drop_config config;

    /* Get rule to find cgroup_id */
    if (bpf_map_lookup_elem(map_fd_drop_configs, &rule_id, &config) < 0) {
        fprintf(stderr, "Rule %u not found\n", rule_id);
        return DROP_ERROR_RULE_NOT_FOUND;
    }

    /* Remove cgroup mapping */
    bpf_map_delete_elem(map_fd_cgroup_rules, &config.cgroup_id);

    /* Remove rule */
    if (bpf_map_delete_elem(map_fd_drop_configs, &rule_id) < 0) {
        fprintf(stderr, "Failed to remove rule %u: %s\n", rule_id, strerror(errno));
        return DROP_ERROR_RULE_NOT_FOUND;
    }

    printf("Removed drop rule: ID=%u\n", rule_id);
    return DROP_SUCCESS;
}

int get_drop_stats(struct drop_stats *stats)
{
    __u32 key = 0;

    if (bpf_map_lookup_elem(map_fd_stats, &key, stats) < 0) {
        fprintf(stderr, "Failed to get stats: %s\n", strerror(errno));
        return DROP_ERROR_MAP_NOT_FOUND;
    }

    return DROP_SUCCESS;
}

int list_drop_rules(struct drop_config *rules, int max_rules)
{
    __u32 rule_id = 0;
    struct drop_config config;
    int count = 0;

    /* Iterate through all rules */
    while (bpf_map_get_next_key(map_fd_drop_configs, &rule_id, &rule_id) == 0) {
        if (count >= max_rules) break;

        if (bpf_map_lookup_elem(map_fd_drop_configs, &rule_id, &config) == 0) {
            rules[count] = config;
            count++;
        }
    }

    return count;
}

int is_rule_active(__u32 rule_id)
{
    struct drop_config config;

    if (bpf_map_lookup_elem(map_fd_drop_configs, &rule_id, &config) < 0) {
        return 0; /* Rule not found */
    }

    /* Check if rule has expired */
    __u64 current_time = bpf_ktime_get_ns();
    __u64 ttl_ns = (__u64)config.ttl * 1000000000ULL;

    if ((current_time - config.created_at) > ttl_ns) {
        /* Rule expired, remove it */
        remove_drop_rule(rule_id);
        return 0;
    }

    return 1; /* Rule is active */
}

void print_usage(const char *prog_name)
{
    printf("Usage: %s <command> [args...]\n", prog_name);
    printf("\nCommands:\n");
    printf("  add <rule_id> <dst_ip> <dst_port> <cgroup_id> <ttl>\n");
    printf("  remove <rule_id>\n");
    printf("  stats\n");
    printf("  list\n");
    printf("  check <rule_id>\n");
    printf("\nExamples:\n");
    printf("  %s add 1 8.8.8.8 53 12345 3600\n", prog_name);
    printf("  %s remove 1\n", prog_name);
    printf("  %s stats\n", prog_name);
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
        if (argc != 7) {
            fprintf(stderr, "Usage: %s add <rule_id> <dst_ip> <dst_port> <cgroup_id> <ttl>\n", argv[0]);
            return 1;
        }

        __u32 rule_id = atoi(argv[2]);
        __u32 dst_ip = inet_addr(argv[3]);
        __u16 dst_port = htons(atoi(argv[4]));
        __u64 cgroup_id = atoll(argv[5]);
        __u32 ttl = atoi(argv[6]);

        if (add_drop_rule(rule_id, dst_ip, dst_port, cgroup_id, ttl) != DROP_SUCCESS) {
            return 1;
        }

    } else if (strcmp(argv[1], "remove") == 0) {
        if (argc != 3) {
            fprintf(stderr, "Usage: %s remove <rule_id>\n", argv[0]);
            return 1;
        }

        __u32 rule_id = atoi(argv[2]);
        if (remove_drop_rule(rule_id) != DROP_SUCCESS) {
            return 1;
        }

    } else if (strcmp(argv[1], "stats") == 0) {
        struct drop_stats stats;
        if (get_drop_stats(&stats) != DROP_SUCCESS) {
            return 1;
        }

        printf("Statistics:\n");
        printf("  Packets processed: %llu\n", stats.packets_processed);
        printf("  Packets dropped: %llu\n", stats.packets_dropped);
        printf("  Bytes dropped: %llu\n", stats.bytes_dropped);

    } else if (strcmp(argv[1], "list") == 0) {
        struct drop_config rules[MAX_DROP_RULES];
        int count = list_drop_rules(rules, MAX_DROP_RULES);

        printf("Active rules (%d):\n", count);
        for (int i = 0; i < count; i++) {
            printf("  Rule %u: cgroup=%llu, dst=%d.%d.%d.%d:%u, ttl=%us\n",
                   i, rules[i].cgroup_id,
                   (rules[i].dst_ip >> 24) & 0xFF, (rules[i].dst_ip >> 16) & 0xFF,
                   (rules[i].dst_ip >> 8) & 0xFF, rules[i].dst_ip & 0xFF,
                   rules[i].dst_port, rules[i].ttl);
        }

    } else if (strcmp(argv[1], "check") == 0) {
        if (argc != 3) {
            fprintf(stderr, "Usage: %s check <rule_id>\n", argv[0]);
            return 1;
        }

        __u32 rule_id = atoi(argv[2]);
        int active = is_rule_active(rule_id);
        printf("Rule %u is %s\n", rule_id, active ? "active" : "inactive/expired");

    } else {
        fprintf(stderr, "Unknown command: %s\n", argv[1]);
        print_usage(argv[0]);
        return 1;
    }

    close_maps();
    return 0;
}
