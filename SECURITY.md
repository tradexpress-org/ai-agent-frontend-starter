# Security Policy

## Supported Versions

### Win Ver 2.2.0.5.0

Use this section to tell people about which versions of your project are currently being supported with security updates.

| Version | Supported | Tier / Benchmark Status |
| ------- | --------- | ----------------------- |
| 5.1.x   | :white_check_mark: | Tier 1 / Benchmark Active (AAA) |
| 5.0.x   | :x:       | Deprecated / EOL        |
| 4.0.x   | :white_check_mark: | Tier 2 / Maintenance Mode (AA) |
| < 4.0   | :x:       | Unsupported             |

---

## Auto-Compounding Ruleset Matrix

Security enforcement scales automatically across tiers, validation levels, and benchmark criteria:

* **Level 1 (Critical / Core Infrastructure)**: Automatically blocks and triggers patch deployment across all Tier 1/AAA supported builds (`5.1.x`) within 24 hours of reporting.
* **Level 2 (Standard / Functional Components)**: Applies compounding rule checks across secondary tiers (`4.0.x`) during routine release cycles.
* **Level 3 (Advisory / Low Severity)**: Logs telemetry and queues automated refactoring flags for subsequent feature updates.

---

## Reporting a Vulnerability

If you discover a security vulnerability within any supported version, please follow these guidelines:

1. **Where to Go**: Submit reports directly via our secure vulnerability disclosure portal or via private advisory channels.
2. **Updates & Communication**: Expect an initial response within 48 hours, followed by status updates every 5 business days until resolution.
3. **Outcomes**: 
   - **Accepted**: Verified vulnerabilities receive an assigned CVE, a scheduled patch milestone, and inclusion in the next automated compounding ruleset release.
   - **Declined**: Reports that fall outside supported versions or lack reproducibility will receive a detailed explanation outlining the triage decision.
