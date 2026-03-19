---
name: reconnaissance
description: Attack surface discovery — technology fingerprinting, endpoint enumeration, secret leakage, dependency mapping
---

# Reconnaissance

Systematic discovery of the attack surface before any exploitation attempt. Goal: build a complete mental model of the target.

## Technology Fingerprinting

### HTTP Headers
```
Server: nginx/1.18.0          → nginx version
X-Powered-By: PHP/8.1.2       → language + version
X-AspNet-Version: 4.0.30319   → .NET version
X-Generator: Drupal 9         → CMS
Set-Cookie: PHPSESSID=...     → PHP
Set-Cookie: JSESSIONID=...    → Java
Set-Cookie: laravel_session=  → Laravel
```

### Response Patterns
```
/robots.txt           → disallowed paths, admin areas
/sitemap.xml          → full site structure
/.well-known/         → ACME challenges, security.txt
/favicon.ico          → hash to identify framework (shodan favicon)
/WEB-INF/web.xml      → Java deployment descriptor
/package.json         → Node.js version + dependencies
/composer.json        → PHP dependencies
```

### Error Messages
```
Django: "Page not found" with URL patterns
Spring: "Whitelabel Error Page"
Express: "Cannot GET /path" with stack trace
Laravel: "Whoops, looks like something went wrong"
```

## Endpoint Enumeration

### JavaScript Analysis
Modern SPAs expose all routes in the JS bundle:
```bash
# Extract endpoints from bundled JS
grep -oE '"/api/[a-zA-Z0-9/_-]+"' app.js | sort -u
grep -oE 'fetch\("[^"]+"\)' *.js
grep -oE 'axios\.(get|post|put|delete)\("[^"]+"\)' *.js

# Unminify + search
js-beautify app.min.js | grep -E '(fetch|axios|request)\('
```

### API Schema Discovery
```
/swagger.json
/swagger/v1/swagger.json
/api-docs
/v1/api-docs
/openapi.json
/openapi.yaml
/.well-known/openapi
/graphql          → POST { query: "{ __schema { types { name } } }" }
/graphiql
/altair
```

### Common Admin/Debug Paths
```
/admin
/admin/login
/administrator
/wp-admin         → WordPress
/manage
/console          → Jenkins, Quarkus dev
/actuator         → Spring Boot (exposes /actuator/env, /actuator/beans)
/debug/pprof      → Go pprof endpoint
/_ah/admin        → Google App Engine
/phpmyadmin
/adminer.php
```

## Secret Leakage Detection

### Git History
```bash
# Search all commits for secrets
git log --all --full-history -- "*.env"
git log -p | grep -E '(password|secret|key|token)\s*='
trufflehog git file://. --since-commit HEAD~100

# Find deleted sensitive files
git log --diff-filter=D --summary | grep "\.env\|secret\|credential"
```

### Public Sources
```bash
# GitHub search
org:target filename:.env
org:target "password" extension:yaml
org:target "api_key" language:python

# Shodan
ssl.cert.subject.CN:target.com
http.favicon.hash:-335242539   # identify framework by favicon
```

### Source Map Leakage
```bash
# .map files expose original source
curl https://target.com/static/app.js.map
# Contains original filenames, comments, full source
```

## Infrastructure Mapping

### DNS Enumeration
```bash
# Subdomain brute force
subfinder -d target.com -o subs.txt
amass enum -d target.com -passive

# Certificate transparency
curl "https://crt.sh/?q=%.target.com&output=json" | jq '.[].name_value' | sort -u

# DNS zone transfer attempt
dig axfr target.com @ns1.target.com

# Reverse DNS on IP ranges
nmap -sL 192.168.1.0/24 | grep "Nmap scan report"
```

### Cloud Asset Discovery
```bash
# S3 bucket enumeration
aws s3 ls s3://target-backup --no-sign-request
# Common naming: target-prod, target-staging, target-assets, target-backup

# GCS
gsutil ls gs://target-bucket

# Azure Blobs  
# https://target.blob.core.windows.net/container?restype=container&comp=list
```

## Port + Service Discovery

```bash
# Full port scan
nmap -sV -p- --open -T4 target.com

# Common service fingerprinting
nmap -sC -sV -p 22,80,443,3306,5432,6379,27017,8080,8443 target.com

# Service-specific
redis-cli -h target.com ping          # Redis
psql -h target.com -U postgres        # PostgreSQL  
mongosh "mongodb://target.com:27017"  # MongoDB
```

## DuckOps Reconnaissance Integration

In DuckOps, reconnaissance informs the orchestrator plan:
- Technology stack → selects correct SAST scanners (gosec for Go, bandit for Python)
- API exposure → escalates depth for SCA + secrets
- CI/CD files found → escalates IaC + secrets depth
- Large dependency count → escalates deps scan depth

The `AnalyzeProject()` function in `agent/subagents/analyzer.go` performs automated local reconnaissance before any LLM call.
