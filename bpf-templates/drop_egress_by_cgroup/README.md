# Drop Egress by Cgroup BPF Template

This BPF template provides egress traffic filtering based on cgroup membership. It allows dropping outbound network packets for specific cgroups when they match configured destination IP and port combinations.

## Features

- **Cgroup-based filtering**: Drop traffic based on cgroup ID
- **IP and port matching**: Filter by destination IP address and port
- **Time-to-live (TTL)**: Rules automatically expire after specified time
- **Statistics tracking**: Monitor dropped packets and bytes
- **CO-RE support**: Compile Once, Run Everywhere compatibility
- **XDP integration**: High-performance packet processing

## Architecture

The template consists of:

- `src/drop_egress_by_cgroup.bpf.c`: Main BPF program
- `include/drop_egress_by_cgroup.h`: Header file with definitions
- `Makefile`: Build system for CO-RE compilation
- `README.md`: This documentation

## Requirements

- Linux kernel 5.8+ (for cgroup ID support)
- Clang/LLVM with BPF support
- bpftool
- Root privileges for loading/unloading

## Building

```bash
# Build BPF object and skeleton
make

# Show build configuration
make info

# Clean build artifacts
make clean
```

## Installation

```bash
# Load the BPF program (requires root)
sudo make install

# Unload the BPF program (requires root)
sudo make uninstall
```

## Usage

### Parameters

Each drop rule is configured with the following parameters:

- **`dst_ip`**: Destination IP address (network byte order)
- **`dst_port`**: Destination port (host byte order)  
- **`cgroup_id`**: Cgroup ID to match
- **`ttl`**: Time-to-live in seconds (rule expiration)

### Rule Management

Rules are managed through BPF maps:

1. **`drop_configs`**: Hash map storing rule configurations
   - Key: Rule ID (32-bit integer)
   - Value: `drop_config` structure

2. **`cgroup_rules`**: Hash map mapping cgroups to rules
   - Key: Cgroup ID (64-bit integer)
   - Value: Rule ID (32-bit integer)

3. **`stats`**: Per-CPU array for statistics
   - Key: Always 0 (single statistics entry)
   - Value: `drop_stats` structure

### Example Usage

```bash
# Load the BPF program
sudo make install

# Add a rule to drop traffic to 8.8.8.8:53 for cgroup 12345
# (This would typically be done via a management application)
echo 'Adding drop rule: cgroup=12345, dst=8.8.8.8:53, ttl=3600s'

# Monitor statistics
# (This would typically be done via a management application)
echo 'Checking statistics...'
```

### Program Behavior

1. **Packet Processing**: The XDP program processes each egress packet
2. **Cgroup Lookup**: Checks if the packet's cgroup has an active rule
3. **Rule Validation**: Verifies the rule hasn't expired based on TTL
4. **Matching**: Compares destination IP and port against rule criteria
5. **Action**: Drops matching packets with `XDP_DROP`, passes others with `XDP_PASS`

### Statistics

The program tracks:
- **`packets_dropped`**: Number of packets dropped
- **`packets_processed`**: Total packets processed
- **`bytes_dropped`**: Total bytes dropped

## Integration

This template is designed to integrate with the AegisFlux BPF Registry:

1. **Build**: Compile the BPF object using the provided Makefile
2. **Package**: Create a signed artifact containing the BPF object
3. **Deploy**: Upload to the BPF registry for distribution
4. **Load**: Deploy to target systems via the AegisFlux orchestrator

## Security Considerations

- **Root Privileges**: Loading/unloading requires root access
- **Network Impact**: Incorrect rules can block legitimate traffic
- **Performance**: Rules are processed for every packet on the interface
- **TTL Management**: Expired rules are automatically cleaned up

## Troubleshooting

### Common Issues

1. **Build Errors**: Ensure kernel headers and BPF toolchain are installed
2. **Load Failures**: Check kernel version compatibility (5.8+ required)
3. **No Drops**: Verify cgroup ID and rule configuration
4. **Performance**: Monitor CPU usage and adjust rule complexity

### Debugging

Enable BPF debugging with:
```bash
# Enable BPF printk (requires kernel 5.2+)
echo 1 > /sys/kernel/debug/tracing/trace_pipe

# Monitor BPF events
cat /sys/kernel/debug/tracing/trace_pipe
```

## License

GPL-2.0

## Contributing

1. Follow the existing code style
2. Add appropriate error handling
3. Update documentation for new features
4. Test on multiple kernel versions
