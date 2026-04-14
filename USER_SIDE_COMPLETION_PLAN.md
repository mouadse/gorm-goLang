# User-Side Completion Plan

## Goal
Turn the current backend-heavy fitness platform into a complete user-facing app for workouts, nutrition, and AI coaching.

---

## Current Assessment

### Backend
The backend is strong and mostly complete for core product needs:
- Auth, sessions, refresh tokens, 2FA
- User profiles and goals
- Workout logging, sets, cardio, analytics, templates
- Nutrition logging, foods, meals, recipes, favorites
- USDA import and micronutrients
- AI chat coach with tool calling
- Notifications, exports, GDPR flows
- Leaderboard and admin dashboard

### Frontend
The user-facing side is not complete.

Current UI pieces are fragmented:
- `frontend/`: coach dashboard demo for exercise search and program generation
- `nutition_standalone_app/frontend/`: standalone food search UI
- No unified end-user app for full fitness tracking

---

## Main Gaps

### 1. No Real User App
Missing a unified frontend for:
- login/register/2FA
- daily dashboard
- workout logging
- meal logging
- nutrition dashboard
- weight tracking
- notifications
- AI coach chat
- profile/settings
- leaderboard
- exports/account actions

### 2. Backend Gaps Still Worth Finishing
- Workout program admin CRUD and assignment flows
- Automated notification triggering from rules/scheduler
- Some advanced AI/product features from scope remain unimplemented

### 3. Product Gaps
Missing important platform-level user experience items:
- offline/PWA support
- multilingual support (AR/FR/EN)
- richer habit loops and progress reporting

---

## Recommended Build Plan

## Phase 1 — Build the Core User Frontend
Create a single user-facing web app connected to the main Go API.

### Pages / Views to build
1. **Auth**
   - Login
   - Register
   - 2FA challenge/setup/recovery

2. **Dashboard**
   - Today summary
   - Calories/macros vs targets
   - Workout done / pending
   - Weight trend snapshot
   - Recommendations and alerts

3. **Workout Flow**
   - Start workout
   - Add exercises
   - Log sets/reps/weight
   - Add cardio
   - Rest timer
   - Save workout
   - Workout history/calendar
   - PR highlights and analytics

4. **Nutrition Flow**
   - Search foods
   - Create meals
   - Add meal foods
   - Show calories/macros/micros
   - Recent foods and favorites
   - Clone recent meals
   - Daily nutrition dashboard

5. **Recipes**
   - Create/edit recipes
   - Nutrition per serving
   - Log recipe into meal

6. **Weight Tracking**
   - Add weight entry
   - Weight history chart

7. **AI Coach**
   - Chat screen
   - Conversation history
   - Coach summary card

8. **Notifications**
   - List notifications
   - Mark read / mark all read

9. **Profile & Settings**
   - Edit profile
   - Goal, DOB, weight, height, activity level
   - TDEE and target display
   - Sessions and security settings

10. **Leaderboard / Social-lite**
   - Weekly/monthly/all-time ranking

11. **Exports / Account**
   - Request export
   - Download export
   - Request account deletion

### Frontend direction
Recommended approach:
- Build one React/TypeScript app
- Reuse the best UI ideas from `nutition_standalone_app/frontend`
- Replace the current root `frontend/` demo with the real user app or move demo screens into the new app

---

## Phase 2 — Finish Backend Product Gaps

### A. Workout Programs
Implement full backend flows for workout programs:
- Admin CRUD for programs
- Program week/session management
- Assignment to users
- User-side retrieval of assigned program
- Apply program session into actual workout logging flow

### B. Notifications Automation
Wire notification generation into worker/scheduled jobs:
- low protein warning
- meal logging reminder
- workout reminder
- recovery/rest warning
- weekly progress summary
- export ready notification

### C. Better User Dashboard Aggregation
Add or refine endpoints that return a single dashboard payload combining:
- today’s nutrition summary
- adjusted targets
- workout state
- streaks
- notifications summary
- recommendations
- recent PRs

---

## Phase 3 — Make AI Features More Useful
Current AI base is good. Extend it with higher-value user features.

### Priority AI additions
1. **Weekly progress report**
   - summarize workouts, adherence, weight trend, PRs

2. **Smart meal swap suggestions**
   - suggest alternatives based on macro gap

3. **Workout notes summarizer**
   - summarize workout improvements and performance changes

4. **Meal planning assistant**
   - generate a simple daily/weekly meal suggestion using current targets

5. **Expanded coach context**
   - include more user data in chat tools and summaries

### Later AI additions
- adaptive TDEE based on actual weight trends
- nutrition deficiency suggestions
- grocery list generation
- progress prediction

---

## Phase 4 — Platform Completeness

### User experience features
- PWA/offline support for workout and meal logging
- multilingual support: Arabic, French, English
- responsive mobile-first layouts
- browser compatibility pass
- better empty states and onboarding

### Operational features
- audit missing OpenAPI parity for all routes
- ensure every major flow has integration tests
- add seed/demo accounts for frontend QA

---

## Suggested Priority Order

### Highest priority
1. Build the unified frontend
2. Add dashboard + workout logging + meal logging
3. Integrate AI chat into the user app
4. Implement workout program CRUD/assignment
5. Automate notifications

### Medium priority
6. Weekly progress report
7. Meal swap suggestions
8. Better analytics visualizations
9. Export/account UX

### Lower priority
10. PWA offline support
11. Multilingual support
12. Adaptive TDEE and advanced AI planning

---

## Practical Execution Plan

### Sprint 1
- Set up unified frontend app
- Auth flows
- Basic dashboard shell
- Profile fetch/update

### Sprint 2
- Workout logging UI
- Workout history UI
- Cardio and template flows

### Sprint 3
- Food search integration
- Meal logging UI
- Daily nutrition dashboard
- Favorites/recent meals/recipes

### Sprint 4
- AI coach chat UI
- Notifications center
- Weight tracking
- Leaderboard

### Sprint 5
- Workout programs admin/user flows
- Notification automation
- Weekly progress summaries

### Sprint 6
- Polish, mobile responsiveness, QA
- PWA/i18n planning or implementation

---

## Final Recommendation
The project is **backend-strong but user-side incomplete**.

The best next move is **not more backend expansion first**.
The best next move is to **build the actual end-user frontend on top of the already-capable API**, then close the few backend product gaps that directly support that experience.
