import subprocess
import uuid
import sys

# Configuration based on docker-compose.yml
CONTAINER_NAME = "fitness-postgres"
DB_USER = "postgres"
DB_NAME = "fitness_tracker"

# Generate consistent UUIDs for this test run
USER_ID = str(uuid.uuid4())
USER_ID_2 = str(uuid.uuid4())
EXERCISE_ID = str(uuid.uuid4())
EXERCISE_ID_2 = str(uuid.uuid4())
WORKOUT_ID = str(uuid.uuid4())
WORKOUT_EXERCISE_ID = str(uuid.uuid4())
WEIGHT_ENTRY_ID = str(uuid.uuid4())
MEAL_ID = str(uuid.uuid4())
WORKOUT_SET_ID = str(uuid.uuid4())
WORKOUT_SET_ID_2 = str(uuid.uuid4())
BCRYPT_HASH = "$2a$10$9sTz8JqF8mA3r6wGkCjH7uHmp8yM5M6Q6u2mFfB8v0V1nC9Yj4y1K"


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
tests_failed_names = []


def execute_test(name, sql, expect_error=False, expected_output_contains=None):
    global tests_run, tests_passed, tests_failed_names
    tests_run += 1

    print(f"  [{tests_run}] Testing: {name} ... ", end="")
    success, output = run_query(sql, expect_error)

    if success:
        if expected_output_contains and expected_output_contains not in output:
            print("❌ FAILED")
            print(
                f"       Expected output to contain '{expected_output_contains}', but got:\n       {output}"
            )
            tests_failed_names.append(name)
        else:
            print("✅ PASSED")
            tests_passed += 1
    else:
        print("❌ FAILED")
        print(f"       {output}")
        tests_failed_names.append(name)


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
    run_query(f"DELETE FROM workout_sets WHERE workout_exercise_id IN (SELECT id FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}');")
    run_query(f"DELETE FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}';")
    run_query(f"DELETE FROM workouts WHERE id = '{WORKOUT_ID}';")
    run_query(f"DELETE FROM meals WHERE user_id = '{USER_ID}';")
    run_query(f"DELETE FROM weight_entries WHERE user_id = '{USER_ID}';")
    run_query(f"DELETE FROM users WHERE email = 'testuser_{USER_ID}@example.com';")
    run_query(f"DELETE FROM users WHERE email = 'testuser_{USER_ID_2}@example.com';")
    run_query(f"DELETE FROM exercises WHERE id = '{EXERCISE_ID}';")
    run_query(f"DELETE FROM exercises WHERE id = '{EXERCISE_ID_2}';")

    try:
        # ===================================================================
        # SECTION 1: Basic CRUD & Unique Constraints (original tests)
        # ===================================================================
        print("\n--- Basic CRUD & Unique Constraints ---")

        # TEST 1: Insert User
        execute_test(
            "Insert User",
            f"""
            INSERT INTO users (id, email, password_hash, name, age, weight, height, goal, activity_level, tdee, created_at, updated_at)
            VALUES ('{USER_ID}', 'testuser_{USER_ID}@example.com', '{BCRYPT_HASH}', 'Test User', 30, 80.5, 180.0, 'muscle_gain', 'active', 2800, NOW(), NOW())
            RETURNING email;
            """,
            expected_output_contains=f"testuser_{USER_ID}@example.com",
        )

        # TEST 2: Unique Email Constraint
        execute_test(
            "Unique Email Constraint (Expect Error)",
            f"""
            INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'testuser_{USER_ID}@example.com', '{BCRYPT_HASH}', 'Test User 2', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 3: Insert Exercise
        execute_test(
            "Insert Exercise",
            f"""
            INSERT INTO exercises (id, name, muscle_group, equipment, difficulty, created_at, updated_at)
            VALUES ('{EXERCISE_ID}', 'Test Bench Press {EXERCISE_ID}', 'Chest', 'Barbell', 'Intermediate', NOW(), NOW())
            RETURNING name;
            """,
            expected_output_contains="Test Bench Press",
        )

        # TEST 4: Unique Exercise Name Constraint
        execute_test(
            "Unique Exercise Name Constraint (Expect Error)",
            f"""
            INSERT INTO exercises (id, name, muscle_group, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'Test Bench Press {EXERCISE_ID}', 'Legs', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 5: Insert Weight Entry
        execute_test(
            "Insert Weight Entry",
            f"""
            INSERT INTO weight_entries (id, user_id, weight, date, notes, created_at, updated_at)
            VALUES ('{WEIGHT_ENTRY_ID}', '{USER_ID}', 80.5, '2026-03-07', 'Morning weight', NOW(), NOW())
            RETURNING weight;
            """,
            expected_output_contains="80.5",
        )

        # TEST 6: Unique Weight Entry per Date Constraint
        execute_test(
            "Unique Weight Entry Per Date (Expect Error)",
            f"""
            INSERT INTO weight_entries (id, user_id, weight, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID}', 79.0, '2026-03-07', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 7: Insert Meal
        execute_test(
            "Insert Meal",
            f"""
            INSERT INTO meals (id, user_id, meal_type, date, notes, created_at, updated_at)
            VALUES ('{MEAL_ID}', '{USER_ID}', 'breakfast', '2026-03-07', 'Oatmeal', NOW(), NOW())
            RETURNING meal_type;
            """,
            expected_output_contains="breakfast",
        )

        # TEST 8: Non-Unique Meal Type Per Date (should succeed)
        execute_test(
            "Non-Unique Meal Type Per Date (Should Succeed)",
            f"""
            INSERT INTO meals (id, user_id, meal_type, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID}', 'breakfast', '2026-03-07', NOW(), NOW())
            RETURNING meal_type;
            """,
            expected_output_contains="breakfast",
        )

        # TEST 9: Insert Workout Hierarchy
        execute_test(
            "Insert Workout Hierarchy",
            f"""
            BEGIN;
            INSERT INTO workouts (id, user_id, date, duration, type, created_at, updated_at)
            VALUES ('{WORKOUT_ID}', '{USER_ID}', '2026-03-07', 60, 'push', NOW(), NOW());

            INSERT INTO workout_exercises (id, workout_id, exercise_id, "order", sets, reps, weight, created_at, updated_at)
            VALUES ('{WORKOUT_EXERCISE_ID}', '{WORKOUT_ID}', '{EXERCISE_ID}', 1, 3, 10, 80.0, NOW(), NOW());

            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{WORKOUT_SET_ID}', '{WORKOUT_EXERCISE_ID}', 1, 10, 80.0, true, NOW(), NOW());
            COMMIT;
            SELECT count(*) FROM workouts WHERE id = '{WORKOUT_ID}';
            """,
            expected_output_contains="1",
        )

        # TEST 10: Unique Set Number Constraint
        execute_test(
            "Unique Set Number in Workout (Expect Error)",
            f"""
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{str(uuid.uuid4())}', '{WORKOUT_EXERCISE_ID}', 1, 8, 80.0, true, NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 11: Soft Delete User
        execute_test(
            "Soft Delete User",
            f"""
            UPDATE users SET deleted_at = NOW() WHERE id = '{USER_ID}' RETURNING id;
            """,
            expected_output_contains=USER_ID,
        )

        # TEST 12: Unique Email freed after Soft Delete
        execute_test(
            "Insert Same Email After Soft Delete",
            f"""
            INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
            VALUES ('{USER_ID_2}', 'testuser_{USER_ID}@example.com', '{BCRYPT_HASH}', 'Test User Reborn', NOW(), NOW())
            RETURNING name;
            """,
            expected_output_contains="Test User Reborn",
        )

        # ===================================================================
        # SECTION 2: Foreign Key Constraints
        # ===================================================================
        print("\n--- Foreign Key Constraints ---")

        # TEST 13: FK - Weight entry with non-existent user
        execute_test(
            "FK: Weight entry with non-existent user (Expect Error)",
            f"""
            INSERT INTO weight_entries (id, user_id, weight, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{uuid.uuid4()}', 70.0, '2026-04-01', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 14: FK - Meal with non-existent user
        execute_test(
            "FK: Meal with non-existent user (Expect Error)",
            f"""
            INSERT INTO meals (id, user_id, meal_type, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{uuid.uuid4()}', 'lunch', '2026-04-01', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 15: FK - Workout with non-existent user
        execute_test(
            "FK: Workout with non-existent user (Expect Error)",
            f"""
            INSERT INTO workouts (id, user_id, date, duration, type, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{uuid.uuid4()}', '2026-04-01', 60, 'push', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 16: FK - Workout exercise with non-existent workout
        execute_test(
            "FK: Workout exercise with non-existent workout (Expect Error)",
            f"""
            INSERT INTO workout_exercises (id, workout_id, exercise_id, "order", sets, reps, weight, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{uuid.uuid4()}', '{EXERCISE_ID}', 1, 3, 10, 50.0, NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 17: FK - Workout exercise with non-existent exercise
        execute_test(
            "FK: Workout exercise with non-existent exercise (Expect Error)",
            f"""
            INSERT INTO workout_exercises (id, workout_id, exercise_id, "order", sets, reps, weight, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{WORKOUT_ID}', '{uuid.uuid4()}', 1, 3, 10, 50.0, NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 18: FK - Workout set with non-existent workout exercise
        execute_test(
            "FK: Workout set with non-existent workout exercise (Expect Error)",
            f"""
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{uuid.uuid4()}', 1, 10, 50.0, true, NOW(), NOW());
            """,
            expect_error=True,
        )

        # ===================================================================
        # SECTION 3: NOT NULL Constraints
        # ===================================================================
        print("\n--- NOT NULL Constraints ---")

        # TEST 19: User without email
        execute_test(
            "NOT NULL: User without email (Expect Error)",
            f"""
            INSERT INTO users (id, password_hash, name, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'hash', 'No Email', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 20: User without name
        execute_test(
            "NOT NULL: User without name (Expect Error)",
            f"""
            INSERT INTO users (id, email, password_hash, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'noname_{uuid.uuid4()}@test.com', 'hash', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 21: User without password_hash
        execute_test(
            "NOT NULL: User without password_hash (Expect Error)",
            f"""
            INSERT INTO users (id, email, name, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'nopw_{uuid.uuid4()}@test.com', 'No PW', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 22: Weight entry without weight
        execute_test(
            "NOT NULL: Weight entry without weight (Expect Error)",
            f"""
            INSERT INTO weight_entries (id, user_id, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID_2}', '2026-05-01', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 23: Weight entry without date
        execute_test(
            "NOT NULL: Weight entry without date (Expect Error)",
            f"""
            INSERT INTO weight_entries (id, user_id, weight, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID_2}', 70.0, NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 24: Weight entry without user_id
        execute_test(
            "NOT NULL: Weight entry without user_id (Expect Error)",
            f"""
            INSERT INTO weight_entries (id, weight, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 70.0, '2026-05-01', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 25: Meal without meal_type
        execute_test(
            "NOT NULL: Meal without meal_type (Expect Error)",
            f"""
            INSERT INTO meals (id, user_id, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID_2}', '2026-05-01', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 26: Meal without user_id
        execute_test(
            "NOT NULL: Meal without user_id (Expect Error)",
            f"""
            INSERT INTO meals (id, meal_type, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'lunch', '2026-05-01', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 27: Exercise without name
        execute_test(
            "NOT NULL: Exercise without name (Expect Error)",
            f"""
            INSERT INTO exercises (id, muscle_group, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'Chest', NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 28: Workout set without reps
        execute_test(
            "NOT NULL: Workout set without reps (Expect Error)",
            f"""
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, weight, completed, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{WORKOUT_EXERCISE_ID}', 99, 50.0, true, NOW(), NOW());
            """,
            expect_error=True,
        )

        # TEST 29: Workout set without set_number
        execute_test(
            "NOT NULL: Workout set without set_number (Expect Error)",
            f"""
            INSERT INTO workout_sets (id, workout_exercise_id, reps, weight, completed, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{WORKOUT_EXERCISE_ID}', 10, 50.0, true, NOW(), NOW());
            """,
            expect_error=True,
        )

        # ===================================================================
        # SECTION 4: Update Operations
        # ===================================================================
        print("\n--- Update Operations ---")

        # TEST 30: Update user fields
        execute_test(
            "Update User name and weight",
            f"""
            UPDATE users SET name = 'Updated Name', weight = 85.0 WHERE id = '{USER_ID_2}'
            RETURNING name;
            """,
            expected_output_contains="Updated Name",
        )

        # TEST 31: Update exercise fields
        execute_test(
            "Update Exercise difficulty",
            f"""
            UPDATE exercises SET difficulty = 'Advanced', equipment = 'Dumbbell' WHERE id = '{EXERCISE_ID}'
            RETURNING difficulty;
            """,
            expected_output_contains="Advanced",
        )

        # TEST 32: Update weight entry
        execute_test(
            "Update Weight Entry weight",
            f"""
            UPDATE weight_entries SET weight = 82.0, notes = 'Updated note' WHERE id = '{WEIGHT_ENTRY_ID}'
            RETURNING weight;
            """,
            expected_output_contains="82.0",
        )

        # TEST 33: Update workout duration
        execute_test(
            "Update Workout duration",
            f"""
            UPDATE workouts SET duration = 90, type = 'pull' WHERE id = '{WORKOUT_ID}'
            RETURNING duration;
            """,
            expected_output_contains="90",
        )

        # TEST 34: Update workout set
        execute_test(
            "Update Workout Set reps and weight",
            f"""
            UPDATE workout_sets SET reps = 12, weight = 85.0 WHERE id = '{WORKOUT_SET_ID}'
            RETURNING reps;
            """,
            expected_output_contains="12",
        )

        # ===================================================================
        # SECTION 5: Soft Delete Behavior
        # ===================================================================
        print("\n--- Soft Delete Behavior ---")

        # TEST 35: Read after soft delete - verify soft-deleted user excluded
        execute_test(
            "Soft-deleted user excluded from WHERE deleted_at IS NULL",
            f"""
            SELECT count(*) FROM users WHERE id = '{USER_ID}' AND deleted_at IS NULL;
            """,
            expected_output_contains="0",
        )

        # TEST 36: Soft-deleted user still visible without filter
        execute_test(
            "Soft-deleted user still in table (no filter)",
            f"""
            SELECT count(*) FROM users WHERE id = '{USER_ID}';
            """,
            expected_output_contains="1",
        )

        # TEST 37: Exercise soft delete + re-insert same name
        execute_test(
            "Soft Delete Exercise",
            f"""
            INSERT INTO exercises (id, name, muscle_group, created_at, updated_at)
            VALUES ('{EXERCISE_ID_2}', 'Unique Squat {EXERCISE_ID_2}', 'Legs', NOW(), NOW())
            RETURNING name;
            """,
            expected_output_contains="Unique Squat",
        )

        execute_test(
            "Soft Delete the exercise",
            f"""
            UPDATE exercises SET deleted_at = NOW() WHERE id = '{EXERCISE_ID_2}' RETURNING id;
            """,
            expected_output_contains=EXERCISE_ID_2,
        )

        execute_test(
            "Re-insert exercise with same name after soft delete",
            f"""
            INSERT INTO exercises (id, name, muscle_group, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', 'Unique Squat {EXERCISE_ID_2}', 'Legs', NOW(), NOW())
            RETURNING name;
            """,
            expected_output_contains="Unique Squat",
        )

        # ===================================================================
        # SECTION 6: Multiple Valid Entries (no conflict)
        # ===================================================================
        print("\n--- Multiple Valid Entries ---")

        # TEST 40: Weight entry on different date (no conflict)
        execute_test(
            "Weight Entry on different date (should succeed)",
            f"""
            INSERT INTO weight_entries (id, user_id, weight, date, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{USER_ID_2}', 79.0, '2026-03-08', NOW(), NOW())
            RETURNING weight;
            """,
            expected_output_contains="79.0",
        )

        # TEST 41: Multiple workout sets with different set_numbers
        execute_test(
            "Multiple Workout Sets different set_numbers",
            f"""
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{WORKOUT_SET_ID_2}', '{WORKOUT_EXERCISE_ID}', 2, 8, 85.0, true, NOW(), NOW())
            RETURNING set_number;
            """,
            expected_output_contains="2",
        )

        # TEST 42: Third set
        execute_test(
            "Third Workout Set (set_number=3)",
            f"""
            INSERT INTO workout_sets (id, workout_exercise_id, set_number, reps, weight, completed, created_at, updated_at)
            VALUES ('{uuid.uuid4()}', '{WORKOUT_EXERCISE_ID}', 3, 6, 90.0, false, NOW(), NOW())
            RETURNING set_number;
            """,
            expected_output_contains="3",
        )

        # TEST 43: Verify all 3 sets exist
        execute_test(
            "Verify 3 workout sets exist",
            f"""
            SELECT count(*) FROM workout_sets WHERE workout_exercise_id = '{WORKOUT_EXERCISE_ID}';
            """,
            expected_output_contains="3",
        )

        # ===================================================================
        # SECTION 7: Cascade / Referential Cleanup
        # ===================================================================
        print("\n--- Cascade & Referential Cleanup ---")

        # TEST 44: Verify workout exercises count
        execute_test(
            "Verify workout exercise exists",
            f"""
            SELECT count(*) FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}';
            """,
            expected_output_contains="1",
        )

        # TEST 45: Hard-delete workout sets first, then workout exercise, then workout
        execute_test(
            "Hard delete workout sets",
            f"""
            DELETE FROM workout_sets WHERE workout_exercise_id = '{WORKOUT_EXERCISE_ID}';
            SELECT count(*) FROM workout_sets WHERE workout_exercise_id = '{WORKOUT_EXERCISE_ID}';
            """,
            expected_output_contains="0",
        )

        execute_test(
            "Hard delete workout exercise",
            f"""
            DELETE FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}';
            SELECT count(*) FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}';
            """,
            expected_output_contains="0",
        )

        execute_test(
            "Hard delete workout",
            f"""
            DELETE FROM workouts WHERE id = '{WORKOUT_ID}';
            SELECT count(*) FROM workouts WHERE id = '{WORKOUT_ID}';
            """,
            expected_output_contains="0",
        )

        # ===================================================================
        # SECTION 8: Data Integrity Reads
        # ===================================================================
        print("\n--- Data Integrity Reads ---")

        # TEST 48: Read a specific weight entry
        execute_test(
            "Read weight entry by ID",
            f"""
            SELECT weight FROM weight_entries WHERE id = '{WEIGHT_ENTRY_ID}';
            """,
            expected_output_contains="82.0",
        )

        # TEST 49: Read meals for user
        execute_test(
            "Read meals for user",
            f"""
            SELECT count(*) FROM meals WHERE user_id = '{USER_ID}';
            """,
            # Original user is soft-deleted; meals still have user_id that matches
            expected_output_contains="2",
        )

        # TEST 50: Verify exercise still exists (not soft-deleted)
        execute_test(
            "Exercise still exists",
            f"""
            SELECT count(*) FROM exercises WHERE id = '{EXERCISE_ID}' AND deleted_at IS NULL;
            """,
            expected_output_contains="1",
        )

    finally:
        print("\nCleaning up test data...")
        cleanup_sql = f"""
            BEGIN;
            DELETE FROM workout_sets WHERE workout_exercise_id IN (SELECT id FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}');
            DELETE FROM workout_exercises WHERE workout_id = '{WORKOUT_ID}';
            DELETE FROM workouts WHERE id = '{WORKOUT_ID}';
            DELETE FROM meals WHERE user_id = '{USER_ID}';
            DELETE FROM meals WHERE user_id = '{USER_ID_2}';
            DELETE FROM weight_entries WHERE user_id = '{USER_ID}';
            DELETE FROM weight_entries WHERE user_id = '{USER_ID_2}';
            DELETE FROM exercises WHERE name LIKE 'Test Bench Press%';
            DELETE FROM exercises WHERE name LIKE 'Unique Squat%';
            DELETE FROM users WHERE email = 'testuser_{USER_ID}@example.com';
            DELETE FROM users WHERE id = '{USER_ID_2}';
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
    print(f"Success Rate: {percentage:.1f}%")

    if tests_passed == tests_run:
        print("🎉 ALL TESTS PASSED!")
    else:
        print("⚠️  SOME TESTS FAILED:")
        for name in tests_failed_names:
            print(f"   • {name}")
        sys.exit(1)


if __name__ == "__main__":
    main()
