# SPDX-License-Identifier: GPL-2.0
# Copyright (c) 2024 AegisFlux

CC ?= gcc
CFLAGS ?= -Wall -Wextra -O2 -g
LDFLAGS ?= -lbpf

TARGET := manager
SRC := manager.c

.PHONY: all clean install

all: $(TARGET)

$(TARGET): $(SRC)
	$(CC) $(CFLAGS) -o $@ $< $(LDFLAGS)

clean:
	rm -f $(TARGET)

install: $(TARGET)
	cp $(TARGET) /usr/local/bin/

# Example usage targets
.PHONY: example-add example-remove example-stats example-list

example-add: $(TARGET)
	@echo "Example: Adding drop rule for cgroup 12345 to block 8.8.8.8:53"
	@echo "Run: sudo ./$(TARGET) add 1 8.8.8.8 53 12345 3600"

example-remove: $(TARGET)
	@echo "Example: Removing rule ID 1"
	@echo "Run: sudo ./$(TARGET) remove 1"

example-stats: $(TARGET)
	@echo "Example: Show statistics"
	@echo "Run: sudo ./$(TARGET) stats"

example-list: $(TARGET)
	@echo "Example: List all active rules"
	@echo "Run: sudo ./$(TARGET) list"
