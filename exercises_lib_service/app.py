import hashlib
import json
import os
import re
import tempfile
import threading
from collections import Counter
from contextlib import asynccontextmanager
from functools import lru_cache
from pathlib import Path
from typing import Any, Literal, Optional

import numpy as np
from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from fastembed import TextEmbedding
from pydantic import BaseModel, Field

VECTOR_DIM = 384
BASE_DIR = Path(__file__).resolve().parent
CATALOG_CACHE_DIR = Path(
    os.getenv("CATALOG_CACHE_DIR", str(BASE_DIR / ".catalog_cache"))
).expanduser()
EMBEDDING_MODEL_NAME = "BAAI/bge-small-en-v1.5"
EMBEDDING_CACHE_DIR = os.getenv("EMBEDDING_CACHE_DIR") or None
SEARCH_INDEX_VERSION = "v3"
STATIC_DIR = BASE_DIR / "static"
DATA_DIR = Path(os.getenv("EXERCISE_DATA_DIR", str(BASE_DIR / "exercise_data"))).expanduser()
EXERCISES_JSON_PATH = DATA_DIR / "exercises.json"
EXERCISE_IMAGES_DIR = DATA_DIR / "exercises"
EMBEDDING_THREADS = max(1, int(os.getenv("EMBEDDING_THREADS", "2")))
EMBEDDING_PARALLEL = max(1, int(os.getenv("EMBEDDING_PARALLEL", "1")))
EMBEDDING_BATCH_SIZE = max(1, int(os.getenv("EMBEDDING_BATCH_SIZE", "32")))
TOKEN_RE = re.compile(r"[a-z0-9]+")
LEVEL_RANK = {"beginner": 0, "intermediate": 1, "expert": 2}

GOAL_LABELS = {
    "build_muscle": "Build Muscle",
    "get_stronger": "Get Stronger",
    "lose_fat": "Lose Fat",
    "move_better": "Move Better",
    "athletic_engine": "Athletic Engine",
}

EQUIPMENT_PROFILE_LABELS = {
    "any": "Any Setup",
    "home-bodyweight": "Home Bodyweight",
    "dumbbell-kit": "Dumbbell Kit",
    "kettlebell-kit": "Kettlebell Kit",
    "garage-strength": "Garage Strength",
    "full-gym": "Full Gym",
    "mobility-reset": "Mobility Reset",
}

EQUIPMENT_PROFILES = {
    "any": None,
    "home-bodyweight": {"body only", "bands", "exercise ball"},
    "dumbbell-kit": {
        "body only",
        "bands",
        "dumbbell",
        "exercise ball",
        "medicine ball",
    },
    "kettlebell-kit": {"body only", "bands", "kettlebells"},
    "garage-strength": {
        "barbell",
        "bands",
        "body only",
        "dumbbell",
        "e-z curl bar",
        "medicine ball",
    },
    "full-gym": {
        "bands",
        "barbell",
        "body only",
        "cable",
        "dumbbell",
        "e-z curl bar",
        "exercise ball",
        "foam roll",
        "kettlebells",
        "machine",
        "medicine ball",
        "other",
    },
    "mobility-reset": {"bands", "body only", "exercise ball", "foam roll"},
}

QUERY_EQUIPMENT_ALIASES = {
    "band": "bands",
    "bands": "bands",
    "barbell": "barbell",
    "bodyweight": "body only",
    "body": "body only",
    "cable": "cable",
    "dumbbell": "dumbbell",
    "dumbbells": "dumbbell",
    "ez": "e-z curl bar",
    "kettlebell": "kettlebells",
    "kettlebells": "kettlebells",
    "machine": "machine",
    "medicine": "medicine ball",
    "medball": "medicine ball",
    "mobility": "foam roll",
}

QUERY_CATEGORY_HINTS = {
    "cardio": "cardio",
    "conditioning": "cardio",
    "explosive": "plyometrics",
    "jump": "plyometrics",
    "jumps": "plyometrics",
    "plyometric": "plyometrics",
    "power": "powerlifting",
    "strength": "strength",
    "stretch": "stretching",
    "stretching": "stretching",
    "mobility": "stretching",
    "strongman": "strongman",
    "yoga": "stretching",
    "pilates": "stretching",
    "flexibility": "stretching",
    "warmup": "stretching",
    "cooldown": "stretching",
    "recovery": "stretching",
}

QUERY_EQUIPMENT_NGRAMS = {
    "foam roll": "foam roll",
    "curl bar": "e-z curl bar",
    "ez curl": "e-z curl bar",
    "exercise ball": "exercise ball",
    "medicine ball": "medicine ball",
    "body only": "body only",
}

MUSCLE_SYNONYMS = {
    "arms": ["biceps", "triceps", "forearms"],
    "back": ["lats", "middle back", "lower back", "traps"],
    "core": ["abdominals", "lower back"],
    "legs": ["quadriceps", "hamstrings", "glutes", "calves"],
    "posterior": ["hamstrings", "glutes", "lower back"],
    "push": ["chest", "shoulders", "triceps"],
    "pull": ["lats", "middle back", "biceps", "forearms"],
    "shoulder": ["shoulders"],
}

QUERY_TERM_SYNONYMS = {
    "aerobic": ["cardio", "conditioning", "endurance"],
    "bike": ["bicycling", "cycling", "pedaling"],
    "bikes": ["bicycling", "cycling", "pedaling"],
    "bicycle": ["bicycling", "cycling", "pedaling"],
    "bicycling": ["cycling", "bike", "pedaling"],
    "biking": ["bicycling", "cycling", "bike"],
    "cardio": ["conditioning", "engine", "endurance"],
    "conditioning": ["cardio", "intervals", "endurance"],
    "cycle": ["cycling", "bicycling", "pedaling"],
    "cycles": ["cycling", "bicycling", "pedaling"],
    "cycling": ["bicycling", "bike", "pedaling"],
    "elliptical": ["cardio", "conditioning", "machine"],
    "endurance": ["cardio", "conditioning", "engine"],
    "erg": ["rowing", "rower", "cardio"],
    "foam": ["rolling", "recovery", "mobility"],
    "hiit": ["conditioning", "cardio", "intervals"],
    "intervals": ["conditioning", "cardio", "hiit"],
    "jog": ["running", "treadmill", "cardio"],
    "jogging": ["running", "treadmill", "cardio"],
    "mobility": ["stretching", "recovery", "flexibility"],
    "pedal": ["pedaling", "cycling", "bicycling"],
    "pedaling": ["cycling", "bicycling", "bike"],
    "recovery": ["mobility", "stretching", "cooldown"],
    "roll": ["rolling", "foam", "recovery"],
    "rolling": ["foam", "recovery", "mobility"],
    "row": ["rowing", "rower", "erg"],
    "rower": ["rowing", "erg", "cardio"],
    "rowing": ["rower", "erg", "cardio"],
    "run": ["running", "treadmill", "cardio"],
    "runner": ["running", "treadmill", "cardio"],
    "running": ["run", "treadmill", "cardio"],
    "smr": ["foam", "rolling", "recovery"],
    "spin": ["spinning", "cycling", "bicycling"],
    "spinning": ["spin", "cycling", "bicycling"],
    "stretch": ["stretching", "mobility", "flexibility"],
    "stretching": ["stretch", "mobility", "recovery"],
    "treadmill": ["running", "walking", "cardio"],
    "walk": ["walking", "treadmill", "cardio"],
    "walking": ["walk", "treadmill", "cardio"],
}

QUERY_PHRASE_SYNONYMS = {
    "cardio machine": ["machine", "conditioning", "endurance"],
    "exercise bike": ["stationary bike", "cycling", "bicycling"],
    "foam roll": ["foam rolling", "smr", "recovery", "mobility"],
    "lower body": ["legs", "glutes", "hamstrings", "quadriceps"],
    "rowing machine": ["rowing", "rower", "erg", "cardio"],
    "spin bike": ["cycling", "bicycling", "stationary bike"],
    "stationary bike": ["cycling", "bicycling", "exercise bike"],
    "upper body": ["push", "pull", "shoulders", "back"],
}

QUERY_MUSCLE_ALIASES = {
    "ab": {"abdominals"},
    "abs": {"abdominals"},
    "arm": {"biceps", "triceps", "forearms"},
    "arms": {"biceps", "triceps", "forearms"},
    "back": {"lats", "middle back", "lower back", "traps"},
    "calf": {"calves"},
    "calves": {"calves"},
    "core": {"abdominals", "lower back"},
    "delt": {"shoulders"},
    "delts": {"shoulders"},
    "glute": {"glutes"},
    "glutes": {"glutes"},
    "hamstring": {"hamstrings"},
    "hamstrings": {"hamstrings"},
    "lat": {"lats"},
    "lats": {"lats"},
    "leg": {"quadriceps", "hamstrings", "glutes", "calves", "adductors", "abductors"},
    "legs": {"quadriceps", "hamstrings", "glutes", "calves", "adductors", "abductors"},
    "lower back": {"lower back"},
    "lower body": {
        "quadriceps",
        "hamstrings",
        "glutes",
        "calves",
        "adductors",
        "abductors",
    },
    "middle back": {"middle back"},
    "pec": {"chest"},
    "pecs": {"chest"},
    "posterior chain": {"hamstrings", "glutes", "lower back"},
    "quad": {"quadriceps"},
    "quads": {"quadriceps"},
    "rear delt": {"shoulders"},
    "rear delts": {"shoulders"},
    "shoulder": {"shoulders"},
    "shoulders": {"shoulders"},
    "trap": {"traps"},
    "traps": {"traps"},
    "tricep": {"triceps"},
    "upper back": {"middle back", "traps"},
    "upper body": {
        "chest",
        "shoulders",
        "triceps",
        "lats",
        "middle back",
        "biceps",
        "forearms",
        "traps",
    },
}

