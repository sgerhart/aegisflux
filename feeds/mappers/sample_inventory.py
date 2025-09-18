#!/usr/bin/env python3
"""
Sample inventory.packages message generator for testing package mappers.
This simulates what agents would send to the inventory.packages NATS subject.
"""

import asyncio
import json
import random
import time
from datetime import datetime
from nats.aio.client import Client as NATS

# Sample package data for different distributions
SAMPLE_PACKAGES = {
    "ubuntu": [
        {"name": "openssl", "version": "3.0.2-0ubuntu1.6", "epoch": "", "release": "0ubuntu1.6", "arch": "amd64"},
        {"name": "libssl3", "version": "3.0.2-0ubuntu1.6", "epoch": "", "release": "0ubuntu1.6", "arch": "amd64"},
        {"name": "curl", "version": "7.81.0-1ubuntu1.8", "epoch": "", "release": "1ubuntu1.8", "arch": "amd64"},
        {"name": "wget", "version": "1.21.2-2ubuntu1", "epoch": "", "release": "2ubuntu1", "arch": "amd64"},
        {"name": "git", "version": "1:2.34.1-1ubuntu1.4", "epoch": "1", "release": "1ubuntu1.4", "arch": "amd64"},
        {"name": "python3", "version": "3.10.6-1~22.04", "epoch": "", "release": "1~22.04", "arch": "amd64"},
        {"name": "python3-requests", "version": "2.25.1-1ubuntu0.2", "epoch": "", "release": "1ubuntu0.2", "arch": "amd64"},
        {"name": "nginx", "version": "1.18.0-6ubuntu14.4", "epoch": "", "release": "6ubuntu14.4", "arch": "amd64"},
        {"name": "apache2", "version": "2.4.52-1ubuntu4.4", "epoch": "", "release": "1ubuntu4.4", "arch": "amd64"},
        {"name": "mysql-server", "version": "8.0.32-0ubuntu0.22.04.2", "epoch": "", "release": "0ubuntu0.22.04.2", "arch": "amd64"},
    ],
    "rhel": [
        {"name": "openssl", "version": "1.1.1k-7.el8_6", "epoch": "", "release": "7.el8_6", "arch": "x86_64"},
        {"name": "openssl-libs", "version": "1.1.1k-7.el8_6", "epoch": "", "release": "7.el8_6", "arch": "x86_64"},
        {"name": "curl", "version": "7.61.1-25.el8_7.1", "epoch": "", "release": "25.el8_7.1", "arch": "x86_64"},
        {"name": "wget", "version": "1.19.5-10.el8", "epoch": "", "release": "10.el8", "arch": "x86_64"},
        {"name": "git", "version": "2.27.0-1.el8", "epoch": "", "release": "1.el8", "arch": "x86_64"},
        {"name": "python3", "version": "3.6.8-51.el8", "epoch": "", "release": "51.el8", "arch": "x86_64"},
        {"name": "python3-requests", "version": "2.20.0-1.el8", "epoch": "", "release": "1.el8", "arch": "x86_64"},
        {"name": "nginx", "version": "1.14.1-9.module_el8.0.0+184+e34fea82", "epoch": "", "release": "9.module_el8.0.0+184+e34fea82", "arch": "x86_64"},
        {"name": "httpd", "version": "2.4.37-47.module_el8.5.0+1022+b9f8a6a2", "epoch": "", "release": "47.module_el8.5.0+1022+b9f8a2", "arch": "x86_64"},
        {"name": "mariadb-server", "version": "3:10.3.28-1.module_el8.3.0+757+382f9b3c", "epoch": "3", "release": "10.3.28-1.module_el8.3.0+757+382f9b3c", "arch": "x86_64"},
    ],
    "debian": [
        {"name": "openssl", "version": "1.1.1n-0+deb11u4", "epoch": "", "release": "0+deb11u4", "arch": "amd64"},
        {"name": "libssl1.1", "version": "1.1.1n-0+deb11u4", "epoch": "", "release": "0+deb11u4", "arch": "amd64"},
        {"name": "curl", "version": "7.74.0-1.3+deb11u7", "epoch": "", "release": "1.3+deb11u7", "arch": "amd64"},
        {"name": "wget", "version": "1.21-1+deb11u1", "epoch": "", "release": "1+deb11u1", "arch": "amd64"},
        {"name": "git", "version": "1:2.30.2-1+deb11u1", "epoch": "1", "release": "2.30.2-1+deb11u1", "arch": "amd64"},
        {"name": "python3", "version": "3.9.2-3", "epoch": "", "release": "3", "arch": "amd64"},
        {"name": "python3-requests", "version": "2.25.1+dfsg-2", "epoch": "", "release": "1+dfsg-2", "arch": "amd64"},
        {"name": "nginx", "version": "1.18.0-6.1+deb11u3", "epoch": "", "release": "6.1+deb11u3", "arch": "amd64"},
        {"name": "apache2", "version": "2.4.48-3.1+deb11u2", "epoch": "", "release": "2.4.48-3.1+deb11u2", "arch": "amd64"},
        {"name": "mariadb-server", "version": "1:10.5.15-0+deb11u1", "epoch": "1", "release": "10.5.15-0+deb11u1", "arch": "amd64"},
    ]
}

def generate_inventory_message(host_id: str, distro: str, packages: list) -> dict:
    """Generate a sample inventory.packages message."""
    return {
        "host_id": host_id,
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "distro": {
            "name": distro,
            "version": "22.04" if distro == "ubuntu" else "8" if distro == "rhel" else "11",
            "codename": "jammy" if distro == "ubuntu" else "rhel8" if distro == "rhel" else "bullseye"
        },
        "packages": packages,
        "metadata": {
            "agent_version": "1.0.0",
            "scan_duration_ms": random.randint(1000, 5000),
            "total_packages": len(packages)
        }
    }

async def publish_sample_inventory(nats_url: str, host_id: str, distro: str, count: int = 5):
    """Publish sample inventory messages to NATS."""
    nc = NATS()
    await nc.connect(servers=[nats_url])
    
    try:
        packages = SAMPLE_PACKAGES.get(distro, SAMPLE_PACKAGES["ubuntu"])
        selected_packages = random.sample(packages, min(count, len(packages)))
        
        message = generate_inventory_message(host_id, distro, selected_packages)
        
        await nc.publish("inventory.packages", json.dumps(message).encode())
        print(f"Published inventory for {host_id} ({distro}): {len(selected_packages)} packages")
        
        # Pretty print the message
        print(json.dumps(message, indent=2))
        
    finally:
        await nc.drain()

async def main():
    """Main function to publish sample inventory messages."""
    nats_url = "nats://localhost:4222"
    
    print("Publishing sample inventory messages...")
    
    # Publish samples for different hosts and distributions
    await publish_sample_inventory(nats_url, "web-01", "ubuntu", 8)
    await asyncio.sleep(1)
    
    await publish_sample_inventory(nats_url, "db-01", "rhel", 6)
    await asyncio.sleep(1)
    
    await publish_sample_inventory(nats_url, "api-01", "debian", 7)
    await asyncio.sleep(1)
    
    await publish_sample_inventory(nats_url, "cache-01", "ubuntu", 5)
    
    print("Sample inventory messages published!")

if __name__ == "__main__":
    asyncio.run(main())
