# bpf-templates
Parametric CO-RE eBPF program templates used for proactive mitigations. 
Examples:
- Drop socket by cgroup/destination
- Block syscall (execve/ptrace) for target cgroup
- Deny open() of vulnerable library path
Each template has its own README, Makefile, and test.
