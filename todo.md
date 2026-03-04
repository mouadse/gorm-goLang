# Fitness App Backend - API Development Roadmap

## Current State

**Stack:** Go 1.25 + GORM + PostgreSQL

**Models (13 total):**
| Domain | Models |
|--------|--------|
| Users | User (profile, TDEE, goals) |
| Workouts | Exercise, Workout, WorkoutExercise, WorkoutProgram |
| Nutrition | Food, Meal, MealFood |
| Progress | WeightEntry, WeeklyAdjustment |
| Social | Friendship, Message, Notification |

**Status:** Data models complete, but no API layer exists. `main.go` only runs migrations.

---

## 1. Authentication & User APIs

- [ ] `POST /auth/register` - Create account
- [ ] `POST /auth/login` - Authenticate
- [ ] `POST /auth/logout` - Invalidate session
- [ ] `POST /auth/refresh` - Refresh JWT
- [ ] `POST /auth/forgot-password` - Password reset flow
- [ ] `GET /users/me` - Current user profile
- [ ] `PUT /users/me` - Update profile
- [ ] `PUT /users/me/goals` - Update fitness goals
- [ ] `PUT /users/me/tdee` - Update/recalculate TDEE

---

## 2. Exercise Library APIs

- [ ] `GET /exercises` - List/search exercises (filter by muscle, equipment, difficulty)
- [ ] `GET /exercises/:id` - Exercise details
- [ ] `POST /exercises` - Create exercise (admin)
- [ ] `PUT /exercises/:id` - Update exercise (admin)
- [ ] `DELETE /exercises/:id` - Delete exercise (admin)

---

## 3. Workout APIs

- [ ] `GET /workouts` - List user's workouts (filter by date range)
- [ ] `POST /workouts` - Log a workout
- [ ] `GET /workouts/:id` - Workout details with exercises
- [ ] `PUT /workouts/:id` - Update workout
- [ ] `DELETE /workouts/:id` - Delete workout
- [ ] `POST /workouts/:id/exercises` - Add exercise to workout
- [ ] `PUT /workouts/:id/exercises/:eid` - Update exercise in workout
- [ ] `DELETE /workouts/:id/exercises/:eid` - Remove exercise from workout
- [ ] `GET /workouts/stats` - Aggregated stats (volume, frequency, PRs)

---

## 4. Workout Program APIs

- [ ] `GET /programs` - List available programs
- [ ] `GET /programs/:id` - Program details
- [ ] `POST /programs` - Create program (admin)
- [ ] `PUT /programs/:id` - Update program (admin)
- [ ] `DELETE /programs/:id` - Delete program (admin)
- [ ] `POST /users/me/programs` - Enroll in program
- [ ] `GET /users/me/programs` - User's enrolled programs
- [ ] `GET /users/me/programs/:id/progress` - Track program progress

---

## 5. Nutrition APIs

- [ ] `GET /foods` - Search food database
- [ ] `GET /foods/:id` - Food details with macros
- [ ] `POST /foods` - Add custom food
- [ ] `GET /meals` - List meals (filter by date)
- [ ] `POST /meals` - Log a meal
- [ ] `PUT /meals/:id` - Update meal
- [ ] `DELETE /meals/:id` - Delete meal
- [ ] `POST /meals/:id/foods` - Add food to meal
- [ ] `PUT /meals/:id/foods/:fid` - Update food quantity
- [ ] `DELETE /meals/:id/foods/:fid` - Remove food from meal
- [ ] `GET /nutrition/daily` - Daily macro summary
- [ ] `GET /nutrition/summary` - Weekly/monthly nutrition stats

---

## 6. Progress Tracking APIs

- [ ] `GET /weight-entries` - Weight history
- [ ] `POST /weight-entries` - Log weight
- [ ] `PUT /weight-entries/:id` - Update entry
- [ ] `DELETE /weight-entries/:id` - Delete entry
- [ ] `GET /weight-entries/trend` - Weight trend analysis
- [ ] `GET /progress/stats` - Body composition, measurements
- [ ] `GET /weekly-adjustments` - AI TDEE adjustments history

---

## 7. Social APIs

- [ ] `GET /friends` - List friends
- [ ] `POST /friends/request` - Send friend request
- [ ] `PUT /friends/:id/accept` - Accept request
- [ ] `DELETE /friends/:id` - Remove friend
- [ ] `GET /messages` - List conversations
- [ ] `GET /messages/:userId` - Chat history with user
- [ ] `POST /messages` - Send message
- [ ] `GET /notifications` - List notifications
- [ ] `PUT /notifications/:id/read` - Mark as read

---

## 8. Analytics & Insights APIs

- [ ] `GET /analytics/dashboard` - Overview stats
- [ ] `GET /analytics/workouts` - Workout trends, PRs
- [ ] `GET /analytics/nutrition` - Macro breakdown, streaks
- [ ] `GET /analytics/progress` - Weight loss/gain trajectory
- [ ] `GET /analytics/recommendations` - AI-powered suggestions

---

## 9. Missing Models to Add

- [ ] `BodyMeasurement` - Chest, waist, hips, arms, thighs
- [ ] `ProgressPhoto` - Progress pictures with timestamps
- [ ] `ExercisePR` - Personal records (1RM, etc.)
- [ ] `UserGoal` - Specific goals with deadlines
- [ ] `Achievement` - Gamification badges
- [ ] `Challenge` - Community challenges
- [ ] `ChallengeParticipation` - User challenge entries
- [ ] `Integration` - Wearable connections (Garmin, Apple Health, etc.)

---

## 10. Infrastructure Setup

- [ ] HTTP Router (Gin, Echo, Chi, or Fiber)
- [ ] Auth middleware (JWT validation)
- [ ] CORS middleware
- [ ] Rate limiting middleware
- [ ] Request logging middleware
- [ ] JWT/OAuth2 authentication
- [ ] Input validation (go-playground/validator)
- [ ] Structured error handling with proper HTTP codes
- [ ] API documentation (Swagger/OpenAPI)
- [ ] Background jobs for notifications, TDEE calculations

---

## Priority Order

1. **Infrastructure** - Router, middleware, auth
2. **Authentication** - Core to everything else
3. **Workouts** - Core fitness feature
4. **Nutrition** - Core fitness feature
5. **Progress Tracking** - Essential for motivation
6. **Exercise Library** - Foundation for workouts
7. **Social** - Engagement boosters
8. **Analytics** - Insights and value-add
9. **Programs** - Structured training
10. **New Models** - Enhanced features