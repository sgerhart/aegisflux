#!/usr/bin/env python3
"""
Test script for package CVE enrichment functionality.
Tests the join logic and exploitability scoring.
"""

import asyncio
import json
import sys
from pathlib import Path

# Add the app module to the path
sys.path.insert(0, str(Path(__file__).parent / "app"))

# Import the functions directly
from app.enrich import enrich_pkg_cve_event, calculate_exploitability_score, _determine_risk_level

def test_exploitability_scoring():
    """Test exploitability score calculation."""
    print("🧪 Testing exploitability score calculation...")
    
    # Test case 1: High-risk CVE with high CVSS
    cve_data = {
        "cve_id": "CVE-2023-1234",
        "published": "2023-01-01T00:00:00Z",
        "cwe": {
            "cwe_ids": ["CWE-89", "CWE-79"]  # SQL Injection, XSS
        },
        "references": [
            {"url": "https://example.com/1"},
            {"url": "https://example.com/2"},
            {"url": "https://example.com/3"},
            {"url": "https://example.com/4"},
            {"url": "https://example.com/5"},
            {"url": "https://example.com/6"}  # 6 references
        ]
    }
    
    candidate = {
        "cve_id": "CVE-2023-1234",
        "score": 0.9,
        "cvss_score": 8.5,
        "severity": "HIGH"
    }
    
    score = calculate_exploitability_score(cve_data, candidate)
    print(f"  High-risk CVE score: {score:.3f}")
    assert score > 0.8, f"Expected score > 0.8, got {score}"
    
    # Test case 2: Low-risk CVE
    cve_data_low = {
        "cve_id": "CVE-2023-5678",
        "published": "2022-01-01T00:00:00Z",  # Old CVE
        "cwe": {
            "cwe_ids": ["CWE-200"]  # Information Disclosure
        },
        "references": [{"url": "https://example.com/1"}]
    }
    
    candidate_low = {
        "cve_id": "CVE-2023-5678",
        "score": 0.3,
        "cvss_score": 3.5,
        "severity": "LOW"
    }
    
    score_low = calculate_exploitability_score(cve_data_low, candidate_low)
    print(f"  Low-risk CVE score: {score_low:.3f}")
    assert score_low < 0.5, f"Expected score < 0.5, got {score_low}"
    
    print("✅ Exploitability scoring test passed")

def test_pkg_cve_enrichment():
    """Test package CVE enrichment."""
    print("\n🧪 Testing package CVE enrichment...")
    
    # Sample package CVE data
    pkg_cve_data = {
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
                "reason": "Package openssl version 3.0.2-0ubuntu1.6 matches affected products (CVSS: 7.5)",
                "cvss_score": 7.5,
                "severity": "HIGH"
            }
        ],
        "timestamp": "2023-01-01T00:00:00Z",
        "total_candidates": 1
    }
    
    # Sample CVE data
    cve_data = {
        "cve_id": "CVE-2023-1234",
        "published": "2023-01-01T00:00:00Z",
        "last_modified": "2023-01-15T00:00:00Z",
        "descriptions": [
            {
                "lang": "en",
                "value": "A vulnerability in OpenSSL allows remote code execution",
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
            {"url": "https://www.openssl.org/news/secadv/20230101.txt"}
        ]
    }
    
    candidate = pkg_cve_data["candidates"][0]
    
    # Test enrichment
    enriched_record = enrich_pkg_cve_event(pkg_cve_data, candidate, cve_data)
    
    print(f"  Enriched record type: {enriched_record['record_type']}")
    print(f"  Host ID: {enriched_record['host_id']}")
    print(f"  Package: {enriched_record['package']['name']}")
    print(f"  CVE ID: {enriched_record['cve_candidate']['cve_id']}")
    print(f"  Exploitability score: {enriched_record['enrichment']['exploitability_score']}")
    print(f"  Risk level: {enriched_record['enrichment']['risk_level']}")
    
    # Validate structure
    assert enriched_record["record_type"] == "pkg_cve_enriched"
    assert enriched_record["host_id"] == "web-01"
    assert enriched_record["package"]["name"] == "openssl"
    assert enriched_record["cve_candidate"]["cve_id"] == "CVE-2023-1234"
    assert "exploitability_score" in enriched_record["enrichment"]
    assert "risk_level" in enriched_record["enrichment"]
    assert enriched_record["enrichment"]["exploitability_score"] > 0
    assert enriched_record["enrichment"]["risk_level"] in ["CRITICAL", "HIGH", "MEDIUM", "LOW", "MINIMAL"]
    
    print("✅ Package CVE enrichment test passed")

def test_risk_level_determination():
    """Test risk level determination."""
    print("\n🧪 Testing risk level determination...")
    
    test_cases = [
        (0.9, "CRITICAL"),
        (0.7, "HIGH"),
        (0.5, "MEDIUM"),
        (0.3, "LOW"),
        (0.1, "MINIMAL")
    ]
    
    for score, expected_level in test_cases:
        result = _determine_risk_level(score)
        print(f"  Score {score} -> {result} (expected: {expected_level})")
        assert result == expected_level, f"Expected {expected_level}, got {result}"
    
    print("✅ Risk level determination test passed")

def test_edge_cases():
    """Test edge cases for enrichment."""
    print("\n🧪 Testing edge cases...")
    
    # Test with minimal data
    pkg_cve_data_minimal = {
        "host_id": "test-01",
        "package": {"name": "test-pkg", "version": "1.0.0"},
        "candidates": [{"cve_id": "CVE-TEST", "score": 0.5}],
        "timestamp": "2023-01-01T00:00:00Z"
    }
    
    cve_data_minimal = {
        "cve_id": "CVE-TEST",
        "published": "2023-01-01T00:00:00Z"
    }
    
    candidate_minimal = {"cve_id": "CVE-TEST", "score": 0.5}
    
    enriched = enrich_pkg_cve_event(pkg_cve_data_minimal, candidate_minimal, cve_data_minimal)
    
    print(f"  Minimal data enrichment: {enriched['record_type']}")
    assert enriched["record_type"] == "pkg_cve_enriched"
    assert enriched["enrichment"]["exploitability_score"] >= 0
    assert enriched["enrichment"]["risk_level"] in ["CRITICAL", "HIGH", "MEDIUM", "LOW", "MINIMAL"]
    
    print("✅ Edge cases test passed")

def main():
    """Run all tests."""
    print("🚀 ETL Enrich Package CVE Tests\n")
    
    try:
        test_exploitability_scoring()
        test_pkg_cve_enrichment()
        test_risk_level_determination()
        test_edge_cases()
        
        print("\n🎉 All tests passed!")
        
    except Exception as e:
        print(f"\n❌ Test failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
