# Next Steps Plan

## 1. What the current platform can already support

After reading the models, migrations, handlers, and tests, the current platform is built around a lean personal tracking loop:

- User profile and fitness context
- Exercise library
- Workout session logging
- Exercise-level workout structure
- Set-by-set performance logging
- Meal journaling
- Body weight tracking
- JWT-authenticated private user data

This is backed by these core entities:

- `User`: profile, goal, activity level, TDEE, weight, height, date of birth
- `Exercise`: reusable exercise catalog with muscle group, equipment, difficulty, instructions, video URL
- `Workout`: one logged session per user with date, duration, type, notes
- `WorkoutExercise`: an exercise performed inside a workout with order, summary sets/reps/weight/rest
- `WorkoutSet`: detailed set log with set number, reps, weight, RPE, rest, completed flag
- `Meal`: lightweight meal log with date, meal type, notes
- `WeightEntry`: one bodyweight entry per user per date with notes

## 2. Product direction to keep

The best 80/20 direction is:

- Be the best manual workout and progress tracker first
- Treat nutrition as a lightweight accountability journal, not a calorie app
- Use derived insights from workouts, sets, meals, and weight history instead of adding heavy new data models now

That fits the actual schema much better than trying to build coaching, social, food database, or program management features immediately.

## 3. Recommended priority order

### P0: Make the core loop excellent

#### Feature 1: Guided onboarding and profile completion

Why now:

- `User` already stores goal, activity level, TDEE, weight, height, age/date of birth
- This is the fastest way to personalize the app without changing schema

Business logic:

- After registration, require the user to complete profile fields before showing the full app
- Show profile completion status based on presence of `goal`, `activity_level`, `height`, `weight`, and either `date_of_birth` or `age`
- Treat `TDEE` as user-provided or app-calculated guidance, not a medically precise result
- Use `goal` plus `activity_level` to personalize dashboard copy, default empty states, and reminders

Implementation scope:

- Backend mostly exists already through `User` CRUD and auth
- Main work is frontend flow and simple completeness rules

#### Feature 2: Complete workout logging flow

Why now:

- This is the strongest part of the current model
- The app already supports workout -> workout exercises -> workout sets

Business logic:

- A workout is one training session for one user on one date
- A workout contains ordered exercises
- Each workout exercise stores both a summary plan (`sets`, `reps`, `weight`, `rest_time`) and optional set-by-set actual performance
- `WorkoutSet.completed` should represent whether the set was actually performed
- Auto-assign set numbers when not provided
- Show workout history by date and by type such as push, pull, legs, cardio

Implementation scope:

- Keep create/edit/delete flows for workouts, workout exercises, and sets
- Build a clean “start workout -> add exercises -> log sets -> finish workout” UI
- Add client-side validations that match backend rules

#### Feature 3: Workout history and exercise history views

Why now:

- The schema already supports strong historical views with no DB changes
- This gives users immediate value and retention

Business logic:

- Workout history view:
  - List workouts by date desc
  - Filter by workout type and date
  - Open a workout to inspect exercises and sets
- Exercise history view:
  - For a selected exercise, show every `WorkoutExercise` and its related `WorkoutSet` records
  - Surface latest performance, best set, and consistency over time
- Recent activity:
  - Combine workouts, meals, and weight entries into a timeline on the client or service layer

Implementation scope:

- Backend already supports most list/detail access patterns
- Add composed read models or frontend aggregation where needed

#### Feature 4: Weight progress tracking

Why now:

- `WeightEntry` is clean, constrained, and already useful
- One entry per user per day prevents noisy duplicate weigh-ins

Business logic:

- Users log one morning bodyweight entry per day
- Show trend by date range
- Show delta from:
  - previous entry
  - 7-day start vs end
  - first recorded weight
- If user profile has a current `User.Weight`, treat it as profile snapshot, while `WeightEntry` is the actual history source

Implementation scope:

- Build charting and summary cards
- Use existing date-range query support

#### Feature 5: Lightweight meal journal

Why now:

- `Meal` is intentionally minimal and should stay that way for now
- This can support accountability without adding macros or food database complexity

Business logic:

- A meal is a dated journal entry with a meal type such as breakfast, lunch, dinner, snack
- Allow multiple meals of the same type on the same date if needed
- Use `notes` as freeform description, for example “chicken rice bowl after training”
- Position this as habit logging, not nutritional precision
- Show adherence views such as:
  - meals logged today
  - meal consistency this week
  - post-workout meal notes

Implementation scope:

- Keep CRUD simple
- Prioritize timeline and calendar visibility over advanced nutrition analytics

### P1: Add insights derived from existing data

#### Feature 6: Simple dashboard

Why now:

- All inputs already exist in the database
- This turns raw logs into a product

