---
name: api-security
description: REST and GraphQL API security — authentication, authorization, rate limiting, input validation, OWASP API Top 10
---

# API Security

APIs are the primary attack surface of modern applications. OWASP API Top 10 covers the most common and impactful vulnerabilities.

## OWASP API Top 10 (2023)

### API1 — Broken Object Level Authorization (BOLA/IDOR)

```http
GET /api/orders/1234    → returns your order
GET /api/orders/1235    → should 403, but returns another user's order
```

**Fix:** Always verify the authenticated user owns or has access to the requested resource.

```go
// ❌ Trusts the ID from the request
order := db.GetOrder(params.OrderID)

// ✅ Scopes query to authenticated user
order := db.GetOrderForUser(params.OrderID, ctx.UserID)
if order == nil {
    return 403
}
```

### API2 — Broken Authentication

- Weak JWT secrets (< 32 bytes)
- `alg: none` accepted
- Missing token expiry
- Tokens not invalidated on logout

```go
// ❌ 
token.Method = jwt.SigningMethodNone   // allows unsigned tokens

// ❌ Accepts any algorithm
jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
    return secretKey, nil  // doesn't check token.Method
})

// ✅ Enforce algorithm
jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    return secretKey, nil
})
```

### API3 — Broken Object Property Level Authorization

Mass assignment — binding all request fields to a model:

```go
// ❌ Binds isAdmin from request body
json.Unmarshal(body, &user)
db.Save(user)

// ✅ Allowlist bindable fields
user.Name = input.Name
user.Email = input.Email
// Never: user.IsAdmin, user.Role, user.Balance
```

### API5 — Broken Function Level Authorization

Admin endpoints accessible without admin role:

```http
GET  /api/users/me     → OK for any user
POST /api/users/delete → should require admin, but only checks auth
```

### API6 — Unrestricted Access to Sensitive Business Flows

Rate limiting gaps:
- Password reset: try 10,000 codes without lockout
- OTP: no attempt limit
- Purchase: no stock limit enforcement

### API8 — Security Misconfiguration

```
Exposed: GET /api/debug/config → returns DB credentials
Exposed: GET /actuator/env    → Spring Boot env vars
Missing: CORS allowing *
Missing: TLS on internal APIs
```

## Rate Limiting

```go
// Per-IP rate limiting with token bucket
limiter := rate.NewLimiter(rate.Every(time.Second), 10)  // 10 req/s

func middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip := r.RemoteAddr
        if !getLimiter(ip).Allow() {
            http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

## Input Validation Checklist

```
✓ Validate content-type matches declared type
✓ Max request body size enforced (e.g. 10MB)
✓ Integer overflow — validate ranges not just types
✓ Array/slice length limits — prevent memory exhaustion
✓ String length limits — prevent column overflow in DB
✓ Regex patterns anchored (^ and $) — prevent ReDoS
✓ File upload: validate MIME by content, not extension
✓ JSON: reject unknown fields (strict mode)
```

## Security Headers for APIs

```http
Content-Type: application/json
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Strict-Transport-Security: max-age=31536000; includeSubDomains
Cache-Control: no-store          # for sensitive responses
Content-Security-Policy: default-src 'none'
```

## GraphQL Specific

```graphql
# ❌ Deeply nested query = DoS
query {
  user {
    friends {
      friends {
        friends { ... }  # exponential cost
      }
    }
  }
}
```

**Mitigations:**
- Query depth limit (max 5-7 levels)
- Query complexity analysis and budget
- Disable introspection in production
- Persisted queries only (allowlist known queries)
- Field-level authorization on resolvers

## Scanner Coverage

| Risk | Scanner | Signal |
|------|---------|--------|
| BOLA patterns | semgrep | Missing ownership check |
| JWT misconfig | semgrep | `alg: none`, weak secrets |
| Missing rate limit | semgrep | Route handlers without middleware |
| Debug endpoints | semgrep | `/debug`, `/actuator`, `/metrics` exposed |
