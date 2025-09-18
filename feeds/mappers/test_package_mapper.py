#!/usr/bin/env python3
"""
Test script for package mapper functionality.
Tests the package matching logic and CVE candidate generation.
"""

import asyncio
import json
import sys
from pathlib import Path

# Add the mapper module to the path
sys.path.insert(0, str(Path(__file__).parent))

from mapper.main import PackageMatcher, PackageInfo, CVECandidate

def test_package_normalization():
    """Test package name normalization."""
    print("🧪 Testing package name normalization...")
    
    matcher = PackageMatcher()
    
    test_cases = [
        ("libssl3", "ssl3"),
        ("python3-requests", "requests"),
        ("node-express", "express"),
        ("ruby-rails", "rails"),
        ("php-mysql", "mysql"),
        ("go-gin", "gin"),
        ("golang-gin", "gin"),
        ("openjdk-11", "11"),
        ("java-spring", "spring"),
        ("libcurl4", "curl4"),
    ]
    
    for input_name, expected in test_cases:
        result = matcher.normalize_package_name(input_name)
        print(f"  {input_name} -> {result} (expected: {expected})")
        assert result == expected, f"Expected {expected}, got {result}"
    
    print("✅ Package normalization test passed")

def test_version_parsing():
    """Test version parsing."""
    print("\n🧪 Testing version parsing...")
    
    matcher = PackageMatcher()
    
    test_cases = [
        ("1.2.3", ("", "1.2.3", "")),
        ("1:2.3.4", ("1", "2.3.4", "")),
        ("1.2.3-1ubuntu1", ("", "1.2.3", "1ubuntu1")),
        ("1:2.3.4-5.el8", ("1", "2.3.4", "5.el8")),
    ]
    
    for version, expected in test_cases:
        result = matcher.parse_version(version)
        print(f"  {version} -> {result} (expected: {expected})")
        assert result == expected, f"Expected {expected}, got {result}"
    
    print("✅ Version parsing test passed")

def test_version_comparison():
    """Test version comparison."""
    print("\n🧪 Testing version comparison...")
    
    matcher = PackageMatcher()
    
    test_cases = [
        ("1.2.3", "1.2.4", -1),
        ("1.2.4", "1.2.3", 1),
        ("1.2.3", "1.2.3", 0),
        ("2.0.0", "1.9.9", 1),
        ("1.0.0", "2.0.0", -1),
    ]
    
    for v1, v2, expected in test_cases:
        result = matcher.version_compare(v1, v2)
        print(f"  {v1} vs {v2} -> {result} (expected: {expected})")
        assert result == expected, f"Expected {expected}, got {result}"
    
    print("✅ Version comparison test passed")

def test_vulnerable_version_check():
    """Test vulnerable version checking."""
    print("\n🧪 Testing vulnerable version checking...")
    
    matcher = PackageMatcher()
    
    # Test case 1: Version in range
    affected_products = [
        {
            "cpe_name": "cpe:2.3:a:openssl:openssl:1.1.1:*:*:*:*:*:*:*",
            "version_start_including": "1.1.1",
            "version_end_excluding": "1.1.2"
        }
    ]
    
    result = matcher.is_vulnerable_version("1.1.1k", affected_products)
    print(f"  Version 1.1.1k in range 1.1.1-1.1.2: {result}")
    assert result == True, "Should be vulnerable"
    
    # Test case 2: Version outside range
    result = matcher.is_vulnerable_version("1.1.2", affected_products)
    print(f"  Version 1.1.2 outside range 1.1.1-1.1.2: {result}")
    assert result == False, "Should not be vulnerable"
    
    print("✅ Vulnerable version check test passed")

