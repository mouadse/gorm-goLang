import streamlit as st
import requests
import json
import os
import time

# Configuration
API_KEY = os.getenv("OPENROUTER_API_KEY")
MODEL = os.getenv("LLM_MODEL", "google/gemini-2.0-flash-001")
BASE_URL = os.getenv("BASE_URL", "http://localhost:8080/v1")
DEFAULT_EMAIL = "alex@example.com"
DEFAULT_PASSWORD = "password123"

OPENROUTER_HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "HTTP-Referer": "http://localhost:8501",
    "X-Title": "Fitness Tracker AI Coach"
}

st.set_page_config(page_title="Fitness Tracker AI Coach", page_icon="🏋️‍♂️", layout="centered")

# --- Session State Initialization ---
if "messages" not in st.session_state:
    st.session_state.messages = []
if "token" not in st.session_state:
    st.session_state.token = None
if "user_id" not in st.session_state:
    st.session_state.user_id = None

# --- Helper Functions ---
def get_api_headers():
    return {
        "Authorization": f"Bearer {st.session_state.token}",
        "Content-Type": "application/json",
    }

def login(email, password):
    try:
        resp = requests.post(f"{BASE_URL}/auth/login", json={"email": email, "password": password})
        if resp.status_code == 200:
            data = resp.json()
            st.session_state.token = data.get("token") or data.get("access_token")
            user_data = data.get("user", {})
            st.session_state.user_id = user_data.get("id") or data.get("id")
            
            # Reset messages with proper system prompt
            st.session_state.messages = [{
                "role": "system", 
                "content": f"""You are a helpful AI assistant for a Fitness Tracker application. Use the provided tools to fetch real-time data from the database to answer the user's questions.

The current logged-in user is {email} with user ID: {st.session_state.user_id}

You can help users with:
- Finding users and their profile information
- Browsing and searching the exercise library
- Viewing workout history and details including exercises and sets
- Tracking weight progress over time
- Viewing meal logs and nutrition information
- Checking personal records (PRs) and workout statistics
- Monitoring streaks and adherence metrics
- Getting daily/weekly summaries and personalized recommendations
- Checking notifications

When answering questions:
- Use appropriate tools to fetch the requested data
- Summarize the data in a natural, easy-to-read format
- Be specific with numbers and make comparisons when relevant
- NEVER show raw JSON, tool code, function names, or user IDs to the user
- NEVER mention that you're using tools or APIs - just answer naturally
- Always respond in plain, conversational language"""
            }]
            return True
        else:
            st.error(f"Login failed: {resp.text}")
            return False
    except Exception as e:
        st.error(f"Error connecting to server: {e}")
        return False

# --- Tool Implementations ---
def api_request(method, endpoint, params=None):
    try:
        url = f"{BASE_URL}{endpoint}"
        resp = requests.request(method, url, headers=get_api_headers(), params=params)
        resp.raise_for_status()
        return resp.json()
    except requests.exceptions.RequestException as e:
        return {"error": f"API Error: {str(e)}"}

# Users
def get_users(email=None, name=None): return api_request("GET", "/users", {"email": email, "name": name})
def get_user(user_id): return api_request("GET", f"/users/{user_id}")

# Exercises
def get_exercises(name=None, muscle_group=None, equipment=None, difficulty=None): return api_request("GET", "/exercises", {"name": name, "muscle_group": muscle_group, "equipment": equipment, "difficulty": difficulty})
def get_exercise(exercise_id): return api_request("GET", f"/exercises/{exercise_id}")
def get_exercise_history(exercise_id, limit=10): return api_request("GET", f"/exercises/{exercise_id}/history", {"limit": limit})

# Workouts
def get_workouts(user_id=None, date=None, workout_type=None): return api_request("GET", "/workouts", {"user_id": user_id, "date": date, "type": workout_type})
def get_user_workouts(user_id, date=None, workout_type=None): return api_request("GET", f"/users/{user_id}/workouts", {"date": date, "type": workout_type})
def get_workout(workout_id): return api_request("GET", f"/workouts/{workout_id}")

