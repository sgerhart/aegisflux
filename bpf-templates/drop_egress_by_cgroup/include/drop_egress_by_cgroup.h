/* SPDX-License-Identifier: GPL-2.0 */
/* Copyright (c) 2024 AegisFlux */

#ifndef DROP_EGRESS_BY_CGROUP_H
#define DROP_EGRESS_BY_CGROUP_H

#include <linux/types.h>
#include <stdint.h>

/* Maximum number of concurrent drop rules */
#define MAX_DROP_RULES 1024

/* Maximum rule ID */
#define MAX_RULE_ID 0xFFFFFFFF

/* Default TTL for rules (in seconds) */
#define DEFAULT_RULE_TTL 3600

/* Configuration structure for the drop rule */
struct drop_config {
    __u32 dst_ip;       /* Destination IP address (network byte order) */
    __u16 dst_port;     /* Destination port (host byte order) */
    __u64 cgroup_id;    /* Cgroup ID to match */
    __u32 ttl;          /* Time-to-live in seconds */
    __u64 created_at;   /* Timestamp when rule was created */
} __attribute__((packed));

/* Statistics structure */
struct drop_stats {
    __u64 packets_dropped;   /* Number of packets dropped */
    __u64 packets_processed; /* Number of packets processed */
    __u64 bytes_dropped;     /* Number of bytes dropped */
} __attribute__((packed));

/* Map names (must match BPF program) */
#define DROP_CONFIGS_MAP_NAME "drop_configs"
#define CGROUP_RULES_MAP_NAME "cgroup_rules"
#define STATS_MAP_NAME "stats"

/* Program sections */
#define XDP_PROG_SEC "xdp"
#define CGROUP_PROG_SEC "cgroup/skb"

/* Return codes */
#define DROP_SUCCESS 0
#define DROP_ERROR_INVALID_PARAMS -1
#define DROP_ERROR_MAP_NOT_FOUND -2
#define DROP_ERROR_RULE_EXISTS -3
#define DROP_ERROR_RULE_NOT_FOUND -4
#define DROP_ERROR_MEMORY_ALLOC -5

/* Helper macros */
#define IP_ADDR(a, b, c, d) ((__u32)((a) << 24) | (b) << 16 | (c) << 8 | (d))
#define PORT(port) ((__u16)(port))

/* Function prototypes for user space management */
#ifdef __cplusplus
extern "C" {
#endif

/**
 * Add a new drop rule for a cgroup
 * @param rule_id: Unique identifier for the rule
 * @param dst_ip: Destination IP address (network byte order)
 * @param dst_port: Destination port (host byte order)
 * @param cgroup_id: Cgroup ID to match
 * @param ttl: Time-to-live in seconds
 * @return: DROP_SUCCESS on success, negative error code on failure
 */
int add_drop_rule(__u32 rule_id, __u32 dst_ip, __u16 dst_port, 
                  __u64 cgroup_id, __u32 ttl);

/**
 * Remove a drop rule
 * @param rule_id: Rule ID to remove
 * @return: DROP_SUCCESS on success, negative error code on failure
 */
int remove_drop_rule(__u32 rule_id);

/**
 * Get statistics for the drop program
 * @param stats: Pointer to stats structure to fill
 * @return: DROP_SUCCESS on success, negative error code on failure
 */
int get_drop_stats(struct drop_stats *stats);

/**
 * List all active drop rules
 * @param rules: Array to store rule information
 * @param max_rules: Maximum number of rules to retrieve
 * @return: Number of rules retrieved, negative error code on failure
 */
int list_drop_rules(struct drop_config *rules, int max_rules);

/**
 * Check if a rule exists and is active
 * @param rule_id: Rule ID to check
 * @return: 1 if rule exists and is active, 0 if not found or expired, negative on error
 */
int is_rule_active(__u32 rule_id);

#ifdef __cplusplus
}
#endif

#endif /* DROP_EGRESS_BY_CGROUP_H */
