# Security Policy

## Scope

AgentOS accepts security reports for the open-source core in this repository, including:

- control plane services
- Rust worker and sandbox baseline
- CLI and HTTP/API surfaces
- scheduler, audit, telemetry, and persistence code

Commercial add-ons and hosted cloud operations may follow separate private processes.

## Supported Versions

Security fixes are best-effort for:

- the latest `main` branch
- the most recent tagged release, once releases are published

Older forks or heavily modified downstream deployments may require independent patching.

## Reporting a Vulnerability

Please do **not** open a public GitHub issue for suspected vulnerabilities.

Preferred process:

1. Use GitHub's private vulnerability reporting or security advisory flow for this repository if it is enabled.
2. If that private path is unavailable, contact the repository maintainer through a private channel before any public disclosure.
3. Include reproduction steps, affected components, impact, and any suggested mitigation.

## Response Goals

Best-effort targets:

- initial triage acknowledgement: within 5 business days
- severity assessment and next-step update: within 10 business days
- coordinated disclosure after a fix or mitigation is ready

These are goals, not contractual SLAs.

## Disclosure Expectations

Please allow reasonable time for validation, remediation, and coordinated disclosure.

When a fix is shipped, maintainers may publish:

- affected versions
- mitigation guidance
- upgrade instructions
- related CVE or advisory metadata when applicable
