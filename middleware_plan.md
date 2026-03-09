# Middleware Plan

## What the Hell Is Middleware?

Picture a nightclub. Before you reach the dance floor (your handler), you pass through a series of people at the door:

1. **The Bouncer** checks your ID (authentication)
2. **The Coat Check** takes your jacket and gives you a ticket (request transformation)
3. **The Stamp Guy** marks your wrist so they know you're allowed in (context injection)

On your way **out**, you pass through them again in reverse:

4. **The Coat Check** gives your jacket back (response transformation)
5. **The Bouncer** logs that you left (logging)

That's middleware. It's code that runs **before** and/or **after** your actual handler, forming a chain. Every HTTP request passes through each layer of middleware to reach your handler, and the response passes back through those same layers on the way out.

```
Request
  │
  ▼
┌──────────────────┐
│  Logging          │  ← starts timer
│  ┌──────────────┐ │
│  │ Recovery      │ │  ← sets up panic catcher
│  │ ┌──────────┐  │ │
│  │ │ CORS      │  │ │  ← adds headers
│  │ │ ┌──────┐  │  │ │
│  │ │ │ Auth  │  │  │ │  ← checks token (can REJECT here)
│  │ │ │ ┌──┐ │  │  │ │
│  │ │ │ │  │ │  │  │ │  ← YOUR HANDLER (the actual work)
│  │ │ │ └──┘ │  │  │ │
│  │ │ └──────┘  │  │ │
│  │ └──────────┘  │ │
│  └──────────────┘ │
└──────────────────┘
  │
  ▼
Response
```

This is called the **onion model** — each middleware is a layer of the onion wrapping your core logic.

### Why Not Just Put All That Logic in Every Handler?

You could. Here's what it would look like:

```go
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
    // CORS headers (copy-pasted into EVERY handler)
    w.Header().Set("Access-Control-Allow-Origin", "*")

    // Logging (copy-pasted into EVERY handler)
    start := time.Now()
    defer func() {
        log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
    }()

    // Panic recovery (copy-pasted into EVERY handler)
    defer func() {
        if err := recover(); err != nil {
            log.Printf("panic: %v", err)
            http.Error(w, "Internal Server Error", 500)
        }
    }()

    // Auth check (copy-pasted into EVERY handler)
    token := r.Header.Get("Authorization")
    if !isValidToken(token) {
        writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
        return
    }

    // NOW the actual business logic starts, 25 lines later...
    var req createUserRequest
    if err := decodeJSON(r, &req); err != nil {
        writeError(w, http.StatusBadRequest, err)
        return
    }
    // ...
}
```

That's 25 lines of boilerplate **in every single handler**. You have 49 routes. That's 1,225 lines of copy-pasted code. Change the CORS policy? Edit 49 files. Add a new header? 49 files. Miss one? Bug.

Middleware solves this by extracting these **cross-cutting concerns** — things that apply to many routes but aren't part of the business logic — into reusable, composable layers.

---

## How Middleware Works in Go

Go's entire HTTP system is built on one interface:

```go
type Handler interface {
    ServeHTTP(http.ResponseWriter, *http.Request)
}
```

An `http.ServeMux` implements this interface. Each handler function implements this interface (via `http.HandlerFunc`). That means **anything that takes an `http.Handler` can also take a mux, a single handler, or another middleware**.

### The Core Pattern

A middleware in Go is a function that takes a handler and returns a new handler:

```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Code that runs BEFORE the handler
        fmt.Println("before")

        next.ServeHTTP(w, r)  // Call the actual handler (or next middleware)

        // Code that runs AFTER the handler
        fmt.Println("after")
    })
}
```

The signature is always: **`func(http.Handler) http.Handler`**

That's it. That's the whole concept. Everything else is just applications of this pattern.

### Breaking It Down

1. `next http.Handler` — the thing you're wrapping (could be your handler, could be another middleware)
2. `http.HandlerFunc(func(...) {...})` — creates a new handler from a closure
3. Inside the closure, you do your pre-work, call `next.ServeHTTP(w, r)`, then do your post-work
4. If you want to **stop the chain** (reject the request), just `return` without calling `next.ServeHTTP`

