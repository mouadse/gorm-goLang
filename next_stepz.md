# Fitness Tracker - Next Steps and Milestones

## Current State (Verified from Codebase)

You already have a strong backend foundation:

- GORM models and relations for users, exercises, workouts, workout exercises, workout sets, meals, and weight entries.
- Full CRUD API handlers with nested routes and OpenAPI docs.
- DB connection + auto-migrations + seed data + automated tests.

The main blocker before this can be treated as a real product backend is **auth + user data protection**.

## Next Step (Do This First)

### Milestone 1 - Authentication and Access Control (MVP blocker)

Goal: make the API usable by real users safely.

Deliverables:

1. Add auth endpoints:
   - `POST /v1/auth/register`
   - `POST /v1/auth/login`
   - optional: `POST /v1/auth/refresh`
2. Replace placeholder password behavior with proper hashing (`bcrypt`/`argon2`).
3. Add JWT-based auth middleware.
4. Enforce ownership checks on user-scoped resources (workouts, meals, weight entries).
5. Add tests for:
   - unauthorized access
   - wrong-user access denied
   - successful login/register flow

Definition of done:

- Protected routes require valid token.
- User A cannot read or mutate User B data.
- Auth tests pass in CI.

## MVP Milestones After Auth

### Milestone 2 - API Hardening and Developer Experience

Goal: make the API production-ready for clients.

Deliverables:

1. Pagination for list endpoints (`limit`, `offset` or cursor).
2. Consistent error contract (typed error codes + messages).
3. Request/response logging middleware + request ID.
4. CORS and security headers.
5. Config cleanup (`env` validation at startup).

Definition of done:

- Large list endpoints do not return unbounded results.
- All errors follow one response schema.
- Logs are traceable per request.

### Milestone 3 - Product Features for Fitness Value

Goal: deliver features users feel immediately.

Deliverables:

1. Workout templates/routines:
   - create template
   - apply template to generate workout
2. Progress insights endpoints:
   - weight trend (7d/30d)
   - workout volume trend per exercise
3. Personal records endpoint (best set per exercise).

Definition of done:

- Client can build dashboard charts from backend endpoints.
- User can repeat routines without re-entering all sets manually.

### Milestone 4 - Client Integration Readiness

Goal: smooth handoff to frontend/mobile app.

Deliverables:

1. API versioning strategy documented (`/v1` evolution rules).
2. Contract tests for critical endpoints.
3. Seed scenarios for frontend demos.
4. Postman/Bruno collection generated from OpenAPI.

Definition of done:

- Frontend team can integrate with stable contracts and test data.

## Suggested Timeline (Fast Path)

- Week 1: Milestone 1 (Auth + ownership checks)
- Week 2: Milestone 2 (hardening + pagination + middleware)
- Week 3: Milestone 3 (templates + analytics endpoints)
- Week 4: Milestone 4 (client integration and contract stability)

## Priority Order Summary

1. Auth and authorization (must-have now)
2. API hardening/pagination (must-have before scale)
3. Fitness insights and templates (high user value)
4. Integration tooling and contract stability (team velocity)