def test_heuristic_scoring():
    """Test heuristic scoring."""
    print("\n🧪 Testing heuristic scoring...")
    
    matcher = PackageMatcher()
    
    # Create test package
    package = PackageInfo(
        name="openssl",
        version="1.1.1k",
        epoch="",
        release="7.el8_6",
        arch="x86_64",
        distro="rhel",
        distro_version="8"
    )
    
    # Create test CVE data
    cve_data = {
        "cve_id": "CVE-2023-1234",
        "affected_products": [
            {
                "cpe_name": "cpe:2.3:a:openssl:openssl:1.1.1:*:*:*:*:*:*:*",
                "version_start_including": "1.1.1",
                "version_end_excluding": "1.1.2"
            }
        ],
        "cvss": {
            "base": {
                "v3.1": {
                    "score": 7.5,
                    "severity": "HIGH"
                }
            }
        }
    }
    
    score = matcher.calculate_heuristic_score(package, cve_data)
    print(f"  Score for openssl 1.1.1k: {score:.3f}")
    assert score > 0.5, f"Score should be > 0.5, got {score}"
    
    print("✅ Heuristic scoring test passed")

def test_cve_candidate_generation():
    """Test CVE candidate generation."""
    print("\n🧪 Testing CVE candidate generation...")
    
    matcher = PackageMatcher()
    
    # Create test package
    package = PackageInfo(
        name="openssl",
        version="1.1.1k",
        epoch="",
        release="7.el8_6",
        arch="x86_64",
        distro="rhel",
        distro_version="8"
    )
    
    # Create test CVE data
    cve_data = {
        "cve_id": "CVE-2023-1234",
        "affected_products": [
            {
                "cpe_name": "cpe:2.3:a:openssl:openssl:1.1.1:*:*:*:*:*:*:*",
                "version_start_including": "1.1.1",
                "version_end_excluding": "1.1.2"
            }
        ],
        "cvss": {
            "base": {
                "v3.1": {
                    "score": 7.5,
                    "severity": "HIGH"
                }
            }
        }
    }
    
    candidates = matcher.find_cve_candidates(package, cve_data)
    print(f"  Generated {len(candidates)} candidates")
    
    if candidates:
        candidate = candidates[0]
        print(f"  Top candidate: {candidate.cve_id} (score: {candidate.score:.3f})")
        print(f"  Reason: {candidate.reason}")
        print(f"  CVSS Score: {candidate.cvss_score}")
        print(f"  Severity: {candidate.severity}")
        
        assert candidate.cve_id == "CVE-2023-1234"
        assert candidate.score > 0.5
        assert candidate.cvss_score == 7.5
        assert candidate.severity == "HIGH"
    
    print("✅ CVE candidate generation test passed")

def test_package_patterns():
    """Test package pattern matching."""
    print("\n🧪 Testing package pattern matching...")
    
    matcher = PackageMatcher()
    
    test_cases = [
        ("openssl", True),
        ("libssl3", True),  # Should match openssl pattern
        ("libcrypto", True),  # Should match openssl pattern
        ("curl", True),
        ("libcurl4", True),  # Should match curl pattern
        ("wget", True),
        ("git", True),
        ("python3", True),
        ("python-requests", True),
        ("nginx", True),
        ("apache2", True),
        ("httpd", True),
        ("mysql", True),
        ("mariadb", True),
        ("unknown-package", False),
    ]
    
    for package_name, should_match in test_cases:
        normalized = matcher.normalize_package_name(package_name)
        matches = any(
            any(alias in normalized for alias in aliases) or
            any(alias in package_name.lower() for alias in aliases)
            for aliases in matcher.package_patterns.values()
        )
        print(f"  {package_name} -> {normalized} (matches: {matches})")
        assert matches == should_match, f"Expected {should_match}, got {matches}"
    
    print("✅ Package pattern matching test passed")

def main():
    """Run all tests."""
    print("🚀 Package Mapper Tests\n")
    
    try:
        test_package_normalization()
        test_version_parsing()
        test_version_comparison()
        test_vulnerable_version_check()
        test_heuristic_scoring()
        test_cve_candidate_generation()
        test_package_patterns()
        
        print("\n🎉 All tests passed!")
        
    except Exception as e:
        print(f"\n❌ Test failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
