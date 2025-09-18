# Deny Syscall for Cgroup BPF Template

This BPF template provides syscall blocking based on cgroup membership. It allows denying specific system calls (execve, execveat, ptrace) for processes running in designated cgroups.

## Features

- **Cgroup-based syscall filtering**: Block syscalls based on cgroup ID
- **Multiple syscall support**: Block execve, execveat, and ptrace syscalls
- **LSM and kprobe hooks**: Uses both LSM and kprobe mechanisms for comprehensive coverage
- **Time-to-live (TTL)**: Rules automatically expire after specified time
- **Statistics tracking**: Monitor blocked syscalls by type
- **CO-RE support**: Compile Once, Run Everywhere compatibility
- **Security enforcement**: Returns -EPERM to block unauthorized syscalls

## Architecture

The template consists of:

- `src/deny_syscall_for_cgroup.bpf.c`: Main BPF program with LSM and kprobe hooks
- `include/deny_syscall_for_cgroup.h`: Header file with definitions
- `Makefile`: Build system for CO-RE compilation
- `example/manager.c`: Management program for rule administration
- `README.md`: This documentation

## Requirements

- Linux kernel 5.8+ (for cgroup ID support)
- Linux kernel 5.2+ (for BPF LSM support)
- Clang/LLVM with BPF support
- bpftool
- Root privileges for loading/unloading

## Building

```bash
# Build BPF object and skeleton
make

# Show build configuration
make info

# Check kernel requirements
make check-kernel

# Test compilation without installing
make test-build

# Clean build artifacts
make clean
```

## Installation

```bash
# Load the BPF program (requires root)
sudo make install

# Attach LSM hook (requires root)
sudo make attach-lsm

# Attach kprobes (requires root)
sudo make attach-kprobes

# Unload the BPF program (requires root)
sudo make uninstall
```

## Usage

### Parameters

Each deny rule is configured with the following parameters:

- **`cgroup_id`**: Cgroup ID to match
- **`syscall`**: Syscall number to deny (execve=59, execveat=322, ptrace=101)
- **`ttl`**: Time-to-live in seconds (rule expiration)

### Supported Syscalls

1. **`execve`**: Execute program
2. **`execveat`**: Execute program at directory file descriptor
3. **`ptrace`**: Process trace and debug

### Rule Management

Rules are managed through BPF maps:

1. **`deny_configs`**: Hash map storing rule configurations
   - Key: Rule ID (32-bit integer)
   - Value: `deny_config` structure

2. **`cgroup_rules`**: Hash map mapping cgroups to rules
   - Key: Cgroup ID (64-bit integer)
   - Value: Rule ID (32-bit integer)

3. **`stats`**: Per-CPU array for statistics
   - Key: Always 0 (single statistics entry)
   - Value: `deny_stats` structure

### Example Usage

```bash
# Build and install the BPF program
make
sudo make install
sudo make attach-lsm
sudo make attach-kprobes

# Build the management tool
cd example
make

# Add a rule to block execve for cgroup 12345
sudo ./manager add 1 12345 execve 3600

# Add a rule to block ptrace for cgroup 67890
sudo ./manager add 2 67890 ptrace 1800

# Check statistics
sudo ./manager stats

# List active rules
sudo ./manager list

# Remove a rule
sudo ./manager remove 1

# Get cgroup ID for a process
sudo ./manager get-cgroup 1234
```

### Program Behavior

1. **LSM Hook**: The `bprm_check_security` LSM hook intercepts execve calls
2. **Kprobe Hooks**: Kprobes on `__x64_sys_ptrace` and `__x64_sys_execve` intercept syscalls
3. **Cgroup Lookup**: Checks if the process's cgroup has an active rule
4. **Rule Validation**: Verifies the rule hasn't expired based on TTL
5. **Matching**: Compares syscall against rule criteria
6. **Action**: Blocks matching syscalls with -EPERM, allows others to proceed

### Statistics

The program tracks:
- **`syscalls_processed`**: Total syscalls processed
- **`syscalls_blocked`**: Total syscalls blocked
- **`execve_blocked`**: Number of execve calls blocked
- **`ptrace_blocked`**: Number of ptrace calls blocked

## Security Considerations

- **Root Privileges**: Loading/unloading requires root access
- **System Impact**: Blocking execve can prevent legitimate processes from running
- **Performance**: Rules are processed for every matching syscall
- **TTL Management**: Expired rules are automatically cleaned up
- **LSM Integration**: Works with existing LSM security modules

## Integration

This template is designed to integrate with the AegisFlux BPF Registry:

1. **Build**: Compile the BPF object using the provided Makefile
2. **Package**: Create a signed artifact containing the BPF object
3. **Deploy**: Upload to the BPF registry for distribution
4. **Load**: Deploy to target systems via the AegisFlux orchestrator

## Troubleshooting

### Common Issues

1. **Build Errors**: Ensure kernel headers and BPF toolchain are installed
2. **Load Failures**: Check kernel version compatibility (5.8+ required for cgroup, 5.2+ for LSM)
3. **No Blocks**: Verify cgroup ID and rule configuration
4. **LSM Conflicts**: Ensure no other LSM modules conflict with bprm_check_security

### Debugging

Enable BPF debugging with:
```bash
# Enable BPF printk (requires kernel 5.2+)
echo 1 > /sys/kernel/debug/tracing/trace_pipe

# Monitor BPF events
cat /sys/kernel/debug/tracing/trace_pipe

# Check loaded BPF programs
bpftool prog list

# Check BPF maps
bpftool map list
```

### Kernel Configuration

Ensure the following kernel options are enabled:
```bash
# Check kernel config
zcat /proc/config.gz | grep -E "(BPF|CGROUP|LSM)"
```

Required options:
- `CONFIG_BPF=y`
- `CONFIG_BPF_SYSCALL=y`
- `CONFIG_CGROUPS=y`
- `CONFIG_SECURITY=y`
- `CONFIG_SECURITY_SELINUX=y` (or other LSM)

## Performance Impact

- **LSM Hook**: Minimal overhead for execve calls
- **Kprobe Hooks**: Small overhead for ptrace and execve calls
- **Memory Usage**: ~1KB per rule in BPF maps
- **CPU Usage**: Negligible for rule lookups

## License

GPL-2.0

## Contributing

1. Follow the existing code style
2. Add appropriate error handling
3. Update documentation for new features
4. Test on multiple kernel versions
5. Ensure compatibility with existing LSM modules