EXCLUSIVE_QUERY_TOKENS = {
    "exclusive",
    "exclusively",
    "just",
    "only",
    "purely",
    "solely",
    "strictly",
}
NEGATION_TOKENS = {"exclude", "excluding", "no", "not", "without"}
NEGATION_CONNECTORS = {"and", "or"}
NEGATION_BREAK_TOKENS = EXCLUSIVE_QUERY_TOKENS | {
    "exercise",
    "exercises",
    "focus",
    "focused",
    "for",
    "plan",
    "program",
    "session",
    "target",
    "targeting",
    "using",
    "with",
    "workout",
    "workouts",
}
NEUTRAL_SUPPORT_MUSCLES = {"abdominals", "lower back"}
PREFERRED_PROFILE_EQUIPMENT = {
    "home-bodyweight": {"body only"},
    "dumbbell-kit": {"dumbbell"},
    "kettlebell-kit": {"kettlebells"},
    "garage-strength": {"barbell", "dumbbell", "e-z curl bar"},
    "mobility-reset": {"bands", "exercise ball", "foam roll"},
}
MUSCLE_FAMILIES = {
    "push": {"chest", "shoulders", "triceps"},
    "pull": {"lats", "middle back", "biceps", "forearms", "traps", "neck"},
    "lower": {"quadriceps", "hamstrings", "glutes", "calves", "adductors", "abductors"},
    "core": {"abdominals", "lower back"},
}
MUSCLE_SUPPORT_MAP = {
    "abdominals": ["lower back"],
    "adductors": ["quadriceps", "hamstrings"],
    "biceps": ["lats", "middle back", "forearms"],
    "calves": ["quadriceps", "hamstrings"],
    "chest": ["shoulders", "triceps"],
    "forearms": ["biceps", "lats"],
    "glutes": ["hamstrings", "quadriceps"],
    "hamstrings": ["quadriceps", "calves"],
    "lats": ["middle back", "biceps", "forearms"],
    "lower back": ["abdominals", "hamstrings"],
    "middle back": ["lats", "biceps", "forearms"],
    "neck": ["traps", "middle back"],
    "quadriceps": ["hamstrings", "calves"],
    "shoulders": ["chest", "triceps", "middle back"],
    "traps": ["middle back", "shoulders"],
    "triceps": ["chest", "shoulders"],
}
FOCUS_DAY_SUFFIXES = ["Strength", "Volume", "Capacity", "Control", "Power", "Reload"]

SCORING_WEIGHTS = {
    "search_base_multiplier": 4.0,
    "expanded_token_overlap": 0.24,
    "primary_muscle_hit": 0.95,
    "secondary_muscle_hit": 0.4,
    "focus_coverage_bonus": 1.1,
    "secondary_only_penalty": -0.4,
    "strength_category_bonus": 0.45,
    "stretching_penalty": -0.9,
    "cardio_plyo_penalty": -0.6,
    "exclusive_perfect_bonus": 0.9,
    "exclusive_off_primary_penalty": -1.5,
    "exclusive_off_secondary_penalty": -0.9,
    "equipment_match_bonus": 1.0,
    "equipment_mismatch_penalty": -0.55,
    "category_match_bonus": 0.8,
    "category_stretching_penalty": -0.45,
    "force_match_bonus": 0.65,
    "level_match_bonus": 0.35,
    "filter_equipment_bonus": 0.55,
    "filter_category_bonus": 0.55,
    "filter_muscle_bonus": 0.7,
    "program_base_multiplier": 6.0,
    "program_emphasis_primary": 1.4,
    "program_emphasis_secondary": 0.5,
    "program_priority_primary": 1.6,
    "program_priority_secondary": 0.35,
    "program_priority_miss_penalty": -0.45,
    "program_focus_primary": 1.5,
    "program_focus_secondary": 0.35,
    "program_focus_secondary_penalty": -0.65,
    "program_focus_partial_penalty": -0.9,
    "program_focus_miss_penalty": -4.0,
    "program_goal_strength_bonus": 1.2,
    "program_goal_stretching_penalty": -1.2,
    "program_goal_cardio_penalty": -0.45,
    "program_goal_conditioning_bonus": 1.1,
    "program_move_better_stretch_bonus": 1.5,
    "program_compound_bonus": 0.4,
    "program_query_overlap": 0.18,
    "program_query_equipment_bonus": 0.75,
    "program_query_equipment_penalty": -0.25,
    "program_query_category_bonus": 0.55,
    "program_query_force_bonus": 0.35,
    "program_level_match": 0.3,
    "program_preferred_equipment": 0.45,
    "program_non_preferred_penalty": -0.1,
}

_catalog_lock = threading.Lock()
_embedding_model_lock = threading.Lock()
_catalog_state_lock = threading.Lock()

GoalType = Literal[
    "build_muscle",
    "get_stronger",
    "lose_fat",
    "move_better",
    "athletic_engine",
]

EquipmentProfileType = Literal[
    "any",
    "home-bodyweight",
    "dumbbell-kit",
    "kettlebell-kit",
    "garage-strength",
    "full-gym",
    "mobility-reset",
]

embedding_model: TextEmbedding | None = None
catalog: list[dict[str, Any]] = []
catalog_by_exercise_id: dict[str, dict[str, Any]] = {}
catalog_embeddings = np.empty((0, VECTOR_DIM), dtype=np.float32)
catalog_meta: dict[str, Any] = {}
catalog_runtime_source: int | None = None
catalog_level_masks: dict[str, np.ndarray] = {}
catalog_equipment_masks: dict[str, np.ndarray] = {}
catalog_category_masks: dict[str, np.ndarray] = {}
catalog_muscle_masks: dict[str, np.ndarray] = {}
catalog_all_indices = np.empty(0, dtype=np.int32)
catalog_program_cache: dict[tuple[str, str], dict[str, Any]] = {}
catalog_status = "not_started"
catalog_error: str | None = None
_catalog_init_thread: threading.Thread | None = None


class SearchRequest(BaseModel):
    query: str = Field(min_length=1)
    top_k: int = Field(default=8, ge=1, le=24)
    level: Optional[str] = None
    equipment: Optional[str] = None
    category: Optional[str] = None
    muscle: Optional[str] = None


class ExerciseResult(BaseModel):
    exercise_id: str
    score: float
    name: str
    force: str
    level: str
    mechanic: str
    equipment: str
    category: str
    primary_muscles: list[str]
    secondary_muscles: list[str]
    instructions: list[str]
    image_url: Optional[str] = None
    alt_image_url: Optional[str] = None
    match_reasons: list[str]


class SearchResponse(BaseModel):
    results: list[ExerciseResult]


class CatalogExercise(BaseModel):
    exercise_id: str
    name: str
    force: str
    level: str
    mechanic: str
    equipment: str
    category: str
    primary_muscles: list[str]
    secondary_muscles: list[str]
    instructions: list[str]
    image_url: Optional[str] = None
    alt_image_url: Optional[str] = None


class CatalogExercisesResponse(BaseModel):
    total: int
    exercises: list[CatalogExercise]


class InitResponse(BaseModel):
    status: str
    exercises_loaded: int


class MetaResponse(BaseModel):
    library_size: int
    levels: list[str]
    categories: list[str]
    equipment: list[str]
    muscles: list[str]
    equipment_profiles: list[dict[str, str]]
    sample_queries: list[str]
    spotlights: list[ExerciseResult]


class ProgramRequest(BaseModel):
    goal: GoalType = "build_muscle"
    days_per_week: int = Field(default=4, ge=2, le=6)
    session_minutes: int = Field(default=45, ge=20, le=120)
    level: str = "beginner"
    equipment_profile: EquipmentProfileType = "home-bodyweight"
    focus: list[str] = Field(default_factory=list)
    notes: str = ""


class ProgramExercise(BaseModel):
    exercise_id: str
    name: str
    image_url: Optional[str] = None
    alt_image_url: Optional[str] = None
    category: str
    mechanic: str
    equipment: str
    primary_muscles: list[str]
    secondary_muscles: list[str]
    prescription: str
    reason: str
    instructions: list[str]


class ProgramDay(BaseModel):
    day: int
    title: str
    focus: str
    duration_label: str
    exercises: list[ProgramExercise]


class ProgramResponse(BaseModel):
    summary: str
    recovery_note: str
    warmup: list[str]
    days: list[ProgramDay]


def normalize_text(value: Any, fallback: str = "Unspecified") -> str:
    if value is None:
        return fallback
    text = str(value).strip()
    return text if text else fallback


def normalize_string_list(values: Any) -> list[str]:
    items = []
    for value in values or []:
        text = str(value).strip().lower()
        if text and text not in items:
            items.append(text)
    return items


def image_url_for(relative_path: str | None) -> str | None:
    if not relative_path:
        return None
    return f"/exercise-images/{relative_path}"


def compute_derived_exercise_terms(item: dict[str, Any]) -> list[str]:
    tokens = tokenize(
        " ".join(
            [
                item["name"],
                item["category"],
                item["equipment"],
                item["force"],
                item["mechanic"],
            ]
        )
    )
    derived = list(expanded_terms(tokens))
    token_set = set(tokens)

    if item["category"] == "cardio":
        append_unique_terms(derived, ["conditioning", "engine", "endurance", "intervals"])
    elif item["category"] == "stretching":
        append_unique_terms(derived, ["mobility", "flexibility", "recovery", "warmup", "cooldown"])
    elif item["category"] == "plyometrics":
        append_unique_terms(derived, ["plyo", "jump", "explosive", "power", "athletic"])
    elif item["category"] == "strongman":
        append_unique_terms(derived, ["carry", "loaded", "conditioning", "athletic"])

    if item["equipment"] == "machine":
        append_unique_terms(derived, ["machine", "station"])
    elif item["equipment"] == "foam roll":
        append_unique_terms(derived, ["foam", "rolling", "smr", "recovery", "mobility"])

    if item["mechanic"] == "compound":
        append_unique_terms(derived, ["compound", "multi", "joint"])
    elif item["mechanic"] == "isolation":
        append_unique_terms(derived, ["accessory", "single", "joint"])

    if {"bike", "bicycling", "cycling", "recumbent"}.intersection(token_set):
        append_unique_terms(derived, ["bike", "cycling", "bicycling", "pedaling", "spin", "stationary"])
    if {"row", "rowing"}.intersection(token_set) and item["category"] == "cardio":
        append_unique_terms(derived, ["rowing", "rower", "erg", "cardio"])
    if "treadmill" in token_set:
        append_unique_terms(derived, ["running", "walking", "jogging", "cardio", "machine"])
    if "smr" in token_set:
        append_unique_terms(derived, ["foam", "rolling", "recovery", "mobility"])

    return derived


