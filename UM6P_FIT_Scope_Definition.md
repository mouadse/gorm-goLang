# UM6P_FIT — Product Scope Definition

**UM6P_FIT**
**PRODUCT SCOPE DEFINITION**

Complete Fitness Platform  
50% Workout Tracking + 50% Nutrition Tracking  

| | |
|---|---|
| **Timeline** | 45 Days |
| **Level 1** | ~18 pts |
| **Level 2** | ~28 pts |
| **Delivery** | Day 35 → 42 |

**Product Management Team**  
February 2026 — REVISED

---

## Table of Contents

| # | Chapter | Content | Page |
|---|---------|---------|------|
| 01 | EXECUTIVE SUMMARY | Dual value propositions — Scope overview | p.3 |
| 02 | CORE USE CASE ANALYSIS | Workout tracking · Nutrition tracking · Integration Magic | p.3 |
| 03 | LEVEL 1 — MUST-HAVE FEATURES (~18 pts) | Workout · Nutrition · Platform · FE/BE/IA breakdown | p.4–6 |
| 04 | LEVEL 2 — ADVANCED FEATURES (~28 pts) | Enhanced workout · Nutrition · Platform · Cumulative summary | p.7–9 |
| 05 | WORKOUT-NUTRITION INTEGRATION STRATEGY | Smart calorie adjustments · Performance insights · Dashboard | p.10 |
| 06 | OPEN PROPOSITIONS — FEATURES TO CHOOSE | 10 candidate features to vote on and include in the final scope | p.10 |

**Tag Legend:**  
- **FE** Frontend
- **BE** Backend
- **IA** AI / Intelligence
- **FE/BE** Full-Stack
- **BE/IA** Backend + AI

---

## 01 | EXECUTIVE SUMMARY

This document defines the product scope for UM6P_FIT, a comprehensive fitness platform built for the UM6P community. The application provides equal focus on workout tracking and nutrition tracking, enabling users to manage their complete fitness journey in one place.

### WORKOUT TRACKING (50%)

Log workouts · Track progressive overload · Analyze training volume · Monitor recovery

### NUTRITION TRACKING (50%)

Log meals · Calculate macros & calories · Track micronutrients · Optimize nutrition for goals

Unlike competitors that excel at one dimension (MyFitnessPal = nutrition, Strong = workouts), UM6P_FIT delivers a unified experience where workout data informs nutrition targets and vice-versa. Example: logging a heavy leg day automatically increases the carb target by +20%.

| Level | Scope | Risk |
|-------|-------|------|
| **LEVEL 1** (Must Have) | ~18 pts · Core workout + nutrition | Low |
| **LEVEL 2** (Advanced) | ~28 pts · AI coaching + analytics | Medium |
| **PROPOSITIONS** (Choose) | 10 candidate features to pick from and discuss | — |

---

## 02 | CORE USE CASE ANALYSIS

### 2.1 Use Case A — Workout Tracking (50%)

Users need to log gym sessions, track progressive overload, and understand training volume — equally important to nutrition.

1. Start a workout session (e.g., 'Push Day', 'Leg Day')
2. Log exercises with sets, reps and weight (e.g., Bench Press: 3x8 @ 80 kg)
3. App shows previous workout data ('Last time: 3x8 @ 75 kg — You improved!')
4. Track rest time between sets with the built-in timer.
5. Log cardio: duration, distance, calories burned
6. App calculates total training volume and suggests rest days
7. User views workout history and strength progression charts

### 2.2 Use Case B — Nutrition Tracking (50%)

Nutrition fuels training, enables recovery, and drives body composition changes.

1. Log a meal (manual entry)
2. System calculates calories and macros (protein/carbs/fats)
3. User sees daily totals vs. targets (adjusted based on training day)
4. User views micronutrient breakdown (vitamins, minerals)
5. System flags deficiencies (e.g., 'Low protein today')
6. User views historical trends (7-day, 30-day nutrition patterns)
7. System provides feedback on nutrition-workout alignment

### 2.3 The Integration Magic

Workout data and nutrition data talk to each other — this separates UM6P_FIT from single-purpose apps.

- **Training Day Adjustments:** Heavy leg day logged -> carb target +20% automatically
- **Recovery Recommendations:** High training volume this week -> app suggests higher protein intake
- **Progress Correlation:** Weight not changing + consistent workouts -> app suggests calorie increase
- **Rest Day Optimization:** No workout today -> app lowers calorie target, maintains protein
- **Performance Insights:** 'Your bench press improved 10% after increasing daily protein from 120g to 150g'

---

## 03 | LEVEL 1 — MUST-HAVE FEATURES (~18 pts)

**Delivery target:** Day 35 | **Risk:** Low | **Status:** Non-Negotiable

