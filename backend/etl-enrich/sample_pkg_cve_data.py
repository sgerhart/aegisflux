#!/usr/bin/env python3
"""
Sample data generator for testing ETL enrich package CVE functionality.
Generates sample CVE data and package CVE mappings for testing.
"""

import asyncio
import json
import random
from datetime import datetime, timedelta
from nats.aio.client import Client as NATS

# Sample CVE data
SAMPLE_CVE_DATA = [
    {
        "cve_id": "CVE-2023-1234",
        "published": "2023-01-01T00:00:00Z",
        "last_modified": "2023-01-15T00:00:00Z",
        "descriptions": [
            {
                "lang": "en",
                "value": "A critical vulnerability in OpenSSL allows remote code execution",
                "source": "nvd"
            }
        ],
        "cvss": {
            "base": {
                "v3.1": {
                    "score": 9.8,
                    "severity": "CRITICAL"
                }
            }
        },
        "cwe": {
            "cwe_ids": ["CWE-89", "CWE-79"],
            "cwe_names": ["SQL Injection", "Cross-site Scripting"]
        },
        "affected_products": [
            {
                "cpe_name": "cpe:2.3:a:openssl:openssl:3.0.2:*:*:*:*:*:*:*",
                "version_start_including": "3.0.2",
                "version_end_excluding": "3.0.3"
            }
        ],
        "references": [
            {"url": "https://nvd.nist.gov/vuln/detail/CVE-2023-1234"},
            {"url": "https://www.openssl.org/news/secadv/20230101.txt"},
            {"url": "https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1234"}
        ]
    },
    {
        "cve_id": "CVE-2023-5678",
        "published": "2023-02-01T00:00:00Z",
        "last_modified": "2023-02-15T00:00:00Z",
        "descriptions": [
            {
                "lang": "en",
                "value": "A vulnerability in curl allows information disclosure",
                "source": "nvd"
            }
        ],
        "cvss": {
            "base": {
                "v3.1": {
                    "score": 6.5,
                    "severity": "MEDIUM"
                }
            }
        },
        "cwe": {
            "cwe_ids": ["CWE-200"],
            "cwe_names": ["Information Disclosure"]
        },
        "affected_products": [
            {
                "cpe_name": "cpe:2.3:a:curl:curl:7.81.0:*:*:*:*:*:*:*",
                "version_start_including": "7.81.0",
                "version_end_excluding": "7.82.0"
            }
        ],
        "references": [
            {"url": "https://nvd.nist.gov/vuln/detail/CVE-2023-5678"}
        ]
    },
    {
        "cve_id": "CVE-2023-9012",
        "published": "2023-03-01T00:00:00Z",
        "last_modified": "2023-03-15T00:00:00Z",
        "descriptions": [
            {
                "lang": "en",
                "value": "A vulnerability in nginx allows denial of service",
                "source": "nvd"
            }
        ],
        "cvss": {
            "base": {
                "v3.1": {
                    "score": 7.5,
                    "severity": "HIGH"
                }
            }
        },
        "cwe": {
            "cwe_ids": ["CWE-400"],
            "cwe_names": ["Denial of Service"]
        },
        "affected_products": [
            {
                "cpe_name": "cpe:2.3:a:nginx:nginx:1.18.0:*:*:*:*:*:*:*",
                "version_start_including": "1.18.0",
                "version_end_excluding": "1.19.0"
            }
        ],
        "references": [
            {"url": "https://nvd.nist.gov/vuln/detail/CVE-2023-9012"},
            {"url": "https://nginx.org/en/security_advisories.html"}
        ]
    }
]

