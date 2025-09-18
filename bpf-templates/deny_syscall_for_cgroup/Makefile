# SPDX-License-Identifier: GPL-2.0
# Copyright (c) 2024 AegisFlux

CLANG ?= clang
LLC ?= llc
BPFTOOL ?= bpftool
ARCH ?= $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

# Source files
BPF_SRC := src/deny_syscall_for_cgroup.bpf.c
BPF_OBJ := deny_syscall_for_cgroup.bpf.o
SKEL := deny_syscall_for_cgroup.skel.h

# Compiler flags
CLANG_FLAGS := \
	-target bpf \
	-O2 -g \
	-Wall -Wextra \
	-Wno-unused-value \
	-Wno-pointer-sign \
	-Wno-compare-distinct-pointer-types \
	-Wno-gnu-variable-sized-type-not-at-end \
	-Wno-address-of-packed-member \
	-Wno-tautological-compare \
	-Wno-unknown-warning-option \
	-Wno-array-bounds \
	-Wno-div-by-zero \
	-Wno-sizeof-pointer-memaccess \
	-Wno-unused-but-set-variable \
	-fno-stack-protector \
	-fno-jump-tables \
	-fno-unwind-tables \
	-fno-asynchronous-unwind-tables \
	-xc

# Include paths
INCLUDES := \
	-I/usr/include \
	-I/usr/include/$(shell uname -m)-linux-gnu \
	-I/usr/lib/llvm-*/lib/clang/*/include

# Kernel source path (adjust as needed for your system)
KERNEL_SRC ?= /lib/modules/$(shell uname -r)/build

.PHONY: all clean install uninstall help

all: $(BPF_OBJ) $(SKEL)

# Build BPF object file
$(BPF_OBJ): $(BPF_SRC)
	@echo "Building BPF object: $@"
	$(CLANG) $(CLANG_FLAGS) $(INCLUDES) \
		-I$(KERNEL_SRC)/arch/$(ARCH)/include \
		-I$(KERNEL_SRC)/arch/$(ARCH)/include/generated \
		-I$(KERNEL_SRC)/include \
		-I$(KERNEL_SRC)/include/generated \
		-I$(KERNEL_SRC)/arch/$(ARCH)/include/uapi \
		-I$(KERNEL_SRC)/arch/$(ARCH)/include/generated/uapi \
		-I$(KERNEL_SRC)/include/uapi \
		-I$(KERNEL_SRC)/include/generated/uapi \
		-D__TARGET_ARCH_$(ARCH) \
		-c $< -o $@

# Generate skeleton header
$(SKEL): $(BPF_OBJ)
	@echo "Generating skeleton: $@"
	$(BPFTOOL) gen skeleton $< > $@

# Install BPF program (requires root)
install: $(BPF_OBJ)
	@echo "Installing BPF program..."
	@if [ $$(id -u) -ne 0 ]; then \
		echo "Error: Installation requires root privileges"; \
		exit 1; \
	fi
	$(BPFTOOL) prog load $(BPF_OBJ) /sys/fs/bpf/deny_syscall_for_cgroup

# Attach LSM hook (requires root)
attach-lsm: $(BPF_OBJ)
	@echo "Attaching LSM hook..."
	@if [ $$(id -u) -ne 0 ]; then \
		echo "Error: Attachment requires root privileges"; \
		exit 1; \
	fi
	$(BPFTOOL) prog attach /sys/fs/bpf/deny_syscall_for_cgroup lsm/bprm_check_security

# Attach kprobes (requires root)
attach-kprobes: $(BPF_OBJ)
	@echo "Attaching kprobes..."
	@if [ $$(id -u) -ne 0 ]; then \
		echo "Error: Attachment requires root privileges"; \
		exit 1; \
	fi
	$(BPFTOOL) prog attach /sys/fs/bpf/deny_syscall_for_cgroup kprobe/__x64_sys_ptrace
	$(BPFTOOL) prog attach /sys/fs/bpf/deny_syscall_for_cgroup kprobe/__x64_sys_execve

# Uninstall BPF program (requires root)
uninstall:
	@echo "Uninstalling BPF program..."
	@if [ $$(id -u) -ne 0 ]; then \
		echo "Error: Uninstallation requires root privileges"; \
		exit 1; \
	fi
	rm -f /sys/fs/bpf/deny_syscall_for_cgroup

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BPF_OBJ) $(SKEL)

# Show help
help:
	@echo "Available targets:"
	@echo "  all          - Build BPF object and skeleton (default)"
	@echo "  clean        - Remove build artifacts"
	@echo "  install      - Install BPF program (requires root)"
	@echo "  attach-lsm   - Attach LSM hook (requires root)"
	@echo "  attach-kprobes - Attach kprobes (requires root)"
	@echo "  uninstall    - Remove installed BPF program (requires root)"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  CLANG        - Clang compiler path (default: clang)"
	@echo "  LLC          - LLVM static compiler path (default: llc)"
	@echo "  BPFTOOL      - bpftool path (default: bpftool)"
	@echo "  ARCH         - Target architecture (default: detected)"
	@echo "  KERNEL_SRC   - Kernel source path (default: /lib/modules/\$$(uname -r)/build)"

# Show build info
info:
	@echo "Build configuration:"
	@echo "  CLANG: $(CLANG)"
	@echo "  LLC: $(LLC)"
	@echo "  BPFTOOL: $(BPFTOOL)"
	@echo "  ARCH: $(ARCH)"
	@echo "  KERNEL_SRC: $(KERNEL_SRC)"
	@echo "  BPF_SRC: $(BPF_SRC)"
	@echo "  BPF_OBJ: $(BPF_OBJ)"
	@echo "  SKEL: $(SKEL)"

# Check kernel requirements
check-kernel:
	@echo "Checking kernel requirements..."
	@if [ ! -d "$(KERNEL_SRC)" ]; then \
		echo "Error: Kernel source not found at $(KERNEL_SRC)"; \
		echo "Please install kernel headers or set KERNEL_SRC"; \
		exit 1; \
	fi
	@echo "Kernel source found: $(KERNEL_SRC)"
	@if [ ! -f "/proc/kallsyms" ]; then \
		echo "Warning: /proc/kallsyms not found. Kprobes may not work."; \
	fi
	@echo "Kernel version: $$(uname -r)"
	@echo "Architecture: $$(uname -m)"

# Test compilation without installing
test-build: check-kernel $(BPF_OBJ) $(SKEL)
	@echo "Build test successful!"
	@echo "BPF object: $(BPF_OBJ)"
	@echo "Skeleton: $(SKEL)"
