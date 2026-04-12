# Frontend To-Do List for the Fitness App

This plan is based on the backend routes already registered in `api/server.go`, the product goals in `UM6P_FIT_Scope_Definition.md`, and the UI pieces that already exist in:

- `frontend/` → coach demo for auth + exercise search + program generation
- `nutition_standalone_app/frontend/` → better React/Vite/shadcn starting point for the real product

---

## 1. Real Customer Problem We Should Solve

The frontend should solve this real user problem:

> "Help me know what to do today, log workouts and meals quickly, and clearly see if I am making progress."

That means the product should feel like a **daily fitness companion**, not just a collection of CRUD screens.

The strongest value already supported by the backend is the combination of:

- workout logging
- nutrition logging
- daily and weekly summaries
- auto-calculated nutrition targets
- recommendations and deficiency flags
- streaks and analytics
- templates, recipes, and reuse flows
- AI coach support

---

## 2. Recommended Frontend Direction

### Build one unified user app
Best path:
- use the React/TypeScript app structure from `nutition_standalone_app/frontend/`
- move the current root `frontend/` coach demo into the real app as reusable modules
- keep one main app for end users instead of separate demos

### Main navigation
Use this primary navigation:
- Home
- Workouts
- Nutrition
- Progress
- Coach

Secondary navigation:
- Programs
- Templates
- Recipes
- Notifications
- Settings

---

## 3. Foundation Tasks Before Pages

These are not pages, but they should be done first.

- [ ] Create the real frontend app shell in React + TypeScript
- [ ] Add auth state, token storage, and refresh-token handling
- [ ] Add protected routes for signed-in users
- [ ] Create a typed API layer for `/v1/*`
- [ ] Reuse the nutrition standalone app design system where possible
- [ ] Reuse the exercise search/program generator logic from `frontend/app.js`
- [ ] Add mobile-first layout and bottom/top navigation structure
- [ ] Add global empty, loading, and error states

---

## 4. Clear Page-by-Page To-Do List

## Phase 1 — Must-Build Pages
These pages directly solve the main customer issue and should be built first.

### 1. Login / Register / 2FA
**Priority:** P0

**Customer job:**
Let users securely enter the app and manage secure access.

**What this page should do:**
- sign in
- register
- handle 2FA challenge after login
- allow 2FA setup and disable flows later from settings