This tier delivers the non-negotiable core of both workout and nutrition tracking. Every feature below is essential for a fully functional fitness app.

### 3.1 Workout Features — Level 1

| Feature | Description | Module Mapping | Tag | Pts |
|---------|-------------|----------------|-----|-----|
| Exercise Library (100+) | Database of 100+ exercises with instructions, muscle groups, difficulty | Database + Backend | BE | — |
| Manual Workout Logging | Log sets/reps/weight with minimal taps; copy from previous session | Frontend Framework (Minor) | FE | +1 |
| Sets / Reps / Weight Tracking | Full per-set tracking with optional notes | Core Feature | FE | — |
| Rest Timer | Customizable countdown between sets (60–180 s) | Frontend Feature | FE | — |
| Workout History (Calendar) | Calendar view of all sessions, filterable by muscle group or exercise | Backend + ORM (Minor) | BE | +1 |
| Progressive Overload Tracking | Compare current vs. previous session; highlight PRs (personal records) | Core Logic | FE/BE | — |
| Cardio Tracking | Log running/cycling/swimming: duration, distance, pace, calories | Core Feature | FE | — |
| Workout Templates | Reusable session templates (Push/Pull/Legs, Upper/Lower, Full Body…) | Database Feature | BE | — |
| Training Volume Charts | Weekly/monthly volume (sets×reps×weight) with trend lines | Data: Dashboard (Major) | FE | +2 |
| Workout Programs (Admin) | Pre-built programs created by gym staff and assignable to members | User Mgmt: Permissions (Major) | BE | +2 |

**Workout Level 1 Subtotal:** +6 pts

### 3.2 Nutrition Features — Level 1

| Feature | Description | Module Mapping | Tag | Pts |
|---------|-------------|----------------|-----|-----|
| Food Database (500+ items) | Verified nutrition data — USDA database + Moroccan staples | Database + Backend | BE | — |
| Manual Food Logging | Manual entry; recent foods; favorites; copy previous meals | Core Feature | FE | — |
| Calorie & Macro Calculation | Real-time protein/carbs/fats calculation against daily targets | Core Logic | BE | — |
| TDEE Calculator | Calculated from age, weight, height, activity level + workout frequency | Core Logic | BE | — |
| Daily Macro Dashboard | Live dashboard with macro progress bars and daily targets | Frontend Feature | FE | — |
| Meal History (7 days) | Retrieve and reuse meals from the last 7 days | Backend Feature | BE | — |
| Micronutrient Tracking (15–20) | Track 15–20 key nutrients: vitamins, calcium, iron, zinc, etc. | Core Feature | FE/BE | — |
| Weight Logging / day | Daily body weight entry with trend chart | Core Feature | FE | — |
| Recipe Creation | Build custom recipes with automatic nutrition breakdown | Core Feature | FE/BE | — |
| Weekly Macro Adjustments (AI) | AI adjusts weekly macro targets based on real weight trends vs. goals | AI: LLM Interface (Major) | IA | +2 |

**Nutrition Level 1 Subtotal:** +2 pts

### 3.3 Platform Features — Level 1

| Feature | Description | Module Mapping | Tag | Pts |
|---------|-------------|----------------|-----|-----|
| User Authentication (JWT) | Registration, secure login, token management and refresh | User Mgmt: Standard (Major) | BE | +2 |
| User Profiles (Avatar, Goals) | User profile: avatar, fitness goals, body metrics | User Mgmt: Standard (Major) | FE/BE | — |
| Real-time Updates (WebSocket) | Live notifications and updates via Socket.io | Web: Real-time (Major) | BE | +2 |
| Notifications System | Nutrition alerts, workout reminders, smart suggestions | Web: Notifications (Minor) | BE | +1 |
| PWA Offline Support | Offline-first: workout and meal logging work without internet | Web: PWA (Minor) | FE | — |
| Data Export | Export workout/nutrition data as CSV or JSON | Data: Export (Minor) | BE | +1 |
| Multi-Language (AR/FR/EN) | Full trilingual support: Arabic, French, English | i18n (Minor) | FE | +1 |
| 2FA Authentication | Two-factor authentication for enhanced account security | User Mgmt: 2FA (Minor) | BE | +1 |
| GDPR Compliance | Personal data export and account deletion requests | Data: GDPR (Minor) | BE | +1 |

**Platform Level 1 Subtotal:** +10 pts

### 3.4 Level 1 — Points Breakdown by Layer

| Category | Key Features | FE pts | BE pts | IA pts | Total |
|----------|--------------|--------|--------|--------|-------|
| Workout | 10 features: library, logging, charts, programs | 3 | 2 | 0 | 6+ |
| Nutrition | 10 features: DB, logging, macros, TDEE, micronutrients, AI adjust | 0 | 0 | 2 | 2+ |
| Platform | 9 features: auth, real-time, notifs, export, i18n, 2FA, GDPR | 2 | 5 | 0 | 10+ |
| **LEVEL 1 TOTAL** | | **5 pts** | **7 pts** | **4 pts** | **~18 pts** |

