Create template deny_syscall_for_cgroup:
- LSM or kprobe on sys_enter_execve/ptrace
- params: {cgroup_id, syscall=["execve","ptrace"], ttl}
- Block when in cgroup and syscall matches → return -EPERM
- Makefile + README
