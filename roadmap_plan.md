# Backend Roadmap Plan

This roadmap focuses only on backend business logic and backend platform work. It excludes frontend-only work and defers AI-heavy features until the deterministic backend foundation is stable.

## Baseline

Use the current CRUD implementation as the stable base:

- `User`, JWT auth, and profile fields
- `Exercise` library CRUD
- `Workout`, `WorkoutExercise`, and `WorkoutSet` CRUD
- `Meal`, `Food`, and `MealFood` CRUD
- `WeightEntry` CRUD
- basic daily summary endpoint

Every roadmap step below should also update:

- `database/migrations.go`
- `api/server.go`
- `api/openapi.yaml`
- tests

## Phase 0: Architecture Hardening

### 1. Introduce a service layer

Create a dedicated business-logic layer before expanding features.

Implementation:

- add `services/` or `internal/services/`
- move analytics, nutrition target logic, integration rules, exports, and auth session logic out of handlers
- keep handlers thin: parse request, authorize, call service, serialize response

Why first:

- current handlers already do too much orchestration
- future analytics and nutrition logic will become hard to test if it stays inside HTTP handlers

Deliverables:

- initial service package layout
- clear separation between repository/query code and business rules
- unit-testable service functions

## Phase 1: Authentication Completion

### 2. Add refresh tokens and session management

Current auth stops at registration, login, and short-lived JWT issuance. It needs a real session model.

Schema:

- `refresh_tokens`
- `user_sessions`

Suggested fields:

- session id
- user id
- hashed refresh token
- user agent
- ip address or last_ip
- expires at
- revoked at
- created at
- updated at

Endpoints:

- `POST /v1/auth/refresh`
- `POST /v1/auth/logout`
- `GET /v1/auth/sessions`
- optionally `DELETE /v1/auth/sessions/{id}`

Business rules:

- rotate refresh tokens on every refresh
- revoke token chain on suspicious reuse
- allow logout of current session and all sessions
- keep access tokens stateless and short-lived

Tests:

- refresh success
- refresh with revoked token
- refresh after expiry
- logout invalidates token
- multiple sessions per user

## Phase 2: Workout Analytics Foundation

### 3. Add deterministic workout analytics services

Do not change the core workout schema yet. Derive useful signals from existing workout data first.

Business logic:

- previous workout comparison for the same exercise
- personal record detection
- estimated 1RM from top sets
- completed-set volume calculation
- workout volume totals
- weekly workout frequency
- workout-type frequency

Core formulas:

- per set volume = `reps * weight`
- per exercise session volume = sum of completed set volumes
- per workout volume = sum of exercise session volumes
- per week volume = sum of workout volumes
- estimated 1RM can use one transparent formula such as Epley

Deliverables:

- analytics service functions
- stable response DTOs for derived workout data

### 4. Expose workout insight read models

Once the service exists, expose read endpoints for actual product use.

Endpoints:

- `GET /v1/users/{user_id}/records`
- `GET /v1/users/{user_id}/workout-stats`
- `GET /v1/exercises/{id}/history`
- `GET /v1/users/{user_id}/activity-calendar`

Business rules:

- records should include heaviest set, best estimated 1RM, best volume set, and best rep performance
- exercise history should include ordered past sessions with related sets
- activity calendar should mark workout, meal, and weight-entry days

Tests:

- fixed fixture data with exact expected PR outputs
- date-range filtering
- authorization boundaries

### 5. Add streak and adherence logic

Derive streaks from existing date data instead of creating new tracking tables.

Business logic:

- workout streak = consecutive weeks with at least one workout
- meal streak = consecutive days with at least one meal
- weigh-in streak = consecutive days or weeks with a weight entry
- adherence summaries for last 7, 30, and 90 days

Endpoints:

- `GET /v1/users/{user_id}/streaks`
- or include streaks in dashboard/stat endpoints

Tests:

- gap-day and gap-week behavior
- timezone-safe date handling

## Phase 3: Workout Domain Expansion

### 6. Add cardio as a first-class backend model

The current `Workout.Type` is not enough to represent cardio details.

Schema:

- `workout_cardio_entries`

Suggested fields:

- id
- workout_id
- modality
- duration_minutes
- distance
- distance_unit
- pace
- calories_burned
- avg_heart_rate optional
- notes

Endpoints:

- `GET /v1/workouts/{id}/cardio`
- `POST /v1/workouts/{id}/cardio`
- `PATCH /v1/workout-cardio/{id}`
- `DELETE /v1/workout-cardio/{id}`

Business rules:

- support running, cycling, walking, swimming, rowing, and generic cardio
- allow a workout to contain both resistance exercises and cardio entries

Tests:

- mixed resistance + cardio workout
- invalid negative values
- derived calorie adjustment compatibility later

### 7. Add workout templates

Templates are a backend deliverable and should come before full programs.

Schema:

- `workout_templates`
- `workout_template_exercises`
- `workout_template_sets`

Suggested fields:

- template owner id
- template name
- template type
- notes
- ordered exercises
- default sets/reps/weight/rest

Endpoints:

- CRUD for `/v1/workout-templates`
- `POST /v1/workout-templates/{id}/apply`

Business rules:

- applying a template creates a normal workout snapshot
- editing a template never mutates existing workouts

Tests:

- template apply creates correct workout hierarchy
- ordering is preserved
- auth scoping

### 8. Reintroduce workout programs after templates

Programs were explicitly removed and should only return after templates are stable.

Schema:

- `workout_programs`
- `program_weeks`
- `program_sessions`
- `program_assignments`

Optional later:

- `program_progress`

Business rules:

- admins create programs
- programs are assignable to members
- assigned program content should be snapshotted
- avoid mutable references that rewrite a member’s historical plan

Endpoints:

- admin CRUD for programs
- member assignment endpoints
- program detail and current progress endpoints

Tests:

- admin-only access
- assignment lifecycle
- historical snapshot safety

## Phase 4: Nutrition Data Expansion

### 9. Expand the food catalog and ingestion pipeline

The current food catalog is too small and too manual for the scoped product.

Implementation:

- add import tooling for USDA data
- add a curated Moroccan staples dataset
- make ingestion idempotent
- add normalization and deduplication rules

Requirements:

- 500+ verified food items
- better search quality
- stable serving-unit handling

Deliverables:

- import scripts
- seed/import docs
- validation checks for duplicate foods

### 10. Add normalized micronutrient support

Do not add dozens of nutrient columns to `foods`. Normalize nutrients instead.

Schema:

- `nutrients`
- `food_nutrients`

Suggested fields:

- nutrient code
- nutrient name
- unit
- food id
- nutrient id
- amount per serving

Business rules:

- Level 1 can track 15 to 20 key nutrients through this schema
- Level 2 can later expand to 84 nutrients without a redesign

Endpoints:

- nutrient data can initially be returned through food detail and summary endpoints

Tests:

- nutrient ingestion
- nutrient aggregation correctness
- mixed macro + micronutrient responses

## Phase 5: Nutrition Targets and Meal Reuse

### 11. Add nutrition-target calculation as a first-class service

Current daily summary only returns consumed totals and `user.TDEE`. That is not enough.

Schema options:

- compute targets from `users`
- or add `user_nutrition_profiles` if you want persisted overrides

Target outputs:

- calories
- protein
- carbs
- fat
- optional fiber target

Inputs:

- goal
- weight
- height
- date of birth or age
- activity level
- workout frequency

Endpoints:

- `GET /v1/users/{user_id}/nutrition-targets`

Business rules:

- show both calculated target and any manual override
- keep formulas explicit and deterministic

Tests:

- each supported goal
- each activity level
- override precedence

### 12. Add meal reuse features

The scope expects recent foods, favorites, and copy-previous behavior.

Schema:

- `favorite_foods`
- optionally `favorite_meals`

Endpoints:

- `GET /v1/meals/recent?days=7`
- `POST /v1/meals/{id}/clone`
- `GET /v1/foods/recent`
- `POST /v1/foods/{id}/favorite`
- `DELETE /v1/foods/{id}/favorite`

Business rules:

- recent meals should be user-scoped and date-bounded
- meal cloning should duplicate meal foods, not just the meal row
- recent foods should be derived from the user’s meal history

Tests:

- 7-day and 30-day windows
- clone integrity
- no cross-user leakage

### 13. Add recipes as reusable nutrition objects

Recipes should be modeled separately from meals.

Schema:

- `recipes`
- `recipe_items`

Suggested fields:

- recipe owner id
- recipe name
- servings
- notes
- recipe ingredient food items and quantities