# Weight
def get_weight_entries(user_id=None, date=None, start_date=None, end_date=None): return api_request("GET", "/weight-entries", {"user_id": user_id, "date": date, "start_date": start_date, "end_date": end_date})
def get_user_weight_entries(user_id, date=None, start_date=None, end_date=None): return api_request("GET", f"/users/{user_id}/weight-entries", {"date": date, "start_date": start_date, "end_date": end_date})

# Meals
def get_meals(user_id=None, date=None, meal_type=None): return api_request("GET", "/meals", {"user_id": user_id, "date": date, "meal_type": meal_type})
def get_user_meals(user_id, date=None, meal_type=None): return api_request("GET", f"/users/{user_id}/meals", {"date": date, "meal_type": meal_type})

# Analytics
def get_user_records(user_id): return api_request("GET", f"/users/{user_id}/records")
def get_user_workout_stats(user_id): return api_request("GET", f"/users/{user_id}/workout-stats")
def get_user_streaks(user_id, date=None): return api_request("GET", f"/users/{user_id}/streaks", {"date": date})
def get_activity_calendar(user_id, start=None, end=None): return api_request("GET", f"/users/{user_id}/activity-calendar", {"start": start, "end": end})

# Summaries
def get_daily_summary(user_id, date=None): return api_request("GET", f"/users/{user_id}/summary", {"date": date})
def get_weekly_summary(user_id, date=None): return api_request("GET", f"/users/{user_id}/weekly-summary", {"date": date})
def get_recommendations(user_id, date=None): return api_request("GET", f"/users/{user_id}/recommendations", {"date": date})

# Notifications
def get_notifications(limit=20, offset=0): return api_request("GET", "/notifications", {"limit": limit, "offset": offset})
def get_unread_notification_count(): return api_request("GET", "/notifications/unread-count")

AVAILABLE_FUNCTIONS = {
    "get_users": get_users, "get_user": get_user,
    "get_exercises": get_exercises, "get_exercise": get_exercise, "get_exercise_history": get_exercise_history,
    "get_workouts": get_workouts, "get_user_workouts": get_user_workouts, "get_workout": get_workout,
    "get_weight_entries": get_weight_entries, "get_user_weight_entries": get_user_weight_entries,
    "get_meals": get_meals, "get_user_meals": get_user_meals,
    "get_user_records": get_user_records, "get_user_workout_stats": get_user_workout_stats, 
    "get_user_streaks": get_user_streaks, "get_activity_calendar": get_activity_calendar,
    "get_daily_summary": get_daily_summary, "get_weekly_summary": get_weekly_summary, "get_recommendations": get_recommendations,
    "get_notifications": get_notifications, "get_unread_notification_count": get_unread_notification_count,
}

