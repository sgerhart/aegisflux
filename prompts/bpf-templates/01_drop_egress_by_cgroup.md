Create template bpf-templates/drop_egress_by_cgroup:
- src/drop_egress_by_cgroup.bpf.c (XDP or TC egress) or Rust/aya equivalent
- params: {dst_ip, dst_port, cgroup_id, ttl}
- Map for ttl/expiry; return XDP_DROP when match
- Makefile builds CO-RE object with clang/llvm and bpftool gen skeleton
- README with usage/params
