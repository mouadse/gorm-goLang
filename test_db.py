import subprocess
import uuid
import sys

# Configuration based on docker-compose.yml
CONTAINER_NAME = "fitness-postgres"
DB_USER = "postgres"
DB_NAME = "fitness_tracker"

# Generate consistent UUIDs for this test run
USER_ID = str(uuid.uuid4())
EXERCISE_ID = str(uuid.uuid4())
WORKOUT_ID = str(uuid.uuid4())
WORKOUT_EXERCISE_ID = str(uuid.uuid4())
WEIGHT_ENTRY_ID = str(uuid.uuid4())
MEAL_ID = str(uuid.uuid4())


def run_query(sql, expect_error=False):
    """Executes a SQL query in the docker container using psql."""
    cmd = [
        "docker",
        "exec",
        "-i",
        CONTAINER_NAME,
        "psql",
        "-U",
        DB_USER,
        "-d",
        DB_NAME,
        "-v",
        "ON_ERROR_STOP=1",
        "-t",
        "-A",
    ]
    result = subprocess.run(cmd, input=sql, capture_output=True, text=True)

    if expect_error:
        if result.returncode != 0:
            return True, result.stderr.strip()
        else:
            return (
                False,
                f"Expected an error, but query succeeded.\nOutput: {result.stdout.strip()}",
            )
    else:
        if result.returncode == 0:
            return True, result.stdout.strip()
        else:
            return False, f"Query failed.\nError: {result.stderr.strip()}"


tests_run = 0
tests_passed = 0


def execute_test(name, sql, expect_error=False, expected_output_contains=None):
    global tests_run, tests_passed
    tests_run += 1

    print(f"[{tests_run}] Testing: {name} ... ", end="")
    success, output = run_query(sql, expect_error)

    if success:
        if expected_output_contains and expected_output_contains not in output:
            print("❌ FAILED")
            print(
                f"  -> Expected output to contain '{expected_output_contains}', but got:\n{output}"
            )
        else:
            print("✅ PASSED")
            tests_passed += 1
    else:
        print("❌ FAILED")
        print(f"  -> {output}")