# --- Tool Definitions ---
tools = [
    {"type": "function", "function": {"name": "get_users", "description": "List all users in the system. Filter by name or email if provided.", "parameters": {"type": "object", "properties": {"name": {"type": "string"}, "email": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user", "description": "Get detailed information about a user.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_exercises", "description": "Search exercise library.", "parameters": {"type": "object", "properties": {"name": {"type": "string"}, "muscle_group": {"type": "string"}, "equipment": {"type": "string"}, "difficulty": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_exercise", "description": "Get exercise details.", "parameters": {"type": "object", "required": ["exercise_id"], "properties": {"exercise_id": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_exercise_history", "description": "Get workout history for an exercise.", "parameters": {"type": "object", "required": ["exercise_id"], "properties": {"exercise_id": {"type": "string"}, "limit": {"type": "integer", "default": 10}}}}},
    {"type": "function", "function": {"name": "get_workouts", "description": "List workouts.", "parameters": {"type": "object", "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}, "workout_type": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user_workouts", "description": "List workouts for a user.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}, "workout_type": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_workout", "description": "Get detailed workout info including exercises and sets.", "parameters": {"type": "object", "required": ["workout_id"], "properties": {"workout_id": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_weight_entries", "description": "List weight entries.", "parameters": {"type": "object", "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}, "start_date": {"type": "string"}, "end_date": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user_weight_entries", "description": "List weight entries for a user.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}, "start_date": {"type": "string"}, "end_date": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_meals", "description": "List meals.", "parameters": {"type": "object", "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}, "meal_type": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user_meals", "description": "List meals for a user.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}, "meal_type": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user_records", "description": "Get personal records for a user.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user_workout_stats", "description": "Get workout statistics for a user.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_user_streaks", "description": "Get current streaks and adherence.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_activity_calendar", "description": "Get a calendar of user activities.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "start": {"type": "string"}, "end": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_daily_summary", "description": "Get daily summary including nutrition and workout.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_weekly_summary", "description": "Get weekly summary.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_recommendations", "description": "Get personalized recommendations.", "parameters": {"type": "object", "required": ["user_id"], "properties": {"user_id": {"type": "string"}, "date": {"type": "string"}}}}},
    {"type": "function", "function": {"name": "get_notifications", "description": "List notifications.", "parameters": {"type": "object", "properties": {"limit": {"type": "integer"}, "offset": {"type": "integer"}}}}},
    {"type": "function", "function": {"name": "get_unread_notification_count", "description": "Get unread notification count.", "parameters": {"type": "object", "properties": {}}}}
]

# --- UI Layout ---
st.title("🏋️‍♂️ Fitness Tracker AI Coach")

# --- Sidebar ---
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
            st.rerun()
            
    st.divider()
    if st.button("Clear Chat"):
        st.session_state.messages = st.session_state.messages[:1] if len(st.session_state.messages) > 0 else []
        st.rerun()

# --- Chat Interface ---
if not st.session_state.token:
    st.warning("Please login from the sidebar to start chatting.")
    st.stop()

# Display chat messages (hide system and tool messages)
for msg in st.session_state.messages:
    if msg["role"] == "user":
        with st.chat_message("user"): st.markdown(msg["content"])
    elif msg["role"] == "assistant" and msg.get("content"):
        with st.chat_message("assistant"): st.markdown(msg["content"])

if prompt := st.chat_input("Ask me about your workout streaks, records, or nutrition..."):
    # Show user message
    with st.chat_message("user"): st.markdown(prompt)
    st.session_state.messages.append({"role": "user", "content": prompt})

    with st.chat_message("assistant"):
        # Function to process LLM request (handles tool calls recursively)
        def process_llm_request():
            payload = {
                "model": MODEL,
                "messages": st.session_state.messages,
                "tools": tools,
                "tool_choice": "auto"
            }
            
            with st.spinner("Analyzing..."):
                response = requests.post(
                    "https://openrouter.ai/api/v1/chat/completions",
                    headers=OPENROUTER_HEADERS,
                    json=payload
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
            
            # Handle tool calls if any
            if message.get("tool_calls"):
                for tc in message["tool_calls"]:
                    func_name = tc["function"]["name"]
                    try:
                        args_str = tc["function"].get("arguments", "{}")
                        func_args = json.loads(args_str) if args_str else {}
                    except json.JSONDecodeError:
                        func_args = {}
                    
                    with st.status(f"Fetching data from {func_name}..."):
                        func = AVAILABLE_FUNCTIONS.get(func_name)
                        if func:
                            try:
                                # Clean up func_args to remove invalid Python identifiers like '{}'
                                clean_args = {k: v for k, v in func_args.items() if str(k).isidentifier()}
                                result = func(**clean_args)
                            except TypeError as e:
                                result = {"error": f"TypeError: {str(e)}. The LLM passed invalid arguments."}
                            except Exception as e:
                                result = {"error": f"Error executing tool: {str(e)}"}
                        else:
                            result = {"error": "Function not found"}
                            
                        st.session_state.messages.append({
                            "role": "tool",
                            "name": func_name,
                            "content": json.dumps(result),
                            "tool_call_id": tc["id"]
                        })
                
                # Make a second call after tools are done
                return process_llm_request()
            
            content = message.get("content")
            if not content:
                content = message.get("reasoning", "I'm sorry, I couldn't generate a response.")
            return content

        # Initial call
        final_answer = process_llm_request()
        
        if final_answer:
            # We already have the full answer, but we'll stream it word by word
            # to make the UI feel fast and smooth instead of blocking.
            def stream_text():
                for chunk in final_answer.split(" "):
                    yield chunk + " "
                    time.sleep(0.01)
            
            st.write_stream(stream_text())
