# OAuth vs Bcrypt - Corrected Analysis for This Repo

## Verified Current State

This repo currently has:

| Area | Current state |
|---|---|
| HTTP layer | Go `net/http` with `ServeMux` |
| Data layer | GORM + PostgreSQL |
| User model | `models.User` has required `PasswordHash string` |
| User creation | `POST /v1/users` accepts `password_hash` from the client |
| Fallback behavior | Empty `password_hash` becomes `"pending-auth"` |
| Auth routes | None |
| JWT/session middleware | None |
| Authorization checks | None |

That means the API is still effectively unauthenticated for real product use. The current `password_hash` handling is only a placeholder to satisfy the schema.

## The Main Correction

`bcrypt` and `OAuth` are not interchangeable choices.

| Topic | bcrypt | OAuth 2.0 / OIDC |
|---|---|---|
| What it is | Password hashing algorithm | Login delegation protocol |
| What problem it solves | Safely storing local passwords | Letting users sign in via Google/GitHub/etc. |
| Where it fits here | Email/password auth on your own API | Optional later login method |

Related but separate:

- `JWT` is not OAuth.
- `JWT` is just the token format your API can issue after login.
- You can issue JWTs after local password login, after OAuth login, or both.

## Recommendation For The Next Step

For this codebase, the right next step is:

**Build local auth first: password hashing + login + JWT + ownership checks.**

Do not start with OAuth yet.

Why this is the right sequence:

1. Your current schema already expects a local password hash.
2. Your existing project plan in `next_stepz.md` already points to JWT-based auth as Milestone 1.
3. This is an API-first backend; local auth is the shortest path to making it safely usable.
4. OAuth adds provider setup, redirect handling, PKCE/state handling, callback URLs, and more moving parts.
5. You still need your own authorization rules even if OAuth is added later.

## What Was Wrong Or Misleading In The Previous Draft

The earlier version had a few issues that would make the next implementation step noisier or riskier than it needs to be:

1. It spent most of the document on Google OAuth even though the repo's real next step is local auth.
2. It suggested making `PasswordHash` nullable immediately. That is not needed for the next milestone and would force unnecessary schema/test changes now.
3. It treated `POST /v1/users` as registration without calling out that accepting `password_hash` from clients is the wrong long-term contract.
4. It proposed OAuth callback code with a hard-coded `state` string. That is not safe enough for a real OAuth flow.
5. It referenced files and config that do not currently exist in the repo, such as `.env.example`.
6. It skipped important repo-specific follow-up work: updating tests, seed data expectations, OpenAPI docs, and route protection boundaries.

## Immediate Plan You Can Implement Next

### Decision

Use this auth model for the next milestone:

- Registration with email + password
- Password hashing on the server with `bcrypt`
- Login endpoint that verifies password
- JWT access token issued by your API
- Middleware that reads `Authorization: Bearer <token>`
- Authorization checks so a user can only access their own data

### Scope For Milestone 1

#### 1. Add dedicated auth endpoints

Add:

- `POST /v1/auth/register`
- `POST /v1/auth/login`

Optional later:

- `POST /v1/auth/refresh`

Keep the first milestone small. `register` and `login` are enough.

#### 2. Stop accepting `password_hash` from API clients

This is important.

Clients should send:

```json
{
  "email": "alex@example.com",
  "password": "plain-text-password",
  "name": "Alex Johnson"
}
```

The server should:

1. validate the request
2. hash `password` with bcrypt
3. store only the hash in `users.password_hash`

The API should not trust a client-supplied `password_hash`.

#### 3. Keep the current `User` schema for now

For the next step, do **not** change `models.User` for OAuth yet.

Current phase:

- keep `PasswordHash` required
- keep local accounts only
- avoid `AuthProvider` / `ProviderID` until social login is actually being added

This keeps the change set smaller and aligned with current tests and seed data.

#### 4. Add JWT utilities

Add a small auth utility layer that can:

- hash a password
- compare a password against a stored hash
- issue a JWT with at least `sub`, `iat`, and `exp`
- validate and parse a JWT

Suggested packages:

- `golang.org/x/crypto/bcrypt`
- `github.com/golang-jwt/jwt/v5`

Note: `golang.org/x/crypto` is already present indirectly in `go.mod`, but `github.com/golang-jwt/jwt/v5` still needs to be added.

#### 5. Add auth middleware

