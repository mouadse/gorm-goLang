# OAuth vs Bcrypt — Analysis for Your Fitness Tracker API

## Your Codebase Today

Your fitness-tracker is a Go REST API with:

| Layer | Stack |
|---|---|
| Framework | `net/http` ServeMux (Go 1.25) |
| ORM | GORM v1.31 + PostgreSQL |
| Models | User, Workout, Exercise, Meal, WeightEntry |
| Auth | **None** — [PasswordHash](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) field stores `"pending-auth"` placeholder |
| Middleware | Planned but not yet implemented (see [middleware_plan.md](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/middleware_plan.md)) |

The [User](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/models/user.go#11-33) model in [user.go](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/models/user.go) already has a [PasswordHash](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) field, and [handleCreateUser](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#44-95) in [user_handlers.go](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#L289-L295) uses a [defaultPasswordHash()](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) helper that falls back to `"pending-auth"`. **Your API currently has zero authentication — anyone can read/write any user's data.**

---

## The Key Misunderstanding: OAuth and Bcrypt Are Not Alternatives

> [!IMPORTANT]
> **OAuth and bcrypt solve completely different problems.** You don't choose one over the other — you can use both, either, or neither.

| | Bcrypt | OAuth 2.0 |
|---|---|---|
| **What it is** | A password hashing algorithm | An authorization framework/protocol |
| **What it does** | Turns `"mypassword123"` into an irreversible hash stored in your DB | Lets users log in via Google/GitHub/etc. without giving you their password |
| **Problem it solves** | "How do I safely store passwords?" | "How do I let users sign in without managing passwords myself?" |
| **Analogy** | A safe where you lock the house keys | A valet key that has limited access |

### When You Say "OAuth for auth," You Probably Mean One of Two Things

1. **OAuth 2.0 + OpenID Connect (OIDC)** — "Sign in with Google" button. The user authenticates with Google, Google tells you who they are, you create a session. **You never see their password.**

2. **Your own JWT-based auth** — The user registers with email/password on YOUR system, you hash the password with bcrypt, verify it on login, and issue a JWT token. **This is NOT OAuth**, even though it uses tokens.

---

## Comparison for YOUR Fitness Tracker

### Option A: Bcrypt + JWT (Traditional Self-Hosted Auth)

The user creates an account with email + password on your API.

```
┌─────────┐     POST /auth/register      ┌──────────┐
│  Client  │ ─── {email, password} ────→  │ Your API │
└─────────┘                               └──────────┘
                                               │
                                     bcrypt.Hash(password)
                                               │
                                               ▼
                                          ┌─────────┐
                                          │ Postgres │  (stores hash)
                                          └─────────┘

┌─────────┐     POST /auth/login          ┌──────────┐
│  Client  │ ─── {email, password} ────→  │ Your API │
└─────────┘                               └──────────┘
                                               │
                                     bcrypt.Compare(hash, password)
                                               │
                                     if match → issue JWT
                                               │
                  ← { token: "eyJhb..." } ─────┘
```

**Pros:**
- ✅ Full control — you own the auth flow
- ✅ Simpler to implement (your [PasswordHash](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) field is already in the model)
- ✅ No third-party dependency for auth
- ✅ Works offline / in isolated environments
- ✅ Great for learning (you understand every piece)

**Cons:**
- ❌ You're responsible for password security (hashing, salting, reset flows)
- ❌ Users need to remember yet another password
- ❌ You must build: registration, login, password reset, email verification
- ❌ Liability — if your DB leaks, you have password hashes

---

### Option B: OAuth 2.0 / OIDC (Social Login)

The user clicks "Sign in with Google." Google handles the authentication.

```
┌─────────┐                        ┌────────┐                    ┌──────────┐
│  Client  │ ── "Sign in w/ Google" → │ Google │                    │ Your API │
└─────────┘                        └────────┘                    └──────────┘
     │                                  │                              │
     │  ← redirect to Google login ─────┘                              │
     │  → user enters Google password ──→                              │
     │  ← Google gives authorization code                              │
     │  → send code to your API ──────────────────────────────────────→│
     │                                                                 │
     │                     Your API exchanges code for tokens          │
     │                     with Google's token endpoint                 │
     │                                                                 │
     │                     Gets user info (email, name, avatar)        │
     │                     Creates/finds user in Postgres              │
     │                     Issues YOUR JWT                             │
     │                                                                 │
     │  ← { token: "eyJhb..." } ──────────────────────────────────────┘
```

**Pros:**
- ✅ No password management — Google handles it
- ✅ Higher security — you never touch passwords
- ✅ Better UX — one-click login, no registration form
- ✅ Users trust Google's security more than a random fitness app
- ✅ Free email verification (Google already verified their email)

**Cons:**
- ❌ More complex to implement (redirect flows, token exchange)
- ❌ Requires internet connection to authenticate
- ❌ Dependency on third-party provider (Google goes down = your auth goes down)
- ❌ Need OAuth app registration (Google Cloud Console, callback URLs)
- ❌ Doesn't work for API-only clients without a browser

---

### Option C: Both (Recommended for Production)

Most real apps offer both: create an account with email/password **OR** sign in with Google/GitHub/Apple. This gives users choice and avoids lock-in.

---

## My Recommendation for Your Project

> [!TIP]
> **Start with Option A (bcrypt + JWT), then add OAuth later.**

Here's why:

1. Your [User](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/models/user.go#11-33) model already has [PasswordHash](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) — it's designed for bcrypt
2. Your [next_stepz.md](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/next_stepz.md) already plans for this as Milestone 1
3. You're learning Go + GORM — building auth yourself teaches you more
4. OAuth adds significant complexity (redirect flows, provider config, callback URLs) that isn't necessary for an MVP
5. Your [middleware_plan.md](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/middleware_plan.md) Authenticate middleware already expects a JWT Bearer token pattern

**Add OAuth as a separate login method later (Milestone 3+) once the core auth works.**

---

## Step-by-Step: OAuth 2.0 Implementation Guide

If you still want to implement OAuth (e.g., "Sign in with Google"), here's **every step** you'd need:

### Step 1 — Register Your App with Google Cloud

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a project or select your existing one
3. Navigate to **APIs & Services → Credentials**
4. Create an **OAuth 2.0 Client ID** (type: Web Application)
5. Set the **Authorized redirect URI** to `http://localhost:8080/v1/auth/google/callback`
6. Save the **Client ID** and **Client Secret**

Add these to your [.env](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/.env):
```env
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/v1/auth/google/callback
JWT_SECRET=your-random-secret-key-here
```

---

### Step 2 — Install Dependencies

```bash
go get golang.org/x/oauth2
go get golang.org/x/oauth2/google
go get github.com/golang-jwt/jwt/v5
```

- `golang.org/x/oauth2` — Google's official OAuth2 library for Go
- `github.com/golang-jwt/jwt/v5` — JWT token creation/validation

---

### Step 3 — Update the User Model

Modify [user.go](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/models/user.go) to support OAuth users:

```diff
 type User struct {
     ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
     Email         string         `gorm:"type:varchar(255);uniqueIndex:..." json:"email"`
-    PasswordHash  string         `gorm:"type:varchar(255);not null" json:"-"`
+    PasswordHash  string         `gorm:"type:varchar(255)" json:"-"`
+    AuthProvider  string         `gorm:"type:varchar(50);default:'local'" json:"auth_provider"`
+    ProviderID    string         `gorm:"type:varchar(255)" json:"-"`
     Name          string         `gorm:"type:varchar(255);not null" json:"name"`
     Avatar        string         `gorm:"type:varchar(512)" json:"avatar"`
     // ... rest unchanged
 }
```

- [PasswordHash](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) becomes **nullable** (OAuth users don't have one)
- `AuthProvider` — `"local"` or `"google"`
- `ProviderID` — Google's unique user ID (sub claim)

---

### Step 4 — Create `api/auth.go` (New File)

This file handles the OAuth flow and JWT token issuance:

```go
package api

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "os"
    "time"

    "fitness-tracker/models"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
    "gorm.io/gorm"
)

// googleOAuthConfig builds the OAuth2 config from env vars.
func googleOAuthConfig() *oauth2.Config {
    return &oauth2.Config{
        ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
        ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
        RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
        Scopes:       []string{"openid", "email", "profile"},
        Endpoint:     google.Endpoint,
    }
}

// handleGoogleLogin redirects the user to Google's consent screen.
func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
    cfg := googleOAuthConfig()
    // "state" prevents CSRF — in production, use a random token stored in session
    url := cfg.AuthCodeURL("random-state-string", oauth2.AccessTypeOffline)
    http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// handleGoogleCallback handles the redirect FROM Google after user consents.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
    // 1. Verify state parameter (CSRF protection)
    state := r.URL.Query().Get("state")
    if state != "random-state-string" {
        writeError(w, http.StatusBadRequest, errors.New("invalid state parameter"))
        return
    }

    // 2. Exchange authorization code for tokens
    code := r.URL.Query().Get("code")
    cfg := googleOAuthConfig()
    token, err := cfg.Exchange(context.Background(), code)
    if err != nil {
        writeError(w, http.StatusBadRequest, fmt.Errorf("code exchange failed: %w", err))
        return
    }

    // 3. Use the token to get user info from Google
    client := cfg.Client(context.Background(), token)
    resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
    if err != nil {
        writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get user info: %w", err))
        return
    }
    defer resp.Body.Close()

    var googleUser struct {
        ID      string `json:"id"`
        Email   string `json:"email"`
        Name    string `json:"name"`
        Picture string `json:"picture"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
        writeError(w, http.StatusInternalServerError, err)
        return
    }

    // 4. Find or create user in YOUR database
    var user models.User
    err = s.db.Where("auth_provider = ? AND provider_id = ?", "google", googleUser.ID).
        First(&user).Error

    if errors.Is(err, gorm.ErrRecordNotFound) {
        // New user — create them
        user = models.User{
            Email:        googleUser.Email,
            Name:         googleUser.Name,
            Avatar:       googleUser.Picture,
            AuthProvider: "google",
            ProviderID:   googleUser.ID,
        }
        if err := s.db.Create(&user).Error; err != nil {
            writeError(w, http.StatusInternalServerError, err)
            return
        }
    } else if err != nil {
        writeError(w, http.StatusInternalServerError, err)
        return
    }

    // 5. Issue YOUR app's JWT token
    jwtToken, err := generateJWT(user.ID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err)
        return
    }

    writeJSON(w, http.StatusOK, map[string]any{
        "token": jwtToken,
        "user":  user,
    })
}