---

## 04 | LEVEL 2 — ADVANCED FEATURES (~28 pts)

**Delivery target:** Day 42 | **Risk:** Medium | **Pursue only if Level 1 ships by Day 33**

Level 2 adds AI-powered coaching, advanced analytics and a richer user experience on top of the Level 1 foundations.

### 4.1 Enhanced Workout Features — Level 2

| Feature | Description | Module Mapping | Tag | Pts |
|---------|-------------|----------------|-----|-----|
| Workout Analytics Dashboard | Advanced charts: volume per muscle group, frequency, strength progression curves | Data: Dashboard Extension | FE | — |
| Rest Day AI Recommendations | Detects overtraining (high volume + poor recovery) and suggests rest | AI Feature | IA | — |
| Personal Records Board | Track all PRs (1RM, max reps, max volume) with full history | Core Feature | FE/BE | — |
| AI Workout Program Generator | Creates personalized programs based on goals, equipment and training history | AI: RAG or LLM | IA | — |
| Workout Gamification | Streaks, badges, leaderboards for training volume and consistency | Custom: Major | FE | +2 |

**Workout Level 2 Additional Points:** +2 pts

### 4.2 Enhanced Nutrition Features — Level 2

| Feature | Description | Module Mapping | Tag | Pts |
|---------|-------------|----------------|-----|-----|
| Adaptive TDEE Calculation | Learns from actual weight trends instead of relying on static formulas | AI: Recommendation System | BE/IA | — |
| Calorie Planner (Flexible) | Flexible high/low calorie days within a weekly calorie budget | Core Feature | FE | — |
| 84-Nutrient Tracking | Expand from 15–20 to 84 tracked nutrients (Cronometer-level) | Database Work | BE | — |
| Nutrient Deficiency AI | AI suggests foods to fill identified nutritional gaps | AI: LLM | IA | — |
| Meal Timing Optimization | Pre/post-workout meal suggestions based on training schedule | Integration Feature | BE/IA | — |
| RAG Nutrition Q&A | Ask: 'What foods are high in iron but low in calories?' — RAG engine | AI: RAG (Major) | IA | +2 |
| Meal Planning AI | Generate weekly meal plans that hit macro and calorie targets | AI Feature | IA | — |
| Grocery List Generator | Auto-generate a shopping list from the weekly meal plan | Core Feature | FE | — |
| Supplement Tracking | Log supplements: creatine, protein powder, vitamins, etc. | Core Feature | FE | — |

**Nutrition Level 2 Additional Points:** +2 pts

### 4.3 Enhanced Platform Features — Level 2

| Feature | Description | Module Mapping | Tag | Pts |
|---------|-------------|----------------|-----|-----|
| OAuth 2.0 (Google / Apple) | Sign in with Google or Apple ID | Integration | BE | — |
| Progress Prediction AI | AI predicts when you will reach your weight or strength targets | AI Feature | IA | — |
| User Activity Dashboard (Admin) | Admin dashboard: user engagement and retention metrics | User Mgmt: Analytics (Minor) | BE | +1 |
| RTL Support (Arabic) | Full Arabic language with right-to-left layout | i18n: RTL (Minor) | FE | +1 |
| Browser Compatibility | Tested and optimized for Firefox, Safari, and Edge | Web: Browsers (Minor) | FE | +1 |

**Platform Level 2 Additional Points:** +3 pts

### 4.4 Cumulative Level 1 + Level 2 — Points Breakdown

| Category | Key Features | FE pts | BE pts | IA pts | Total |
|----------|--------------|--------|--------|--------|-------|
| Workout L1 (base) | 10 core features | 3 | 2 | 0 | 6+ |
| Workout L2 (+) | Gamification, AI Program Generator, Analytics | 2 | 0 | 2 | +2 |
| Nutrition L1 (base) | 10 core features | 0 | 0 | 2 | 2+ |
| Nutrition L2 (+) | RAG Q&A, Adaptive TDEE, Meal Planning AI | 0 | 0 | 2 | +2 |
| Platform L1 (base) | 9 core features | 2 | 5 | 0 | 10+ |
| Platform L2 (+) | Activity Dashboard, RTL, Browser Compatibility | 2 | 1 | 0 | +3 |
| **LEVEL 2 TOTAL** | | **9 pts** | **8 pts** | **6 pts** | **~28 pts** |

---

## 05 | WORKOUT-NUTRITION INTEGRATION STRATEGY

