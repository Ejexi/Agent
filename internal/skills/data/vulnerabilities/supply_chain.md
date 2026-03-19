---
name: supply-chain-security
description: Software supply chain attack detection — dependency confusion, typosquatting, malicious packages, CI/CD poisoning
---

# Supply Chain Security

Modern attacks target the build pipeline and dependency ecosystem rather than the application itself. A single compromised package can affect thousands of projects downstream.

## Attack Vectors

### Dependency Confusion
- Internal package names published to public registries (npm, PyPI, RubyGems)
- Package manager resolves public over internal when versions conflict
- Target: private package names in `package.json`, `requirements.txt`, `go.mod`

**Detection signals:**
```
- Internal package names in public registries with higher version numbers
- Packages with 0 downloads but matching internal naming conventions
- `postinstall` scripts in packages that were previously empty
```

### Typosquatting
- `reqeusts` instead of `requests`
- `colourama` instead of `colorama`
- Unicode homoglyphs in package names

**Check methodology:**
```bash
# npm
npm audit
npx @nodesecurity/nsp check

# Python
pip-audit --require-hashes
safety check

# Go
govulncheck ./...
```

### Malicious Package Behaviour
Red flags in package code:
- `postinstall`/`prepare` scripts making network calls
- Reading environment variables (esp. `AWS_*`, `GITHUB_TOKEN`, `CI`)
- Writing to paths outside the package directory
- Obfuscated base64 payloads in install scripts

### CI/CD Pipeline Poisoning
- GitHub Actions: unpinned `uses: actions/checkout@main` (use SHA pins)
- Poisoned caches: cache keys not scoped to lockfile hash
- Secrets leakage: `env:` blocks exposing tokens to forked PRs
- Artifact injection: unsigned build artifacts

**Hardening checklist:**
```yaml
# Pin to SHA, not tag
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683

# Restrict permissions
permissions:
  contents: read

# No secrets in PR workflows from forks
on:
  pull_request_target:  # ⚠ dangerous — avoid or gate carefully
```

## Scanner Coverage

| Risk | Tool | Signal |
|------|------|--------|
| Known CVEs | trivy, grype | CVE in dependency tree |
| Outdated packages | osvscanner | Package has newer version |
| Malicious packages | trivy | OSV malicious package DB |
| Secrets in CI | gitleaks | Token patterns in YAML |

## Remediation Priority

1. Pin all CI actions to commit SHAs
2. Enable dependency review on PRs (GitHub: `dependency-review-action`)
3. Use lockfiles (`package-lock.json`, `go.sum`, `poetry.lock`) — commit them
4. Enable Dependabot or Renovate for automated updates
5. Scope package registry scopes for internal packages (npm: `@org/`, pip: index-url)