def prepare_catalog_item(item: dict[str, Any]) -> dict[str, Any]:
    primary_muscles = frozenset(item["primary_muscles"])
    secondary_muscles = frozenset(item["secondary_muscles"])
    derived_terms = tuple(compute_derived_exercise_terms(item))
    descriptor_tokens = frozenset(
        derived_terms
        + tuple(
            tokenize(
                " ".join(
                    [
                        item["name"],
                        item["category"],
                        item["equipment"],
                        item["force"],
                        item["mechanic"],
                        " ".join(item["primary_muscles"]),
                        " ".join(item["secondary_muscles"]),
                    ]
                )
            )
        )
    )
    item["_primary_muscles_set"] = primary_muscles
    item["_secondary_muscles_set"] = secondary_muscles
    item["_all_muscles_set"] = primary_muscles | secondary_muscles
    item["_derived_terms"] = derived_terms
    item["_descriptor_tokens"] = descriptor_tokens
    item["_level_rank"] = LEVEL_RANK.get(item["level"], 0)
    item["_is_compound"] = item["mechanic"] == "compound"
    item["_is_isolation"] = item["mechanic"] == "isolation"
    item["_is_strength_category"] = item["category"] in {
        "strength",
        "powerlifting",
        "olympic weightlifting",
        "strongman",
    }
    item["_is_program_strength_category"] = item["category"] in {
        "strength",
        "powerlifting",
        "olympic weightlifting",
    }
    item["_is_cardio_plyo"] = item["category"] in {"cardio", "plyometrics"}
    item["_is_conditioning_category"] = item["category"] in {"cardio", "plyometrics", "strongman"}
    item["_is_stretching"] = item["category"] == "stretching"
    item["_is_mobility_item"] = item["_is_stretching"] or item["equipment"] == "foam roll"
    item["_has_core_primary"] = bool({"abdominals", "lower back"}.intersection(primary_muscles))
    item["_category_title"] = item["category"].title()
    item["_equipment_title"] = item["equipment"].title()
    item["_level_title"] = item["level"].title()
    if item["primary_muscles"]:
        item["_fallback_match_reasons"] = [item["_category_title"], item["primary_muscles"][0].title()]
    else:
        item["_fallback_match_reasons"] = [item["_category_title"], item["_equipment_title"]]
    return item


def load_exercise_catalog() -> list[dict[str, Any]]:
    if not EXERCISES_JSON_PATH.exists():
        raise HTTPException(
            status_code=500,
            detail="Local exercise dataset is missing. Expected exercise_data/exercises.json.",
        )

    with EXERCISES_JSON_PATH.open("r", encoding="utf-8") as handle:
        raw_items = json.load(handle)

    items: list[dict[str, Any]] = []
    for index, raw in enumerate(raw_items, start=1):
        instructions = [
            normalize_text(step, "").strip() for step in raw.get("instructions", [])
        ]
        instructions = [step for step in instructions if step]
        images = [normalize_text(path, "").strip() for path in raw.get("images", [])]
        images = [path for path in images if path]
        item = {
            "index_id": index,
            "_catalog_index": len(items),
            "exercise_id": normalize_text(raw.get("id"), f"exercise-{index}"),
            "name": normalize_text(raw.get("name")),
            "force": normalize_text(raw.get("force")),
            "level": normalize_text(raw.get("level")).lower(),
            "mechanic": normalize_text(raw.get("mechanic")).lower(),
            "equipment": normalize_text(raw.get("equipment")).lower(),
            "category": normalize_text(raw.get("category")).lower(),
            "primary_muscles": normalize_string_list(raw.get("primaryMuscles")),
            "secondary_muscles": normalize_string_list(raw.get("secondaryMuscles")),
            "instructions": instructions,
            "images": images,
        }
        item["image_url"] = image_url_for(images[0] if images else None)
        item["alt_image_url"] = image_url_for(
            images[1] if len(images) > 1 else images[0] if images else None
        )
        items.append(prepare_catalog_item(item))
    return items


def catalog_cache_path() -> Path:
    dataset_hash = hashlib.sha256(EXERCISES_JSON_PATH.read_bytes()).hexdigest()[:16]
    model_slug = re.sub(r"[^a-z0-9]+", "-", EMBEDDING_MODEL_NAME.lower()).strip("-")
    return CATALOG_CACHE_DIR / f"{model_slug}-{SEARCH_INDEX_VERSION}-{dataset_hash}.npz"


def load_cached_catalog_embeddings(
    items: list[dict[str, Any]],
) -> np.ndarray | None:
    cache_path = catalog_cache_path()
    if not cache_path.exists():
        return None

    expected_ids = np.array([item["exercise_id"] for item in items])
    try:
        with np.load(cache_path, allow_pickle=False) as payload:
            embeddings = np.asarray(payload["embeddings"], dtype=np.float32)
            exercise_ids = np.asarray(payload["exercise_ids"])
    except Exception:
        return None

    if embeddings.shape != (len(items), VECTOR_DIM):
        return None
    if exercise_ids.shape != expected_ids.shape or not np.array_equal(
        exercise_ids, expected_ids
    ):
        return None
    return embeddings


def save_cached_catalog_embeddings(
    items: list[dict[str, Any]],
    embeddings: np.ndarray,
) -> None:
    cache_path = catalog_cache_path()
    cache_path.parent.mkdir(parents=True, exist_ok=True)
    exercise_ids = np.array([item["exercise_id"] for item in items])
    with tempfile.NamedTemporaryFile(
        dir=cache_path.parent, suffix=".npz", delete=False
    ) as handle:
        temp_path = Path(handle.name)
    try:
        np.savez_compressed(
            temp_path,
            embeddings=np.asarray(embeddings, dtype=np.float32),
            exercise_ids=exercise_ids,
        )
        temp_path.replace(cache_path)
    finally:
        if temp_path.exists():
            temp_path.unlink(missing_ok=True)


def ensure_embedding_model() -> TextEmbedding:
    global embedding_model

    if embedding_model is None:
        with _embedding_model_lock:
            if embedding_model is None:
                embedding_kwargs: dict[str, Any] = {
                    "model_name": EMBEDDING_MODEL_NAME,
                    "threads": EMBEDDING_THREADS,
                }
                if EMBEDDING_CACHE_DIR:
                    embedding_kwargs["cache_dir"] = EMBEDDING_CACHE_DIR
                embedding_model = TextEmbedding(**embedding_kwargs)
    return embedding_model


def tokenize(text: str) -> list[str]:
    return TOKEN_RE.findall(text.lower())


def append_unique_terms(target: list[str], values: list[str]) -> None:
    for value in values:
        if value not in target:
            target.append(value)


@lru_cache(maxsize=512)
def _expanded_terms_cached(tokens: tuple[str, ...]) -> tuple[str, ...]:
    expanded = list(tokens)
    for token in tokens:
        append_unique_terms(expanded, MUSCLE_SYNONYMS.get(token, []))
        append_unique_terms(expanded, QUERY_TERM_SYNONYMS.get(token, []))
    joined = " ".join(tokens)
    for phrase, synonyms in QUERY_PHRASE_SYNONYMS.items():
        if phrase in joined:
            append_unique_terms(expanded, tokenize(" ".join(synonyms)))
    return tuple(expanded)



def expanded_terms(tokens: list[str]) -> tuple[str, ...]:
    return _expanded_terms_cached(tuple(tokens))


def catalog_muscles() -> set[str]:
    return set(catalog_meta.get("muscles", []))


def resolve_query_muscles(fragment: str) -> set[str]:
    normalized = normalize_text(fragment, "").strip().lower()
    if not normalized:
        return set()

    matches = set(QUERY_MUSCLE_ALIASES.get(normalized, set()))
    matches.update(MUSCLE_SYNONYMS.get(normalized, []))
    if normalized in catalog_muscles():
        matches.add(normalized)
    return matches


def extract_query_muscles(tokens: list[str]) -> set[str]:
    muscles = set()
    for token in tokens:
        muscles.update(resolve_query_muscles(token))
    for span in (3, 2):
        for index in range(0, max(len(tokens) - span + 1, 0)):
            muscles.update(
                resolve_query_muscles(" ".join(tokens[index : index + span]))
            )
    return muscles


def extract_negated_muscles(tokens: list[str]) -> set[str]:
    excluded = set()
    index = 0
    while index < len(tokens):
        if tokens[index] not in NEGATION_TOKENS:
            index += 1
            continue

        cursor = index + 1
        while cursor < len(tokens):
            token = tokens[cursor]
            if token in NEGATION_BREAK_TOKENS:
                break
            if token in NEGATION_CONNECTORS:
                cursor += 1
                continue

            matched = False
            for span in (3, 2, 1):
                fragment = " ".join(tokens[cursor : cursor + span])
                muscles = resolve_query_muscles(fragment)
                if muscles:
                    excluded.update(muscles)
                    cursor += span
                    matched = True
                    break
            if not matched:
                cursor += 1

        index = cursor

    return excluded


def derived_exercise_terms(item: dict[str, Any]) -> tuple[str, ...]:
    cached = item.get("_derived_terms")
    if cached is None:
        cached = tuple(compute_derived_exercise_terms(item))
        item["_derived_terms"] = cached
    return cached


def exercise_descriptor_tokens(item: dict[str, Any]) -> frozenset[str]:
    cached = item.get("_descriptor_tokens")
    if cached is None:
        cached = frozenset(
            derived_exercise_terms(item)
            + tuple(
                tokenize(
                    " ".join(
                        [
                            item["name"],
                            item["category"],
                            item["equipment"],
                            item["force"],
                            item["mechanic"],
                            " ".join(item["primary_muscles"]),
                            " ".join(item["secondary_muscles"]),
                        ]
                    )
                )
            )
        )
        item["_descriptor_tokens"] = cached
    return cached


