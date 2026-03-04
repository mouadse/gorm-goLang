import os
import subprocess
import tempfile
import textwrap
from pathlib import Path


ROOT_DIR = Path(__file__).resolve().parents[1]


def run_go_program(
    go_source: str, *, env_overrides: dict[str, str] | None = None
) -> subprocess.CompletedProcess[str]:
    with tempfile.NamedTemporaryFile(
        mode="w",
        suffix="_schema_prog.go",
        dir=ROOT_DIR,
        delete=False,
        encoding="utf-8",
    ) as temp_file:
        temp_file.write(go_source)
        temp_path = Path(temp_file.name)

    env = os.environ.copy()
    if env_overrides:
        env.update(env_overrides)

    try:
        return subprocess.run(
            ["go", "run", temp_path.name],
            cwd=ROOT_DIR,
            env=env,
            capture_output=True,
            text=True,
            check=False,
        )
    finally:
        temp_path.unlink(missing_ok=True)


def get_model_gorm_tag(model_name: str, field_name: str) -> str:
    program = textwrap.dedent(
        f"""
        package main

        import (
            "fmt"
            "reflect"

            "fitness-tracker/models"
        )

        func main() {{
            t := reflect.TypeOf(models.{model_name}{{}})
            field, ok := t.FieldByName("{field_name}")
            if !ok {{
                panic("field not found")
            }}

            fmt.Print(field.Tag.Get("gorm"))
        }}
        """
    )

    result = run_go_program(program)
    if result.returncode != 0:
        raise AssertionError(
            "Go program failed\n"
            f"exit code: {result.returncode}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )
    return result.stdout.strip()


def assert_go_success(
    go_source: str, *, env_overrides: dict[str, str] | None = None
) -> str:
    result = run_go_program(go_source, env_overrides=env_overrides)
    if result.returncode != 0:
        raise AssertionError(
            "Go program failed\n"
            f"exit code: {result.returncode}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )
    return result.stdout.strip()


def test_food_name_and_brand_share_partial_unique_index() -> None:
    name_tag = get_model_gorm_tag("Food", "Name")
    brand_tag = get_model_gorm_tag("Food", "Brand")

    assert "uniqueIndex:idx_name_brand" in name_tag
    assert "uniqueIndex:idx_name_brand" in brand_tag
    assert "where:deleted_at IS NULL" in name_tag
    assert "where:deleted_at IS NULL" in brand_tag


def test_friendship_friend_id_has_no_self_reference_check() -> None:
    friend_id_tag = get_model_gorm_tag("Friendship", "FriendID")
    assert "check:friend_id <> user_id" in friend_id_tag


def test_food_brand_should_be_not_null_for_unique_name_brand_rule() -> None:
    brand_tag = get_model_gorm_tag("Food", "Brand")
    assert "not null" in brand_tag


def test_meal_allows_multiple_meals_same_type_per_day() -> None:
    user_tag = get_model_gorm_tag("Meal", "UserID")
    meal_type_tag = get_model_gorm_tag("Meal", "MealType")
    date_tag = get_model_gorm_tag("Meal", "Date")

    assert "index:idx_meals_user_date_type" in user_tag
    assert "index:idx_meals_user_date_type" in meal_type_tag
    assert "index:idx_meals_user_date_type" in date_tag
    assert "uniqueIndex:idx_user_date_type" not in user_tag
    assert "uniqueIndex:idx_user_date_type" not in meal_type_tag
    assert "uniqueIndex:idx_user_date_type" not in date_tag


def test_friendship_tracks_requester_in_schema() -> None:
    requester_tag = get_model_gorm_tag("Friendship", "RequesterID")
    assert "not null" in requester_tag
    assert "check:requester_id IN (user_id, friend_id)" in requester_tag


def test_program_progress_has_unique_enrollment_week_day() -> None:
    enrollment_tag = get_model_gorm_tag("ProgramProgress", "ProgramEnrollmentID")
    week_tag = get_model_gorm_tag("ProgramProgress", "WeekNumber")
    day_tag = get_model_gorm_tag("ProgramProgress", "DayNumber")

    assert "uniqueIndex:idx_enrollment_week_day" in enrollment_tag
    assert "uniqueIndex:idx_enrollment_week_day" in week_tag
    assert "uniqueIndex:idx_enrollment_week_day" in day_tag


def test_connect_should_not_exit_process_on_invalid_dsn() -> None:
    program = textwrap.dedent(
        """
        package main

        import (
            "fmt"

            "fitness-tracker/database"
        )

        func main() {
            db, err := database.Connect()
            if err == nil {
                panic("expected connect error")
            }
            if db != nil {
                panic("expected nil db on connect error")
            }
            fmt.Println("connect returned")
        }
        """
    )

    output = assert_go_success(
        program, env_overrides={"DATABASE_URL": "postgres://%zz"}
    )
    assert output == "connect returned"


def test_friendship_should_define_canonical_pair_uniqueness() -> None:
    program = textwrap.dedent(
        """
        package main

        import (
            "fmt"

            "fitness-tracker/models"
            "github.com/google/uuid"
        )

        func main() {
            a := uuid.MustParse("11111111-1111-1111-1111-111111111111")
            b := uuid.MustParse("22222222-2222-2222-2222-222222222222")

            friendship := models.Friendship{
                UserID:   b,
                FriendID: a,
            }
            if err := friendship.BeforeCreate(nil); err != nil {
                panic(err)
            }
            if friendship.UserID != a || friendship.FriendID != b {
                panic("friendship pair was not canonicalized")
            }

            fmt.Println("ok")
        }
        """
    )

    output = assert_go_success(program)
    assert output == "ok"
