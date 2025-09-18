// SPDX-License-Identifier: GPL-2.0
/* Copyright (c) 2024 AegisFlux */

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// Configuration structure for the drop rule
struct drop_config {
    __u32 dst_ip;       // Destination IP address (network byte order)
    __u16 dst_port;     // Destination port (host byte order)
    __u64 cgroup_id;    // Cgroup ID to match
    __u32 ttl;          // Time-to-live in seconds
    __u64 created_at;   // Timestamp when rule was created
};

// Map to store drop configurations
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);  // Rule ID
    __type(value, struct drop_config);
} drop_configs SEC(".maps");

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
        __u64 packets_dropped;
        __u64 packets_processed;
        __u64 bytes_dropped;
    });
} stats SEC(".maps");

// Helper function to get current time in nanoseconds
static __always_inline __u64 get_current_time_ns(void)
{
    return bpf_ktime_get_ns();
}

// Helper function to check if a rule has expired
static __always_inline bool is_rule_expired(struct drop_config *config)
{
    __u64 current_time = get_current_time_ns();
    __u64 ttl_ns = (__u64)config->ttl * 1000000000ULL; // Convert seconds to nanoseconds
    
    return (current_time - config->created_at) > ttl_ns;
}

// Main XDP program for egress traffic filtering
SEC("xdp")
int drop_egress_by_cgroup(struct xdp_md *ctx)
{
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;
    
    // Get statistics counter
    __u32 stats_key = 0;
    struct {
        __u64 packets_dropped;
        __u64 packets_processed;
        __u64 bytes_dropped;
    } *stats_val = bpf_map_lookup_elem(&stats, &stats_key);
    if (!stats_val) {
        return XDP_PASS;
    }
    
    // Increment processed packets
    __sync_fetch_and_add(&stats_val->packets_processed, 1);
    
    // Parse Ethernet header
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) {
        return XDP_PASS;
    }
    
    // Only process IP packets
    if (eth->h_proto != bpf_htons(ETH_P_IP)) {
        return XDP_PASS;
    }
    
    // Parse IP header
    struct iphdr *ip = (struct iphdr *)(eth + 1);
    if ((void *)(ip + 1) > data_end) {
        return XDP_PASS;
    }
    
    // Get cgroup ID from the context
    __u64 cgroup_id = bpf_get_current_cgroup_id();
    
    // Look up rule for this cgroup
    __u32 *rule_id = bpf_map_lookup_elem(&cgroup_rules, &cgroup_id);
    if (!rule_id) {
        return XDP_PASS;
    }
    
    // Get drop configuration for this rule
    struct drop_config *config = bpf_map_lookup_elem(&drop_configs, rule_id);
    if (!config) {
        return XDP_PASS;
    }
    
    // Check if rule has expired
    if (is_rule_expired(config)) {
        // Remove expired rule
        bpf_map_delete_elem(&drop_configs, rule_id);
        bpf_map_delete_elem(&cgroup_rules, &cgroup_id);
        return XDP_PASS;
    }
    
    // Check if destination IP matches
    if (ip->daddr != config->dst_ip) {
        return XDP_PASS;
    }
    
    // Check transport layer protocol and port
    if (ip->protocol == IPPROTO_TCP || ip->protocol == IPPROTO_UDP) {
        void *transport = (void *)(ip + 1);
        if ((void *)(transport + 1) > data_end) {
            return XDP_PASS;
        }
        
        __u16 dst_port = 0;
        if (ip->protocol == IPPROTO_TCP) {
            struct tcphdr *tcp = (struct tcphdr *)transport;
            if ((void *)(tcp + 1) > data_end) {
                return XDP_PASS;
            }
            dst_port = bpf_ntohs(tcp->dest);
        } else if (ip->protocol == IPPROTO_UDP) {
            struct udphdr *udp = (struct udphdr *)transport;
            if ((void *)(udp + 1) > data_end) {
                return XDP_PASS;
            }
            dst_port = bpf_ntohs(udp->dest);
        }
        
        // Check if destination port matches
        if (dst_port != config->dst_port) {
            return XDP_PASS;
        }
    }
    
    // All conditions match - drop the packet
    __sync_fetch_and_add(&stats_val->packets_dropped, 1);
    __sync_fetch_and_add(&stats_val->bytes_dropped, ctx->data_end - ctx->data);
    
    bpf_printk("Dropped egress packet: cgroup=%llu, dst_ip=%x, dst_port=%d", 
               cgroup_id, bpf_ntohl(config->dst_ip), config->dst_port);
    
    return XDP_DROP;
}

// Program to add a new drop rule
SEC("cgroup/skb")
int add_drop_rule(struct __sk_buff *skb)
{
    // This program is used by user space to manage rules
    // The actual rule management is done via map operations from user space
    return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
