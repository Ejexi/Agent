---
name: go-frameworks
description: Security patterns for Go web frameworks — Gin, Echo, Fiber, Chi — middleware, input validation, SQL, template injection
---

# Go Web Framework Security

## Common Patterns Across Frameworks

### SQL Injection (database/sql)

```go
// ❌ String interpolation — injectable
db.Query("SELECT * FROM users WHERE name = '" + name + "'")
db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = %s", id))

// ✅ Parameterised queries — always
db.Query("SELECT * FROM users WHERE name = $1", name)   // PostgreSQL
db.Query("SELECT * FROM users WHERE name = ?", name)    // MySQL/SQLite

// ✅ With sqlx named queries
db.NamedQuery(`SELECT * FROM users WHERE name = :name`, map[string]interface{}{
    "name": name,
})
```

### Template Injection

```go
// ❌ text/template does NOT auto-escape HTML
import "text/template"
t.Execute(w, userInput)   // XSS if rendered in browser

// ✅ html/template auto-escapes
import "html/template"
t.Execute(w, userInput)   // safe

// ❌ Marking user data as safe
import "html/template"
template.HTML(userInput)  // bypasses escaping — never do this with untrusted data
template.JS(userInput)
template.URL(userInput)
```

## Gin

```go
import "github.com/gin-gonic/gin"

// ❌ Binding without validation tags
type Input struct {
    Email string `json:"email"`
    Age   int    `json:"age"`
}

// ✅ Validate + bind — rejects missing/invalid fields
type Input struct {
    Email string `json:"email" binding:"required,email"`
    Age   int    `json:"age"  binding:"required,min=0,max=150"`
}

func handler(c *gin.Context) {
    var input Input
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
}

// ❌ Path traversal via c.Param
filePath := c.Param("filename")
data, _ := os.ReadFile("/uploads/" + filePath)  // ../../etc/passwd

// ✅ Validate filename before use
import "path/filepath"
filename := filepath.Base(c.Param("filename"))  // strips ../
if strings.ContainsAny(filename, "/\\") {
    c.AbortWithStatus(400)
    return
}
fullPath := filepath.Join("/uploads", filename)
// Also verify fullPath starts with /uploads after Join

// Security middleware
r := gin.New()
r.Use(gin.Recovery())        // recover from panics
r.Use(rateLimitMiddleware()) // add rate limiting
r.Use(corsMiddleware())      // configure CORS

// CORS — don't use AllowAllOrigins in production
config := cors.DefaultConfig()
config.AllowOrigins = []string{"https://app.example.com"}
config.AllowCredentials = true
r.Use(cors.New(config))
```

## Echo

```go
import "github.com/labstack/echo/v4"
import "github.com/labstack/echo/v4/middleware"

e := echo.New()

// Security middleware stack
e.Use(middleware.Recover())
e.Use(middleware.Secure())           // sets security headers
e.Use(middleware.RateLimiter(       // rate limiting
    middleware.NewRateLimiterMemoryStore(20),
))

// ❌ Echo's Bind doesn't validate by default
var user User
e.Bind(c, &user)   // binds but doesn't validate

// ✅ Validate after bind
if err := c.Validate(user); err != nil {
    return echo.NewHTTPError(400, err.Error())
}
```

## Fiber

```go
import "github.com/gofiber/fiber/v2"
import "github.com/gofiber/fiber/v2/middleware/limiter"
import "github.com/gofiber/helmet/v2"

app := fiber.New(fiber.Config{
    // ❌ Default: discloses framework version in Server header
    // ✅ Suppress framework info
    ServerHeader: "",
    AppName:      "",
})

// Security headers via helmet
app.Use(helmet.New())

// Rate limiting
app.Use(limiter.New(limiter.Config{
    Max:        20,
    Expiration: 30 * time.Second,
}))

// ❌ c.Params("id") returns string — must validate
id := c.Params("id")
// convert and validate:
userID, err := strconv.ParseInt(id, 10, 64)
if err != nil || userID <= 0 {
    return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
}
```

## Chi

```go
import "github.com/go-chi/chi/v5"
import "github.com/go-chi/chi/v5/middleware"

r := chi.NewRouter()

// Middleware stack
r.Use(middleware.RealIP)
r.Use(middleware.Recoverer)
r.Use(middleware.Throttle(100))     // 100 concurrent requests
r.Use(middleware.Timeout(30 * time.Second))

// ❌ chi.URLParam not validated
id := chi.URLParam(r, "id")

// ✅ validate before use
```

## Universal Security Checklist for Go APIs

```
✓ Use parameterised queries — never fmt.Sprintf into SQL
✓ Use html/template for HTML — never text/template
✓ Validate all input with binding tags
✓ Rate limit all public endpoints
✓ Add security headers (helmet or manual)
✓ Validate file paths with filepath.Base + prefix check
✓ Use crypto/rand for tokens — never math/rand
✓ Minimum TLS 1.2 in tls.Config
✓ No InsecureSkipVerify in production TLS config
✓ Log errors server-side, return generic messages to client
```