def main():
    print("==================================================")
    print(" FITNESS TRACKER - DATABASE INTEGRATION TEST SUITE")
    print("==================================================")

    # 0. Check container connection
    success, output = run_query("SELECT 1;")
    if not success:
        print(
            f"\nCRITICAL ERROR: Could not connect to database container '{CONTAINER_NAME}'."
        )
        print("Is the container running? Run 'docker compose up -d postgres' first.")
        sys.exit(1)

    print(f"Successfully connected to '{CONTAINER_NAME}' PostgreSQL instance.\n")

    # Clean up just in case previous tests aborted
    run_query(f"DELETE FROM users WHERE email = 'testuser_{USER_ID}@example.com';")
    run_query(f"DELETE FROM exercises WHERE name = 'Test Bench Press {EXERCISE_ID}';")

    try:
        # TEST 1: Insert User
        sql_insert_user = f"""
            INSERT INTO users (id, email, password_hash, name, age, weight, height, goal, activity_level, tdee, created_at, updated_at) 
            VALUES ('{USER_ID}', 'testuser_{USER_ID}@example.com', 'hashed_pw', 'Test User', 30, 80.5, 180.0, 'muscle_gain', 'active', 2800, NOW(), NOW())
            RETURNING email;
        """
        execute_test(
            "Insert User",
            sql_insert_user,
            expected_output_contains=f"testuser_{USER_ID}@example.com",
        )

        # TEST 2: Unique Email Constraint
        sql_insert_user_duplicate = f"""
            INSERT INTO users (id, email, password_hash, name, created_at, updated_at) 
            VALUES ('{uuid.uuid4()}', 'testuser_{USER_ID}@example.com', 'hashed_pw', 'Test User 2', NOW(), NOW());
        """
        execute_test(
            "Unique Email Constraint (Expect Error)",
            sql_insert_user_duplicate,
            expect_error=True,
        )

        # TEST 3: Insert Exercise
        sql_insert_exercise = f"""
            INSERT INTO exercises (id, name, muscle_group, equipment, difficulty, created_at, updated_at)
            VALUES ('{EXERCISE_ID}', 'Test Bench Press {EXERCISE_ID}', 'Chest', 'Barbell', 'Intermediate', NOW(), NOW())
            RETURNING name;
        """
        execute_test(
            "Insert Exercise",
            sql_insert_exercise,
            expected_output_contains="Test Bench Press",
        )

        # TEST 4: Unique Exercise Name Constraint
        sql_insert_exercise_duplicate = f"""
            INSERT INTO exercises (id, name, muscle_group, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'Test Bench Press {EXERCISE_ID}', 'Legs', NOW(), NOW());
        """
        execute_test(
            "Unique Exercise Name Constraint (Expect Error)",
            sql_insert_exercise_duplicate,
            expect_error=True,
        )

        # TEST 5: Insert Weight Entry
        sql_insert_weight = f"""
            INSERT INTO weight_entries (id, user_id, weight, date, notes, created_at, updated_at)
            VALUES ('{WEIGHT_ENTRY_ID}', '{USER_ID}', 80.5, '2026-03-07', 'Morning weight', NOW(), NOW())
            RETURNING weight;
        """
        execute_test(
            "Insert Weight Entry", sql_insert_weight, expected_output_contains="80.5"
        )

        # TEST 6: Unique Weight Entry per Date Constraint (idx_user_date)
        sql_insert_weight_duplicate = f"""
            INSERT INTO weight_entries (id, user_id, weight, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID}', 79.0, '2026-03-07', NOW(), NOW());
        """
        execute_test(
            "Unique Weight Entry Per Date (Expect Error)",
            sql_insert_weight_duplicate,
            expect_error=True,
        )

        # TEST 7: Insert Meal
        sql_insert_meal = f"""
            INSERT INTO meals (id, user_id, meal_type, date, notes, created_at, updated_at)
            VALUES ('{MEAL_ID}', '{USER_ID}', 'breakfast', '2026-03-07', 'Oatmeal', NOW(), NOW())
            RETURNING meal_type;
        """
        execute_test(
            "Insert Meal", sql_insert_meal, expected_output_contains="breakfast"
        )

        # TEST 8: Regular Meal Index (idx_meals_user_date_type)
        # Note: GORM model defines this as a regular index, NOT uniqueIndex.
        # A user CAN have multiple meals of the same type on the same date.
        sql_insert_meal_duplicate = f"""
            INSERT INTO meals (id, user_id, meal_type, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID}', 'breakfast', '2026-03-07', NOW(), NOW())
            RETURNING meal_type;
        """
        execute_test(
            "Non-Unique Meal Type Per Date (Should Succeed)",
            sql_insert_meal_duplicate,
            expected_output_contains="breakfast",
        )

        # TEST 9: Insert Workout Hierarchy (Workout -> Exercise -> Set)
        sql_insert_workout = f"""
            BEGIN;
            INSERT INTO workouts (id, user_id, date, duration, type, created_at, updated_at)
            VALUES ('{WORKOUT_ID}', '{USER_ID}', '2026-03-07', 60, 'push', NOW(), NOW());
            
            INSERT INTO workout_exercises (id, workout_id, exercise_id, "order", sets, reps, weight, created_at, updated_at)
            VALUES ('{WORKOUT_EXERCISE_ID}', '{WORKOUT_ID}', '{EXERCISE_ID}', 1, 3, 10, 80.0, NOW(), NOW());
            
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{str(uuid.uuid4())}', '{WORKOUT_EXERCISE_ID}', 1, 10, 80.0, true, NOW(), NOW());
            COMMIT;
            SELECT count(*) FROM workouts WHERE id = '{WORKOUT_ID}';
        """
        execute_test(
            "Insert Workout Hierarchy", sql_insert_workout, expected_output_contains="1"
        )

        # TEST 10: Unique Set Number Constraint (idx_workout_exercise_set_number)
        sql_insert_set_duplicate = f"""
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{str(uuid.uuid4())}', '{WORKOUT_EXERCISE_ID}', 1, 8, 80.0, true, NOW(), NOW());
        """
        execute_test(
            "Unique Set Number in Workout (Expect Error)",
            sql_insert_set_duplicate,
            expect_error=True,
        )

        # TEST 11: Soft Delete User (Testing GORM's deleted_at pattern)
        sql_soft_delete_user = f"""
            UPDATE users SET deleted_at = NOW() WHERE id = '{USER_ID}' RETURNING id;
        """
        execute_test(
            "Soft Delete User", sql_soft_delete_user, expected_output_contains=USER_ID
        )

        # TEST 12: Unique Email freed after Soft Delete
        # Since the first user is soft-deleted, inserting the same email should now work because of `where:deleted_at IS NULL`
        sql_insert_user_again = f"""
            INSERT INTO users (id, email, password_hash, name, created_at, updated_at) 
            VALUES ('{uuid.uuid4()}', 'testuser_{USER_ID}@example.com', 'hashed_pw', 'Test User Reborn', NOW(), NOW())
            RETURNING name;
        """
        execute_test(
            "Insert Same Email After Soft Delete",
            sql_insert_user_again,
            expected_output_contains="Test User Reborn",
        )

    finally:
        print("\nCleaning up test data...")
        # Hard delete all test data generated during this run
        cleanup_sql = f"""
            BEGIN;
            DELETE FROM workout_sets WHERE workout_exercise_id IN (SELECT id FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}');
            DELETE FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}';
            DELETE FROM workouts WHERE id = '{WORKOUT_ID}';
            DELETE FROM meals WHERE user_id = '{USER_ID}';
            DELETE FROM weight_entries WHERE user_id = '{USER_ID}';
            DELETE FROM exercises WHERE id = '{EXERCISE_ID}';
            DELETE FROM users WHERE email = 'testuser_{USER_ID}@example.com';
            COMMIT;
        """
        run_query(cleanup_sql)

    print("\n==================================================")
    print(" TEST RESULTS")
    print("==================================================")
    print(f"Total Tests Run: {tests_run}")
    print(f"Passed: {tests_passed}")
    print(f"Failed: {tests_run - tests_passed}")

    percentage = (tests_passed / tests_run) * 100 if tests_run > 0 else 0
    print(f"Success Rate: {percentage:.2f}%")

    if tests_passed == tests_run:
        print("🎉 ALL TESTS PASSED!")
    else:
        print("⚠️ SOME TESTS FAILED.")
        sys.exit(1)


if __name__ == "__main__":
    main()