Business logic:

- Dashboard cards:
  - workouts this week
  - latest body weight
  - last workout date
  - meals logged today
- Goal-aware copy:
  - if goal is fat loss, emphasize consistency and weight trend
  - if goal is muscle gain, emphasize workout volume and bodyweight trend
  - if goal is maintenance, emphasize adherence streaks

Implementation scope:

- No schema changes
- Mostly aggregated queries and frontend presentation

#### Feature 7: Workout analytics and personal records

Why now:

- `WorkoutSet` has enough detail for meaningful analytics
- This is a high-value retention feature with low schema cost

Business logic:

- Personal records can be derived from set history:
  - heaviest set for an exercise
  - highest reps at a given weight
  - best estimated top set using reps + weight
- Volume summaries:
  - per workout exercise: sum of `reps * weight` across completed sets
  - per workout: sum of exercise volumes
  - per week: sum across workouts
- Consistency:
  - count workouts per week
  - count sessions per workout type

Implementation scope:

- Add service-level queries or computed response objects
- Keep formulas simple and transparent

#### Feature 8: Calendar and streaks

Why now:

- Dates exist on workouts, meals, and weight entries already
- This is easy to explain and useful for retention

Business logic:

- Activity calendar marks days with:
  - workout logged
  - meal logged
  - weight entry logged
- Streaks should be defined explicitly:
  - workout streak: consecutive weeks with at least one workout
  - weigh-in streak: consecutive days or weeks with a weight entry
  - meal logging streak: consecutive days with at least one meal
- Weekly streaks are safer than daily workout streaks for real fitness behavior

Implementation scope:

- Purely derived from existing records
- No new tables required

### P2: Quality-of-life features still supported by the current schema

#### Feature 9: Exercise library UX improvements

Why now:

- The `Exercise` model already supports filtering and useful metadata

Business logic:

- Browse exercises by muscle group, equipment, and difficulty
- Show instruction and video URL when available
- Allow admins or trusted users to curate the library if you want governance later
- Prevent deletion of exercises already used in workout history to preserve referential integrity

Implementation scope:

- Mostly search, filtering, and better create/edit UX

#### Feature 10: Session notes and qualitative reflection

Why now:

- `Workout.Notes`, `WorkoutExercise.Notes`, `Meal.Notes`, and `WeightEntry.Notes` already exist

Business logic:

- Let users capture why a workout felt good or bad
- Let users annotate an exercise with pain, form cues, or substitutions
- Let users annotate weight entries with context such as travel, sleep, stress, sodium, or hydration
- Use notes later in timeline views and session review

Implementation scope:

- No schema changes
- High UX leverage for low engineering cost

## 4. Concrete next build sequence

### Sprint 1

- Finish auth-to-app onboarding
- Add profile completion gate
- Build the primary workout logging flow
- Build workout detail and history pages

Success metric:

- A new user can register, complete profile, log a workout with exercises and sets, and review that workout later

### Sprint 2

- Build weight logging chart and trend cards
- Build meal journal timeline
- Add dashboard with weekly summary cards

Success metric:

- A returning user can understand training consistency and bodyweight trend in under 30 seconds

### Sprint 3

- Add exercise history pages
- Add PR and volume summaries
- Add calendar and streak views

Success metric:

- A user can answer “Am I progressing?” from existing data without exporting anything

## 5. Business logic rules to enforce now

- Users can only read and write their own workouts, meals, and weight entries
- Exercise reads can stay public, but writes should stay authenticated
- Weight entries should stay limited to one per user per date
- Workout set numbering should stay unique within a workout exercise
- Negative values for duration, reps, weight, rest, TDEE, height, and weight should remain invalid
- Do not let exercise deletion break workout history
- Treat `User.Weight` as profile metadata, not the authoritative trend history table
- Prefer `DateOfBirth` over `Age` for future calculations; keep `Age` only for backward compatibility

## 6. Features to avoid right now because the schema does not support them cleanly

- Macro and calorie tracking
- Food database and ingredient-level nutrition
- Progress photos
- Body measurements beyond weight
- Habit/reminder engine
- Workout templates or reusable programs
- Coach-client workflows
- Social feed, friends, chat, and notifications
- Challenges, leaderboards, or community features
- Advanced cardio tracking like pace, distance, heart rate zones, or GPS

These are not good 80/20 bets for the current database because the required models were removed or do not exist.

## 7. Final recommendation

The strongest version of this product, given the current ORM, is:

- a private fitness log
- centered on workouts and set performance
- supported by lightweight meal journaling
- grounded by bodyweight trend tracking
- improved with derived insights, streaks, and progress summaries

If you stay disciplined and ship that loop first, the current schema is already enough to deliver a credible MVP with real daily utility.
