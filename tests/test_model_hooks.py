import subprocess
import tempfile
import textwrap
from pathlib import Path


ROOT_DIR = Path(__file__).resolve().parents[1]


def run_go_program(go_source: str) -> subprocess.CompletedProcess[str]:
    with tempfile.NamedTemporaryFile(
        mode="w",
        suffix="_hook_prog.go",
        dir=ROOT_DIR,
        delete=False,
        encoding="utf-8",
    ) as temp_file:
        temp_file.write(go_source)
        temp_path = Path(temp_file.name)

    try:
        return subprocess.run(
            ["go", "run", temp_path.name],
            cwd=ROOT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    finally:
        temp_path.unlink(missing_ok=True)


def assert_go_success(go_source: str) -> str:
    result = run_go_program(go_source)
    if result.returncode != 0:
        raise AssertionError(
            "Go program failed\n"
            f"exit code: {result.returncode}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )
    return result.stdout.strip()


def test_before_create_assigns_uuid_for_all_models() -> None:
    program = textwrap.dedent(
        """
        package main

        import (
            "fitness-tracker/models"
            "github.com/google/uuid"
        )

        func main() {
            user := models.User{}
            if err := user.BeforeCreate(nil); err != nil || user.ID == uuid.Nil {
                panic("user before_create failed")
            }

            exercise := models.Exercise{}
            if err := exercise.BeforeCreate(nil); err != nil || exercise.ID == uuid.Nil {
                panic("exercise before_create failed")
            }

            workout := models.Workout{}
            if err := workout.BeforeCreate(nil); err != nil || workout.ID == uuid.Nil {
                panic("workout before_create failed")
            }

            workoutExercise := models.WorkoutExercise{}
            if err := workoutExercise.BeforeCreate(nil); err != nil || workoutExercise.ID == uuid.Nil {
                panic("workout exercise before_create failed")
            }

            workoutProgram := models.WorkoutProgram{}
            if err := workoutProgram.BeforeCreate(nil); err != nil || workoutProgram.ID == uuid.Nil {
                panic("workout program before_create failed")
            }

            food := models.Food{}
            if err := food.BeforeCreate(nil); err != nil || food.ID == uuid.Nil {
                panic("food before_create failed")
            }

            meal := models.Meal{}
            if err := meal.BeforeCreate(nil); err != nil || meal.ID == uuid.Nil {
                panic("meal before_create failed")
            }

            mealFood := models.MealFood{}
            if err := mealFood.BeforeCreate(nil); err != nil || mealFood.ID == uuid.Nil {
                panic("meal food before_create failed")
            }

            weightEntry := models.WeightEntry{}
            if err := weightEntry.BeforeCreate(nil); err != nil || weightEntry.ID == uuid.Nil {
                panic("weight entry before_create failed")
            }

            message := models.Message{}
            if err := message.BeforeCreate(nil); err != nil || message.ID == uuid.Nil {
                panic("message before_create failed")
            }

            notification := models.Notification{}
            if err := notification.BeforeCreate(nil); err != nil || notification.ID == uuid.Nil {
                panic("notification before_create failed")
            }

            weeklyAdjustment := models.WeeklyAdjustment{}
            if err := weeklyAdjustment.BeforeCreate(nil); err != nil || weeklyAdjustment.ID == uuid.Nil {
                panic("weekly adjustment before_create failed")
            }

            friendship := models.Friendship{
                UserID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
                FriendID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
            }
            if err := friendship.BeforeCreate(nil); err != nil || friendship.ID == uuid.Nil {
                panic("friendship before_create failed")
            }
        }
        """
    )

    assert_go_success(program)


def test_before_create_does_not_override_existing_uuid() -> None:
    program = textwrap.dedent(
        """
        package main

        import (
            "fitness-tracker/models"
            "github.com/google/uuid"
        )

        func main() {
            existing := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
            user := models.User{ID: existing}

            if err := user.BeforeCreate(nil); err != nil {
                panic(err)
            }
            if user.ID != existing {
                panic("user ID was overwritten")
            }
        }
        """
    )

    assert_go_success(program)


def test_friendship_rejects_self_friendship() -> None:
    program = textwrap.dedent(
        """
        package main

        import (
            "fmt"
            "fitness-tracker/models"
            "github.com/google/uuid"
        )

        func main() {
            same := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
            friendship := models.Friendship{UserID: same, FriendID: same}

            if err := friendship.BeforeCreate(nil); err == nil {
                panic("expected self-friendship validation error")
            }

            fmt.Println("ok")
        }
        """
    )

    output = assert_go_success(program)
    assert output == "ok"


def test_friendship_allows_distinct_users() -> None:
    program = textwrap.dedent(
        """
        package main

        import (
            "fmt"
            "fitness-tracker/models"
            "github.com/google/uuid"
        )

        func main() {
            friendship := models.Friendship{
                UserID:   uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
                FriendID: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
            }

            if err := friendship.BeforeCreate(nil); err != nil {
                panic(err)
            }

            fmt.Println("ok")
        }
        """
    )

    output = assert_go_success(program)
    assert output == "ok"