def build_exercise_text(item: dict[str, Any]) -> str:
    instructions = " ".join(item["instructions"][:2])
    search_tags = " ".join(derived_exercise_terms(item)[:32])
    return (
        f"{item['name']}. "
        f"Category: {item['category']}. "
        f"Equipment: {item['equipment']}. "
        f"Primary muscles: {', '.join(item['primary_muscles'])}. "
        f"Secondary muscles: {', '.join(item['secondary_muscles'])}. "
        f"{item['force']} exercise. {item['mechanic']} movement. "
        f"{item['level']} level. "
        f"Instructions: {instructions}. "
        f"Search tags: {search_tags}."
    )


def build_embedding_for_exercise(item: dict[str, Any]) -> list[float]:
    model = ensure_embedding_model()
    text = build_exercise_text(item)
    embedding = list(model.passage_embed([text]))[0]
    return np.asarray(embedding, dtype=np.float32).tolist()


@lru_cache(maxsize=256)
def _cached_query_embedding(
    query: str,
    context_terms: tuple[str, ...],
    model_key: int,
) -> np.ndarray:
    del model_key
    model = ensure_embedding_model()
    text = " ".join(expanded_terms(tokenize(query))) or query
    if context_terms:
        text = f"{text} {' '.join(expanded_terms(list(context_terms)))}"
    embedding = np.asarray(list(model.query_embed([text]))[0], dtype=np.float32)
    return embedding



def build_query_embedding(
    query: str, context_terms: list[str] | None = None
) -> np.ndarray:
    model = ensure_embedding_model()
    return _cached_query_embedding(query, tuple(context_terms or ()), id(model))


def build_catalog_meta(items: list[dict[str, Any]]) -> dict[str, Any]:
    muscles = Counter()
    for item in items:
        muscles.update(item["primary_muscles"])

    spotlights: list[ExerciseResult] = []
    spotlight_categories = ["strength", "cardio", "plyometrics", "stretching"]
    for category in spotlight_categories:
        match = next(
            (
                item
                for item in items
                if item["category"] == category and item["image_url"]
            ),
            None,
        )
        if match:
            spotlights.append(
                serialize_exercise(
                    match, 0.94, [category.title(), match["equipment"].title()]
                )
            )

    return {
        "library_size": len(items),
        "levels": sorted({item["level"] for item in items}),
        "categories": sorted({item["category"] for item in items}),
        "equipment": sorted({item["equipment"] for item in items}),
        "muscles": [name for name, _ in muscles.most_common()],
        "equipment_profiles": [
            {"value": value, "label": label}
            for value, label in EQUIPMENT_PROFILE_LABELS.items()
        ],
        "sample_queries": [
            "push workout for chest and shoulders with dumbbells",
            "beginner lower-body session for home training",
            "mobility moves for tight hips and hamstrings",
            "conditioning circuit for athletic engine",
            "core work with bodyweight only",
        ],
        "spotlights": spotlights,
    }


def rebuild_catalog_runtime_state() -> None:
    global catalog_runtime_source
    global catalog_level_masks, catalog_equipment_masks, catalog_category_masks
    global catalog_muscle_masks, catalog_all_indices, catalog_program_cache

    catalog_runtime_source = id(catalog)
    catalog_size = len(catalog)
    catalog_all_indices = np.arange(catalog_size, dtype=np.int32)

    catalog_level_masks = {
        level: np.fromiter(
            (item["level"] == level for item in catalog),
            dtype=bool,
            count=catalog_size,
        )
        for level in catalog_meta.get("levels", [])
    }
    catalog_equipment_masks = {
        equipment: np.fromiter(
            (item["equipment"] == equipment for item in catalog),
            dtype=bool,
            count=catalog_size,
        )
        for equipment in catalog_meta.get("equipment", [])
    }
    catalog_category_masks = {
        category: np.fromiter(
            (item["category"] == category for item in catalog),
            dtype=bool,
            count=catalog_size,
        )
        for category in catalog_meta.get("categories", [])
    }

    all_muscles = sorted(
        {
            muscle
            for item in catalog
            for muscle in item["_all_muscles_set"]
        }
    )
    catalog_muscle_masks = {
        muscle: np.fromiter(
            (muscle in item["_all_muscles_set"] for item in catalog),
            dtype=bool,
            count=catalog_size,
        )
        for muscle in all_muscles
    }

    level_buckets = {
        requested_level: [
            item for item in catalog if item["_level_rank"] <= LEVEL_RANK[requested_level]
        ]
        for requested_level in LEVEL_RANK
    }
    catalog_program_cache = {}
    for requested_level, level_items in level_buckets.items():
        level_entry = {
            "items": level_items,
            "id_set": frozenset(item["exercise_id"] for item in level_items),
            "index_set": frozenset(item["_catalog_index"] for item in level_items),
        }
        for equipment_profile, allowed_equipment in EQUIPMENT_PROFILES.items():
            if allowed_equipment is None:
                catalog_program_cache[(requested_level, equipment_profile)] = level_entry
                continue
            filtered_items = [
                item for item in level_items if item["equipment"] in allowed_equipment
            ]
            chosen_items = filtered_items or level_items
            catalog_program_cache[(requested_level, equipment_profile)] = {
                "items": chosen_items,
                "id_set": frozenset(item["exercise_id"] for item in chosen_items),
                "index_set": frozenset(item["_catalog_index"] for item in chosen_items),
            }



def ensure_catalog_runtime_state() -> None:
    if catalog and catalog_runtime_source != id(catalog):
        rebuild_catalog_runtime_state()


def set_catalog_state(status: str, error: str | None = None) -> None:
    global catalog_status, catalog_error
    with _catalog_state_lock:
        catalog_status = status
        catalog_error = error


def get_catalog_state() -> tuple[str, str | None]:
    with _catalog_state_lock:
        return catalog_status, catalog_error


def trigger_catalog_initialization(force_rebuild: bool = False) -> None:
    global _catalog_init_thread, catalog_status, catalog_error

    with _catalog_state_lock:
        if _catalog_init_thread and _catalog_init_thread.is_alive():
            return
        if not force_rebuild and catalog_status == "ready" and catalog:
            return

        catalog_status = "starting"
        catalog_error = None

        def runner() -> None:
            global _catalog_init_thread
            try:
                initialize_catalog(force_rebuild=force_rebuild)
            except Exception as exc:
                set_catalog_state("error", str(exc))
            finally:
                with _catalog_state_lock:
                    _catalog_init_thread = None

        _catalog_init_thread = threading.Thread(
            target=runner,
            name="catalog-initializer",
            daemon=True,
        )
        _catalog_init_thread.start()


def initialize_catalog(force_rebuild: bool = False) -> int:
    global catalog, catalog_by_exercise_id, catalog_embeddings, catalog_meta

    items = load_exercise_catalog()
    embeddings = None if force_rebuild else load_cached_catalog_embeddings(items)
    if embeddings is None:
        model = ensure_embedding_model()
        exercise_texts = [build_exercise_text(item) for item in items]
        embeddings = np.array(
            [
                np.asarray(emb, dtype=np.float32)
                for emb in model.passage_embed(
                    exercise_texts,
                    batch_size=EMBEDDING_BATCH_SIZE,
                    parallel=EMBEDDING_PARALLEL,
                )
            ],
            dtype=np.float32,
        )
        save_cached_catalog_embeddings(items, embeddings)

    with _catalog_lock:
        catalog = items
        catalog_by_exercise_id = {item["exercise_id"]: item for item in items}
        catalog_embeddings = embeddings
        catalog_meta = build_catalog_meta(items)
        rebuild_catalog_runtime_state()
    set_catalog_state("ready")
    return len(items)


def ensure_catalog() -> None:
    if not catalog:
        trigger_catalog_initialization()
        status, error = get_catalog_state()
        if status == "error":
            raise HTTPException(
                status_code=503,
                detail=f"Catalog initialization failed: {error}",
            )
        raise HTTPException(
            status_code=503,
            detail="Catalog is still initializing. Retry in a moment.",
        )
    ensure_catalog_runtime_state()


def serialize_exercise(
    item: dict[str, Any],
    score: float,
    match_reasons: list[str] | None = None,
) -> ExerciseResult:
    reasons = []
    for reason in match_reasons or []:
        if reason and reason not in reasons:
            reasons.append(reason)

    return ExerciseResult.model_construct(
        exercise_id=item["exercise_id"],
        score=max(0.0, score),
        name=item["name"],
        force=item["force"],
        level=item["level"],
        mechanic=item["mechanic"],
        equipment=item["equipment"],
        category=item["category"],
        primary_muscles=item["primary_muscles"],
        secondary_muscles=item["secondary_muscles"],
        instructions=item["instructions"],
        image_url=item["image_url"],
        alt_image_url=item["alt_image_url"],
        match_reasons=reasons,
    )




@lru_cache(maxsize=256)
def _cached_combined_muscle_mask(
    muscles_key: tuple[str, ...],
    runtime_source: int | None,
) -> np.ndarray:
    del runtime_source
    muscle_mask = np.zeros(len(catalog), dtype=bool)
    for muscle in muscles_key:
        current_mask = catalog_muscle_masks.get(muscle)
        if current_mask is not None:
            muscle_mask |= current_mask
    return muscle_mask