// generateJWT creates a signed JWT with the user's ID.
func generateJWT(userID uuid.UUID) (string, error) {
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        return "", errors.New("JWT_SECRET not set")
    }

    claims := jwt.MapClaims{
        "sub": userID.String(),
        "iat": time.Now().Unix(),
        "exp": time.Now().Add(24 * time.Hour).Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

// validateJWT parses and validates a JWT, returning the user ID.
func validateJWT(tokenString string) (uuid.UUID, error) {
    secret := os.Getenv("JWT_SECRET")

    token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return []byte(secret), nil
    })
    if err != nil {
        return uuid.Nil, err
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid {
        return uuid.Nil, errors.New("invalid token")
    }

    sub, ok := claims["sub"].(string)
    if !ok {
        return uuid.Nil, errors.New("missing sub claim")
    }

    return uuid.Parse(sub)
}
```

---

### Step 5 — Register the OAuth Routes

In [server.go](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/server.go), add to [registerRoutes()](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/server.go#32-97):

```diff
 func (s *Server) registerRoutes() {
     s.mux.HandleFunc("GET /healthz", s.handleHealth)
+
+    // OAuth routes (public — no auth required)
+    s.mux.HandleFunc("GET /v1/auth/google/login", s.handleGoogleLogin)
+    s.mux.HandleFunc("GET /v1/auth/google/callback", s.handleGoogleCallback)
 
     // Users
     s.mux.HandleFunc("POST /v1/users", s.handleCreateUser)
```

---

### Step 6 — Wire Up the Auth Middleware

Update the `Authenticate` middleware in your planned `api/middleware.go` to use the `validateJWT` function:

```go
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

        userID, err := validateJWT(parts[1])
        if err != nil {
            writeError(w, http.StatusUnauthorized, errors.New("invalid or expired token"))
            return
        }

        // Inject authenticated user ID into request context
        ctx := context.WithValue(r.Context(), "user_id", userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

### Step 7 — Protect Routes

In [registerRoutes()](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/server.go#32-97), wrap protected routes:

```go
// Public routes (no auth)
s.mux.HandleFunc("GET /healthz", s.handleHealth)
s.mux.HandleFunc("GET /v1/auth/google/login", s.handleGoogleLogin)
s.mux.HandleFunc("GET /v1/auth/google/callback", s.handleGoogleCallback)
s.mux.HandleFunc("POST /v1/users", s.handleCreateUser)  // registration

// Protected routes (require valid JWT)
s.mux.Handle("GET /v1/users/{id}", Authenticate(http.HandlerFunc(s.handleGetUser)))
s.mux.Handle("PATCH /v1/users/{id}", Authenticate(http.HandlerFunc(s.handleUpdateUser)))
s.mux.Handle("DELETE /v1/users/{id}", Authenticate(http.HandlerFunc(s.handleDeleteUser)))
s.mux.Handle("POST /v1/workouts", Authenticate(http.HandlerFunc(s.handleCreateWorkout)))
// ... etc for all user-scoped routes
```

---

## Summary: OAuth vs Bcrypt Decision Matrix

| Factor | Bcrypt + JWT | OAuth (Google) | Both |
|---|---|---|---|
| **Complexity** | Low | Medium-High | High |
| **Time to implement** | ~1 day | ~2-3 days | ~3-4 days |
| **Security** | Good (if done right) | Excellent (Google handles it) | Excellent |
| **User experience** | Standard form | One-click login | Best — user chooses |
| **Dependencies** | `golang.org/x/crypto` (already in go.sum) | `golang.org/x/oauth2` + provider setup | Both |
| **Offline capability** | Yes | No (needs Google) | Partial |
| **Best for** | Learning, MVP, API-first | Production apps with frontend | Production apps |

> [!NOTE]
> **Bottom line:** Start with bcrypt + JWT for your learning project. Add OAuth later as a second login method. They complement each other — they're not competitors.

---

## Files That Would Change

| File | Change | Why |
|---|---|---|
| [user.go](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/models/user.go) | Add `AuthProvider`, `ProviderID` fields; make [PasswordHash](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/user_handlers.go#289-296) nullable | Support both local and OAuth users |
| [server.go](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/api/server.go) | Add OAuth routes, protect existing routes with middleware | Wire up the auth flow |
| `api/auth.go` **[NEW]** | OAuth handlers, JWT generation/validation | Core auth logic |
| `api/middleware.go` **[NEW]** | `Authenticate` middleware using `validateJWT` | Protect routes |
| [.env.example](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/.env.example) | Add `GOOGLE_CLIENT_ID/SECRET`, `JWT_SECRET` | New config vars |
| [go.mod](file:///home/mouad/Work/tries/2026-03-04-mouadse-gorm-goLang/go.mod) | Add `golang.org/x/oauth2`, `github.com/golang-jwt/jwt/v5` | New dependencies |
