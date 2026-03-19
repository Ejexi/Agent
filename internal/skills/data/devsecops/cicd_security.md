---
name: cicd-security
description: CI/CD pipeline security — GitHub Actions hardening, secrets management, artifact signing, pipeline poisoning
---

# CI/CD Security

The build pipeline is an increasingly targeted attack surface. A compromised pipeline can inject malicious code into every artifact it produces.

## GitHub Actions Hardening

### Pin Actions to Commit SHA

```yaml
# ❌ Mutable tag — can be changed to point to malicious code
- uses: actions/checkout@v4
- uses: actions/setup-go@main

# ✅ Pin to immutable commit SHA
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2
- uses: actions/setup-go@f111f3307d8850f501ac008e886eec1fd1932a34  # v5.3.0
```

### Minimal Permissions

```yaml
# ❌ Default: all permissions (write to contents, packages, etc.)

# ✅ Deny all by default, grant only what's needed
permissions: {}

jobs:
  build:
    permissions:
      contents: read      # checkout only
      packages: write     # push to GHCR only if needed
```

### Secrets in Forked PR Workflows

```yaml
# ❌ DANGEROUS: pull_request_target has secrets access + runs fork code
on:
  pull_request_target:
    types: [opened, synchronize]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@...
        with:
          ref: ${{ github.event.pull_request.head.sha }}  # fork code!
      - run: ./build.sh  # fork code with YOUR secrets

# ✅ Separate workflow: test without secrets, deploy only on trusted code
on:
  pull_request:   # no secrets — safe for fork code
```

### Injection via `github.event` Values

```yaml
# ❌ Injects PR title into shell command — PR title can contain shell metacharacters
- run: echo "Testing PR: ${{ github.event.pull_request.title }}"

# ❌ Issue body injected into shell
- run: |
    BODY="${{ github.event.issue.body }}"
    send_notification "$BODY"

# ✅ Use environment variable (value not interpolated into shell syntax)
- name: Safe echo
  env:
    PR_TITLE: ${{ github.event.pull_request.title }}
  run: echo "Testing PR: $PR_TITLE"
```

## Secrets Management

### Never in Code or Config Files

```bash
# ❌ Committed secrets
DATABASE_URL=postgres://user:password@host/db
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# ❌ In Dockerfile
ENV API_KEY=abc123
```

### Correct Patterns

```yaml
# GitHub Actions — use secrets context
- name: Deploy
  env:
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
  run: aws s3 sync ./dist s3://my-bucket
```

### Secrets Scanning in CI

```yaml
# Add to every pipeline
- name: Scan for secrets
  uses: gitleaks/gitleaks-action@v2
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    GITLEAKS_LICENSE: ${{ secrets.GITLEAKS_LICENSE }}

# Or trufflehog
- name: TruffleHog OSS
  uses: trufflesecurity/trufflehog@main
  with:
    path: ./
    base: ${{ github.event.repository.default_branch }}
    head: HEAD
    extra_args: --debug --only-verified
```

## Artifact Integrity

### SLSA (Supply-chain Levels for Software Artifacts)

```yaml
# Generate SLSA provenance with slsa-github-generator
- uses: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.0.0
  with:
    base64-subjects: ${{ needs.build.outputs.digests }}
```

### Cosign (Sigstore) for Container Images

```bash
# Sign image after push
cosign sign --key cosign.key ghcr.io/org/app:sha256-abc123

# Verify before deploy
cosign verify --key cosign.pub ghcr.io/org/app:sha256-abc123
```

## Cache Poisoning

```yaml
# ❌ Cache key not scoped to lockfile — can serve stale/poisoned deps
- uses: actions/cache@v4
  with:
    key: ${{ runner.os }}-node

# ✅ Include lockfile hash in cache key
- uses: actions/cache@v4
  with:
    path: ~/.npm
    key: ${{ runner.os }}-node-${{ hashFiles('**/package-lock.json') }}
```

## DuckOps CI Integration

```yaml
# Add DuckOps scan to your pipeline
- name: DuckOps Security Scan
  run: |
    duckops --scan <<'EOF'
    scan this project for critical and high issues only
    EOF
  continue-on-error: false  # fail pipeline on findings
```

Exit code `1` = findings found above threshold → pipeline fails.
Exit code `0` = clean → pipeline continues.