def vector_search(
    query: str,
    *,
    top_k: int,
    level: str | None = None,
    equipment: str | None = None,
    category: str | None = None,
    muscles: set[str] | None = None,
    extra_context: list[str] | None = None,
) -> list[dict[str, Any]]:
    ensure_catalog_runtime_state()
    context_terms = [
        value for value in [level, equipment, category, *(extra_context or [])] if value
    ]
    query_embedding = build_query_embedding(query, context_terms)

    mask = None
    if level:
        level_mask = catalog_level_masks.get(level)
        if level_mask is None:
            return []
        mask = level_mask.copy()
    if equipment:
        equipment_mask = catalog_equipment_masks.get(equipment)
        if equipment_mask is None:
            return []
        mask = equipment_mask.copy() if mask is None else (mask & equipment_mask)
    if category:
        category_mask = catalog_category_masks.get(category)
        if category_mask is None:
            return []
        mask = category_mask.copy() if mask is None else (mask & category_mask)
    if muscles:
        muscle_mask = _cached_combined_muscle_mask(
            tuple(sorted(muscles)),
            catalog_runtime_source,
        )
        mask = muscle_mask if mask is None else (mask & muscle_mask)

    filtered_indices = catalog_all_indices if mask is None else np.flatnonzero(mask)
    if len(filtered_indices) == 0:
        return []

    top_limit = max(top_k, 1)
    similarities = np.dot(catalog_embeddings[filtered_indices], query_embedding)
    if len(filtered_indices) > top_limit:
        top_positions = np.argpartition(similarities, -top_limit)[-top_limit:]
        order = top_positions[np.argsort(similarities[top_positions])[::-1]]
        filtered_indices = filtered_indices[order]
        similarities = similarities[order]
    else:
        order = np.argsort(similarities)[::-1]
        filtered_indices = filtered_indices[order]
        similarities = similarities[order]

    return list(zip(filtered_indices.tolist(), similarities.tolist()))


def score_to_match_strength(raw_score: float) -> float:
    """Map cosine similarity to [0, 1]. BGE embeddings typically produce
    similarities in [0.2, 0.95], so we rescale from [0.2, 1.0] to preserve
    fine-grained ranking signal."""
    normalized = (raw_score - 0.2) / 0.8
    return round(max(0.0, min(1.0, normalized)), 3)


@lru_cache(maxsize=512)
def _cached_analyze_query(
    query: str,
    runtime_source: int | None,
) -> dict[str, frozenset[str] | bool]:
    del runtime_source
    tokens = tokenize(query)
    expanded = set(expanded_terms(tokens))
    muscles = {token for token in expanded if token in catalog_muscles()}
    muscles.update(extract_query_muscles(tokens))
    excluded_muscles = extract_negated_muscles(tokens)
    muscles.difference_update(excluded_muscles)
    expanded.update(muscles)
    expanded.difference_update(excluded_muscles)

    equipment = {
        QUERY_EQUIPMENT_ALIASES[token]
        for token in tokens
        if token in QUERY_EQUIPMENT_ALIASES
    }
    joined = " ".join(tokens)
    for ngram, canonical in QUERY_EQUIPMENT_NGRAMS.items():
        if ngram in joined:
            equipment.add(canonical)

    categories = {
        QUERY_CATEGORY_HINTS[token] for token in tokens if token in QUERY_CATEGORY_HINTS
    }
    forces = set()
    if "push" in tokens or "press" in tokens:
        forces.add("push")
    if "pull" in tokens or "row" in tokens:
        forces.add("pull")

    exclusive_tokens = EXCLUSIVE_QUERY_TOKENS.intersection(tokens)
    has_body_only_equipment_phrase = any(
        tokens[index : index + 2] == ["body", "only"]
        and (index == 0 or tokens[index - 1] not in {"lower", "upper"})
        for index in range(max(len(tokens) - 1, 0))
    )
    if exclusive_tokens == {"only"} and has_body_only_equipment_phrase:
        is_exclusive = False
    else:
        is_exclusive = bool(exclusive_tokens) or "nothing but" in joined

    return {
        "tokens": frozenset(tokens),
        "expanded": frozenset(expanded),
        "muscles": frozenset(muscles),
        "excluded_muscles": frozenset(excluded_muscles),
        "equipment": frozenset(equipment),
        "categories": frozenset(categories),
        "forces": frozenset(forces),
        "exclusive": is_exclusive,
    }



def analyze_query(query: str) -> dict[str, frozenset[str] | bool]:
    ensure_catalog_runtime_state()
    return _cached_analyze_query(query, catalog_runtime_source)


def target_muscles_for_search(
    request: SearchRequest,
    query_signals: dict[str, set[str] | bool],
) -> set[str]:
    target_muscles = set(query_signals["muscles"])
    if request.muscle:
        target_muscles.add(request.muscle)
    return target_muscles


def search_context_terms(
    request: SearchRequest,
    query_signals: dict[str, set[str] | bool],
) -> list[str] | None:
    context = sorted(target_muscles_for_search(request, query_signals))
    return context or None


def matches_search_intent(
    item: dict[str, Any],
    request: SearchRequest,
    query_signals: dict[str, set[str] | bool],
    *,
    target_muscles: set[str] | None = None,
    excluded_muscles: set[str] | None = None,
) -> bool:
    primary_muscles = item["_primary_muscles_set"]
    item_muscles = item["_all_muscles_set"]
    excluded_muscles = (
        set(query_signals["excluded_muscles"])
        if excluded_muscles is None
        else excluded_muscles
    )
    target_muscles = (
        target_muscles_for_search(request, query_signals)
        if target_muscles is None
        else target_muscles
    )

    if request.muscle and request.muscle not in item_muscles:
        return False
    if excluded_muscles and excluded_muscles.intersection(item_muscles):
        return False
    if target_muscles:
        if not item_muscles.intersection(target_muscles):
            return False
        if query_signals["exclusive"] and not primary_muscles.intersection(
            target_muscles
        ):
            return False
    return True


def search_result_score(
    item: dict[str, Any],
    base_score: float,
    request: SearchRequest,
    query_signals: dict[str, set[str] | bool],
    *,
    target_muscles: set[str] | None = None,
) -> float:
    primary_muscles = item["_primary_muscles_set"]
    secondary_muscles = item["_secondary_muscles_set"]
    all_muscles = item["_all_muscles_set"]
    descriptor_tokens = item["_descriptor_tokens"]
    target_muscles = (
        target_muscles_for_search(request, query_signals)
        if target_muscles is None
        else target_muscles
    )
    primary_hits = target_muscles.intersection(primary_muscles)
    secondary_hits = target_muscles.intersection(secondary_muscles)
    w = SCORING_WEIGHTS
    score = base_score * w["search_base_multiplier"]
    score += len(query_signals["expanded"].intersection(descriptor_tokens)) * w["expanded_token_overlap"]
    score += len(primary_hits) * w["primary_muscle_hit"]
    score += len(secondary_hits) * w["secondary_muscle_hit"]

    if target_muscles:
        focus_hits = primary_hits | secondary_hits
        score += (len(focus_hits) / max(1, len(all_muscles))) * w["focus_coverage_bonus"]
        if not primary_hits and secondary_hits:
            score += w["secondary_only_penalty"]
        if not query_signals["categories"]:
            if item["_is_strength_category"]:
                score += w["strength_category_bonus"]
            elif item["_is_stretching"]:
                score += w["stretching_penalty"]
            elif item["_is_cardio_plyo"]:
                score += w["cardio_plyo_penalty"]
        if query_signals["exclusive"]:
            off_target_primary = primary_muscles - target_muscles
            off_target_secondary = (
                secondary_muscles - target_muscles - NEUTRAL_SUPPORT_MUSCLES
            )
            if not off_target_primary and not off_target_secondary:
                score += w["exclusive_perfect_bonus"]
            score += len(off_target_primary) * w["exclusive_off_primary_penalty"]
            score += len(off_target_secondary) * w["exclusive_off_secondary_penalty"]

    if query_signals["equipment"]:
        if item["equipment"] in query_signals["equipment"]:
            score += w["equipment_match_bonus"]
        elif item["equipment"] not in {"body only", "unspecified"}:
            score += w["equipment_mismatch_penalty"]

    if query_signals["categories"]:
        if item["category"] in query_signals["categories"]:
            score += w["category_match_bonus"]
        elif item["_is_stretching"]:
            score += w["category_stretching_penalty"]

    if query_signals["forces"] and item["force"] in query_signals["forces"]:
        score += w["force_match_bonus"]

    if request.level and item["level"] == request.level:
        score += w["level_match_bonus"]
    if request.equipment and item["equipment"] == request.equipment:
        score += w["filter_equipment_bonus"]
    if request.category and item["category"] == request.category:
        score += w["filter_category_bonus"]
    if request.muscle and request.muscle in item["primary_muscles"]:
        score += w["filter_muscle_bonus"]

    return score


def build_match_reasons(
    item: dict[str, Any],
    *,
    muscle: str | None,
    level: str | None,
    equipment: str | None,
    category: str | None,
) -> list[str]:
    reasons = []
    if muscle and muscle in item["primary_muscles"]:
        reasons.append(f"Primary: {muscle}")
    elif muscle and muscle in item["secondary_muscles"]:
        reasons.append(f"Secondary: {muscle}")
    if category and item["category"] == category:
        reasons.append(item["_category_title"])
    if equipment and item["equipment"] == equipment:
        reasons.append(item["_equipment_title"])
    if level and item["level"] == level:
        reasons.append(item["_level_title"])
    if not reasons:
        reasons = item["_fallback_match_reasons"]
    return reasons


def within_requested_level(candidate_level: str, requested_level: str) -> bool:
    candidate_rank = LEVEL_RANK.get(candidate_level, 0)
    requested_rank = LEVEL_RANK.get(requested_level, 0)
    return candidate_rank <= requested_rank


def program_pool_for_request(request: ProgramRequest) -> dict[str, Any]:
    ensure_catalog_runtime_state()
    cached = catalog_program_cache.get((request.level, request.equipment_profile))
    if cached is not None:
        return cached

    allowed_equipment = EQUIPMENT_PROFILES[request.equipment_profile]
    level_items = [
        item for item in catalog if item["_level_rank"] <= LEVEL_RANK.get(request.level, 0)
    ]
    if allowed_equipment is None:
        chosen_items = level_items
    else:
        filtered_items = [
            item for item in level_items if item["equipment"] in allowed_equipment
        ]
        chosen_items = filtered_items or level_items

    return {
        "items": chosen_items,
        "id_set": frozenset(item["exercise_id"] for item in chosen_items),
        "index_set": frozenset(item["_catalog_index"] for item in chosen_items),
    }



