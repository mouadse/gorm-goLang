import urllib.request
import urllib.error
import json
import uuid
import datetime
import sys

BASE_URL = "http://localhost:8080"
API_URL = f"{BASE_URL}/v1"

tests_run = 0
tests_passed = 0
tests_failed_names = []


class APIClient:
    """Thin HTTP client that tracks test results."""

    def __init__(self):
        self.auth_token = None

    def request(self, method, url, data=None, headers=None):
        """Send an HTTP request and return (status_code, parsed_body | None)."""
        req = urllib.request.Request(url, method=method)
        headers = headers or {}
        if data is not None:
            json_data = json.dumps(data).encode("utf-8")
            headers.setdefault("Content-Type", "application/json")
            headers.setdefault("Content-Length", str(len(json_data)))
            req.data = json_data
        if self.auth_token and "Authorization" not in headers:
            headers["Authorization"] = f"Bearer {self.auth_token}"
        for key, value in headers.items():
            req.add_header(key, value)

        try:
            response = urllib.request.urlopen(req)
            body = response.read().decode("utf-8")
            return response.status, json.loads(body) if body else None
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8")
            try:
                parsed = json.loads(body)
            except Exception:
                parsed = body
            return e.code, parsed
        except Exception as e:
            return None, str(e)

    def run_test(self, name, method, url, data=None, expect_status=None, headers=None):
        """Run a single test, print result, and track pass/fail."""
        global tests_run, tests_passed, tests_failed_names
        tests_run += 1

        status, body = self.request(method, url, data, headers=headers)
        passed = True
        reason = ""

        if expect_status is not None:
            if status != expect_status:
                passed = False
                reason = f"expected status {expect_status}, got {status}"
        elif status is None:
            passed = False
            reason = f"connection error: {body}"
        elif status >= 400:
            passed = False
            reason = f"unexpected error status {status}"

        if passed:
            tests_passed += 1
            print(f"  [{tests_run}] ✅ {name}")
        else:
            tests_failed_names.append(name)
            print(f"  [{tests_run}] ❌ {name}")
            if reason:
                print(f"       -> {reason}")
            if body and isinstance(body, dict):
                print(f"       -> body: {json.dumps(body)}")

        return status, body


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
FAKE_UUID = "00000000-0000-0000-0000-000000000000"
NONEXISTENT_UUID = str(uuid.uuid4())


