# Security Policy

## Supported Versions

| Version  | Supported          |
| -------- | ------------------ |
| v0.x.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in K8s Core Operator, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **juandsolerc [at] gmail [dot] com**

Include the following details:

- A description of the vulnerability.
- Steps to reproduce.
- Potential impact.
- Suggested fix (if any).

### Response Timeline

- **Acknowledgment**: Within 48 hours.
- **Initial assessment**: Within 5 business days.
- **Fix and disclosure**: We aim to release a fix within 30 days for confirmed vulnerabilities.

## Security Best Practices

This operator follows security best practices:

- **Secure by Default**: Pods run as non-root with all Linux capabilities dropped.
- **Network Policies**: Zero-trust egress/ingress by default.
- **RBAC**: Minimal permissions via dedicated ServiceAccounts.
- **Distroless Image**: The operator runs on `gcr.io/distroless/static:nonroot`.
- **No hardcoded secrets**: CI/CD uses OIDC authentication and GitHub Secrets.