The key differentiator of UM6P_FIT is how workout and nutrition data intelligently communicate with each other — this separates it from single-purpose apps.

### 5.1 Smart Calorie Adjustments

| Trigger | Automatic Action | Impact |
|---------|-----------------|--------|
| Leg day logged | +200 kcal (mostly carbs) automatically | +200 kcal |
| No workout today | -200 kcal, protein target maintained | -200 kcal |
| High volume week (20+ sets/muscle) | +15% of total daily calories | +15% |
| 30 min cardio logged | +300 kcal added to the day's target | +300 kcal |

### 5.2 Performance Insights

- **Protein-Strength Correlation:** 'Your bench press increased 15% after raising protein from 100g to 150g'
- **Carb-Performance Analysis:** 'Your best leg workouts happen when you eat 200g+ carbs the day before'
- **Recovery Warning:** 'You lifted heavy 4 days in a row and hit only 60% protein target — consider a rest day'
- **Goal Alignment Check:** 'You want to lose weight but are in a 500 kcal surplus — adjust nutrition or change goal'

### 5.3 Unified Dashboard

Users access a single dashboard displaying both workout and nutrition data simultaneously:

| Dashboard Block | Content |
|-----------------|---------|
| **Today's Summary** | Workout completed (Pull Day), 2,150/2,300 kcal consumed, 140/150g protein |
| **Weekly Trends** | 5 workouts, avg. 18 sets/session, avg. 2,100 kcal/day, 135g protein/day |
| **Progress Indicators** | Weight: -1.2 kg this month, Bench Press: +5 kg, Body fat estimate: 16% |
| **Alerts** | 'Low protein 3 days this week' \| 'No leg workout in 7 days' \| 'Consider a rest day' |
| **Achievements** | '30-day logging streak' \| 'Hit 150g protein 20/30 days' \| 'Squatted 100 kg PR' |

---

## 06 | OPEN PROPOSITIONS — FEATURES TO CHOOSE

The following 4 features were removed from the original scope: Form Check AI, Voice Workout Logging, AI Photo Food Logging, Sentiment Analysis. The table below lists 10 alternative candidate features as replacements. Select the ones that best fit your team's capacity and product vision.

**Priority Legend:**  
- ★★★ High priority — strongly recommended
- ★★ Medium priority — good addition
- ★ Lower priority — optional

| # | Feature | Description | Module Mapping | Tag | Pts | Priority |
|---|---------|-------------|----------------|-----|-----|----------|
| P1 | Workout Streak & Recovery Score | Daily score combining training consistency, rest days taken and sleep quality. More meaningful than raw gamification. | AI: LLM + Core Feature | IA | — | ★★★ |
| P2 | In-App Workout Video Guidance | Admin uploads short instructional clips per exercise. User sees the video inline while logging. | File Upload (Minor) | FE | +1 | ★★★ |
| P3 | Smart Meal Swap Suggestions | When a food is logged, AI proposes nutritionally equivalent alternatives that better fit the user's current macro gap. | AI: Recommendation System | BE/IA | — | ★★★ |
| P4 | Body Measurement Tracker | Log chest/waist/arms/legs over time with visual progress charts. Complements weight logging. | Core Feature | FE/BE | — | ★★ |
| P5 | AI Workout Notes Summarizer | After each session, AI generates a plain-text summary: 'Increased bench +5 kg, volume up 12%, rest times stable'. | AI: LLM Interface (Minor) | IA | — | ★★ |
| P6 | Food Diary Reminder | Smart daily notification at meal times if no meal has been logged yet. Learns from the user's usual logging patterns. | Web: Notifications Extension | BE | — | ★★ |
| P7 | Peer Challenge System | One-on-one challenges (most workouts, most protein hit, etc.) between two users — no public newsfeed. | Web: User Interaction (Minor) | FE/BE | +1 | ★ |
| P8 | Custom Exercise Creator | Users can create their own exercises with name, muscle group, equipment, instructions. Shareable with admin. | Database Feature | BE | — | ★★ |
| P9 | Hydration Tracker | Log daily water intake against a TDEE-adjusted target. Visual cup-fill UI. Hydration affects performance and recovery. | Core Feature | FE | — | ★★ |
| P10 | Weekly Progress Report (Push/Email) | Auto-generated weekly summary: workouts done, macro adherence %, weight trend, top PR. Uses existing notification system. | AI: LLM + Notifications | BE/IA | — | ★★★ |

---

## FINAL VERDICT

Commit to Level 1 (~18 pts) with a strict 50/50 workout-nutrition balance. Shipping a solid, fully functional product takes priority over rushing advanced features. **Quality > Quantity.** Only pursue Level 2 (~28 pts) if Level 1 ships by Day 33. For the 4 removed features, select replacements from the Propositions table above.

---

**Product Management Team**  
February 2026