Endpoints:

- CRUD for `/v1/recipes`
- `GET /v1/recipes/{id}/nutrition`
- `POST /v1/recipes/{id}/log-to-meal`

Business rules:

- recipes are reusable building blocks
- logging a recipe to a meal should create actual meal-food rows
- nutrition should be computed from ingredient totals and serving count

Tests:

- nutrition calculation
- serving scaling
- recipe-to-meal expansion

## Phase 6: Nutrition Summaries and Integration Rules

### 14. Add richer daily and weekly nutrition summaries

Extend summaries before adding notifications or advanced coaching.

Endpoints:

- extend current daily summary
- add `GET /v1/users/{user_id}/weekly-summary`

Response additions:

- consumed calories/macros
- target calories/macros
- deltas versus target
- meal counts
- micronutrient totals
- flagged deficiencies

Business rules:

- deficiency flags should start deterministic, for example low protein or low iron
- weekly summary should aggregate both intake and workout context

Tests:

- delta correctness
- weekly aggregation windows
- deficiency threshold behavior

### 15. Add the workout-nutrition integration engine

Implement deterministic rules before any AI layer.

Rules from scope:

- leg day logged -> `+200 kcal`
- no workout today -> `-200 kcal`
- high volume week -> `+15%` calories
- cardio session -> calorie bonus
- recovery warning when training load is high and protein is too low
- goal-alignment warning when intake conflicts with stated goal

Implementation:

- central integration/rules service
- endpoint responses should expose both:
  - final adjusted targets
  - explanation of which rules fired

Endpoints:

- extend summary endpoints
- optionally add `GET /v1/users/{user_id}/recommendations`

Tests:

- one test per rule
- combined rule precedence
- no silent adjustments without explanation payload

## Phase 7: Platform Features

### 16. Add exports and GDPR-oriented account workflows

Account deletion exists as raw deletion, but scope also requires export and GDPR-style flows.

Schema:

- `exports`
- optionally `deletion_requests`

Endpoints:

- `POST /v1/exports`
- `GET /v1/exports/{id}`
- `POST /v1/account/delete-request`

Export formats:

- JSON first
- CSV second

Export scope:

- user profile
- workouts
- workout exercises
- workout sets
- cardio entries
- meals
- meal foods
- foods
- recipes
- weight entries
- settings and targets

Tests:

- export job creation
- export package completeness
- ownership and auth checks

### 17. Add notifications after the rules engine exists

Do not build delivery infrastructure before the system has meaningful events to send.

Schema:

- `notifications`

Suggested fields:

- user id
- type
- title
- message
- payload json
- read at
- created at

Endpoints:

- `GET /v1/notifications`
- `PATCH /v1/notifications/{id}/read`

Possible trigger sources:

- low protein warning
- missed meal logging
- workout reminder
- rest-day warning
- export ready

Realtime delivery:

- websocket or SSE can come after persistence

Tests:

- notification creation from rule outputs
- mark-read flow
- no duplicate spam for the same trigger window

### 18. Add 2FA last in the backend platform pass

2FA is important but not the highest-value product differentiator, so add it after the core domain is stable.

Schema:

- `two_factor_secrets`
- `recovery_codes`

Endpoints:

- `POST /v1/auth/2fa/setup`
- `POST /v1/auth/2fa/verify`
- `POST /v1/auth/2fa/disable`

Business rules:

- TOTP-based flow
- recovery code fallback
- login challenge after password verification

Tests:

- valid TOTP
- invalid TOTP
- recovery code consumption
- disable flow

## Strict Deferrals

Do not schedule these until the deterministic backend foundation above is stable:

- AI workout program generation
- adaptive TDEE AI
- RAG nutrition Q&A
- meal-planning AI
- grocery-list AI
- progress prediction AI
- OAuth login
- frontend work

## Recommended Build Order

If you want the shortest path with the best backend return, build in this exact order:

1. service layer
2. refresh tokens and sessions
3. workout analytics services
4. workout read-model endpoints
5. streaks and adherence
6. cardio entries
7. workout templates
8. workout programs
9. food ingestion and catalog expansion
10. normalized micronutrients
11. nutrition-target service
12. meal reuse
13. recipes
14. richer summaries
15. workout-nutrition rule engine
16. exports and GDPR workflows
17. notifications
18. 2FA