def filtered_catalog_for_program(request: ProgramRequest) -> list[dict[str, Any]]:
    return program_pool_for_request(request)["items"]


def muscle_family(muscle: str) -> str:
    for family, muscles in MUSCLE_FAMILIES.items():
        if muscle in muscles:
            return family
    return "mixed"


def focus_support_muscles(muscles: list[str]) -> list[str]:
    support: list[str] = []
    for muscle in muscles:
        for candidate in MUSCLE_SUPPORT_MAP.get(muscle, []):
            if candidate not in muscles and candidate not in support:
                support.append(candidate)
    return support


def selectors_for_focus(request: ProgramRequest, families: set[str]) -> list[str]:
    if request.goal == "move_better":
        return ["mobility", "compound", "accessory", "core", "mobility"]
    if families == {"core"}:
        return ["core", "compound", "accessory", "mobility", "fallback"]
    if request.goal in {"lose_fat", "athletic_engine"}:
        return ["compound", "conditioning", "compound", "accessory", "core"]
    if "lower" in families:
        return ["compound", "compound", "accessory", "accessory", "mobility"]
    return ["compound", "compound", "accessory", "accessory", "core"]


def query_terms_for_focus(
    request: ProgramRequest, priority: list[str], support: list[str]
) -> str:
    families = {muscle_family(muscle) for muscle in priority}
    family_terms = {
        "core": "trunk stability posture control",
        "lower": "lower body squat hinge unilateral leg strength",
        "pull": "pull row grip upper back",
        "push": "push press upper body",
        "mixed": "strength athletic balance",
    }
    terms = [" ".join(priority), " ".join(support[:2])]
    terms.extend(
        sorted({family_terms.get(family, family_terms["mixed"]) for family in families})
    )
    goal_terms = {
        "athletic_engine": "athletic explosive conditioning",
        "build_muscle": "strength hypertrophy controlled reps",
        "get_stronger": "heavy strength compound clean reps",
        "lose_fat": "conditioning circuit work capacity",
        "move_better": "mobility controlled range posture",
    }
    terms.append(goal_terms[request.goal])
    return " ".join(term for term in terms if term).strip()


def focus_text_for_blueprint(priority: list[str], support: list[str]) -> str:
    lead = " and ".join(priority[:2]) if len(priority) > 1 else priority[0]
    if support:
        return f"{lead}, with support from {', '.join(support[:2])}"
    return lead


def focus_driven_program_blueprints(request: ProgramRequest) -> list[dict[str, Any]]:
    focus = request.focus[:]
    if not focus:
        return []

    primary_width = 2 if len(focus) > 1 else 1
    blueprints = []
    for day_index in range(request.days_per_week):
        priority: list[str] = []
        for offset in range(primary_width):
            muscle = focus[(day_index + offset) % len(focus)]
            if muscle not in priority:
                priority.append(muscle)

        support = focus_support_muscles(priority)
        emphasis = priority + [muscle for muscle in focus if muscle not in priority]
        emphasis.extend(muscle for muscle in support if muscle not in emphasis)
        families = {muscle_family(muscle) for muscle in priority}

        blueprints.append(
            {
                "title": f"{' + '.join(muscle.title() for muscle in priority)} {FOCUS_DAY_SUFFIXES[day_index % len(FOCUS_DAY_SUFFIXES)]}",
                "focus": focus_text_for_blueprint(priority, support),
                "query": query_terms_for_focus(request, priority, support),
                "emphasis": emphasis,
                "priority": priority,
                "selectors": selectors_for_focus(request, families),
                "strict_focus": True,
            }
        )

    return blueprints


def program_blueprints(request: ProgramRequest) -> list[dict[str, Any]]:
    focus_blueprints = focus_driven_program_blueprints(request)
    if focus_blueprints:
        return focus_blueprints

    focus = request.focus[:3]
    if request.goal == "move_better":
        base = [
            {
                "title": "Mobility Reset",
                "focus": "hips, thoracic rotation, and breathing",
                "query": "mobility stretching hips hamstrings thoracic spine breathing control",
                "emphasis": ["hamstrings", "glutes", "lower back"],
                "selectors": ["mobility", "mobility", "core", "mobility", "fallback"],
            },
            {
                "title": "Stability Builder",
                "focus": "single-leg control and trunk tension",
                "query": "stability core balance lower body bodyweight control",
                "emphasis": ["quadriceps", "glutes", "abdominals"],
                "selectors": ["compound", "core", "accessory", "mobility", "fallback"],
            },
            {
                "title": "Upper Body Range",
                "focus": "shoulders, upper back, and posture",
                "query": "shoulder mobility posture upper back control",
                "emphasis": ["shoulders", "middle back", "lats"],
                "selectors": ["mobility", "accessory", "core", "mobility", "fallback"],
            },
            {
                "title": "Recovery Flow",
                "focus": "deep tissue, length, and cooldown",
                "query": "recovery stretching foam roll cooldown full body",
                "emphasis": focus or ["hamstrings", "lower back"],
                "selectors": ["mobility", "mobility", "mobility", "fallback"],
            },
        ]
    elif request.goal == "lose_fat":
        base = [
            {
                "title": "Lower Body Heat",
                "focus": "legs, glutes, and output",
                "query": "lower body conditioning legs glutes cardio power",
                "emphasis": ["quadriceps", "hamstrings", "glutes"],
                "selectors": [
                    "compound",
                    "conditioning",
                    "accessory",
                    "core",
                    "fallback",
                ],
            },
            {
                "title": "Upper Body Push",
                "focus": "chest, shoulders, triceps",
                "query": "upper body push circuit chest shoulders triceps conditioning",
                "emphasis": ["chest", "shoulders", "triceps"],
                "selectors": [
                    "compound",
                    "compound",
                    "accessory",
                    "conditioning",
                    "fallback",
                ],
            },
            {
                "title": "Pull and Sprint",
                "focus": "back, arms, and work capacity",
                "query": "pull workout back biceps cardio athletic",
                "emphasis": ["lats", "middle back", "biceps"],
                "selectors": [
                    "compound",
                    "accessory",
                    "conditioning",
                    "core",
                    "fallback",
                ],
            },
            {
                "title": "Athletic Engine",
                "focus": "full-body conditioning",
                "query": "athletic engine cardio plyometrics circuit full body",
                "emphasis": focus or ["abdominals", "quadriceps", "shoulders"],
                "selectors": [
                    "conditioning",
                    "compound",
                    "conditioning",
                    "core",
                    "mobility",
                ],
            },
        ]
    else:
        base = [
            {
                "title": "Push Session",
                "focus": "chest, shoulders, and triceps",
                "query": "push workout bench press shoulder press chest triceps",
                "emphasis": ["chest", "shoulders", "triceps"],
                "selectors": ["compound", "compound", "accessory", "accessory", "core"],
            },
            {
                "title": "Lower Body",
                "focus": "squat, hinge, and leg strength",
                "query": "lower body squat hinge legs glutes hamstrings",
                "emphasis": ["quadriceps", "hamstrings", "glutes", "calves"],
                "selectors": ["compound", "compound", "accessory", "accessory", "core"],
            },
            {
                "title": "Pull Session",
                "focus": "back, rear delts, and arms",
                "query": "pull workout rows lats back biceps",
                "emphasis": ["lats", "middle back", "biceps", "forearms"],
                "selectors": ["compound", "compound", "accessory", "accessory", "core"],
            },
            {
                "title": "Full-Body Finish",
                "focus": "trunk, power, and polish",
                "query": "full body strength athletic trunk power",
                "emphasis": focus or ["abdominals", "glutes", "shoulders"],
                "selectors": [
                    "compound",
                    "accessory",
                    "conditioning",
                    "core",
                    "mobility",
                ],
            },
        ]

    if request.goal == "get_stronger":
        base[0]["query"] += " heavy strength compound"
        base[1]["query"] += " heavy strength compound"
        base[2]["query"] += " heavy strength compound"
        for item in base:
            item["focus"] = f"{item['focus']} with longer rest and cleaner output"

    if request.goal == "athletic_engine":
        base = [
            {
                "title": "Explosive Lower",
                "focus": "jumps, legs, and athletic intent",
                "query": "explosive lower body jumps power plyometrics sprint",
                "emphasis": ["quadriceps", "hamstrings", "glutes", "calves"],
                "selectors": [
                    "conditioning",
                    "compound",
                    "conditioning",
                    "core",
                    "fallback",
                ],
            },
            {
                "title": "Upper Force",
                "focus": "pressing power and shoulder speed",
                "query": "upper body power press shoulders athletic",
                "emphasis": ["shoulders", "chest", "triceps"],
                "selectors": [
                    "compound",
                    "compound",
                    "conditioning",
                    "accessory",
                    "core",
                ],
            },
            {
                "title": "Pull Drive",
                "focus": "posterior chain and grip",
                "query": "posterior chain pull grip strongman back",
                "emphasis": ["lats", "middle back", "lower back", "forearms"],
                "selectors": [
                    "compound",
                    "compound",
                    "accessory",
                    "conditioning",
                    "fallback",
                ],
            },
            {
                "title": "Conditioning Wave",
                "focus": "engine work and trunk resilience",
                "query": "conditioning circuit engine core cardio athletic",
                "emphasis": focus or ["abdominals", "quadriceps", "shoulders"],
                "selectors": [
                    "conditioning",
                    "conditioning",
                    "compound",
                    "core",
                    "mobility",
                ],
            },
        ]

    if request.days_per_week == 2:
        return [base[0], base[1]]
    if request.days_per_week == 3:
        return [base[0], base[1], base[2]]
    if request.days_per_week == 4:
        return base[:4]

    extras = [
        {
            "title": "Accessory Reload",
            "focus": "lagging areas and controlled volume",
            "query": "accessory hypertrophy isolation lagging muscles",
            "emphasis": focus or ["shoulders", "biceps", "triceps"],
            "selectors": ["accessory", "accessory", "core", "mobility", "fallback"],
        },
        {
            "title": "Recovery Engine",
            "focus": "tempo cardio and soft tissue",
            "query": "tempo cardio recovery mobility",
            "emphasis": focus or ["abdominals", "hamstrings"],
            "selectors": ["conditioning", "mobility", "core", "fallback"],
        },
    ]
    if request.days_per_week == 5:
        return base[:4] + [extras[0]]
    return base[:4] + extras


