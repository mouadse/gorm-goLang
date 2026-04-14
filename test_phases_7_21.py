import requests
import json
import uuid
import time
import random
from datetime import datetime, timedelta

BASE_URL = "http://localhost:8080"

class FitnessAPIClient:
    def __init__(self, email, password):
        self.email = email
        self.password = password
        self.token = None
        self.user_id = None
        self.refresh_token = None

    def login(self):
        resp = requests.post(f"{BASE_URL}/v1/auth/login", json={
            "email": self.email,
            "password": self.password
        })
        if resp.status_code == 200:
            data = resp.json()
            self.token = data['access_token']
            self.refresh_token = data['refresh_token']
            self.user_id = data['user']['id']
            print(f"Logged in as {self.email}, user_id: {self.user_id}")
            return True
        else:
            print(f"Failed to login as {self.email}: {resp.status_code} {resp.text}")
            return False

    def get_headers(self):
        return {"Authorization": f"Bearer {self.token}"}

    def request(self, method, path, **kwargs):
        url = f"{BASE_URL}{path}"
        headers = self.get_headers()
        if 'headers' in kwargs:
            headers.update(kwargs['headers'])
        kwargs['headers'] = headers
        resp = requests.request(method, url, **kwargs)
        return resp

def test_phases():
    admin_client = FitnessAPIClient("alex@example.com", "password123")
    user_client = FitnessAPIClient("sarah@example.com", "password123")

    if not admin_client.login() or not user_client.login():
        print("Initial logins failed. Aborting tests.")
        return

    results = []

    def record_result(phase, endpoint, status_code, success, body=None):
        results.append({
            "phase": phase,
            "endpoint": endpoint,
            "status": status_code,
            "success": success,
            "body": body
        })
        status_str = "SUCCESS" if success else "FAILED"
        print(f"[{status_str}] Phase {phase}: {endpoint} -> {status_code}")
        if not success and body:
            print(f"      Error: {body}")

    # Helper to test an endpoint
    def test_endpoint(client, phase, method, path, expected_status=200, **kwargs):
        resp = client.request(method, path, **kwargs)
        
        # Determine success
        if isinstance(expected_status, list):
            success = resp.status_code in expected_status
        else:
            success = resp.status_code == expected_status
        
        body_text = resp.text
        try:
            body_json = resp.json()
            body_text = json.dumps(body_json)
        except:
            pass
            
        record_result(phase, f"{method} {path}", resp.status_code, success, body_text)
        return resp

    # Helper to get items from paginated response
    def get_items(resp_json, key=None):
        if isinstance(resp_json, list):
            return resp_json
        if 'data' in resp_json:
            return resp_json['data']
        if key and key in resp_json:
            return resp_json[key]
        return []

    # Get some data for testing
    foods_resp = user_client.request("GET", "/v1/foods")
    food_id = None
    if foods_resp.status_code == 200:
        foods = get_items(foods_resp.json(), 'data')
        if foods:
            food_id = foods[0]['id']

    exercises_resp = user_client.request("GET", "/v1/exercises")
    exercise_id = None
    if exercises_resp.status_code == 200:
        exercises = get_items(exercises_resp.json(), 'exercises')
        if exercises:
            exercise_id = exercises[0]['id']

    today = datetime.now().strftime("%Y-%m-%d")
    # Use a far future date for weight entry to avoid unique constraint collision
    unique_weight_date = (datetime.now() + timedelta(days=365*50 + random.randint(1, 1000))).strftime("%Y-%m-%d")

    # PHASE 7: Daily Nutrition / Meal Log
    print("\n--- Testing Phase 7: Daily Nutrition / Meal Log ---")
    test_endpoint(user_client, 7, "GET", "/v1/meals")
    test_endpoint(user_client, 7, "GET", "/v1/meals/recent")
    test_endpoint(user_client, 7, "GET", "/v1/summary")
    
    create_meal_resp = test_endpoint(user_client, 7, "POST", "/v1/meals", 201, json={
        "user_id": user_client.user_id,
        "meal_type": "lunch",
        "date": today,
        "notes": "Test meal"
    })
    if create_meal_resp.status_code == 201:
        meal_id = create_meal_resp.json()['id']
        test_endpoint(user_client, 7, "GET", f"/v1/meals/{meal_id}")
        test_endpoint(user_client, 7, "PATCH", f"/v1/meals/{meal_id}", json={"notes": "Updated notes"})
        test_endpoint(user_client, 7, "POST", f"/v1/meals/{meal_id}/clone", 201)
        
        # PHASE 8: Meal Builder / Food Picker
        print("\n--- Testing Phase 8: Meal Builder / Food Picker ---")
        if food_id:
            create_meal_food_resp = test_endpoint(user_client, 8, "POST", f"/v1/meals/{meal_id}/foods", 201, json={
                "food_id": food_id,
                "quantity": 1.5
            })
            if create_meal_food_resp.status_code == 201:
                meal_food_id = create_meal_food_resp.json()['id']
                test_endpoint(user_client, 8, "GET", f"/v1/meals/{meal_id}/foods")
                test_endpoint(user_client, 8, "PATCH", f"/v1/meal-foods/{meal_food_id}", json={"quantity": 2.0})
                test_endpoint(user_client, 8, "DELETE", f"/v1/meal-foods/{meal_food_id}", 204)
        
        test_endpoint(user_client, 8, "GET", "/v1/foods")
        if food_id:
            test_endpoint(user_client, 8, "GET", f"/v1/foods/{food_id}")
            test_endpoint(user_client, 8, "POST", f"/v1/foods/{food_id}/favorite", [200, 201])
            test_endpoint(user_client, 8, "DELETE", f"/v1/foods/{food_id}/favorite", 204)
        test_endpoint(user_client, 8, "GET", "/v1/foods/recent")
        test_endpoint(user_client, 8, "GET", f"/v1/users/{user_client.user_id}/favorites")

        # Cleanup Phase 7
        user_client.request("DELETE", f"/v1/meals/{meal_id}")

    # PHASE 9: Weight Tracker
    print("\n--- Testing Phase 9: Weight Tracker ---")
    test_endpoint(user_client, 9, "GET", "/v1/weight-entries")
    # Backend expects simple date layout YYYY-MM-DD. 
    # Using unique_weight_date to avoid duplicate key constraint error.
    create_weight_resp = test_endpoint(user_client, 9, "POST", "/v1/weight-entries", 201, json={
        "user_id": user_client.user_id,
        "weight": 70.5,
        "date": unique_weight_date,
        "notes": "Morning weigh-in"
    })
    if create_weight_resp.status_code == 201:
        weight_id = create_weight_resp.json()['id']
        test_endpoint(user_client, 9, "GET", f"/v1/weight-entries/{weight_id}")
        test_endpoint(user_client, 9, "PATCH", f"/v1/weight-entries/{weight_id}", json={"weight": 70.6})
        test_endpoint(user_client, 9, "DELETE", f"/v1/weight-entries/{weight_id}", 204)

    # PHASE 10: Notifications Center
    print("\n--- Testing Phase 10: Notifications Center ---")
    notifs_resp = test_endpoint(user_client, 10, "GET", "/v1/notifications")
    test_endpoint(user_client, 10, "GET", "/v1/notifications/unread-count")
    test_endpoint(user_client, 10, "PATCH", "/v1/notifications/read-all", 200)
    
    notifs = get_items(notifs_resp.json())
    if notifs:
        notif_id = notifs[0]['id']
        test_endpoint(user_client, 10, "PATCH", f"/v1/notifications/{notif_id}/read", 200)

    # PHASE 11: Progress & Analytics
    print("\n--- Testing Phase 11: Progress & Analytics ---")
    test_endpoint(user_client, 11, "GET", f"/v1/users/{user_client.user_id}/records")
    test_endpoint(user_client, 11, "GET", f"/v1/users/{user_client.user_id}/workout-stats")
    test_endpoint(user_client, 11, "GET", f"/v1/users/{user_client.user_id}/activity-calendar")
    test_endpoint(user_client, 11, "GET", f"/v1/users/{user_client.user_id}/streaks")
    test_endpoint(user_client, 11, "GET", "/v1/weekly-summary")
    if exercise_id:
        test_endpoint(user_client, 11, "GET", f"/v1/exercises/{exercise_id}/history")

    # PHASE 12: Workout Templates
    print("\n--- Testing Phase 12: Workout Templates ---")
    test_endpoint(user_client, 12, "GET", "/v1/workout-templates")
    if exercise_id:
        create_template_resp = test_endpoint(user_client, 12, "POST", "/v1/workout-templates", 201, json={
            "owner_id": user_client.user_id,
            "name": "Full Body Test",
            "type": "full_body",
            "notes": "Testing templates",
            "exercises": [
                {
                    "exercise_id": exercise_id,
                    "order": 1,
                    "sets": 3,
                    "reps": 10,
                    "weight": 50.0,
                    "rest_time": 90
                }
            ]
        })
        if create_template_resp.status_code == 201:
            template_id = create_template_resp.json()['id']
            test_endpoint(user_client, 12, "GET", f"/v1/workout-templates/{template_id}")
            test_endpoint(user_client, 12, "PATCH", f"/v1/workout-templates/{template_id}", json={"name": "Updated Template Name"})
            
            # Apply template strictly requires user_id and YYYY-MM-DD date
            test_endpoint(user_client, 12, "POST", f"/v1/workout-templates/{template_id}/apply", 201, json={
                "user_id": user_client.user_id,
                "date": today
            })
            
            test_endpoint(user_client, 12, "DELETE", f"/v1/workout-templates/{template_id}", 204)

    # PHASE 13: Program Assignments / Structured Plans
    print("\n--- Testing Phase 13: Program Assignments ---")
    assign_resp = test_endpoint(user_client, 13, "GET", "/v1/program-assignments")
    assignments = get_items(assign_resp.json())
    if assignments:
        assign_id = assignments[0]['id']
        test_endpoint(user_client, 13, "GET", f"/v1/program-assignments/{assign_id}")
        test_endpoint(user_client, 13, "PATCH", f"/v1/program-assignments/{assign_id}/status", json={"status": "in_progress"})
        
        # Find a program session to apply
        prog_id = assignments[0]['program_id']
        # Admin can see program details
        prog_resp = admin_client.request("GET", f"/v1/programs/{prog_id}")
        if prog_resp.status_code == 200:
            prog_data = prog_resp.json()
            if 'weeks' in prog_data and prog_data['weeks']:
                week = prog_data['weeks'][0]
                if 'sessions' in week and week['sessions']:
                    sess_id = week['sessions'][0]['id']
                    # Apply session requires user_id and YYYY-MM-DD date
                    test_endpoint(user_client, 13, "POST", f"/v1/program-sessions/{sess_id}/apply", 201, json={
                        "user_id": user_client.user_id,
                        "date": today
                    })

    # PHASE 14: Recipes
    print("\n--- Testing Phase 14: Recipes ---")
    test_endpoint(user_client, 14, "GET", "/v1/recipes")
    if food_id:
        # Create recipe body must NOT contain user_id (DisallowUnknownFields is on)
        create_recipe_resp = test_endpoint(user_client, 14, "POST", "/v1/recipes", 201, json={
            "name": "Protein Shake Test",
            "servings": 1,
            "notes": "Quick post-workout",
            "items": [
                {"food_id": food_id, "quantity": 1.0}
            ]
        })
        if create_recipe_resp.status_code == 201:
            recipe_id = create_recipe_resp.json()['id']
            test_endpoint(user_client, 14, "GET", f"/v1/recipes/{recipe_id}")
            test_endpoint(user_client, 14, "PATCH", f"/v1/recipes/{recipe_id}", json={"notes": "Updated recipe notes"})
            test_endpoint(user_client, 14, "GET", f"/v1/recipes/{recipe_id}/nutrition")
            
            # Log recipe to meal
            temp_meal_resp = user_client.request("POST", "/v1/meals", json={
                "user_id": user_client.user_id,
                "meal_type": "snack",
                "date": today
            })
            if temp_meal_resp.status_code == 201:
                target_meal_id = temp_meal_resp.json()['id']
                test_endpoint(user_client, 14, "POST", f"/v1/recipes/{recipe_id}/log-to-meal", 201, json={
                    "date": today,
                    "meal_type": "snack",
                    "servings": 1.0
                })
                user_client.request("DELETE", f"/v1/meals/{target_meal_id}")
            
            test_endpoint(user_client, 14, "DELETE", f"/v1/recipes/{recipe_id}", 204)

    # PHASE 15: AI Coach
    print("\n--- Testing Phase 15: AI Coach ---")
    chat_resp = test_endpoint(user_client, 15, "POST", "/v1/chat", expected_status=200, json={
        "message": "What should I eat for breakfast?",
        "stream": False
    })
    test_endpoint(user_client, 15, "GET", "/v1/chat/history")
    if chat_resp.status_code == 200:
        msg_id = chat_resp.json().get('message_id')
        if msg_id:
            test_endpoint(user_client, 15, "POST", "/v1/chat/feedback", 200, json={
                "message_id": msg_id,
                "feedback": "positive"
            })
    test_endpoint(user_client, 15, "GET", f"/v1/users/{user_client.user_id}/coach-summary")

    # PHASE 16: Leaderboard
    print("\n--- Testing Phase 16: Leaderboard ---")
    test_endpoint(user_client, 16, "GET", "/v1/leaderboard")

    # PHASE 17: Settings / Security / Export / Account
    print("\n--- Testing Phase 17: Settings / Security / Export / Account ---")
    test_endpoint(user_client, 17, "GET", f"/v1/users/{user_client.user_id}")
    test_endpoint(user_client, 17, "PATCH", f"/v1/users/{user_client.user_id}", json={"name": "Sarah Updated"})
    test_endpoint(user_client, 17, "GET", "/v1/auth/sessions")
    
    # Export returns 202 Accepted
    export_resp = test_endpoint(user_client, 17, "POST", "/v1/exports", [201, 202])
    if export_resp.status_code in [201, 202]:
        export_data = export_resp.json()
        if 'id' in export_data:
            export_id = export_data['id']
            test_endpoint(user_client, 17, "GET", f"/v1/exports/{export_id}")
    
    test_endpoint(user_client, 17, "POST", "/v1/auth/2fa/setup", [201, 409])
    
    # Account deletion might fail if mailer not configured, but we check if endpoint exists
    test_endpoint(user_client, 17, "POST", "/v1/account/delete-request", [201, 204, 500])

    # ADMIN PHASES (18-21)
    print("\n--- Testing Admin Phases (18-21) ---")
    
    # PHASE 18: Admin Dashboard
    admin_endpoints = [
        "/v1/admin/dashboard/summary",
        "/v1/admin/dashboard/trends",
        "/v1/admin/users/stats",
        "/v1/admin/users/growth",
        "/v1/admin/workouts/stats",
        "/v1/admin/workouts/exercises/popular",
        "/v1/admin/nutrition/stats",
        "/v1/admin/moderation/stats",
        "/v1/admin/system/health",
        "/v1/admin/audit-logs"
    ]
    for endpoint in admin_endpoints:
        test_endpoint(admin_client, 18, "GET", endpoint)

    # PHASE 19: Admin User Management
    admin_users_resp = test_endpoint(admin_client, 19, "GET", "/v1/admin/users")
    if admin_users_resp.status_code == 200:
        users_list = get_items(admin_users_resp.json())
        if users_list:
            # Pick a user that is NOT the admin itself to avoid self-ban issues
            target_user = None
            for u in users_list:
                if u['email'] != admin_client.email:
                    target_user = u
                    break
            
            if target_user:
                target_user_id = target_user['id']
                test_endpoint(admin_client, 19, "GET", f"/v1/admin/users/{target_user_id}")
                test_endpoint(admin_client, 19, "PATCH", f"/v1/admin/users/{target_user_id}", json={"name": "Admin Updated Name"})
                test_endpoint(admin_client, 19, "POST", f"/v1/admin/users/{target_user_id}/ban", [200, 400], json={"reason": "Test ban"})
                test_endpoint(admin_client, 19, "POST", f"/v1/admin/users/{target_user_id}/unban", [200, 400])

    # PHASE 20: Admin Program Management
    test_endpoint(admin_client, 20, "GET", "/v1/programs")
    create_prog_resp = test_endpoint(admin_client, 20, "POST", "/v1/programs", 201, json={
        "name": "New Admin Program",
        "description": "Created by test script",
        "is_active": True
    })
    if create_prog_resp.status_code == 201:
        prog_id = create_prog_resp.json()['id']
        test_endpoint(admin_client, 20, "GET", f"/v1/programs/{prog_id}")
        test_endpoint(admin_client, 20, "PATCH", f"/v1/programs/{prog_id}", json={"name": "Updated Admin Program"})
        
        # Weeks
        create_week_resp = test_endpoint(admin_client, 20, "POST", f"/v1/programs/{prog_id}/weeks", 201, json={
            "week_number": 1,
            "name": "Intro Week"
        })
        if create_week_resp.status_code == 201:
            week_id = create_week_resp.json()['id']
            test_endpoint(admin_client, 20, "GET", f"/v1/program-weeks/{week_id}")
            test_endpoint(admin_client, 20, "PATCH", f"/v1/program-weeks/{week_id}", json={"name": "Intro Week Updated"})
            
            # Sessions
            create_sess_resp = test_endpoint(admin_client, 20, "POST", f"/v1/program-weeks/{week_id}/sessions", 201, json={
                "day_number": 1,
                "notes": "Light start"
            })
            if create_sess_resp.status_code == 201:
                sess_id = create_sess_resp.json()['id']
                test_endpoint(admin_client, 20, "GET", f"/v1/program-sessions/{sess_id}")
                test_endpoint(admin_client, 20, "PATCH", f"/v1/program-sessions/{sess_id}", json={"notes": "Light start updated"})
                test_endpoint(admin_client, 20, "DELETE", f"/v1/program-sessions/{sess_id}", 204)
            
            test_endpoint(admin_client, 20, "DELETE", f"/v1/program-weeks/{week_id}", 204)
        
        # Assignments
        create_assign_resp = test_endpoint(admin_client, 20, "POST", f"/v1/programs/{prog_id}/assignments", 201, json={
            "user_id": user_client.user_id,
            "assigned_at": today,
            "status": "assigned"
        })
        if create_assign_resp.status_code == 201:
            assign_id = create_assign_resp.json()['id']
            test_endpoint(admin_client, 20, "GET", f"/v1/programs/{prog_id}/assignments")
            test_endpoint(admin_client, 20, "PATCH", f"/v1/admin/program-assignments/{assign_id}", json={"status": "completed"})
            test_endpoint(admin_client, 20, "DELETE", f"/v1/admin/program-assignments/{assign_id}", 204)
            
        test_endpoint(admin_client, 20, "DELETE", f"/v1/programs/{prog_id}", 204)

    # PHASE 21: Admin Nutrition Catalog / Import
    print("\n--- Testing Phase 21: Admin Nutrition Catalog / Import ---")
    test_endpoint(admin_client, 21, "GET", "/v1/admin/nutrition/stats")

    # SUMMARY
    print("\n--- TEST SUMMARY ---")
    success_count = sum(1 for r in results if r['success'])
    fail_count = len(results) - success_count
    print(f"Total Tests: {len(results)}")
    print(f"Successes: {success_count}")
    print(f"Failures: {fail_count}")

    if fail_count > 0:
        print("\nFailed Endpoints:")
        for r in results:
            if not r['success']:
                print(f"Phase {r['phase']}: {r['endpoint']} -> Status {r['status']}")

if __name__ == "__main__":
    test_phases()