**Backend endpoints:**
- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/2fa/setup`
- `POST /v1/auth/2fa/verify`
- `POST /v1/auth/2fa/disable`
- `POST /v1/auth/2fa/recover`
- `POST /v1/auth/logout`
- `GET /v1/auth/sessions`
- `DELETE /v1/auth/sessions/{id}`

**Frontend to-do:**
- [ ] Build login screen
- [ ] Build register screen
- [ ] Build 2FA challenge screen
- [ ] Build 2FA setup screen with QR / secret / recovery codes
- [ ] Add session expiry + silent refresh handling

**Important note:**
There is no forgot-password endpoint in the backend yet, so do **not** design a full reset-password flow right now.

---

### 2. Onboarding / Profile Setup
**Priority:** P0

**Customer job:**
Collect the user data needed to personalize calories, macros, and goals.

**What this page should do:**
- capture name, DOB, weight, height, goal, activity level
- show calculated TDEE and nutrition targets
- explain why targets were generated

**Backend endpoints:**
- `GET /v1/users/{id}`
- `PATCH /v1/users/{id}`
- `GET /v1/users/{user_id}/nutrition-targets`

**Frontend to-do:**
- [ ] Build post-signup onboarding wizard
- [ ] Build editable profile form
- [ ] Show target calories, protein, carbs, fat, fiber, water
- [ ] Add goal-focused copy: build muscle / lose fat / maintain

---

### 3. Home / Today Dashboard
**Priority:** P0

**Customer job:**
Answer: "What should I do today, and am I on track?"

**What this page should show:**
- today calories and macros vs targets
- workout done or not done today
- quick summary of meals and workouts
- low-protein / low-iron / deficiency flags
- recommendations for today
- unread notification count
- weekly snapshot
- quick actions

**Backend endpoints:**
- `GET /v1/summary`
- `GET /v1/weekly-summary`
- `GET /v1/recommendations`
- `GET /v1/users/{user_id}/streaks`
- `GET /v1/users/{user_id}/coach-summary`
- `GET /v1/notifications/unread-count`

**Frontend to-do:**
- [ ] Build dashboard hero cards
- [ ] Build macro progress bars/rings
- [ ] Add recommendation card
- [ ] Add quick actions: log workout, log meal, add weight
- [ ] Add mini streak and weekly summary section
- [ ] Add empty state for new users with first actions

---

### 4. Workout History
**Priority:** P0

**Customer job:**
Review past sessions and stay consistent.

**What this page should do:**
- list workouts by date
- filter by type and date
- open workout detail
- start a new workout

**Backend endpoints:**
- `GET /v1/workouts`
- `GET /v1/users/{user_id}/workouts`
- `GET /v1/workouts/{id}`

**Frontend to-do:**
- [ ] Build workout list page
- [ ] Add filters for date and workout type
- [ ] Add empty state and start-workout CTA
- [ ] Add simple calendar/list toggle if useful

---

### 5. Workout Detail / Log Workout
**Priority:** P0

**Customer job:**
Log a workout quickly enough that users will actually use it every session.

**What this page should do:**
- create a workout
- add exercises into the workout
- log set-by-set reps, weight, rest, RPE, completion
- add cardio inside the workout
- edit notes and duration
- save or delete the session

**Backend endpoints:**
- `POST /v1/workouts`
- `PATCH /v1/workouts/{id}`
- `DELETE /v1/workouts/{id}`
- `GET /v1/workouts/{id}/exercises`
- `POST /v1/workouts/{id}/exercises`
- `PATCH /v1/workout-exercises/{id}`
- `DELETE /v1/workout-exercises/{id}`
- `GET /v1/workout-exercises/{id}/sets`
- `POST /v1/workout-exercises/{id}/sets`
- `PATCH /v1/workout-sets/{id}`
- `DELETE /v1/workout-sets/{id}`
- `GET /v1/workouts/{id}/cardio`
- `POST /v1/workouts/{id}/cardio`
- `PATCH /v1/workout-cardio/{id}`
- `DELETE /v1/workout-cardio/{id}`

**Frontend to-do:**
- [ ] Build create workout flow
- [ ] Build add-exercise flow
- [ ] Build set logger table
- [ ] Add notes, duration, workout type editing
- [ ] Build cardio block
- [ ] Add local rest timer as a client-side feature
- [ ] Add fast "repeat last set" / quick entry UX

---

### 6. Exercise Library / Exercise Search
**Priority:** P0

**Customer job:**
Help users choose the right movement fast and understand how to perform it.

**What this page should do:**
- search exercises semantically
- filter by level, equipment, category, muscle
- show instructions and images
- show the user's history for that exercise

**Backend endpoints:**
- `GET /v1/exercises`
- `GET /v1/exercises/{id}`
- `POST /v1/exercises/search`
- `GET /v1/exercises/library-meta`
- `GET /v1/exercise-images/{path...}`
- `GET /v1/exercises/{id}/history`

**Frontend to-do:**
- [ ] Reuse and improve the current exercise search demo
- [ ] Build proper filter sidebar/sheet
- [ ] Build exercise detail page/modal
- [ ] Show history/progression for the selected exercise
- [ ] Add "add to workout" action from results

---

### 7. Daily Nutrition / Meal Log
**Priority:** P0

**Customer job:**
Log meals quickly and see macro impact immediately.

**What this page should do:**
- show meals for a selected day
- show running daily calories/macros
- create, edit, and delete meals
- clone recent meals

**Backend endpoints:**
- `GET /v1/meals`
- `POST /v1/meals`
- `GET /v1/meals/{id}`
- `PATCH /v1/meals/{id}`
- `DELETE /v1/meals/{id}`
- `GET /v1/meals/recent`
- `POST /v1/meals/{id}/clone`
- `GET /v1/summary`

**Frontend to-do:**
- [ ] Build daily meal timeline
- [ ] Build add meal modal/page
- [ ] Build edit meal flow
- [ ] Add clone recent meal shortcut
- [ ] Show summary strip for calories/protein/carbs/fat

---

### 8. Meal Builder / Food Picker
**Priority:** P0

**Customer job:**
Make food logging fast enough to become a habit.

**What this page should do:**
- search foods
- add foods to a meal
- edit food quantities
- browse recent foods
- browse favorite foods
- see updated meal totals in real time

**Backend endpoints:**
- `GET /v1/meals/{id}/foods`
- `POST /v1/meals/{id}/foods`
- `PATCH /v1/meal-foods/{id}`
- `DELETE /v1/meal-foods/{id}`
- `GET /v1/foods`
- `GET /v1/foods/{id}`
- `GET /v1/foods/recent`
- `POST /v1/foods/{id}/favorite`
- `DELETE /v1/foods/{id}/favorite`
- `GET /v1/users/{user_id}/favorites`

**Frontend to-do:**
- [ ] Build searchable food picker
- [ ] Add recent foods tab
- [ ] Add favorites tab
- [ ] Build quantity editor
- [ ] Build meal nutrition summary panel
- [ ] Add optional custom food creation later if needed

---

### 9. Weight Tracker
**Priority:** P0

**Customer job:**
See whether body-weight trend matches the goal.

**What this page should do:**
- add weigh-ins
- view chart of weight over time
- compare recent change vs goal direction
- edit or delete entries

**Backend endpoints:**
- `GET /v1/weight-entries`
- `POST /v1/weight-entries`
- `GET /v1/weight-entries/{id}`
- `PATCH /v1/weight-entries/{id}`
- `DELETE /v1/weight-entries/{id}`

**Frontend to-do:**
- [ ] Build weight chart page
- [ ] Add quick weigh-in card
- [ ] Add range filters
- [ ] Show recent trend summary

---

### 10. Notifications Center
**Priority:** P0

**Customer job:**
See reminders and important warnings in one place.

**What this page should do:**
- list notifications
- mark one as read
- mark all as read
- deep-link users into the relevant page

**Backend endpoints:**
- `GET /v1/notifications`
- `PATCH /v1/notifications/{id}/read`
- `PATCH /v1/notifications/read-all`
- `GET /v1/notifications/unread-count`

**Frontend to-do:**
- [ ] Build notifications page or drawer
- [ ] Add unread badge in nav
- [ ] Add mark-read actions
- [ ] Map notification types to deep links

---

## Phase 2 — Retention and Differentiation Pages
Build these after the MVP pages above are working well.

### 11. Progress & Analytics
**Priority:** P1

**Customer job:**
Give users proof that their effort is working.

**What this page should show:**
- personal records
- workout statistics
- activity calendar
- streaks
- weekly summary
- exercise progression

**Backend endpoints:**
- `GET /v1/users/{user_id}/records`
- `GET /v1/users/{user_id}/workout-stats`
- `GET /v1/users/{user_id}/activity-calendar`
- `GET /v1/users/{user_id}/streaks`
- `GET /v1/weekly-summary`
- `GET /v1/exercises/{id}/history`

**Frontend to-do:**
- [ ] Build progress dashboard
- [ ] Build PR cards
- [ ] Build volume/progression charts
- [ ] Add adherence heatmap/calendar
- [ ] Add weekly trends module

---

### 12. Workout Templates
**Priority:** P1

**Customer job:**
Reduce repeated logging work for common routines.

**What this page should do:**
- list templates
- create template from exercises and set entries
- edit template metadata
- apply template into a workout

**Backend endpoints:**
- `GET /v1/workout-templates`
- `POST /v1/workout-templates`
- `GET /v1/workout-templates/{id}`
- `PATCH /v1/workout-templates/{id}`
- `DELETE /v1/workout-templates/{id}`
- `POST /v1/workout-templates/{id}/apply`

**Frontend to-do:**
- [ ] Build templates list page
- [ ] Build template builder
- [ ] Add apply-to-today flow
- [ ] Add create-from-existing-workout shortcut later

---

### 13. Program Assignments / Structured Plans
**Priority:** P1

**Customer job:**
Follow a multi-week plan instead of improvising workouts.

**What this page should do:**
- show the active assigned program
- show week/day structure
- show assignment status
- apply a planned session into the workout log

**Backend endpoints:**
- `GET /v1/program-assignments`
- `GET /v1/program-assignments/{id}`
- `PATCH /v1/program-assignments/{id}/status`
- `POST /v1/program-sessions/{id}/apply`

**Frontend to-do:**
- [ ] Build active program overview
- [ ] Build week/day session list
- [ ] Add start-session / apply-session CTA
- [ ] Show assignment state: assigned, in progress, completed, cancelled

**Note:**
This is user-facing program consumption. Admin program creation is a separate internal UI.

---

### 14. Recipes
**Priority:** P1

**Customer job:**
Log repeat home meals faster and keep nutrition accurate.

**What this page should do:**
- list recipes
- create/edit recipes from foods
- show per-serving nutrition
- log recipe directly into a meal

**Backend endpoints:**
- `GET /v1/recipes`
- `POST /v1/recipes`
- `GET /v1/recipes/{id}`
- `PATCH /v1/recipes/{id}`
- `DELETE /v1/recipes/{id}`
- `GET /v1/recipes/{id}/nutrition`
- `POST /v1/recipes/{id}/log-to-meal`

**Frontend to-do:**
- [ ] Build recipe list page
- [ ] Build recipe builder
- [ ] Show per-serving nutrition
- [ ] Add log-to-meal shortcut

---

### 15. AI Coach
**Priority:** P1

**Customer job:**
Ask natural questions instead of manually interpreting all the data.

**What this page should do:**
- show coach chat thread
- allow new and ongoing conversations
- suggest prompts
- show coach summary context
- collect answer feedback

**Backend endpoints:**
- `POST /v1/chat`
- `GET /v1/chat/history`
- `POST /v1/chat/feedback`
- `GET /v1/users/{user_id}/coach-summary`
- plus summary/recommendation/streak/records data exposed through coach tools

**Frontend to-do:**
- [ ] Build coach chat page
- [ ] Build chat history sidebar/list
- [ ] Add starter prompt chips
- [ ] Add thumbs up/down feedback
- [ ] Add links from coach answers to workouts, nutrition, and exercise pages where possible

**Reuse note:**
The current `frontend/` coach demo already proves the exercise search and program generation UX patterns.

---

### 16. Leaderboard / Motivation
**Priority:** P2

**Customer job:**
Increase accountability and consistency through friendly competition.

**What this page should do:**
- show rankings
- switch between weekly, monthly, yearly, all-time
- filter by training, nutrition, consistency, or all
- highlight current user rank

**Backend endpoints:**
- `GET /v1/leaderboard`

**Frontend to-do:**
- [ ] Build leaderboard page
- [ ] Add period filters
- [ ] Add pillar filters
- [ ] Highlight current user row

---

### 17. Settings / Security / Export / Account
**Priority:** P1

**Customer job:**
Give users control over their account, privacy, and security.

**What this page should do:**
- edit profile
- manage sessions
- setup/disable 2FA
- request data export
- request account deletion

**Backend endpoints:**
- `GET /v1/users/{id}`
- `PATCH /v1/users/{id}`
- `GET /v1/auth/sessions`
- `DELETE /v1/auth/sessions/{id}`
- `POST /v1/auth/2fa/setup`
- `POST /v1/auth/2fa/verify`
- `POST /v1/auth/2fa/disable`
- `POST /v1/exports`
- `GET /v1/exports/{id}`
- `POST /v1/account/delete-request`
- `POST /v1/auth/logout`

**Frontend to-do:**
- [ ] Build profile settings section
- [ ] Build security section
- [ ] Build active sessions list
- [ ] Build export data flow
- [ ] Build delete account confirmation flow

---

## Phase 3 — Internal Admin Pages
Only build these if the product needs an internal operations dashboard.

### 18. Admin Dashboard
**Priority:** Internal only

**Backend endpoints:**
- `GET /v1/admin/dashboard/summary`
- `GET /v1/admin/dashboard/trends`
- `GET /v1/admin/dashboard/realtime`
- `GET /v1/admin/system/health`
- `GET /v1/admin/users/stats`
- `GET /v1/admin/users/growth`
- `GET /v1/admin/workouts/stats`
- `GET /v1/admin/workouts/exercises/popular`
- `GET /v1/admin/nutrition/stats`
- `GET /v1/admin/moderation/stats`
- `GET /v1/admin/audit-logs`

**Use:**
Operations visibility, system health, product usage, moderation insight.

---

### 19. Admin User Management
**Priority:** Internal only

**Backend endpoints:**
- `GET /v1/admin/users`
- `GET /v1/admin/users/{id}`
- `PATCH /v1/admin/users/{id}`
- `DELETE /v1/admin/users/{id}`
- `POST /v1/admin/users/{id}/ban`
- `POST /v1/admin/users/{id}/unban`

**Use:**
Support, moderation, account review.

---

### 20. Admin Program Management
**Priority:** Internal only

**Backend endpoints:**
- `POST /v1/programs`
- `GET /v1/programs`
- `GET /v1/programs/{id}`
- `PATCH /v1/programs/{id}`
- `DELETE /v1/programs/{id}`
- `POST /v1/programs/{id}/weeks`
- `POST /v1/programs/{id}/assignments`
- `GET /v1/programs/{id}/assignments`
- `GET /v1/program-weeks/{id}`
- `PATCH /v1/program-weeks/{id}`
- `DELETE /v1/program-weeks/{id}`
- `POST /v1/program-weeks/{id}/sessions`
- `GET /v1/program-sessions/{id}`
- `PATCH /v1/program-sessions/{id}`
- `DELETE /v1/program-sessions/{id}`
- `PATCH /v1/admin/program-assignments/{id}`
- `DELETE /v1/admin/program-assignments/{id}`

**Use:**
Create structured programs and assign them to users.

---

### 21. Admin Nutrition Catalog / Import
**Priority:** Internal only

**Backend endpoints:**
- `POST /v1/admin/import-usda`
- `GET /v1/admin/nutrition/stats`

**Use:**
Maintain and expand the nutrition database.

---

## 5. Build Order I Recommend

### MVP build order
1. Login / Register / 2FA
2. Onboarding / Profile Setup
3. Home / Today Dashboard
4. Workout History
5. Workout Detail / Log Workout
6. Exercise Library / Exercise Search
7. Daily Nutrition / Meal Log
8. Meal Builder / Food Picker
9. Weight Tracker
10. Notifications Center

### Next after MVP
11. Progress & Analytics
12. Workout Templates
13. Program Assignments
14. Recipes
15. AI Coach
16. Settings / Security / Export / Account
17. Leaderboard

### Internal only later
18. Admin Dashboard
19. Admin User Management
20. Admin Program Management
21. Admin Nutrition Catalog

---

## 6. Product Guidance: What Not To Do First

Do **not** make the first version of the app feel like:
- an admin panel
- a database browser
- a standalone AI chatbot
- an exercise-search-only tool

Even though those capabilities exist, the best user value is still:
- today dashboard
- fast workout logging
- fast meal logging
- visible progress
- reusable templates/recipes/programs

---

## 7. Final Recommendation

If we want the frontend to solve a real customer issue, the product should be positioned as:

> a unified workout + nutrition companion that helps users act today, stay consistent this week, and see proof of progress over time.

That is exactly where this backend is strongest.