def candidate_score(
    item: dict[str, Any],
    hit_score: float,
    blueprint: dict[str, Any],
    request: ProgramRequest,
    program_query_terms: set[str],
    program_query_signals: dict[str, set[str] | bool],
    *,
    emphasis: set[str] | None = None,
    priority: set[str] | None = None,
    request_focus: set[str] | None = None,
) -> float:
    w = SCORING_WEIGHTS
    score = hit_score * w["program_base_multiplier"]
    request_focus = set(request.focus) if request_focus is None else request_focus
    emphasis = (set(blueprint["emphasis"]) | request_focus) if emphasis is None else emphasis
    priority = set(blueprint.get("priority", [])) if priority is None else priority
    primary_muscles = item["_primary_muscles_set"]
    secondary_muscles = item["_secondary_muscles_set"]

    primary_hits = emphasis.intersection(primary_muscles)
    secondary_hits = emphasis.intersection(secondary_muscles)
    score += len(primary_hits) * w["program_emphasis_primary"]
    score += len(secondary_hits) * w["program_emphasis_secondary"]

    priority_primary_hits = priority.intersection(primary_muscles)
    priority_secondary_hits = priority.intersection(secondary_muscles)
    score += len(priority_primary_hits) * w["program_priority_primary"]
    score += len(priority_secondary_hits) * w["program_priority_secondary"]
    if priority and primary_hits and not priority_primary_hits:
        score += w["program_priority_miss_penalty"]

    if request_focus:
        focus_primary_hits = request_focus.intersection(primary_muscles)
        focus_secondary_hits = request_focus.intersection(secondary_muscles)
        if focus_primary_hits:
            score += len(focus_primary_hits) * w["program_focus_primary"]
        elif focus_secondary_hits:
            score += len(focus_secondary_hits) * w["program_focus_secondary"]
            score += w["program_focus_secondary_penalty"]
        elif primary_hits:
            score += w["program_focus_partial_penalty"]
        else:
            score += w["program_focus_miss_penalty"]

    if request.goal in {"build_muscle", "get_stronger"} and item["_is_program_strength_category"]:
        score += w["program_goal_strength_bonus"]
    elif request.goal in {"build_muscle", "get_stronger"} and item["_is_stretching"]:
        score += w["program_goal_stretching_penalty"]
    elif request.goal in {"build_muscle", "get_stronger"} and item["_is_cardio_plyo"]:
        score += w["program_goal_cardio_penalty"]
    if request.goal in {"lose_fat", "athletic_engine"} and item["_is_conditioning_category"]:
        score += w["program_goal_conditioning_bonus"]
    if request.goal == "move_better" and item["_is_stretching"]:
        score += w["program_move_better_stretch_bonus"]

    if item["_is_compound"]:
        score += w["program_compound_bonus"]
    score += len(program_query_terms.intersection(item["_descriptor_tokens"])) * w[
        "program_query_overlap"
    ]
    if program_query_signals["equipment"]:
        if item["equipment"] in program_query_signals["equipment"]:
            score += w["program_query_equipment_bonus"]
        elif item["equipment"] not in {"body only", "unspecified"}:
            score += w["program_query_equipment_penalty"]
    if program_query_signals["categories"] and item["category"] in program_query_signals["categories"]:
        score += w["program_query_category_bonus"]
    if program_query_signals["forces"] and item["force"] in program_query_signals["forces"]:
        score += w["program_query_force_bonus"]
    if item["level"] == request.level:
        score += w["program_level_match"]

    preferred_equipment = PREFERRED_PROFILE_EQUIPMENT.get(request.equipment_profile)
    if preferred_equipment:
        if item["equipment"] in preferred_equipment:
            score += w["program_preferred_equipment"]
        elif item["equipment"] != "body only":
            score += w["program_non_preferred_penalty"]

    return score


def matches_selector(item: dict[str, Any], selector: str, emphasis: set[str]) -> bool:
    if selector == "compound":
        return item["_is_compound"] and not item["_is_stretching"]
    if selector == "conditioning":
        return item["_is_conditioning_category"]
    if selector == "accessory":
        return item["_is_isolation"] or not emphasis.isdisjoint(item["_secondary_muscles_set"])
    if selector == "core":
        return item["_has_core_primary"]
    if selector == "mobility":
        return item["_is_mobility_item"]
    return True


def prescription_for(item: dict[str, Any], goal: GoalType, selector: str) -> str:
    if selector == "mobility" or item["category"] == "stretching":
        return "2 rounds of 45 sec per side"
    if selector == "conditioning" or item["category"] in {"cardio", "plyometrics"}:
        return "4 rounds of 40 sec on / 20 sec off"
    if goal == "get_stronger" and item["mechanic"] == "compound":
        return "5 sets x 4-6 reps"
    if goal == "build_muscle" and item["mechanic"] == "compound":
        return "4 sets x 6-10 reps"
    if goal == "build_muscle":
        return "3 sets x 10-15 reps"
    if goal == "lose_fat":
        return "3 rounds x 10-14 reps"
    if goal == "athletic_engine":
        return "4 sets x 6-8 crisp reps"
    return "3 sets x 8-12 reps"


def reason_for(item: dict[str, Any], blueprint: dict[str, Any]) -> str:
    primary = ", ".join(muscle.title() for muscle in item["primary_muscles"][:2])
    if item["category"] == "stretching":
        return f"Offsets stiffness while keeping the {blueprint['focus']} session recoverable."
    return f"Targets {primary or item['category']} and fits the {blueprint['focus']} emphasis."


def fallback_category_allowed(item: dict[str, Any], request: ProgramRequest) -> bool:
    if request.goal in {"build_muscle", "get_stronger"}:
        return item["category"] not in {"stretching"}
    if request.goal == "move_better":
        return item["category"] not in {"strongman"}
    return True


def update_primary_coverage_state(
    item: dict[str, Any],
    coverage_targets: set[str],
    represented: set[str],
    primary_counts: Counter,
) -> None:
    overlap = coverage_targets.intersection(item["_primary_muscles_set"])
    represented.update(overlap)
    for muscle in overlap:
        primary_counts[muscle] += 1



def can_use_program_item(
    item: dict[str, Any],
    *,
    emphasis: set[str],
    priority: set[str],
    request_focus: set[str],
    request: ProgramRequest,
    selected_ids: set[str],
    coverage_targets: set[str],
    represented: set[str],
    primary_counts: Counter,
    selector: str,
    favor_missing_primary: bool,
    require_focus_primary: bool,
) -> bool:
    if item["exercise_id"] in selected_ids:
        return False
    if not matches_selector(item, selector, emphasis):
        return False
    if selector == "fallback" and not fallback_category_allowed(item, request):
        return False

    primary_muscles = item["_primary_muscles_set"]
    all_muscles = item["_all_muscles_set"]
    target_muscles = request_focus or emphasis
    if target_muscles:
        if selector == "mobility":
            if not item["_is_stretching"] or not all_muscles.intersection(
                target_muscles
            ):
                return False
        elif selector == "core":
            if not (
                all_muscles.intersection(target_muscles)
                or item["_has_core_primary"]
            ):
                return False
        else:
            if not all_muscles.intersection(target_muscles):
                return False
            if request_focus and not primary_muscles.intersection(emphasis):
                return False
            focus_targets = priority or request_focus
            if (
                require_focus_primary
                and focus_targets
                and not primary_muscles.intersection(focus_targets)
            ):
                return False

    item_overlap = coverage_targets.intersection(primary_muscles)
    missing = coverage_targets - represented
    if (
        favor_missing_primary
        and missing
        and item_overlap
        and not item_overlap.intersection(missing)
    ):
        return False
    if item_overlap and any(primary_counts[muscle] >= 2 for muscle in item_overlap):
        return False
    return True