def main():
    client = APIClient()

    # -----------------------------------------------------------------------
    # 0. Health check
    # -----------------------------------------------------------------------
    print("\n=== Health Check ===")
    status, _ = client.run_test("Health Check", "GET", f"{BASE_URL}/healthz", expect_status=200)
    if status != 200:
        print("Server is not running. Aborting.")
        sys.exit(1)

    uid = str(uuid.uuid4())
    today = datetime.datetime.now().strftime("%Y-%m-%d")

    # -----------------------------------------------------------------------
    # 1. Users
    # -----------------------------------------------------------------------
    print("\n=== Auth - Happy Path ===")
    email = f"testuser_{uid}@example.com"
    register_data = {
        "email": email,
        "password": "password123",
        "name": "Test User",
        "avatar": "http://example.com/avatar.jpg",
        "age": 25,
        "date_of_birth": "1999-01-01",
        "weight": 75.5,
        "height": 180.0,
        "goal": "Build muscle",
        "activity_level": "active",
        "tdee": 2500,
    }

    _, register_response = client.run_test("Register", "POST", f"{API_URL}/auth/register", data=register_data, expect_status=201)
    if not register_response or "user" not in register_response or "token" not in register_response:
        print("Cannot proceed without a registered user and token. Aborting.")
        sys.exit(1)

    client.auth_token = register_response["token"]
    user = register_response["user"]
    user_id = user["id"]

    client.run_test("Login", "POST", f"{API_URL}/auth/login", data={
        "email": email,
        "password": "password123",
    }, expect_status=200)
    client.run_test("Login wrong password", "POST", f"{API_URL}/auth/login", data={
        "email": email,
        "password": "wrongpass123",
    }, expect_status=401)

    anon_client = APIClient()
    anon_client.run_test("Protected route without token", "GET", f"{API_URL}/users", expect_status=401)

    if not user or "id" not in user:
        print("Cannot proceed without a user. Aborting.")
        sys.exit(1)

    print("\n=== Users - Happy Path ===")

    client.run_test("Get User", "GET", f"{API_URL}/users/{user_id}", expect_status=200)
    client.run_test("List Users", "GET", f"{API_URL}/users", expect_status=200)
    client.run_test("Update User name", "PATCH", f"{API_URL}/users/{user_id}", data={"name": "Updated User"}, expect_status=200)
    client.run_test("Update User multiple fields", "PATCH", f"{API_URL}/users/{user_id}", data={
        "weight": 80.0, "height": 185.0, "goal": "Lose fat", "activity_level": "moderate", "tdee": 2200, "age": 26,
    }, expect_status=200)
    client.run_test("PATCH User empty body (no-op)", "PATCH", f"{API_URL}/users/{user_id}", data={}, expect_status=200)

    print("\n=== Users - Validation / Edge Cases ===")
    client.run_test("Create user blank email", "POST", f"{API_URL}/users", data={"email": "", "password": "password123", "name": "X"}, expect_status=400)
    client.run_test("Create user missing email", "POST", f"{API_URL}/users", data={"password": "password123", "name": "X"}, expect_status=400)
    client.run_test("Create user blank name", "POST", f"{API_URL}/users", data={"email": "a@b.com", "password": "password123", "name": ""}, expect_status=400)
    client.run_test("Create user missing name", "POST", f"{API_URL}/users", data={"email": "a@b.com", "password": "password123"}, expect_status=400)
    client.run_test("Create user short password", "POST", f"{API_URL}/users", data={"email": "short@b.com", "password": "short", "name": "Short"}, expect_status=400)
    client.run_test("Create user negative age", "POST", f"{API_URL}/users", data={"email": "neg@b.com", "password": "password123", "name": "N", "age": -1}, expect_status=400)
    client.run_test("Create user negative weight", "POST", f"{API_URL}/users", data={"email": "neg@b.com", "password": "password123", "name": "N", "weight": -5}, expect_status=400)
    client.run_test("Create user negative height", "POST", f"{API_URL}/users", data={"email": "neg@b.com", "password": "password123", "name": "N", "height": -1}, expect_status=400)
    client.run_test("Create user negative tdee", "POST", f"{API_URL}/users", data={"email": "neg@b.com", "password": "password123", "name": "N", "tdee": -100}, expect_status=400)
    client.run_test("Create user invalid date_of_birth", "POST", f"{API_URL}/users", data={"email": "bad@b.com", "password": "password123", "name": "N", "date_of_birth": "not-a-date"}, expect_status=400)
    client.run_test("Create user duplicate email", "POST", f"{API_URL}/users", data={"email": email, "password": "password123", "name": "Dup"}, expect_status=400)
    client.run_test("Get non-existent user", "GET", f"{API_URL}/users/{NONEXISTENT_UUID}", expect_status=403)
    client.run_test("Get user invalid UUID", "GET", f"{API_URL}/users/not-a-uuid", expect_status=400)
    client.run_test("Update non-existent user", "PATCH", f"{API_URL}/users/{NONEXISTENT_UUID}", data={"name": "X"}, expect_status=403)
    client.run_test("Delete non-existent user", "DELETE", f"{API_URL}/users/{NONEXISTENT_UUID}", expect_status=403)
    client.run_test("Update user blank name", "PATCH", f"{API_URL}/users/{user_id}", data={"name": "  "}, expect_status=400)
    client.run_test("Update user negative weight", "PATCH", f"{API_URL}/users/{user_id}", data={"weight": -1}, expect_status=400)
    client.run_test("Update user negative height", "PATCH", f"{API_URL}/users/{user_id}", data={"height": -1}, expect_status=400)
    client.run_test("Update user negative age", "PATCH", f"{API_URL}/users/{user_id}", data={"age": -5}, expect_status=400)
    client.run_test("Update user negative tdee", "PATCH", f"{API_URL}/users/{user_id}", data={"tdee": -10}, expect_status=400)
    client.run_test("Update user invalid DOB", "PATCH", f"{API_URL}/users/{user_id}", data={"date_of_birth": "bad"}, expect_status=400)

    print("\n=== Users - Query Filters ===")
    client.run_test("List users filter by email", "GET", f"{API_URL}/users?email={email}", expect_status=200)
    client.run_test("List users filter by name", "GET", f"{API_URL}/users?name=Updated", expect_status=200)

    # -----------------------------------------------------------------------
    # 2. Exercises
    # -----------------------------------------------------------------------
    print("\n=== Exercises – Happy Path ===")
    exercise_data = {
        "name": f"Test Exercise {uid}",
        "muscle_group": "Chest",
        "equipment": "Barbell",
        "difficulty": "Intermediate",
        "instructions": "Push the bar",
        "video_url": "http://youtube.com/something",
    }
    _, exercise = client.run_test("Create Exercise", "POST", f"{API_URL}/exercises", data=exercise_data, expect_status=201)
    if not exercise or "id" not in exercise:
        print("Cannot proceed without an exercise. Aborting.")
        sys.exit(1)
    exercise_id = exercise["id"]

    client.run_test("Get Exercise", "GET", f"{API_URL}/exercises/{exercise_id}", expect_status=200)
    client.run_test("List Exercises", "GET", f"{API_URL}/exercises", expect_status=200)
    client.run_test("Update Exercise difficulty", "PATCH", f"{API_URL}/exercises/{exercise_id}", data={"difficulty": "Advanced"}, expect_status=200)
    client.run_test("Update Exercise multiple", "PATCH", f"{API_URL}/exercises/{exercise_id}", data={
        "muscle_group": "Back", "equipment": "Dumbbell", "instructions": "Pull",
    }, expect_status=200)
    client.run_test("PATCH Exercise empty body", "PATCH", f"{API_URL}/exercises/{exercise_id}", data={}, expect_status=200)

    print("\n=== Exercises – Validation / Edge Cases ===")
    client.run_test("Create exercise blank name", "POST", f"{API_URL}/exercises", data={"name": ""}, expect_status=400)
    client.run_test("Create exercise missing name", "POST", f"{API_URL}/exercises", data={"muscle_group": "Legs"}, expect_status=400)
    client.run_test("Create duplicate exercise name", "POST", f"{API_URL}/exercises", data={"name": f"Test Exercise {uid}"}, expect_status=400)
    client.run_test("Get non-existent exercise", "GET", f"{API_URL}/exercises/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Get exercise invalid UUID", "GET", f"{API_URL}/exercises/not-a-uuid", expect_status=400)
    client.run_test("Update non-existent exercise", "PATCH", f"{API_URL}/exercises/{NONEXISTENT_UUID}", data={"difficulty": "X"}, expect_status=404)
    client.run_test("Delete non-existent exercise", "DELETE", f"{API_URL}/exercises/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update exercise blank name", "PATCH", f"{API_URL}/exercises/{exercise_id}", data={"name": "  "}, expect_status=400)

    print("\n=== Exercises – Query Filters ===")
    client.run_test("List exercises by muscle_group", "GET", f"{API_URL}/exercises?muscle_group=Back", expect_status=200)
    client.run_test("List exercises by difficulty", "GET", f"{API_URL}/exercises?difficulty=Advanced", expect_status=200)
    client.run_test("List exercises by equipment", "GET", f"{API_URL}/exercises?equipment=Dumbbell", expect_status=200)
    client.run_test("List exercises by name", "GET", f"{API_URL}/exercises?name=Test", expect_status=200)

    # -----------------------------------------------------------------------
    # 3. Weight Entries
    # -----------------------------------------------------------------------
    print("\n=== Weight Entries – Happy Path ===")
    weight_data = {
        "user_id": user_id,
        "weight": 76.0,
        "date": today,
        "notes": "Feeling good",
    }
    _, we = client.run_test("Create Weight Entry", "POST", f"{API_URL}/weight-entries", data=weight_data, expect_status=201)
    we_id = we["id"] if we and "id" in we else None

    if we_id:
        client.run_test("Get Weight Entry", "GET", f"{API_URL}/weight-entries/{we_id}", expect_status=200)
        client.run_test("List Weight Entries", "GET", f"{API_URL}/weight-entries", expect_status=200)
        client.run_test("List Weight Entries for User (path)", "GET", f"{API_URL}/users/{user_id}/weight-entries", expect_status=200)
        client.run_test("Update Weight Entry", "PATCH", f"{API_URL}/weight-entries/{we_id}", data={"weight": 75.8}, expect_status=200)
        client.run_test("Update Weight Entry notes", "PATCH", f"{API_URL}/weight-entries/{we_id}", data={"notes": "Updated"}, expect_status=200)

    print("\n=== Weight Entries – Validation / Edge Cases ===")
    client.run_test("Create weight entry weight=0", "POST", f"{API_URL}/weight-entries", data={"user_id": user_id, "weight": 0, "date": today}, expect_status=400)
    client.run_test("Create weight entry weight=-1", "POST", f"{API_URL}/weight-entries", data={"user_id": user_id, "weight": -1, "date": today}, expect_status=400)
    client.run_test("Create weight entry non-existent user", "POST", f"{API_URL}/weight-entries", data={"user_id": NONEXISTENT_UUID, "weight": 70, "date": today}, expect_status=403)
    client.run_test("Create weight entry invalid date", "POST", f"{API_URL}/weight-entries", data={"user_id": user_id, "weight": 70, "date": "bad-date"}, expect_status=400)
    client.run_test("Create weight entry missing user_id", "POST", f"{API_URL}/weight-entries", data={"weight": 70, "date": today}, expect_status=400)
    client.run_test("Create weight entry duplicate date", "POST", f"{API_URL}/weight-entries", data={"user_id": user_id, "weight": 77, "date": today}, expect_status=400)
    client.run_test("Get non-existent weight entry", "GET", f"{API_URL}/weight-entries/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Delete non-existent weight entry", "DELETE", f"{API_URL}/weight-entries/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update weight entry weight=0", "PATCH", f"{API_URL}/weight-entries/{we_id}", data={"weight": 0}, expect_status=400)
    client.run_test("Update weight entry weight=-1", "PATCH", f"{API_URL}/weight-entries/{we_id}", data={"weight": -5}, expect_status=400)
    client.run_test("Update weight entry invalid date", "PATCH", f"{API_URL}/weight-entries/{we_id}", data={"date": "nope"}, expect_status=400)
    client.run_test("Update weight entry non-existent user", "PATCH", f"{API_URL}/weight-entries/{we_id}", data={"user_id": NONEXISTENT_UUID}, expect_status=404)
    client.run_test("Update non-existent weight entry", "PATCH", f"{API_URL}/weight-entries/{NONEXISTENT_UUID}", data={"weight": 80}, expect_status=404)

    print("\n=== Weight Entries – Query Filters ===")
    client.run_test("List weight entries by date", "GET", f"{API_URL}/weight-entries?date={today}", expect_status=200)
    client.run_test("List weight entries by user_id", "GET", f"{API_URL}/weight-entries?user_id={user_id}", expect_status=200)
    client.run_test("List weight entries by start_date", "GET", f"{API_URL}/weight-entries?start_date={today}", expect_status=200)
    client.run_test("List weight entries by end_date", "GET", f"{API_URL}/weight-entries?end_date={today}", expect_status=200)
    client.run_test("List weight entries date range", "GET", f"{API_URL}/weight-entries?start_date=2020-01-01&end_date=2030-12-31", expect_status=200)

    # Clean up the weight entry
    if we_id:
        client.run_test("Delete Weight Entry", "DELETE", f"{API_URL}/weight-entries/{we_id}", expect_status=204)

    # Scoped creation
    print("\n=== Weight Entries – Scoped Endpoint ===")
    _, we2 = client.run_test("Create Weight Entry via scoped path", "POST",
                              f"{API_URL}/users/{user_id}/weight-entries",
                              data={"weight": 74, "date": "2025-06-01"}, expect_status=201)
    if we2 and "id" in we2:
        client.run_test("Delete scoped weight entry", "DELETE", f"{API_URL}/weight-entries/{we2['id']}", expect_status=204)

    # -----------------------------------------------------------------------
    # 4. Workouts
    # -----------------------------------------------------------------------
    print("\n=== Workouts – Happy Path ===")
    workout_data = {
        "user_id": user_id,
        "date": today,
        "duration": 60,
        "notes": "Good session",
        "type": "Strength",
        "exercises": [],
    }
    _, workout = client.run_test("Create Workout", "POST", f"{API_URL}/workouts", data=workout_data, expect_status=201)
    if not workout or "id" not in workout:
        print("Cannot proceed without a workout. Aborting.")
        sys.exit(1)
    workout_id = workout["id"]

    client.run_test("Get Workout", "GET", f"{API_URL}/workouts/{workout_id}", expect_status=200)
    client.run_test("List Workouts", "GET", f"{API_URL}/workouts", expect_status=200)
    client.run_test("List Workouts for User", "GET", f"{API_URL}/users/{user_id}/workouts", expect_status=200)
    client.run_test("Update Workout duration", "PATCH", f"{API_URL}/workouts/{workout_id}", data={"duration": 70}, expect_status=200)
    client.run_test("Update Workout notes+type", "PATCH", f"{API_URL}/workouts/{workout_id}", data={"notes": "Updated", "type": "Cardio"}, expect_status=200)
    client.run_test("PATCH Workout empty body", "PATCH", f"{API_URL}/workouts/{workout_id}", data={}, expect_status=200)

    print("\n=== Workouts – Validation / Edge Cases ===")
    client.run_test("Create workout negative duration", "POST", f"{API_URL}/workouts", data={"user_id": user_id, "date": today, "duration": -10}, expect_status=400)
    client.run_test("Create workout non-existent user", "POST", f"{API_URL}/workouts", data={"user_id": NONEXISTENT_UUID, "date": today, "duration": 30}, expect_status=403)
    client.run_test("Create workout missing user_id", "POST", f"{API_URL}/workouts", data={"date": today, "duration": 30}, expect_status=400)
    client.run_test("Create workout invalid date", "POST", f"{API_URL}/workouts", data={"user_id": user_id, "date": "bad"}, expect_status=400)
    client.run_test("Get non-existent workout", "GET", f"{API_URL}/workouts/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update non-existent workout", "PATCH", f"{API_URL}/workouts/{NONEXISTENT_UUID}", data={"duration": 10}, expect_status=404)
    client.run_test("Delete non-existent workout", "DELETE", f"{API_URL}/workouts/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update workout negative duration", "PATCH", f"{API_URL}/workouts/{workout_id}", data={"duration": -5}, expect_status=400)
    client.run_test("Update workout invalid date", "PATCH", f"{API_URL}/workouts/{workout_id}", data={"date": "nope"}, expect_status=400)
    client.run_test("Update workout non-existent user", "PATCH", f"{API_URL}/workouts/{workout_id}", data={"user_id": NONEXISTENT_UUID}, expect_status=404)

    print("\n=== Workouts – Query Filters ===")
    client.run_test("List workouts by user_id", "GET", f"{API_URL}/workouts?user_id={user_id}", expect_status=200)
    client.run_test("List workouts by type", "GET", f"{API_URL}/workouts?type=Cardio", expect_status=200)
    client.run_test("List workouts by date", "GET", f"{API_URL}/workouts?date={today}", expect_status=200)

    # Scoped creation
    print("\n=== Workouts – Scoped Endpoint ===")
    _, scoped_workout = client.run_test("Create Workout via scoped path", "POST",
                                         f"{API_URL}/users/{user_id}/workouts",
                                         data={"date": today, "duration": 45, "type": "push"}, expect_status=201)
    scoped_workout_id = scoped_workout["id"] if scoped_workout and "id" in scoped_workout else None

    # Nested creation with exercises + sets
    print("\n=== Workouts – Nested Creation ===")
    _, nested_workout = client.run_test("Create Workout with inline exercises",
                                         "POST", f"{API_URL}/workouts",
                                         data={
                                             "user_id": user_id,
                                             "date": "2025-07-01",
                                             "duration": 90,
                                             "type": "push",
                                             "exercises": [
                                                 {
                                                     "exercise_id": exercise_id,
                                                     "order": 1,
                                                     "sets": 3,
                                                     "reps": 10,
                                                     "weight": 100,
                                                     "rest_time": 90,
                                                     "set_entries": [
                                                         {"set_number": 1, "reps": 10, "weight": 100, "rpe": 8, "completed": True},
                                                         {"set_number": 2, "reps": 8, "weight": 100, "rpe": 9, "completed": True},
                                                         {"set_number": 3, "reps": 6, "weight": 100, "rpe": 10, "completed": False},
                                                     ],
                                                 }
                                             ],
                                         }, expect_status=201)
    nested_workout_id = nested_workout["id"] if nested_workout and "id" in nested_workout else None

    # -----------------------------------------------------------------------
    # 5. Workout Exercises
    # -----------------------------------------------------------------------
    print("\n=== Workout Exercises – Happy Path ===")
    we_ex_data = {
        "workout_id": workout_id,
        "exercise_id": exercise_id,
        "order": 1,
        "sets": 3,
        "reps": 10,
        "weight": 100.0,
        "rest_time": 90,
        "notes": "Heavy",
        "set_entries": [],
    }
    _, workout_exercise = client.run_test("Create Workout Exercise", "POST", f"{API_URL}/workout-exercises", data=we_ex_data, expect_status=201)
    if not workout_exercise or "id" not in workout_exercise:
        print("Cannot proceed without a workout exercise. Aborting.")
        sys.exit(1)
    we_ex_id = workout_exercise["id"]

    client.run_test("Get Workout Exercise", "GET", f"{API_URL}/workout-exercises/{we_ex_id}", expect_status=200)
    client.run_test("List Workout Exercises", "GET", f"{API_URL}/workout-exercises", expect_status=200)
    client.run_test("List Workout Exercises for Workout", "GET", f"{API_URL}/workouts/{workout_id}/exercises", expect_status=200)
    client.run_test("Update Workout Exercise reps", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"reps": 12}, expect_status=200)
    client.run_test("Update Workout Exercise multiple", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={
        "sets": 4, "weight": 110.0, "rest_time": 120, "notes": "Heavier"
    }, expect_status=200)

    print("\n=== Workout Exercises – Validation / Edge Cases ===")
    client.run_test("Create WE non-existent workout", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": NONEXISTENT_UUID, "exercise_id": exercise_id, "order": 1, "sets": 3, "reps": 10, "weight": 50,
    }, expect_status=404)
    client.run_test("Create WE non-existent exercise", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": workout_id, "exercise_id": NONEXISTENT_UUID, "order": 1, "sets": 3, "reps": 10, "weight": 50,
    }, expect_status=404)
    client.run_test("Create WE missing workout_id", "POST", f"{API_URL}/workout-exercises", data={
        "exercise_id": exercise_id, "order": 1, "sets": 3, "reps": 10, "weight": 50,
    }, expect_status=400)
    client.run_test("Create WE missing exercise_id", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": workout_id, "order": 1, "sets": 3, "reps": 10, "weight": 50,
    }, expect_status=400)
    client.run_test("Create WE negative sets", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": workout_id, "exercise_id": exercise_id, "order": 1, "sets": -1, "reps": 10, "weight": 50,
    }, expect_status=400)
    client.run_test("Create WE negative reps", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": workout_id, "exercise_id": exercise_id, "order": 1, "sets": 3, "reps": -1, "weight": 50,
    }, expect_status=400)
    client.run_test("Create WE negative weight", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": workout_id, "exercise_id": exercise_id, "order": 1, "sets": 3, "reps": 10, "weight": -50,
    }, expect_status=400)
    client.run_test("Create WE negative rest_time", "POST", f"{API_URL}/workout-exercises", data={
        "workout_id": workout_id, "exercise_id": exercise_id, "order": 1, "sets": 3, "reps": 10, "weight": 50, "rest_time": -10,
    }, expect_status=400)
    client.run_test("Get non-existent workout exercise", "GET", f"{API_URL}/workout-exercises/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update non-existent WE", "PATCH", f"{API_URL}/workout-exercises/{NONEXISTENT_UUID}", data={"reps": 5}, expect_status=404)
    client.run_test("Delete non-existent WE", "DELETE", f"{API_URL}/workout-exercises/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update WE negative sets", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"sets": -1}, expect_status=400)
    client.run_test("Update WE negative reps", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"reps": -1}, expect_status=400)
    client.run_test("Update WE negative weight", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"weight": -1}, expect_status=400)
    client.run_test("Update WE negative rest_time", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"rest_time": -1}, expect_status=400)
    client.run_test("Update WE order=0", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"order": 0}, expect_status=400)
    client.run_test("Update WE order=-1", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"order": -1}, expect_status=400)
    client.run_test("Update WE non-existent exercise_id", "PATCH", f"{API_URL}/workout-exercises/{we_ex_id}", data={"exercise_id": NONEXISTENT_UUID}, expect_status=404)

    print("\n=== Workout Exercises – Query Filters ===")
    client.run_test("List WE by workout_id query", "GET", f"{API_URL}/workout-exercises?workout_id={workout_id}", expect_status=200)
    client.run_test("List WE by exercise_id query", "GET", f"{API_URL}/workout-exercises?exercise_id={exercise_id}", expect_status=200)

    # Scoped creation
    print("\n=== Workout Exercises – Scoped Endpoint ===")
    _, scoped_we = client.run_test("Add WE via scoped path", "POST",
                                    f"{API_URL}/workouts/{workout_id}/exercises",
                                    data={"exercise_id": exercise_id, "order": 2, "sets": 2, "reps": 8, "weight": 60},
                                    expect_status=201)
    scoped_we_id = scoped_we["id"] if scoped_we and "id" in scoped_we else None

    # -----------------------------------------------------------------------
    # 6. Workout Sets
    # -----------------------------------------------------------------------
    print("\n=== Workout Sets – Happy Path ===")
    wset_data = {
        "workout_exercise_id": we_ex_id,
        "set_number": 1,
        "reps": 10,
        "weight": 100.0,
        "rpe": 8.5,
        "rest_seconds": 90,
        "completed": True,
    }
    _, wset = client.run_test("Create Workout Set", "POST", f"{API_URL}/workout-sets", data=wset_data, expect_status=201)
    wset_id = wset["id"] if wset and "id" in wset else None

    if wset_id:
        client.run_test("Get Workout Set", "GET", f"{API_URL}/workout-sets/{wset_id}", expect_status=200)
        client.run_test("List Workout Sets", "GET", f"{API_URL}/workout-sets", expect_status=200)
        client.run_test("List Workout Sets for Exercise", "GET", f"{API_URL}/workout-exercises/{we_ex_id}/sets", expect_status=200)
        client.run_test("Update Workout Set completed", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"completed": False}, expect_status=200)
        client.run_test("Update Workout Set reps+weight", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"reps": 12, "weight": 105}, expect_status=200)

    print("\n=== Workout Sets – Validation / Edge Cases ===")
    client.run_test("Create set negative reps", "POST", f"{API_URL}/workout-sets", data={
        "workout_exercise_id": we_ex_id, "set_number": 99, "reps": -1, "weight": 50,
    }, expect_status=400)
    client.run_test("Create set negative weight", "POST", f"{API_URL}/workout-sets", data={
        "workout_exercise_id": we_ex_id, "set_number": 99, "reps": 10, "weight": -50,
    }, expect_status=400)
    client.run_test("Create set negative RPE", "POST", f"{API_URL}/workout-sets", data={
        "workout_exercise_id": we_ex_id, "set_number": 99, "reps": 10, "weight": 50, "rpe": -1,
    }, expect_status=400)
    client.run_test("Create set negative rest_seconds", "POST", f"{API_URL}/workout-sets", data={
        "workout_exercise_id": we_ex_id, "set_number": 99, "reps": 10, "weight": 50, "rest_seconds": -1,
    }, expect_status=400)
    client.run_test("Create set non-existent WE", "POST", f"{API_URL}/workout-sets", data={
        "workout_exercise_id": NONEXISTENT_UUID, "set_number": 1, "reps": 10, "weight": 50,
    }, expect_status=404)
    client.run_test("Create set missing WE id", "POST", f"{API_URL}/workout-sets", data={
        "set_number": 1, "reps": 10, "weight": 50,
    }, expect_status=400)
    client.run_test("Get non-existent set", "GET", f"{API_URL}/workout-sets/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update non-existent set", "PATCH", f"{API_URL}/workout-sets/{NONEXISTENT_UUID}", data={"reps": 5}, expect_status=404)
    client.run_test("Delete non-existent set", "DELETE", f"{API_URL}/workout-sets/{NONEXISTENT_UUID}", expect_status=404)
    if wset_id:
        client.run_test("Update set set_number=0", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"set_number": 0}, expect_status=400)
        client.run_test("Update set set_number=-1", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"set_number": -1}, expect_status=400)
        client.run_test("Update set negative reps", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"reps": -1}, expect_status=400)
        client.run_test("Update set negative weight", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"weight": -1}, expect_status=400)
        client.run_test("Update set negative RPE", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"rpe": -1}, expect_status=400)
        client.run_test("Update set negative rest_seconds", "PATCH", f"{API_URL}/workout-sets/{wset_id}", data={"rest_seconds": -1}, expect_status=400)

    print("\n=== Workout Sets – Query Filters ===")
    client.run_test("List sets by WE query", "GET", f"{API_URL}/workout-sets?workout_exercise_id={we_ex_id}", expect_status=200)

    # Scoped set creation
    print("\n=== Workout Sets – Scoped Endpoint ===")
    _, scoped_set = client.run_test("Create set via scoped path", "POST",
                                     f"{API_URL}/workout-exercises/{we_ex_id}/sets",
                                     data={"set_number": 2, "reps": 8, "weight": 95, "rpe": 9, "completed": True},
                                     expect_status=201)
    scoped_set_id = scoped_set["id"] if scoped_set and "id" in scoped_set else None

    # -----------------------------------------------------------------------
    # 7. Meals
    # -----------------------------------------------------------------------
    print("\n=== Meals – Happy Path ===")
    meal_data = {
        "user_id": user_id,
        "meal_type": "Breakfast",
        "date": today,
        "notes": "Oatmeal and eggs",
    }
    _, meal = client.run_test("Create Meal", "POST", f"{API_URL}/meals", data=meal_data, expect_status=201)
    meal_id = meal["id"] if meal and "id" in meal else None

    if meal_id:
        client.run_test("Get Meal", "GET", f"{API_URL}/meals/{meal_id}", expect_status=200)
        client.run_test("List Meals", "GET", f"{API_URL}/meals", expect_status=200)
        client.run_test("List Meals for User", "GET", f"{API_URL}/users/{user_id}/meals", expect_status=200)
        client.run_test("Update Meal notes", "PATCH", f"{API_URL}/meals/{meal_id}", data={"notes": "Just oatmeal"}, expect_status=200)
        client.run_test("Update Meal type+date", "PATCH", f"{API_URL}/meals/{meal_id}", data={"meal_type": "Lunch", "date": "2025-06-15"}, expect_status=200)

    # Same type same date → should succeed (non-unique index)
    _, meal2 = client.run_test("Create duplicate meal type/date (allowed)", "POST", f"{API_URL}/meals", data={
        "user_id": user_id, "meal_type": "Breakfast", "date": today,
    }, expect_status=201)
    meal2_id = meal2["id"] if meal2 and "id" in meal2 else None

    print("\n=== Meals – Validation / Edge Cases ===")
    client.run_test("Create meal blank meal_type", "POST", f"{API_URL}/meals", data={"user_id": user_id, "meal_type": "", "date": today}, expect_status=400)
    client.run_test("Create meal missing meal_type", "POST", f"{API_URL}/meals", data={"user_id": user_id, "date": today}, expect_status=400)
    client.run_test("Create meal non-existent user", "POST", f"{API_URL}/meals", data={"user_id": NONEXISTENT_UUID, "meal_type": "Dinner", "date": today}, expect_status=403)
    client.run_test("Create meal invalid date", "POST", f"{API_URL}/meals", data={"user_id": user_id, "meal_type": "Dinner", "date": "bad"}, expect_status=400)
    client.run_test("Create meal missing user_id", "POST", f"{API_URL}/meals", data={"meal_type": "Dinner", "date": today}, expect_status=400)
    client.run_test("Get non-existent meal", "GET", f"{API_URL}/meals/{NONEXISTENT_UUID}", expect_status=404)
    client.run_test("Update non-existent meal", "PATCH", f"{API_URL}/meals/{NONEXISTENT_UUID}", data={"notes": "x"}, expect_status=404)
    client.run_test("Delete non-existent meal", "DELETE", f"{API_URL}/meals/{NONEXISTENT_UUID}", expect_status=404)
    if meal_id:
        client.run_test("Update meal blank meal_type", "PATCH", f"{API_URL}/meals/{meal_id}", data={"meal_type": "  "}, expect_status=400)
        client.run_test("Update meal invalid date", "PATCH", f"{API_URL}/meals/{meal_id}", data={"date": "nope"}, expect_status=400)
        client.run_test("Update meal non-existent user", "PATCH", f"{API_URL}/meals/{meal_id}", data={"user_id": NONEXISTENT_UUID}, expect_status=404)

    print("\n=== Meals – Query Filters ===")
    client.run_test("List meals by user_id", "GET", f"{API_URL}/meals?user_id={user_id}", expect_status=200)
    client.run_test("List meals by meal_type", "GET", f"{API_URL}/meals?meal_type=Breakfast", expect_status=200)
    client.run_test("List meals by date", "GET", f"{API_URL}/meals?date={today}", expect_status=200)

    # Scoped creation
    print("\n=== Meals – Scoped Endpoint ===")
    _, scoped_meal = client.run_test("Create Meal via scoped path", "POST",
                                      f"{API_URL}/users/{user_id}/meals",
                                      data={"meal_type": "Snack", "date": today, "notes": "Protein bar"},
                                      expect_status=201)
    scoped_meal_id = scoped_meal["id"] if scoped_meal and "id" in scoped_meal else None

    # -----------------------------------------------------------------------
    # 8. Exercise in-use conflict (409)
    # -----------------------------------------------------------------------
    print("\n=== Exercise Delete Conflict ===")
    client.run_test("Delete exercise in use by workout (409)", "DELETE",
                    f"{API_URL}/exercises/{exercise_id}", expect_status=409)

    # -----------------------------------------------------------------------
    # 9. Malformed requests
    # -----------------------------------------------------------------------
    print("\n=== Malformed Requests ===")
    client.run_test("Create user invalid JSON body", "POST", f"{API_URL}/users",
                    data="not json at all", expect_status=400)
    client.run_test("Create user with unknown fields", "POST", f"{API_URL}/users",
                    data={"email": "unk@b.com", "name": "U", "unknown_field": 123}, expect_status=400)

    # -----------------------------------------------------------------------
    # 10. Cascade Delete – user deletes cascade sub-resources
    # -----------------------------------------------------------------------
    print("\n=== Cascade Delete ===")
    # Create a separate user with sub-resources, then delete the user
    cascade_email = f"cascade_{uid}@example.com"
    cascade_client = APIClient()
    _, cascade_auth = cascade_client.run_test("Create cascade test user", "POST", f"{API_URL}/auth/register",
                                              data={"email": cascade_email, "name": "Cascade", "password": "password123"},
                                              expect_status=201)
    if cascade_auth and "user" in cascade_auth and "token" in cascade_auth:
        cascade_client.auth_token = cascade_auth["token"]
        cu_id = cascade_auth["user"]["id"]
        # Add a meal
        _, cm = cascade_client.run_test("Create cascading meal", "POST", f"{API_URL}/meals",
                                        data={"user_id": cu_id, "meal_type": "Dinner", "date": today}, expect_status=201)
        # Add a weight entry
        _, cw = cascade_client.run_test("Create cascading weight entry", "POST", f"{API_URL}/weight-entries",
                                        data={"user_id": cu_id, "weight": 70, "date": today}, expect_status=201)
        # Add a workout
        _, cwk = cascade_client.run_test("Create cascading workout", "POST", f"{API_URL}/workouts",
                                         data={"user_id": cu_id, "date": today, "duration": 30, "type": "legs"}, expect_status=201)
        # Delete the user (should cascade)
        cascade_client.run_test("Delete cascade user", "DELETE", f"{API_URL}/users/{cu_id}", expect_status=204)
        # Verify sub-resources are gone
        if cm and "id" in cm:
            cascade_client.run_test("Cascaded meal gone", "GET", f"{API_URL}/meals/{cm['id']}", expect_status=404)
        if cw and "id" in cw:
            cascade_client.run_test("Cascaded weight entry gone", "GET", f"{API_URL}/weight-entries/{cw['id']}", expect_status=404)
        if cwk and "id" in cwk:
            cascade_client.run_test("Cascaded workout gone", "GET", f"{API_URL}/workouts/{cwk['id']}", expect_status=404)

    # -----------------------------------------------------------------------
    # 11. Cleanup
    # -----------------------------------------------------------------------
    print("\n=== Cleanup ===")
    # Delete sub-resources first
    if scoped_set_id:
        client.run_test("Delete scoped set", "DELETE", f"{API_URL}/workout-sets/{scoped_set_id}", expect_status=204)
    if wset_id:
        client.run_test("Delete Workout Set", "DELETE", f"{API_URL}/workout-sets/{wset_id}", expect_status=204)
    if scoped_we_id:
        client.run_test("Delete scoped WE", "DELETE", f"{API_URL}/workout-exercises/{scoped_we_id}", expect_status=204)
    client.run_test("Delete Workout Exercise", "DELETE", f"{API_URL}/workout-exercises/{we_ex_id}", expect_status=204)
    if nested_workout_id:
        client.run_test("Delete nested Workout", "DELETE", f"{API_URL}/workouts/{nested_workout_id}", expect_status=204)
    if scoped_workout_id:
        client.run_test("Delete scoped Workout", "DELETE", f"{API_URL}/workouts/{scoped_workout_id}", expect_status=204)
    client.run_test("Delete Workout", "DELETE", f"{API_URL}/workouts/{workout_id}", expect_status=204)
    if scoped_meal_id:
        client.run_test("Delete scoped Meal", "DELETE", f"{API_URL}/meals/{scoped_meal_id}", expect_status=204)
    if meal2_id:
        client.run_test("Delete duplicate Meal", "DELETE", f"{API_URL}/meals/{meal2_id}", expect_status=204)
    if meal_id:
        client.run_test("Delete Meal", "DELETE", f"{API_URL}/meals/{meal_id}", expect_status=204)
    client.run_test("Delete Exercise", "DELETE", f"{API_URL}/exercises/{exercise_id}", expect_status=204)
    client.run_test("Delete User", "DELETE", f"{API_URL}/users/{user_id}", expect_status=204)

    # -----------------------------------------------------------------------
    # Summary
    # -----------------------------------------------------------------------
    print("\n==================================================")
    print(" API TEST RESULTS")
    print("==================================================")
    print(f"Total Tests Run: {tests_run}")
    print(f"Passed: {tests_passed}")
    print(f"Failed: {tests_run - tests_passed}")
    pct = (tests_passed / tests_run) * 100 if tests_run > 0 else 0
    print(f"Success Rate: {pct:.1f}%")
    if tests_passed == tests_run:
        print("🎉 ALL TESTS PASSED!")
    else:
        print("⚠️  SOME TESTS FAILED:")
        for name in tests_failed_names:
            print(f"   • {name}")
        sys.exit(1)


if __name__ == "__main__":
    main()