---

## Five Essential Middleware (with Real Code)

### 1. Logging — Know What's Happening

The most important middleware. Logs every request with method, path, status code, and duration.

**Problem**: Go's `http.ResponseWriter` doesn't let you read back the status code after it's been written. So we need a small wrapper.

```go
// api/middleware.go

package api

import (
    "log"
    "net/http"
    "time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
    http.ResponseWriter
    status      int
    wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
    return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
    if rw.wroteHeader {
        return
    }
    rw.status = code
    rw.wroteHeader = true
    rw.ResponseWriter.WriteHeader(code)
}

// Logging logs every request's method, path, status, and duration.
func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        wrapped := wrapResponseWriter(w)

        next.ServeHTTP(wrapped, r)

        log.Printf(
            "%s %s %d %s",
            r.Method,
            r.URL.Path,
            wrapped.status,
            time.Since(start),
        )
    })
}
```

**What you'll see in your terminal:**
```
2026/03/09 21:30:00 POST /v1/users 201 2.3ms
2026/03/09 21:30:01 GET /v1/workouts 200 1.1ms
2026/03/09 21:30:02 GET /v1/users/bad-uuid 400 0.2ms
```

### 2. Recovery — Don't Let One Panic Kill the Server

If any handler panics (nil pointer, index out of range, etc.), the entire server crashes. Recovery catches the panic, logs it, and returns a 500 error instead.

