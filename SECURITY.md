# Security Policy

## Supported Versions

The main branch is the supported development line until formal releases begin.

## Reporting a Vulnerability

Please do not report vulnerabilities in public issues.

Use GitHub Security Advisories if they are enabled for the repository. If private advisories are not available, open a minimal public issue asking for a private security contact and do not include exploit details.

Include enough information to reproduce the issue privately:

- Affected version or commit
- Transport mode, configuration, and environment
- Reproduction steps
- Expected and actual behavior
- Impact assessment

## Scope

Security-sensitive areas include:

- HTTP transport exposure
- MCP tool argument handling
- Outbound Maven Central or Context7 requests
- Logging of user-provided data
- Dependency updates
