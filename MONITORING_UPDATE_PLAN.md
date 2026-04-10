# Monitoring Update Plan — Fitness Tracker

## ✅ Implementation Complete

### What Was Done

#### Phase 1: Wired Up Dead Metrics (P0)

All 4 business counters and 3 DB metrics were declared but never incremented. Now wired:

| Metric | File | Where |
|---|---|---|
| `fitness_users_registered_total` | `api/auth_handlers.go` | After successful registration |
| `fitness_workouts_created_total` | `api/workout_handlers.go` | After successful workout creation |
| `fitness_meals_logged_total` | `api/meal_handlers.go` | After successful meal creation |
| `fitness_weight_entries_logged_total` | `api/weight_entry_handlers.go` | After successful weight entry creation |
| DB query tracking | `metrics/gorm_callbacks.go` | GORM plugin registering before/after callbacks for all CRUD ops |
| DB connection pool gauges | `metrics/metrics.go` | `TrackDBConnStats` periodic goroutine updates InUse/Idle/MaxOpen |

GORM plugin registered in `api/server.go` — `NewServer()` now calls `metrics.NewGORMCallbackPlugin(m).Initialize(db)` and starts the DB stats tracker.

#### Phase 2: Added New Metric Categories

**Auth metrics** (`metrics/metrics.go`):
- `fitness_auth_attempts_total{method,result}` — login/register success/failure
- `fitness_auth_token_refreshes_total` — token refresh count
- `fitness_2fa_actions_total{action}` — setup/verify/disable success/failure
- `fitness_active_sessions` — gauge for active session count (future use)

**Chat/AI Coach metrics**:
- `fitness_chat_messages_total` — total chat messages
- `fitness_coach_requests_total{result}` — success/error
- `fitness_coach_request_duration_seconds{model}` — LLM latency

**Export metrics**:
- `fitness_export_jobs_created_total{format}` — created by format
- `fitness_export_jobs_completed_total{format}` — completed by format
- `fitness_export_jobs_failed_total{format}` — failed by format
- `fitness_export_duration_seconds{format}` — processing time

**Worker metrics**:
- `fitness_worker_poll_cycles_total{task_type}` — poll cycle counts
- `fitness_worker_poll_errors_total{task_type}` — poll error counts

**Notification metrics**:
- `fitness_notifications_created_total{type}` — created by notification type
- `fitness_notifications_sent_total` — delivery count

**External service metrics**:
- `fitness_ext_service_requests_total{service,method,status}` — exercise-lib proxy calls
- `fitness_ext_service_duration_seconds{service}` — external call latency
- `fitness_ext_service_errors_total{service,error_type}` — error tracking

#### Phase 3: Worker Observability (P0)

- Worker now creates its own `metrics.New()` instance
- Exposes `/metrics` on `:9091` (configurable via `WORKER_METRICS_PORT`)
- Also has `/healthz` endpoint
- GORM plugin registered for worker DB queries
- DB connection pool stats tracked
- All worker poll cycles and errors tracked with `WorkerPollCycles` / `WorkerPollErrors`

#### Phase 4: Prometheus Config Updated

- Added `fitness-worker` scrape target (`worker:9091`)
- Added `exercise-lib` scrape target (`exercise-lib:8000`)
- Added `alerts.yml` rule file reference
- Alert rules cover: High error rate, high latency, DB pool exhaustion, slow queries, no registrations, high login failures, worker export errors, stuck export jobs, exercise-lib errors, coach error rate

#### Phase 5: Grafana Dashboard Updated

Main dashboard now has 4 row sections:
1. **📡 HTTP Overview** — req/s, P95 latency, status codes, latency percentiles, in-flight
2. **💼 Business Metrics** — total counts + event rate for workouts/meals/weight/registrations, auth attempts
3. **🗄️ Database** — query rate by operation, P95 query latency, connection pool (in-use/idle/max)
4. **🤖 AI Coach & Exports** — coach request rate, export job stats, notifications created
5. **⚙️ Runtime** — Go memory, goroutines

#### Phase 6: docker-compose.yml Updates

- Worker container exposes port 9091
- `WORKER_METRICS_PORT` env variable added
- Prometheus mounts `alerts.yml` volume
- Worker scrape target added

### Files Created

| File | Description |
|---|---|
| `metrics/gorm_callbacks.go` | GORM Prometheus plugin for DB query metrics |
| `metrics/worker.go` | Worker metrics HTTP server helper |
| `monitoring/prometheus/alerts.yml` | Alert rules for all monitored scenarios |

### Files Modified

| File | Change |
|---|---|
| `metrics/metrics.go` | Added 20+ new metric families (auth, chat, export, worker, notification, external, DB pool) |
| `api/server.go` | Register GORM plugin, start DB stats tracker, pass metrics to services |
| `api/auth_handlers.go` | Wire auth attempts, registration, 2FA, token refresh metrics |
| `api/workout_handlers.go` | Wire `WorkoutsCreated` counter |
| `api/meal_handlers.go` | Wire `MealsLogged` counter |
| `api/weight_entry_handlers.go` | Wire `WeightEntriesLogged` counter |
| `api/chat_handlers.go` | Wire chat messages, coach requests/duration metrics |
| `api/export_handlers.go` | Wire export job creation counter |
| `services/notification.go` | Accept metrics, wire `NotificationsCreated` |
| `services/notification_automation.go` | Add `NewNotificationAutomationServiceWithMetrics` |
| `services/exports.go` | Accept metrics, wire export job completed/failed counters |
| `services/exercise_lib_client.go` | Accept metrics, wire ext service request/duration/error metrics |
| `worker/runner.go` | Create metrics, start HTTP server, track poll cycles/errors |
| `monitoring/prometheus/prometheus.yml` | Add worker + exercise-lib scrape targets, alert rules file |
| `monitoring/grafana/.../fitness-tracker.json` | Full dashboard redesign with rows for HTTP, Business, DB, AI Coach, Runtime |
| `docker-compose.yml` | Worker port 9091, metrics env, Prometheus alerts volume |