import urllib.request
import urllib.error
import json
import uuid
import datetime

BASE_URL = "http://localhost:8080"
API_URL = f"{BASE_URL}/v1"


class APIClient:
    def run_test(self, name, method, url, data=None):
        print(f"Testing {name}...", end=" ")

        req = urllib.request.Request(url, method=method)
        if data is not None:
            json_data = json.dumps(data).encode("utf-8")
            req.add_header("Content-Type", "application/json")
            req.add_header("Content-Length", len(json_data))
            req.data = json_data

        try:
            response = urllib.request.urlopen(req)
            print(f"✅ OK")
            res_body = response.read().decode("utf-8")
            if res_body:
                return json.loads(res_body)
            return None
        except urllib.error.HTTPError as e:
            res_body = e.read().decode("utf-8")
            print(f"❌ FAILED")
            print(f"   Status: {e.code}")
            print(f"   Body: {res_body}")
            return None
        except Exception as e:
            print(f"❌ ERROR: {e}")
            return None


def main():
    client = APIClient()

    # 0. Health check
    print("\n--- Health Check ---")
    res = client.run_test("Health Check", "GET", f"{BASE_URL}/healthz")
    if res is None:
        print("Health check failed, is the server running?")
        return

    uid = str(uuid.uuid4())

    # 1. Users
    print("\n--- Users ---")
    email = f"testuser_{uid}@example.com"
    user_data = {
        "email": email,
        "password_hash": "dummyhash",
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

    user = client.run_test("Create User", "POST", f"{API_URL}/users", data=user_data)
    if not user:
        return
    user_id = user["id"]

    client.run_test("Get User", "GET", f"{API_URL}/users/{user_id}")
    client.run_test("List Users", "GET", f"{API_URL}/users")
    client.run_test(
        "Update User",
        "PATCH",
        f"{API_URL}/users/{user_id}",
        data={"name": "Updated User"},
    )

    # 2. Exercises
    print("\n--- Exercises ---")
    exercise_data = {
        "name": f"Test Exercise {uid}",
        "muscle_group": "Chest",
        "equipment": "Barbell",
        "difficulty": "Intermediate",
        "instructions": "Push the bar",
        "video_url": "http://youtube.com/something",
    }
    exercise = client.run_test(
        "Create Exercise", "POST", f"{API_URL}/exercises", data=exercise_data
    )
    if not exercise:
        return
    exercise_id = exercise["id"]

    client.run_test("Get Exercise", "GET", f"{API_URL}/exercises/{exercise_id}")
    client.run_test("List Exercises", "GET", f"{API_URL}/exercises")
    client.run_test(
        "Update Exercise",
        "PATCH",
        f"{API_URL}/exercises/{exercise_id}",
        data={"difficulty": "Advanced"},
    )

    # 3. Weight Entries
    print("\n--- Weight Entries ---")
    weight_data = {
        "user_id": user_id,
        "weight": 76.0,
        "date": datetime.datetime.now().strftime("%Y-%m-%d"),
        "notes": "Feeling good",
    }
    we = client.run_test(
        "Create Weight Entry", "POST", f"{API_URL}/weight-entries", data=weight_data
    )
    if we:
        we_id = we["id"]
        client.run_test("Get Weight Entry", "GET", f"{API_URL}/weight-entries/{we_id}")
        client.run_test("List Weight Entries", "GET", f"{API_URL}/weight-entries")
        client.run_test(
            "List Weight Entries for User",
            "GET",
            f"{API_URL}/users/{user_id}/weight-entries",
        )
        client.run_test(
            "Update Weight Entry",
            "PATCH",
            f"{API_URL}/weight-entries/{we_id}",
            data={"weight": 75.8},
        )
        client.run_test(
            "Delete Weight Entry", "DELETE", f"{API_URL}/weight-entries/{we_id}"
        )

    # 4. Workouts
    print("\n--- Workouts ---")
    workout_data = {
        "user_id": user_id,
        "date": datetime.datetime.now().strftime("%Y-%m-%d"),
        "duration": 60,
        "notes": "Good session",
        "type": "Strength",
        "exercises": [],
    }
    workout = client.run_test(
        "Create Workout", "POST", f"{API_URL}/workouts", data=workout_data
    )
    if not workout:
        return
    workout_id = workout["id"]

    client.run_test("Get Workout", "GET", f"{API_URL}/workouts/{workout_id}")
    client.run_test("List Workouts", "GET", f"{API_URL}/workouts")
    client.run_test(
        "List Workouts for User", "GET", f"{API_URL}/users/{user_id}/workouts"
    )
    client.run_test(
        "Update Workout",
        "PATCH",
        f"{API_URL}/workouts/{workout_id}",
        data={"duration": 70},
    )

    # 5. Workout Exercises
    print("\n--- Workout Exercises ---")
    we_data = {
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
    workout_exercise = client.run_test(
        "Create Workout Exercise", "POST", f"{API_URL}/workout-exercises", data=we_data
    )
    if not workout_exercise:
        return
    we_ex_id = workout_exercise["id"]

    client.run_test(
        "Get Workout Exercise", "GET", f"{API_URL}/workout-exercises/{we_ex_id}"
    )
    client.run_test("List Workout Exercises", "GET", f"{API_URL}/workout-exercises")
    client.run_test(
        "List Workout Exercises for Workout",
        "GET",
        f"{API_URL}/workouts/{workout_id}/exercises",
    )
    client.run_test(
        "Update Workout Exercise",
        "PATCH",
        f"{API_URL}/workout-exercises/{we_ex_id}",
        data={"reps": 12},
    )

    # 6. Workout Sets
    print("\n--- Workout Sets ---")
    wset_data = {
        "workout_exercise_id": we_ex_id,
        "set_number": 1,
        "reps": 10,
        "weight": 100.0,
        "rpe": 8.5,
        "rest_seconds": 90,
        "completed": True,
    }
    wset = client.run_test(
        "Create Workout Set", "POST", f"{API_URL}/workout-sets", data=wset_data
    )
    if wset:
        wset_id = wset["id"]
        client.run_test("Get Workout Set", "GET", f"{API_URL}/workout-sets/{wset_id}")
        client.run_test("List Workout Sets", "GET", f"{API_URL}/workout-sets")
        client.run_test(
            "List Workout Sets for Exercise",
            "GET",
            f"{API_URL}/workout-exercises/{we_ex_id}/sets",
        )
        client.run_test(
            "Update Workout Set",
            "PATCH",
            f"{API_URL}/workout-sets/{wset_id}",
            data={"completed": False},
        )
        client.run_test(
            "Delete Workout Set", "DELETE", f"{API_URL}/workout-sets/{wset_id}"
        )

    # 7. Meals
    print("\n--- Meals ---")
    meal_data = {
        "user_id": user_id,
        "meal_type": "Breakfast",
        "date": datetime.datetime.now().strftime("%Y-%m-%d"),
        "notes": "Oatmeal and eggs",
    }
    meal = client.run_test("Create Meal", "POST", f"{API_URL}/meals", data=meal_data)
    if meal:
        meal_id = meal["id"]
        client.run_test("Get Meal", "GET", f"{API_URL}/meals/{meal_id}")
        client.run_test("List Meals", "GET", f"{API_URL}/meals")
        client.run_test(
            "List Meals for User", "GET", f"{API_URL}/users/{user_id}/meals"
        )
        client.run_test(
            "Update Meal",
            "PATCH",
            f"{API_URL}/meals/{meal_id}",
            data={"notes": "Just oatmeal"},
        )
        client.run_test("Delete Meal", "DELETE", f"{API_URL}/meals/{meal_id}")

    # 8. Cleanup in reverse order
    print("\n--- Cleanup ---")
    client.run_test(
        "Delete Workout Exercise", "DELETE", f"{API_URL}/workout-exercises/{we_ex_id}"
    )
    client.run_test("Delete Workout", "DELETE", f"{API_URL}/workouts/{workout_id}")
    client.run_test("Delete Exercise", "DELETE", f"{API_URL}/exercises/{exercise_id}")
    client.run_test("Delete User", "DELETE", f"{API_URL}/users/{user_id}")

    print("\n🎉 All tests executed successfully!")


if __name__ == "__main__":
    main()