# Sample package CVE mappings
SAMPLE_PKG_CVE_MAPPINGS = [
    {
        "host_id": "web-01",
        "package": {
            "name": "openssl",
            "version": "3.0.2-0ubuntu1.6",
            "epoch": "",
            "release": "0ubuntu1.6",
            "arch": "amd64",
            "distro": "ubuntu",
            "distro_version": "22.04"
        },
        "candidates": [
            {
                "cve_id": "CVE-2023-1234",
                "score": 0.875,
                "reason": "Package openssl version 3.0.2-0ubuntu1.6 matches affected products (CVSS: 9.8)",
                "cvss_score": 9.8,
                "severity": "CRITICAL"
            }
        ],
        "timestamp": "2023-01-01T00:00:00Z",
        "total_candidates": 1
    },
    {
        "host_id": "api-01",
        "package": {
            "name": "curl",
            "version": "7.81.0-1ubuntu1.8",
            "epoch": "",
            "release": "1ubuntu1.8",
            "arch": "amd64",
            "distro": "ubuntu",
            "distro_version": "22.04"
        },
        "candidates": [
            {
                "cve_id": "CVE-2023-5678",
                "score": 0.650,
                "reason": "Package curl version 7.81.0-1ubuntu1.8 matches affected products (CVSS: 6.5)",
                "cvss_score": 6.5,
                "severity": "MEDIUM"
            }
        ],
        "timestamp": "2023-01-01T00:00:00Z",
        "total_candidates": 1
    },
    {
        "host_id": "proxy-01",
        "package": {
            "name": "nginx",
            "version": "1.18.0-6ubuntu14.4",
            "epoch": "",
            "release": "6ubuntu14.4",
            "arch": "amd64",
            "distro": "ubuntu",
            "distro_version": "22.04"
        },
        "candidates": [
            {
                "cve_id": "CVE-2023-9012",
                "score": 0.750,
                "reason": "Package nginx version 1.18.0-6ubuntu14.4 matches affected products (CVSS: 7.5)",
                "cvss_score": 7.5,
                "severity": "HIGH"
            }
        ],
        "timestamp": "2023-01-01T00:00:00Z",
        "total_candidates": 1
    }
]

async def publish_sample_cve_data(nats_url: str):
    """Publish sample CVE data to feeds.cve.updates."""
    nc = NATS()
    await nc.connect(servers=[nats_url])
    
    try:
        print("Publishing sample CVE data...")
        
        for cve_data in SAMPLE_CVE_DATA:
            await nc.publish("feeds.cve.updates", json.dumps(cve_data).encode())
            print(f"  Published CVE: {cve_data['cve_id']}")
            await asyncio.sleep(0.1)  # Small delay between messages
        
        print("Sample CVE data published!")
        
    finally:
        await nc.drain()

async def publish_sample_pkg_cve_mappings(nats_url: str):
    """Publish sample package CVE mappings to feeds.pkg.cve."""
    nc = NATS()
    await nc.connect(servers=[nats_url])
    
    try:
        print("Publishing sample package CVE mappings...")
        
        for pkg_cve_data in SAMPLE_PKG_CVE_MAPPINGS:
            await nc.publish("feeds.pkg.cve", json.dumps(pkg_cve_data).encode())
            print(f"  Published package CVE mapping: {pkg_cve_data['host_id']} -> {pkg_cve_data['package']['name']}")
            await asyncio.sleep(0.1)  # Small delay between messages
        
        print("Sample package CVE mappings published!")
        
    finally:
        await nc.drain()

async def subscribe_to_enriched_records(nats_url: str):
    """Subscribe to etl.enriched to see the enriched records."""
    nc = NATS()
    await nc.connect(servers=[nats_url])
    
    try:
        print("Subscribing to etl.enriched...")
        
        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                record_type = data.get('record_type', 'unknown')
                host_id = data.get('host_id', 'unknown')
                package_name = data.get('package', {}).get('name', 'unknown')
                cve_id = data.get('cve_candidate', {}).get('cve_id', 'unknown')
                exploitability_score = data.get('enrichment', {}).get('exploitability_score', 0)
                risk_level = data.get('enrichment', {}).get('risk_level', 'unknown')
                
                print(f"  📊 Enriched record: {record_type}")
                print(f"     Host: {host_id}")
                print(f"     Package: {package_name}")
                print(f"     CVE: {cve_id}")
                print(f"     Exploitability Score: {exploitability_score}")
                print(f"     Risk Level: {risk_level}")
                print()
                
            except Exception as e:
                print(f"Error processing enriched record: {e}")
        
        await nc.subscribe("etl.enriched", cb=message_handler)
        print("Subscribed to etl.enriched. Waiting for enriched records...")
        
        # Wait for messages
        await asyncio.sleep(10)
        
    finally:
        await nc.drain()

async def main():
    """Main function to publish sample data and monitor enriched records."""
    nats_url = "nats://localhost:4222"
    
    print("🚀 ETL Enrich Sample Data Generator\n")
    
    # Start monitoring enriched records in the background
    monitor_task = asyncio.create_task(subscribe_to_enriched_records(nats_url))
    
    # Wait a bit for the monitor to start
    await asyncio.sleep(1)
    
    # Publish CVE data first
    await publish_sample_cve_data(nats_url)
    await asyncio.sleep(1)
    
    # Then publish package CVE mappings
    await publish_sample_pkg_cve_mappings(nats_url)
    
    # Wait for enriched records to be generated
    await asyncio.sleep(5)
    
    # Cancel the monitor task
    monitor_task.cancel()
    
    print("Sample data generation completed!")

if __name__ == "__main__":
    asyncio.run(main())