```go
// Recovery catches panics in handlers and returns 500 instead of crashing.
func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("PANIC %s %s: %v", r.Method, r.URL.Path, err)
                writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

**Why this matters**: Without this, a single nil pointer dereference in one handler takes down your entire API. With it, only that one request gets a 500 and everything else keeps working.

### 3. CORS — Let Browsers Call Your API

When a frontend app (React, mobile web view, etc.) running on `localhost:3000` tries to call your API on `localhost:8080`, the browser blocks it by default. CORS headers tell the browser "it's OK, let them through."

```go
// CORS adds Cross-Origin Resource Sharing headers to every response.
func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        // Preflight requests: browser sends OPTIONS before the real request
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return  // Stop the chain — don't call the handler
        }

        next.ServeHTTP(w, r)
    })
}
```

**Notice**: For OPTIONS preflight requests, we `return` without calling `next.ServeHTTP`. This is a middleware **rejecting** a request — it never reaches your handler.

### 4. Request ID — Trace Requests Across Logs

Assigns a unique ID to every request and adds it to the response headers and request context. When something goes wrong, you can grep your logs for that ID and see exactly what happened.

```go
import (
    "context"
    "github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID injects a unique ID into each request's context and response header.
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := uuid.New().String()

        // Add to response header so the client can reference it
        w.Header().Set("X-Request-ID", id)

        // Add to request context so handlers/other middleware can access it
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// GetRequestID retrieves the request ID from context (for use in handlers).
func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(requestIDKey).(string); ok {
        return id
    }
    return ""
}
```

### 5. Authentication — Protect Your Routes

Checks for a valid token before allowing access. This is the middleware that makes your `placeholderPasswordHash = "pending-auth"` actually mean something.

```go
// Authenticate checks for a valid Bearer token.
// Returns 401 Unauthorized if missing or invalid.
func Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            writeError(w, http.StatusUnauthorized, errors.New("missing authorization header"))
            return  // Stop the chain
        }

        // Expect "Bearer <token>"
        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
            writeError(w, http.StatusUnauthorized, errors.New("invalid authorization format"))
            return
        }

        token := parts[1]

        // TODO: Replace with real JWT validation
        userID, err := validateToken(token)
        if err != nil {
            writeError(w, http.StatusUnauthorized, errors.New("invalid or expired token"))
            return
        }

        // Inject the authenticated user's ID into the request context
        ctx := context.WithValue(r.Context(), "user_id", userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## Chaining Middleware

You can nest middleware manually:

```go
handler := Logging(Recovery(CORS(RequestID(mux))))
```

This reads inside-out: RequestID runs first, then CORS, then Recovery, then Logging. It's ugly to read when you have 5+ middlewares.

### A Cleaner Chain Helper

```go
// Chain composes middleware left-to-right:
// Chain(A, B, C)(handler) == A(B(C(handler)))
// The first middleware in the list is the outermost (runs first).
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            next = middlewares[i](next)
        }
        return next
    }
}
```

Now you can write:

```go
// Reads naturally: Logging runs first, then Recovery, then CORS, then RequestID
stack := Chain(Logging, Recovery, CORS, RequestID)
handler := stack(mux)
```

### Global vs Route-Specific Middleware

**Global** middleware wraps the entire mux — runs on every request, even 404s:

```go
// In main.go
http.ListenAndServe(addr, Logging(Recovery(CORS(server.Handler()))))
```

**Route-specific** middleware wraps individual handlers — runs only on matched routes:

```go
// In registerRoutes()
s.mux.Handle("DELETE /v1/users/{id}", Authenticate(http.HandlerFunc(s.handleDeleteUser)))
```

You can combine both:

```go
// Global: logging, recovery, CORS (everyone gets these)
// Route-specific: auth (only protected routes get this)
```

---

## How to Implement This in YOUR Codebase

Here's the exact plan, file by file, line by line.

### Step 1: Create `api/middleware.go`

This is a new file. Put all middleware functions here.

```go
package api

import (
    "context"
    "errors"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"
)

// ---------- Response writer wrapper ----------

type responseWriter struct {
    http.ResponseWriter
    status      int
    wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
    return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
    if rw.wroteHeader {
        return
    }
    rw.status = code
    rw.wroteHeader = true
    rw.ResponseWriter.WriteHeader(code)
}

// ---------- Global middleware ----------

// Logging logs method, path, status code, and duration for every request.
func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        wrapped := wrapResponseWriter(w)
        next.ServeHTTP(wrapped, r)
        log.Printf("%s %s %d %s", r.Method, r.URL.Path, wrapped.status, time.Since(start))
    })
}

// Recovery catches panics so one bad request doesn't crash the server.
func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("PANIC %s %s: %v", r.Method, r.URL.Path, err)
                writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
            }
        }()
        next.ServeHTTP(w, r)
    })
}

// CORS adds cross-origin headers and handles preflight OPTIONS requests.
func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// ---------- Request ID ----------

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID injects a unique ID into each request's context and X-Request-ID header.
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := uuid.New().String()
        w.Header().Set("X-Request-ID", id)
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(requestIDKey).(string); ok {
        return id
    }
    return ""
}

// ---------- Authentication ----------

// Authenticate checks for a valid Bearer token in the Authorization header.
func Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            writeError(w, http.StatusUnauthorized, errors.New("missing authorization header"))
            return
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || parts[0] != "Bearer" {
            writeError(w, http.StatusUnauthorized, errors.New("invalid authorization format"))
            return
        }

        // TODO: Replace with real JWT validation once auth is implemented.
        // For now this is a placeholder that demonstrates the middleware pattern.
        token := parts[1]
        _ = token

        next.ServeHTTP(w, r)
    })
}

// ---------- Chain helper ----------

// Chain composes middleware left-to-right.
// Chain(A, B, C)(handler) means A runs first, then B, then C, then the handler.
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        for i := len(middlewares) - 1; i >= 0; i-- {
            next = middlewares[i](next)
        }
        return next
    }
}
```

### Step 2: Update `api/server.go` — Wrap the Handler

Change the `Handler()` method to apply the global middleware chain.

**Current code** (line 28-30):
```go
func (s *Server) Handler() http.Handler {
    return s.mux
}
```

**New code:**
```go
func (s *Server) Handler() http.Handler {
    stack := Chain(Logging, Recovery, CORS, RequestID)
    return stack(s.mux)
}
```

That's it. One line change. Every request now gets logging, panic recovery, CORS headers, and a request ID — without touching a single handler.

### Step 3 (Later — After Auth Is Built): Add Route-Specific Auth

Once you implement JWT authentication, protect specific routes in `registerRoutes()`:

**Current code:**
```go
s.mux.HandleFunc("DELETE /v1/users/{id}", s.handleDeleteUser)
```

**New code:**
```go
s.mux.Handle("DELETE /v1/users/{id}", Authenticate(http.HandlerFunc(s.handleDeleteUser)))
```

Note the change from `HandleFunc` to `Handle` — because `Authenticate(...)` returns an `http.Handler`, not a bare function.

You'd apply this to all routes that require authentication while leaving public routes (like `POST /v1/users` for registration, `GET /healthz`) unwrapped.

### Insertion Point Summary

| What | Where | Line |
|---|---|---|
| New file | `api/middleware.go` | (create) |
| Global middleware chain | `api/server.go` → `Handler()` | 28-30 |
| Route-specific auth (later) | `api/server.go` → `registerRoutes()` | 39-95 |
| No changes needed | `main.go` | — |
| No changes needed | Any handler file | — |

---

## Execution Order Matters

The order you list middleware in `Chain()` determines execution order:

```go
stack := Chain(Logging, Recovery, CORS, RequestID)
```

This means:

| Order | Middleware | Why This Position |
|---|---|---|
| 1st | **Logging** | Outermost — logs everything, including panics and CORS preflight |
| 2nd | **Recovery** | Catches panics from CORS, RequestID, and all handlers |
| 3rd | **CORS** | Runs before auth — preflight OPTIONS must succeed without a token |
| 4th | **RequestID** | Innermost global — ID is available to all handlers |

If you put Recovery before Logging, you'd lose the ability to log panic responses. If you put CORS after Auth, preflight requests would get 401 errors and your frontend couldn't talk to the API.

---

## What Your Request Flow Looks Like After This

**Before (current):**
```
Client → http.ListenAndServe → mux → handleCreateUser
```

**After (with middleware):**
```
Client → http.ListenAndServe → Logging → Recovery → CORS → RequestID → mux → handleCreateUser
                                                                         ↓
                                                              (later) Authenticate → handleDeleteUser
```

---

## Testing Middleware

Your existing tests in `api/server_test.go` use `newTestServer()` which calls `NewServer(db)`. Since middleware is applied in `Handler()`, your tests automatically get middleware coverage when using `httptest.NewServer(server.Handler())`.

To test middleware in isolation:

```go
func TestLoggingMiddleware(t *testing.T) {
    // Create a dummy handler that returns 200
    inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    // Wrap it with the Logging middleware
    handler := Logging(inner)

    // Make a test request
    req := httptest.NewRequest("GET", "/test", nil)
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", rec.Code)
    }
}

func TestRecoveryMiddleware(t *testing.T) {
    // Create a handler that panics
    inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        panic("something went wrong")
    })

    handler := Recovery(inner)

    req := httptest.NewRequest("GET", "/panic", nil)
    rec := httptest.NewRecorder()

    // Should NOT panic — Recovery catches it
    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusInternalServerError {
        t.Errorf("expected 500, got %d", rec.Code)
    }
}

func TestCORSPreflight(t *testing.T) {
    inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Error("handler should not be called for OPTIONS")
    })

    handler := CORS(inner)

    req := httptest.NewRequest("OPTIONS", "/v1/users", nil)
    rec := httptest.NewRecorder()

    handler.ServeHTTP(rec, req)

    if rec.Code != http.StatusNoContent {
        t.Errorf("expected 204, got %d", rec.Code)
    }
    if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
        t.Error("missing CORS origin header")
    }
}
```

---

## Summary

| Concept | One-Liner |
|---|---|
| What is middleware | Code that wraps handlers to run before/after the business logic |
| Why it exists | Eliminates copy-paste of cross-cutting concerns across handlers |
| Go pattern | `func(http.Handler) http.Handler` — takes a handler, returns a handler |
| Chain them | `Chain(A, B, C)(handler)` — A runs first, C runs last |
| Global | Wrap the mux — runs on every request |
| Route-specific | Wrap individual handlers — runs only on matched routes |
| Your codebase | Create `api/middleware.go`, change 3 lines in `server.go Handler()` |

### Implementation Priority

1. **Now**: Logging + Recovery + CORS + RequestID (global, zero-risk)
2. **With auth milestone**: Authenticate (route-specific, after JWT is built)
3. **Later**: Rate limiting, request body size limits, response compression