Add middleware that:

- reads the `Authorization` header
- expects `Bearer <token>`
- validates the JWT
- parses the user ID from the `sub` claim
- stores the authenticated user ID in request context

That middleware should protect all user-owned routes.

#### 6. Add ownership checks

Authentication alone is not enough.

You also need authorization checks such as:

- User A cannot fetch User B with `GET /v1/users/{id}`
- User A cannot create or list workouts for another `user_id`
- User A cannot modify or delete another user's meals or weight entries

For nested routes, the authenticated user ID must match the path user ID or the owner of the fetched resource.

## Repo-Specific File Plan

These are the files that should change for the next step.

| File | What to change |
|---|---|
| `api/auth.go` (new) | Register/login handlers, bcrypt helpers, JWT creation/validation |
| `api/middleware.go` (new) | Auth middleware and request-context helpers |
| `api/server.go` | Register auth routes and wrap protected routes |
| `api/user_handlers.go` | Remove client-facing `password_hash` contract from public registration flow or stop using this route for registration |
| `api/openapi.yaml` | Document auth endpoints and bearer auth |
| `README.md` | Add auth setup and `JWT_SECRET` config |
| `test_api.py` | Update/create auth flow and authorization tests |
| `test_db.py` | Update assumptions around placeholder password behavior if needed |
| `seed/main.go` | Optional: keep fake hashes for seed users or switch to bcrypt hashes for more realistic dev login |

## Recommended Route Boundary

A practical first cut is:

### Public

- `GET /healthz`
- `GET /docs`
- `GET /docs/`
- `GET /openapi.yaml`
- `POST /v1/auth/register`
- `POST /v1/auth/login`

### Protected

- all `GET/PATCH/DELETE /v1/users/{id}` routes
- user-owned workout routes
- user-owned meal routes
- user-owned weight-entry routes

You can decide later whether read-only exercise endpoints stay public or become protected. That is a product choice, not an auth-design blocker.

## Recommended Auth Flow

### Register

`POST /v1/auth/register`

Request:

```json
{
  "email": "alex@example.com",
  "password": "supersecret123",
  "name": "Alex Johnson"
}
```

Server behavior:

1. validate fields
2. ensure email uniqueness
3. bcrypt-hash password
4. create user
5. optionally issue JWT immediately

### Login

`POST /v1/auth/login`

Request:

```json
{
  "email": "alex@example.com",
  "password": "supersecret123"
}
```

Server behavior:

1. find user by email
2. compare bcrypt hash
3. issue JWT

Response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

## Minimal JWT Claims

For this project, keep claims minimal:

```json
{
  "sub": "user-uuid",
  "iat": 1710000000,
  "exp": 1710086400
}
```

That is enough for the first milestone.

## Test Plan For This Milestone

Add or update tests for:

1. successful register
2. duplicate email rejected
3. successful login
4. wrong password rejected
5. missing token rejected
6. malformed token rejected
7. user cannot access another user's resource
8. authenticated user can access their own resource

If you skip these tests, the auth change will look finished while still leaving the API unsafe.

## What OAuth Should Look Like Later

OAuth still makes sense later, just not as the first auth milestone.

When you add it later:

1. keep your own JWT/session layer
2. add provider-specific login endpoints
3. store provider identity on the user model
4. issue your own JWT after provider login succeeds

At that point you would likely add fields such as:

- `AuthProvider`
- `ProviderID`

and possibly make `PasswordHash` optional for provider-only accounts.

## Important OAuth Caveats For Later

If and when you add Google login, do not copy the old draft blindly.

A production-worthy OAuth/OIDC implementation needs at least:

- per-request `state`, not a fixed string
- PKCE for browser/mobile clients
- proper ID token validation for OIDC
- clear account-linking rules when the same email already exists locally

So the correct plan is not "OAuth instead of bcrypt."

The correct long-term plan is:

- local auth first
- OAuth as an additional login method later

## Final Recommendation

Use this as the next implementation sequence:

1. Add `POST /v1/auth/register`
2. Add `POST /v1/auth/login`
3. Hash passwords with bcrypt on the server
4. Issue JWTs
5. Protect user-owned routes with middleware
6. Enforce ownership checks
7. Update tests and OpenAPI docs

Only after that is stable should you add Google OAuth.

That is the cleanest path for this repo and the safest path for the app.
