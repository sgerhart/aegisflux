# SPDX-License-Identifier: GPL-2.0
# Copyright (c) 2024 AegisFlux

CLANG ?= clang
LLC ?= llc
BPFTOOL ?= bpftool
ARCH ?= $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

# Source files
BPF_SRC := src/drop_egress_by_cgroup.bpf.c
BPF_OBJ := drop_egress_by_cgroup.bpf.o
SKEL := drop_egress_by_cgroup.skel.h

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
	$(BPFTOOL) prog load $(BPF_OBJ) /sys/fs/bpf/drop_egress_by_cgroup

# Uninstall BPF program (requires root)
uninstall:
	@echo "Uninstalling BPF program..."
	@if [ $$(id -u) -ne 0 ]; then \
		echo "Error: Uninstallation requires root privileges"; \
		exit 1; \
	fi
	rm -f /sys/fs/bpf/drop_egress_by_cgroup

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BPF_OBJ) $(SKEL)

# Show help
help:
	@echo "Available targets:"
	@echo "  all       - Build BPF object and skeleton (default)"
	@echo "  clean     - Remove build artifacts"
	@echo "  install   - Install BPF program (requires root)"
	@echo "  uninstall - Remove installed BPF program (requires root)"
	@echo "  help      - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  CLANG     - Clang compiler path (default: clang)"
	@echo "  LLC       - LLVM static compiler path (default: llc)"
	@echo "  BPFTOOL   - bpftool path (default: bpftool)"
	@echo "  ARCH      - Target architecture (default: detected)"
	@echo "  KERNEL_SRC - Kernel source path (default: /lib/modules/\$$(uname -r)/build)"

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