def pick_program_exercises(
    request: ProgramRequest,
    blueprint: dict[str, Any],
    used_ids: set[str],
) -> list[ProgramExercise]:
    pool_data = program_pool_for_request(request)
    pool = pool_data["items"]
    pool_index_set = pool_data["index_set"]
    request_focus = set(request.focus)
    emphasis = set(blueprint["emphasis"]) | request_focus
    priority = set(blueprint.get("priority", []))
    program_query = f"{blueprint['query']} {request.notes}".strip()
    program_query_terms = set(expanded_terms(tokenize(program_query)))
    program_query_signals = analyze_query(program_query)
    candidates = vector_search(
        program_query,
        top_k=min(max(len(pool), 32), 64),
        muscles=emphasis if request_focus else None,
        extra_context=list(emphasis),
    )

    ranked_all: list[tuple[float, dict[str, Any]]] = []
    for index, score in candidates:
        if index not in pool_index_set:
            continue
        item = catalog[index]
        normalized_score = max(0.0, min(1.0, (score - 0.2) / 0.8))
        ranked_all.append(
            (
                candidate_score(
                    item,
                    normalized_score,
                    blueprint,
                    request,
                    program_query_terms,
                    program_query_signals,
                    emphasis=emphasis,
                    priority=priority,
                    request_focus=request_focus,
                ),
                item,
            )
        )

    ranked_all.sort(key=lambda pair: pair[0], reverse=True)
    ranked = [pair for pair in ranked_all if pair[1]["exercise_id"] not in used_ids]
    coverage_targets = priority or request_focus or emphasis
    target_count = max(4, min(6, round(request.session_minutes / 12)))
    selected: list[tuple[str, dict[str, Any]]] = []
    selected_ids: set[str] = set()
    represented_primary: set[str] = set()
    selected_primary_counts: Counter = Counter()

    for selector in blueprint["selectors"]:
        if len(selected) >= target_count:
            break
        focus_passes = (
            (True, False)
            if request_focus and selector not in {"mobility", "core"}
            else (False,)
        )
        found = False
        for require_focus_primary in focus_passes:
            for favor_missing_primary in (True, False):
                candidate_groups = [ranked]
                if require_focus_primary and priority:
                    candidate_groups.append(ranked_all)
                for candidate_group in candidate_groups:
                    for _, item in candidate_group:
                        if item["exercise_id"] in used_ids:
                            continue
                        if can_use_program_item(
                            item,
                            emphasis=emphasis,
                            priority=priority,
                            request_focus=request_focus,
                            request=request,
                            selected_ids=selected_ids,
                            coverage_targets=coverage_targets,
                            represented=represented_primary,
                            primary_counts=selected_primary_counts,
                            selector=selector,
                            favor_missing_primary=favor_missing_primary,
                            require_focus_primary=require_focus_primary,
                        ):
                            selected.append((selector, item))
                            selected_ids.add(item["exercise_id"])
                            used_ids.add(item["exercise_id"])
                            update_primary_coverage_state(
                                item,
                                coverage_targets,
                                represented_primary,
                                selected_primary_counts,
                            )
                            found = True
                            break
                    if found:
                        break
                if found:
                    break
            if found:
                break

    for _, item in ranked:
        if len(selected) >= target_count:
            break
        if item["exercise_id"] in used_ids or item["exercise_id"] in selected_ids:
            continue
        accepted = False
        focus_passes = (True, False) if request_focus else (False,)
        for require_focus_primary in focus_passes:
            if not can_use_program_item(
                item,
                emphasis=emphasis,
                priority=priority,
                request_focus=request_focus,
                request=request,
                selected_ids=selected_ids,
                coverage_targets=coverage_targets,
                represented=represented_primary,
                primary_counts=selected_primary_counts,
                selector="fallback",
                favor_missing_primary=False,
                require_focus_primary=require_focus_primary,
            ):
                continue
            selected.append(("fallback", item))
            selected_ids.add(item["exercise_id"])
            used_ids.add(item["exercise_id"])
            update_primary_coverage_state(
                item,
                coverage_targets,
                represented_primary,
                selected_primary_counts,
            )
            accepted = True
            break
        if accepted:
            continue

    if len(selected) < target_count and request_focus:
        for _, item in ranked_all:
            if len(selected) >= target_count:
                break
            if item["exercise_id"] in used_ids or item["exercise_id"] in selected_ids:
                continue
            focus_passes = (True, False)
            for require_focus_primary in focus_passes:
                if not can_use_program_item(
                    item,
                    emphasis=emphasis,
                    priority=priority,
                    request_focus=request_focus,
                    request=request,
                    selected_ids=selected_ids,
                    coverage_targets=coverage_targets,
                    represented=represented_primary,
                    primary_counts=selected_primary_counts,
                    selector="fallback",
                    favor_missing_primary=False,
                    require_focus_primary=require_focus_primary,
                ):
                    continue
                selected.append(("fallback", item))
                selected_ids.add(item["exercise_id"])
                used_ids.add(item["exercise_id"])
                update_primary_coverage_state(
                    item,
                    coverage_targets,
                    represented_primary,
                    selected_primary_counts,
                )
                break

    return [
        ProgramExercise.model_construct(
            exercise_id=item["exercise_id"],
            name=item["name"],
            image_url=item["image_url"],
            alt_image_url=item["alt_image_url"],
            category=item["category"],
            mechanic=item["mechanic"],
            equipment=item["equipment"],
            primary_muscles=item["primary_muscles"],
            secondary_muscles=item["secondary_muscles"],
            prescription=prescription_for(item, request.goal, selector),
            reason=reason_for(item, blueprint),
            instructions=item["instructions"],
        )
        for selector, item in selected
    ]


def build_program(request: ProgramRequest) -> ProgramResponse:
    if not catalog:
        ensure_catalog()
    blueprints = program_blueprints(request)
    used_ids: set[str] = set()
    days = []

    for index, blueprint in enumerate(blueprints, start=1):
        days.append(
            ProgramDay.model_construct(
                day=index,
                title=blueprint["title"],
                focus=blueprint["focus"],
                duration_label=f"{request.session_minutes} min target",
                exercises=pick_program_exercises(request, blueprint, used_ids),
            )
        )

    focus_line = (
        ", ".join(muscle.title() for muscle in request.focus[:3])
        or "balanced development"
    )
    summary = (
        f"{request.days_per_week}-day {GOAL_LABELS[request.goal].lower()} split for "
        f"{request.level} training with the {EQUIPMENT_PROFILE_LABELS[request.equipment_profile].lower()} setup, "
        f"biased toward {focus_line}."
    )

    recovery_note = {
        "build_muscle": "Keep one rest day between the heaviest sessions and stop 1-2 reps before failure on the early sets.",
        "get_stronger": "Take longer rests on the first two lifts of each day and keep the last set technically clean.",
        "lose_fat": "Move briskly between exercises, but protect form before you chase heart rate.",
        "move_better": "Treat every rep as practice. The goal is cleaner positions, not fatigue.",
        "athletic_engine": "Stay explosive early in the session and trim a round if power output drops.",
    }[request.goal]

    return ProgramResponse.model_construct(
        summary=summary,
        recovery_note=recovery_note,
        warmup=[
            "3-5 minutes of easy cyclical work or marching in place",
            "One mobility drill for the hips or thoracic spine",
            "One activation set for the first primary muscle group of the day",
        ],
        days=days,
    )


@asynccontextmanager
async def lifespan(_: FastAPI):
    trigger_catalog_initialization()
    yield


app = FastAPI(title="Workout Program and Exercise Recommender", lifespan=lifespan)
app.mount("/static", StaticFiles(directory=STATIC_DIR), name="static")
app.mount(
    "/exercise-images",
    StaticFiles(directory=EXERCISE_IMAGES_DIR),
    name="exercise-images",
)


@app.get("/", include_in_schema=False)
async def index():
    return FileResponse(STATIC_DIR / "index.html")


@app.get("/health")
async def health():
    trigger_catalog_initialization()
    state, error = get_catalog_state()
    return {
        "status": "healthy" if state == "ready" else state,
        "catalog_status": state,
        "ready": state == "ready",
        "indexed": bool(catalog) and state == "ready",
        "exercises_loaded": len(catalog),
        "embedding_model": EMBEDDING_MODEL_NAME,
        "error": error,
    }


@app.get("/catalog/meta", response_model=MetaResponse)
async def catalog_meta_endpoint():
    ensure_catalog()
    return MetaResponse(**catalog_meta)


@app.get("/catalog/exercises", response_model=CatalogExercisesResponse)
async def list_catalog_exercises(limit: int = 1000, offset: int = 0):
    ensure_catalog()
    page = catalog[offset : offset + limit]
    return CatalogExercisesResponse(
        total=len(catalog),
        exercises=[
            CatalogExercise(
                exercise_id=item["exercise_id"],
                name=item["name"],
                force=item.get("force", ""),
                level=item.get("level", ""),
                mechanic=item.get("mechanic", ""),
                equipment=item.get("equipment", ""),
                category=item.get("category", ""),
                primary_muscles=item.get("primary_muscles", []),
                secondary_muscles=item.get("secondary_muscles", []),
                instructions=item.get("instructions", []),
                image_url=item.get("image_url"),
                alt_image_url=item.get("alt_image_url"),
            )
            for item in page
        ],
    )


@app.post("/init", response_model=InitResponse)
async def initialize_database():
    try:
        set_catalog_state("starting")
        total_exercises = initialize_catalog(force_rebuild=True)
        return InitResponse(status="initialized", exercises_loaded=total_exercises)
    except HTTPException:
        raise
    except Exception as exc:
        set_catalog_state("error", str(exc))
        raise HTTPException(status_code=500, detail=str(exc)) from exc


@app.post("/search", response_model=SearchResponse)
async def search_exercises(request: SearchRequest):
    ensure_catalog()
    try:
        if request.level and request.level not in LEVEL_RANK:
            raise HTTPException(status_code=400, detail="Unsupported level filter")

        query_signals = analyze_query(request.query)
        target_muscles = target_muscles_for_search(request, query_signals)
        excluded_muscles = set(query_signals["excluded_muscles"])
        raw_results = vector_search(
            request.query,
            top_k=max(request.top_k * 2, 28),
            level=request.level,
            equipment=request.equipment,
            category=request.category,
            muscles=target_muscles or None,
            extra_context=search_context_terms(request, query_signals),
        )

        ranked_results: list[tuple[float, float, dict[str, Any], list[str]]] = []
        for index, score in raw_results:
            item = catalog[index]
            if not matches_search_intent(
                item,
                request,
                query_signals,
                target_muscles=target_muscles,
                excluded_muscles=excluded_muscles,
            ):
                continue
            match_strength = max(0.0, min(1.0, (score - 0.2) / 0.8))
            reranked_score = search_result_score(
                item,
                match_strength,
                request,
                query_signals,
                target_muscles=target_muscles,
            )
            ranked_results.append(
                (
                    reranked_score,
                    match_strength,
                    item,
                    build_match_reasons(
                        item,
                        muscle=request.muscle,
                        level=request.level,
                        equipment=request.equipment,
                        category=request.category,
                    ),
                )
            )

        ranked_results.sort(key=lambda pair: pair[0], reverse=True)
        top_results = ranked_results[: request.top_k]
        return SearchResponse.model_construct(
            results=[
                serialize_exercise(item, match_strength, match_reasons)
                for reranked_score, match_strength, item, match_reasons in top_results
            ]
        )
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc


@app.post("/program", response_model=ProgramResponse)
async def recommend_program(request: ProgramRequest):
    ensure_catalog()
    if request.level not in LEVEL_RANK:
        raise HTTPException(status_code=400, detail="Unsupported experience level")
    if any(muscle not in catalog_meta["muscles"] for muscle in request.focus):
        raise HTTPException(status_code=400, detail="Unknown focus muscle requested")
    try:
        return build_program(request)
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc
