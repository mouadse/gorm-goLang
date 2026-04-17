import streamlit as st
import requests
import json
import os
import time
import re
import urllib3
from urllib.parse import urlparse

API_KEY = os.getenv("OPENROUTER_API_KEY")
EXERCISE_LIB_URL = os.getenv("EXERCISE_LIB_URL", "http://localhost:8000")
MODEL = os.getenv("LLM_MODEL", "google/gemini-2.0-flash-001")
BASE_URL = os.getenv("BASE_URL", "https://localhost:8080/v1")
VERIFY_TLS = os.getenv("VERIFY_TLS", "0").strip().lower() not in {"0", "false", "no", "off"}
DEFAULT_EMAIL = "alex@example.com"
DEFAULT_PASSWORD = "password123"

if not VERIFY_TLS:
    urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

_BASE_ORIGIN = urlparse(BASE_URL)


def is_backend_url(url):
    parsed = urlparse(url)
    return parsed.scheme == _BASE_ORIGIN.scheme and parsed.netloc == _BASE_ORIGIN.netloc


def request_url(method, url, **kwargs):
    if not VERIFY_TLS and is_backend_url(url):
        kwargs.setdefault("verify", False)
    return requests.request(method, url, **kwargs)


def backend_request(method, endpoint, **kwargs):
    return request_url(method, f"{BASE_URL}{endpoint}", **kwargs)

OPENROUTER_HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "HTTP-Referer": "http://localhost:8501",
    "X-Title": "Fitness Tracker AI Coach",
}

st.set_page_config(
    page_title="Fitness Tracker AI Coach", page_icon="🏋️‍♂️", layout="centered"
)

if "messages" not in st.session_state:
    st.session_state.messages = []
if "token" not in st.session_state:
    st.session_state.token = None
if "user_id" not in st.session_state:
    st.session_state.user_id = None
if "exercise_image_map" not in st.session_state:
    st.session_state.exercise_image_map = {}
if "pending_exercise_images" not in st.session_state:
    st.session_state.pending_exercise_images = None


def get_api_headers():
    return {
        "Authorization": f"Bearer {st.session_state.token}",
        "Content-Type": "application/json",
    }


def _detect_image_base_url():
    try:
        r = backend_request("HEAD", "/exercise-images/test", timeout=2)
        if r.status_code != 404:
            return BASE_URL
    except Exception:
        pass
    return EXERCISE_LIB_URL


if "image_base_url" not in st.session_state:
    st.session_state.image_base_url = _detect_image_base_url()


def resolve_image_url(url):
    if not url:
        return None
    if url.startswith("/exercise-images/"):
        return st.session_state.image_base_url + url
    return url


@st.cache_data(show_spinner=False)
def fetch_image_bytes(url):
    if not url:
        return None

    try:
        resp = request_url("GET", url, timeout=10)
        resp.raise_for_status()
        return resp.content
    except requests.RequestException:
        return None


EXERCISE_KEYWORDS = re.compile(
    r"\b(exercise|exercises|workout|workouts|movement|movements|lift|lifts|"
    r"routine|routines|program|programs|plan|plans|training|"
    r"chest|back|shoulder|shoulders|bicep|biceps|tricep|triceps|leg|legs|"
    r"glute|glutes|abs|core|forearm|forearms|calves|neck|"
    r"squat|deadlift|bench|press|row|curl|extension|flye|raise|pull|push|dip|"
    r"cardio|stretch|stretching|plyometric|"
    r"beginner|intermediate|expert|novice|advanced|"
    r"dumbbell|barbell|kettlebell|cable|bodyweight|machine)\b",
    re.IGNORECASE,
)
EXERCISE_LEVEL_ALIASES = {
    "novice": "beginner",
    "advanced": "expert",
}
MAX_TOOL_ROUNDS = 4


def is_exercise_query(text):
    return bool(EXERCISE_KEYWORDS.search(text))


def try_exercise_search(prompt):
    result = search_exercises(query=prompt, top_k=8)
    if isinstance(result, dict) and "results" in result and result["results"]:
        return result
    return None


def normalize_exercise_level(value):
    if not value:
        return None
    normalized = str(value).strip().lower()
    if not normalized:
        return None
    return EXERCISE_LEVEL_ALIASES.get(normalized, normalized)


def build_exercise_lookup_query(name=None, muscle=None, equipment=None, level=None):
    parts = []
    if name:
        parts.append(str(name).strip())
    elif muscle:
        parts.append(f"{str(muscle).strip()} exercises")
    else:
        parts.append("exercises")

    if equipment:
        parts.append(f"for {str(equipment).strip()}")
    if level:
        parts.append(f"for {str(level).strip()}")

    return " ".join(part for part in parts if part).strip()


def login(email, password):
    try:
        resp = backend_request(
            "POST",
            "/auth/login",
            json={"email": email, "password": password},
        )
        if resp.status_code == 200:
            data = resp.json()
            st.session_state.token = data.get("token") or data.get("access_token")
            user_data = data.get("user", {})
            st.session_state.user_id = user_data.get("id") or data.get("id")

            st.session_state.messages = [
                {
                    "role": "system",
                    "content": f"""You are a helpful AI assistant for a Fitness Tracker application. Use the provided tools to fetch real-time data from the database to answer the user's questions.

The current logged-in user is {email} with user ID: {st.session_state.user_id}

You can help users with:
- Finding users and their profile information
- Browsing and searching the exercise library (use search_exercises for exercise discovery because it supports semantic search with images)
- Generating personalized workout programs (use generate_program)
- Viewing workout history and details including exercises and sets
- Tracking weight progress over time
- Viewing meal logs and nutrition information
- Checking personal records (PRs) and workout statistics
- Monitoring streaks and adherence metrics
- Getting daily/weekly summaries and personalized recommendations
- Checking notifications

When answering questions:
- ALWAYS use search_exercises when users ask about exercises for specific muscles, goals, or equipment — it returns images and semantic matches
- Use generate_program when users want a workout plan or training program
- Use get_exercise_library_meta to discover available equipment profiles, levels, and options
- Use appropriate tools to fetch the requested data
- Summarize the data in a natural, easy-to-read format
- Be specific with numbers and make comparisons when relevant
- NEVER show raw JSON, tool code, function names, or user IDs to the user
- NEVER mention that you're using tools or APIs - just answer naturally
- Always respond in plain, conversational language""",
                }
            ]
            return True
        else:
            st.error(f"Login failed: {resp.text}")
            return False
    except Exception as e:
        st.error(f"Error connecting to server: {e}")
        return False


def api_request(method, endpoint, params=None):
    try:
        resp = backend_request(method, endpoint, headers=get_api_headers(), params=params)
        resp.raise_for_status()
        return resp.json()
    except requests.exceptions.RequestException as e:
        return {"error": f"API Error: {str(e)}"}


def api_post_request(endpoint, body):
    try:
        resp = backend_request("POST", endpoint, headers=get_api_headers(), json=body)
        resp.raise_for_status()
        return resp.json()
    except requests.exceptions.RequestException as e:
        return {"error": f"API Error: {str(e)}"}


def get_users(email=None, name=None):
    return api_request("GET", "/users", {"email": email, "name": name})


def get_user(user_id):
    return api_request("GET", f"/users/{user_id}")


def get_exercises(
    name=None,
    muscle_group=None,
    equipment=None,
    difficulty=None,
    level=None,
    muscle=None,
    query=None,
    top_k=None,
):
    normalized_level = normalize_exercise_level(level or difficulty)
    normalized_muscle = muscle or muscle_group
    search_query = (query or "").strip() or build_exercise_lookup_query(
        name=name,
        muscle=normalized_muscle,
        equipment=equipment,
        level=normalized_level,
    )

    if search_query:
        return search_exercises(
            query=search_query,
            top_k=top_k or 8,
            level=normalized_level,
            equipment=equipment,
            muscle=normalized_muscle,
        )

    return api_request(
        "GET",
        "/exercises",
        {
            "name": name,
            "muscle": normalized_muscle,
            "equipment": equipment,
            "level": normalized_level,
        },
    )


def get_exercise(exercise_id):
    return api_request("GET", f"/exercises/{exercise_id}")


def get_exercise_history(exercise_id, limit=10):
    return api_request("GET", f"/exercises/{exercise_id}/history", {"limit": limit})


def search_exercises(
    query=None, top_k=None, level=None, equipment=None, category=None, muscle=None
):
    body = {"query": query or ""}
    if top_k:
        body["top_k"] = int(top_k)
    if level:
        body["level"] = level
    if equipment:
        body["equipment"] = equipment
    if category:
        body["category"] = category
    if muscle:
        body["muscle"] = muscle

    result = api_post_request("/exercises/search", body)
    if isinstance(result, dict) and "results" in result:
        return result

    try:
        resp = requests.post(
            f"{EXERCISE_LIB_URL}/search",
            headers={"Content-Type": "application/json"},
            json=body,
            timeout=30,
        )
        resp.raise_for_status()
        data = resp.json()
        for r in data.get("results", []):
            for key in ("image_url", "alt_image_url"):
                url = r.get(key)
                if url and url.startswith("/exercise-images/"):
                    r[key] = BASE_URL + url
        return data
    except Exception as e:
        return result if isinstance(result, dict) else {"error": str(e)}


def generate_program(
    goal=None,
    days_per_week=None,
    session_minutes=None,
    level=None,
    equipment_profile=None,
    focus=None,
    notes=None,
):
    body = {}
    if goal:
        body["goal"] = goal
    if days_per_week:
        body["days_per_week"] = int(days_per_week)
    if session_minutes:
        body["session_minutes"] = int(session_minutes)
    if level:
        body["level"] = level
    if equipment_profile:
        body["equipment_profile"] = equipment_profile
    if focus:
        body["focus"] = focus if isinstance(focus, list) else [focus]
    if notes:
        body["notes"] = notes

    result = api_post_request("/exercises/program", body)
    if isinstance(result, dict) and "days" in result:
        return result

    try:
        resp = requests.post(
            f"{EXERCISE_LIB_URL}/program",
            headers={"Content-Type": "application/json"},
            json=body,
            timeout=60,
        )
        resp.raise_for_status()
        data = resp.json()
        for day in data.get("days", []):
            for ex in day.get("exercises", []):
                for key in ("image_url", "alt_image_url"):
                    url = ex.get(key)
                    if url and url.startswith("/exercise-images/"):
                        ex[key] = BASE_URL + url
        return data
    except Exception as e:
        return result if isinstance(result, dict) else {"error": str(e)}


def get_exercise_library_meta():
    result = api_request("GET", "/exercises/library-meta")
    if isinstance(result, dict) and "levels" in result:
        return result
    try:
        resp = requests.get(f"{EXERCISE_LIB_URL}/catalog/meta", timeout=10)
        resp.raise_for_status()
        return resp.json()
    except Exception:
        return result


def get_workouts(user_id=None, date=None, workout_type=None):
    return api_request(
        "GET", "/workouts", {"user_id": user_id, "date": date, "type": workout_type}
    )


def get_user_workouts(user_id, date=None, workout_type=None):
    return api_request(
        "GET", f"/users/{user_id}/workouts", {"date": date, "type": workout_type}
    )


def get_workout(workout_id):
    return api_request("GET", f"/workouts/{workout_id}")


def get_weight_entries(user_id=None, date=None, start_date=None, end_date=None):
    return api_request(
        "GET",
        "/weight-entries",
        {
            "user_id": user_id,
            "date": date,
            "start_date": start_date,
            "end_date": end_date,
        },
    )


def get_user_weight_entries(user_id, date=None, start_date=None, end_date=None):
    return api_request(
        "GET",
        f"/users/{user_id}/weight-entries",
        {"date": date, "start_date": start_date, "end_date": end_date},
    )


def get_meals(user_id=None, date=None, meal_type=None):
    return api_request(
        "GET", "/meals", {"user_id": user_id, "date": date, "meal_type": meal_type}
    )


def get_user_meals(user_id, date=None, meal_type=None):
    return api_request(
        "GET", f"/users/{user_id}/meals", {"date": date, "meal_type": meal_type}
    )


def get_user_records(user_id):
    return api_request("GET", f"/users/{user_id}/records")


def get_user_workout_stats(user_id):
    return api_request("GET", f"/users/{user_id}/workout-stats")


def get_user_streaks(user_id, date=None):
    return api_request("GET", f"/users/{user_id}/streaks", {"date": date})


def get_activity_calendar(user_id, start=None, end=None):
    return api_request(
        "GET", f"/users/{user_id}/activity-calendar", {"start": start, "end": end}
    )


def get_daily_summary(user_id, date=None):
    return api_request("GET", f"/users/{user_id}/summary", {"date": date})


def get_weekly_summary(user_id, date=None):
    return api_request("GET", f"/users/{user_id}/weekly-summary", {"date": date})


def get_recommendations(user_id, date=None):
    return api_request("GET", f"/users/{user_id}/recommendations", {"date": date})


def get_notifications(limit=20, offset=0):
    return api_request("GET", "/notifications", {"limit": limit, "offset": offset})


def get_unread_notification_count():
    return api_request("GET", "/notifications/unread-count")


AVAILABLE_FUNCTIONS = {
    "get_users": get_users,
    "get_user": get_user,
    "get_exercise": get_exercise,
    "get_exercise_history": get_exercise_history,
    "search_exercises": search_exercises,
    "generate_program": generate_program,
    "get_exercise_library_meta": get_exercise_library_meta,
    "get_workouts": get_workouts,
    "get_user_workouts": get_user_workouts,
    "get_workout": get_workout,
    "get_weight_entries": get_weight_entries,
    "get_user_weight_entries": get_user_weight_entries,
    "get_meals": get_meals,
    "get_user_meals": get_user_meals,
    "get_user_records": get_user_records,
    "get_user_workout_stats": get_user_workout_stats,
    "get_user_streaks": get_user_streaks,
    "get_activity_calendar": get_activity_calendar,
    "get_daily_summary": get_daily_summary,
    "get_weekly_summary": get_weekly_summary,
    "get_recommendations": get_recommendations,
    "get_notifications": get_notifications,
    "get_unread_notification_count": get_unread_notification_count,
}

tools = [
    {
        "type": "function",
        "function": {
            "name": "get_users",
            "description": "List all users in the system. Filter by name or email if provided.",
            "parameters": {
                "type": "object",
                "properties": {"name": {"type": "string"}, "email": {"type": "string"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user",
            "description": "Get detailed information about a user.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {"user_id": {"type": "string"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_exercise",
            "description": "Get details for a specific exercise by ID.",
            "parameters": {
                "type": "object",
                "required": ["exercise_id"],
                "properties": {"exercise_id": {"type": "string"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_exercise_history",
            "description": "Get workout history for an exercise.",
            "parameters": {
                "type": "object",
                "required": ["exercise_id"],
                "properties": {
                    "exercise_id": {"type": "string"},
                    "limit": {"type": "integer", "default": 10},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "search_exercises",
            "description": "PRIMARY exercise search tool. Use this whenever a user asks about exercises for a muscle group, goal, or equipment. Performs semantic search and returns exercises with images, instructions, and match reasons. Always prefer this over other exercise tools.",
            "parameters": {
                "type": "object",
                "required": ["query"],
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Natural language search query",
                    },
                    "top_k": {
                        "type": "integer",
                        "description": "Number of results (default 10)",
                    },
                    "level": {
                        "type": "string",
                        "description": "Difficulty: beginner, intermediate, expert",
                    },
                    "equipment": {
                        "type": "string",
                        "description": "Equipment: barbell, dumbbell, body only, cable, etc.",
                    },
                    "category": {
                        "type": "string",
                        "description": "Category: strength, cardio, stretching, plyometrics",
                    },
                    "muscle": {
                        "type": "string",
                        "description": "Target muscle: chest, back, shoulders, biceps, triceps, legs, etc.",
                    },
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "generate_program",
            "description": "Generate a personalized multi-day workout program with exercises, sets, reps, and rest periods.",
            "parameters": {
                "type": "object",
                "properties": {
                    "goal": {
                        "type": "string",
                        "description": "Training goal: general_fitness, muscle_gain, fat_loss, strength, endurance, flexibility",
                    },
                    "days_per_week": {
                        "type": "integer",
                        "description": "Training days per week (1-7)",
                    },
                    "session_minutes": {
                        "type": "integer",
                        "description": "Session duration in minutes",
                    },
                    "level": {
                        "type": "string",
                        "description": "Experience level: beginner, intermediate, expert",
                    },
                    "equipment_profile": {
                        "type": "string",
                        "description": "Available equipment: full_gym, home_gym, bodyweight_only, dumbbells_only, barbell_only",
                    },
                    "focus": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Muscle groups to focus on",
                    },
                    "notes": {
                        "type": "string",
                        "description": "Additional preferences or constraints",
                    },
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_exercise_library_meta",
            "description": "Get exercise library metadata: available equipment profiles, levels, categories, equipment types, muscle groups, and sample search queries.",
            "parameters": {"type": "object", "properties": {}},
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_workouts",
            "description": "List workouts.",
            "parameters": {
                "type": "object",
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                    "workout_type": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user_workouts",
            "description": "List workouts for a user.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                    "workout_type": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_workout",
            "description": "Get detailed workout info including exercises and sets.",
            "parameters": {
                "type": "object",
                "required": ["workout_id"],
                "properties": {"workout_id": {"type": "string"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_weight_entries",
            "description": "List weight entries.",
            "parameters": {
                "type": "object",
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                    "start_date": {"type": "string"},
                    "end_date": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user_weight_entries",
            "description": "List weight entries for a user.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                    "start_date": {"type": "string"},
                    "end_date": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_meals",
            "description": "List meals.",
            "parameters": {
                "type": "object",
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                    "meal_type": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user_meals",
            "description": "List meals for a user.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                    "meal_type": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user_records",
            "description": "Get personal records for a user.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {"user_id": {"type": "string"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user_workout_stats",
            "description": "Get workout statistics for a user.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {"user_id": {"type": "string"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_user_streaks",
            "description": "Get current streaks and adherence.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_activity_calendar",
            "description": "Get a calendar of user activities.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "start": {"type": "string"},
                    "end": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_daily_summary",
            "description": "Get daily summary including nutrition and workout.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_weekly_summary",
            "description": "Get weekly summary.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_recommendations",
            "description": "Get personalized recommendations.",
            "parameters": {
                "type": "object",
                "required": ["user_id"],
                "properties": {
                    "user_id": {"type": "string"},
                    "date": {"type": "string"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_notifications",
            "description": "List notifications.",
            "parameters": {
                "type": "object",
                "properties": {
                    "limit": {"type": "integer"},
                    "offset": {"type": "integer"},
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_unread_notification_count",
            "description": "Get unread notification count.",
            "parameters": {"type": "object", "properties": {}},
        },
    },
]


def render_exercise_images(image_data):
    if image_data.get("type") == "search":
        exercises = image_data.get("exercises", [])
        if not exercises:
            return
        cols = st.columns(min(len(exercises), 4))
        for col, ex in zip(cols, exercises):
            with col:
                img_url = resolve_image_url(ex.get("image_url"))
                alt_url = resolve_image_url(ex.get("alt_image_url"))
                image_bytes = fetch_image_bytes(img_url) or fetch_image_bytes(alt_url)
                if image_bytes:
                    st.image(image_bytes, use_container_width=True)
                muscles = ex.get("primary_muscles", [])
                if isinstance(muscles, str):
                    muscles = [m.strip() for m in muscles.split(",") if m.strip()]
                muscles_str = (
                    ", ".join(muscles) if isinstance(muscles, list) else str(muscles)
                )
                st.caption(
                    f"**{ex.get('name', '')}**\n{ex.get('equipment', '')} · {ex.get('level', '')}\n{muscles_str}"
                )
    elif image_data.get("type") == "program":
        data = image_data.get("data", {})
        if data.get("summary"):
            st.info(data["summary"])
        for day in data.get("days", []):
            with st.expander(
                f"Day {day['day']}: {day['title']} ({day.get('focus', '')}) — {day.get('duration_label', '')}"
            ):
                for ex in day.get("exercises", []):
                    c1, c2 = st.columns([1, 3])
                    with c1:
                        img_url = resolve_image_url(ex.get("image_url"))
                        alt_url = resolve_image_url(ex.get("alt_image_url"))
                        image_bytes = fetch_image_bytes(img_url) or fetch_image_bytes(
                            alt_url
                        )
                        if image_bytes:
                            st.image(image_bytes, use_container_width=True)
                    with c2:
                        st.markdown(f"**{ex.get('name', '')}**")
                        st.caption(ex.get("prescription", ""))
                        muscles = ex.get("primary_muscles", [])
                        if isinstance(muscles, str):
                            muscles = [
                                m.strip() for m in muscles.split(",") if m.strip()
                            ]
                        muscles_str = (
                            ", ".join(muscles)
                            if isinstance(muscles, list)
                            else str(muscles)
                        )
                        if muscles_str:
                            st.caption(f"Muscles: {muscles_str}")
                        if ex.get("reason"):
                            st.caption(f"*Why: {ex['reason']}*")


def extract_exercise_images(func_name, result):
    if not isinstance(result, dict):
        return None
    if func_name in {"search_exercises", "get_exercises"} and "results" in result:
        return {"type": "search", "exercises": result["results"]}
    if func_name == "generate_program" and "days" in result:
        return {"type": "program", "data": result}
    if func_name == "get_exercises" and isinstance(result, list):
        exercises = [
            ex
            for ex in result
            if isinstance(ex, dict)
            and (ex.get("image_url") or ex.get("alt_image_url") or ex.get("name"))
        ]
        if exercises:
            return {"type": "search", "exercises": exercises}
    if func_name == "get_exercise" and isinstance(result, dict) and result.get("name"):
        return {"type": "search", "exercises": [result]}
    return None


st.title("🏋️‍♂️ Fitness Tracker AI Coach")

with st.sidebar:
    st.header("Authentication")
    if not st.session_state.token:
        email = st.text_input("Email", value=DEFAULT_EMAIL)
        password = st.text_input("Password", value=DEFAULT_PASSWORD, type="password")
        if st.button("Login"):
            if login(email, password):
                st.success("Logged in!")
                st.rerun()
    else:
        st.write(f"Logged in as: **{DEFAULT_EMAIL}**")
        if st.button("Logout"):
            st.session_state.token = None
            st.session_state.messages = []
            st.session_state.user_id = None
            st.session_state.exercise_image_map = {}
            st.session_state.pending_exercise_images = None
            st.rerun()

    st.divider()
    if st.button("Clear Chat"):
        st.session_state.messages = (
            st.session_state.messages[:1] if len(st.session_state.messages) > 0 else []
        )
        st.session_state.exercise_image_map = {}
        st.session_state.pending_exercise_images = None
        st.rerun()

if not st.session_state.token:
    st.warning("Please login from the sidebar to start chatting.")
    st.stop()

for i, msg in enumerate(st.session_state.messages):
    if msg["role"] == "user":
        with st.chat_message("user"):
            st.markdown(msg["content"])
    elif msg["role"] == "assistant" and msg.get("content"):
        with st.chat_message("assistant"):
            st.markdown(msg["content"])
            if i in st.session_state.exercise_image_map:
                render_exercise_images(st.session_state.exercise_image_map[i])

if prompt := st.chat_input(
    "Ask me about exercises, workout programs, your streaks, or nutrition..."
):
    with st.chat_message("user"):
        st.markdown(prompt)
    st.session_state.messages.append({"role": "user", "content": prompt})

    with st.chat_message("assistant"):
        exercise_search_result = None
        if is_exercise_query(prompt):
            with st.status("Searching exercise library..."):
                exercise_search_result = try_exercise_search(prompt)

            if exercise_search_result:
                exercise_img = extract_exercise_images(
                    "search_exercises", exercise_search_result
                )
                if exercise_img:
                    st.session_state.pending_exercise_images = exercise_img

                context_msg = {
                    "role": "system",
                    "content": f"The user asked about exercises. Here are search results from the exercise library (use these to answer, do NOT call search_exercises again):\n\n{json.dumps(exercise_search_result, indent=2)}",
                }
                st.session_state.messages.append(context_msg)

        def process_llm_request(tool_round=0, seen_tool_calls=None):
            if seen_tool_calls is None:
                seen_tool_calls = set()

            llm_messages = []
            for msg in st.session_state.messages:
                llm_messages.append(
                    {k: v for k, v in msg.items() if not k.startswith("_")}
                )

            payload = {
                "model": MODEL,
                "messages": llm_messages,
                "tools": tools,
                "tool_choice": "auto",
            }

            with st.spinner("Analyzing..."):
                response = requests.post(
                    "https://openrouter.ai/api/v1/chat/completions",
                    headers=OPENROUTER_HEADERS,
                    json=payload,
                )

            if response.status_code != 200:
                st.error(f"LLM Error: {response.text}")
                return None

            resp_data = response.json()
            if "choices" not in resp_data:
                st.error("No choices returned from LLM")
                return None

            message = resp_data["choices"][0]["message"]
            st.session_state.messages.append(message)

            if message.get("tool_calls"):
                if tool_round >= MAX_TOOL_ROUNDS:
                    return "I couldn't finish gathering the exercise data in time. Please try again."

                for tc in message["tool_calls"]:
                    func_name = tc["function"]["name"]
                    try:
                        args_str = tc["function"].get("arguments", "{}")
                        func_args = json.loads(args_str) if args_str else {}
                    except json.JSONDecodeError:
                        func_args = {}

                    clean_args = {
                        k: v for k, v in func_args.items() if str(k).isidentifier()
                    }
                    call_signature = (
                        func_name,
                        json.dumps(clean_args, sort_keys=True, default=str),
                    )

                    with st.status(f"Fetching data from {func_name}..."):
                        if call_signature in seen_tool_calls:
                            result = {
                                "error": "This data was already fetched. Use the existing tool result to answer the user."
                            }
                        else:
                            seen_tool_calls.add(call_signature)
                            func = AVAILABLE_FUNCTIONS.get(func_name)
                            if func:
                                try:
                                    result = func(**clean_args)
                                except TypeError as e:
                                    result = {
                                        "error": f"TypeError: {str(e)}. The LLM passed invalid arguments."
                                    }
                                except Exception as e:
                                    result = {
                                        "error": f"Error executing tool: {str(e)}"
                                    }
                            else:
                                result = {
                                    "error": "Function not found. Use search_exercises for exercise lookups."
                                }

                        exercise_img = extract_exercise_images(func_name, result)
                        if exercise_img:
                            st.session_state.pending_exercise_images = exercise_img

                        st.session_state.messages.append(
                            {
                                "role": "tool",
                                "name": func_name,
                                "content": json.dumps(result),
                                "tool_call_id": tc["id"],
                            }
                        )

                return process_llm_request(tool_round + 1, seen_tool_calls)

            content = message.get("content")
            if not content:
                content = message.get(
                    "reasoning", "I'm sorry, I couldn't generate a response."
                )
            return content

        final_answer = process_llm_request()

        if final_answer:

            def stream_text():
                for chunk in final_answer.split(" "):
                    yield chunk + " "
                    time.sleep(0.01)

            st.write_stream(stream_text())

            if st.session_state.pending_exercise_images:
                msg_idx = len(st.session_state.messages) - 1
                st.session_state.exercise_image_map[msg_idx] = (
                    st.session_state.pending_exercise_images
                )
                st.session_state.pending_exercise_images = None
                render_exercise_images(st.session_state.exercise_image_map[msg_idx])
