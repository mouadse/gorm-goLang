import json
import os
from urllib import error, request

import streamlit as st

DEFAULT_API_URL = os.getenv("API_URL", "http://localhost:8080/query")
REQUEST_TIMEOUT_SECONDS = 90
SYSTEM_GREETING = (
    "Ask me anything about the PDFs you ingested. "
    "I will answer using your RAG index."
)


def _inject_styles():
    st.markdown(
        """
        <style>
            :root {
                --bg-main: #070b16;
                --bg-panel: #0d1428;
                --bg-soft: #121c35;
                --line: #24365f;
                --text-main: #f3f7ff;
                --text-muted: #9bb0dc;
                --accent: #56a2ff;
                --accent-2: #7bf7d4;
            }

            .stApp {
                background:
                    radial-gradient(circle at 20% -10%, #1a2e63 0%, transparent 45%),
                    radial-gradient(circle at 100% 0%, #12304d 0%, transparent 35%),
                    var(--bg-main);
                color: var(--text-main);
            }

            html, body, [data-testid="stAppViewContainer"], [data-testid="stMain"] {
                background-color: var(--bg-main) !important;
                color: var(--text-main) !important;
            }

            [data-testid="stHeader"] {
                background: transparent;
            }

            [data-testid="stSidebar"] {
                background: linear-gradient(180deg, #0c1327 0%, #0a1020 100%);
                border-right: 1px solid var(--line);
            }

            [data-testid="stSidebar"] [data-testid="stMarkdownContainer"] p,
            [data-testid="stSidebar"] label,
            [data-testid="stSidebar"] span {
                color: var(--text-muted) !important;
            }

            [data-testid="stTextInputRootElement"] input {
                background: var(--bg-soft) !important;
                border: 1px solid var(--line) !important;
                color: var(--text-main) !important;
                border-radius: 12px !important;
            }

            [data-testid="stTextInputRootElement"] input:focus {
                border-color: var(--accent) !important;
                box-shadow: 0 0 0 1px var(--accent) !important;
            }

            .stButton > button {
                background: linear-gradient(135deg, #173574 0%, #1452a8 100%);
                color: #ffffff !important;
                border: 1px solid #2f69c8;
                border-radius: 12px;
                font-weight: 600;
            }

            [data-testid="stChatInputTextArea"] textarea {
                background: var(--bg-panel) !important;
                border: 1px solid var(--line) !important;
                color: var(--text-main) !important;
                border-radius: 14px !important;
            }

            [data-testid="stChatMessage"], .stChatMessage {
                background: rgba(13, 20, 40, 0.85);
                border: 1px solid var(--line);
                border-radius: 14px;
                padding: 0.45rem 0.75rem;
                margin-bottom: 0.8rem;
            }

            [data-testid="stChatMessage"] [data-testid="stMarkdownContainer"] p,
            [data-testid="stChatMessage"] [data-testid="stMarkdownContainer"] li,
            .stChatMessage [data-testid="stMarkdownContainer"] p,
            .stChatMessage [data-testid="stMarkdownContainer"] li {
                color: var(--text-main) !important;
                line-height: 1.6;
            }

            .hero {
                background: linear-gradient(120deg, rgba(85,162,255,0.16), rgba(123,247,212,0.06));
                border: 1px solid #2a406f;
                border-radius: 16px;
                padding: 18px 18px 10px 18px;
                margin-bottom: 14px;
            }

            .hero-title {
                font-size: 1.35rem;
                font-weight: 700;
                margin-bottom: 0.35rem;
                color: #ffffff;
            }

            .hero-sub {
                color: var(--text-muted);
                font-size: 0.96rem;
            }
        </style>
        """,
        unsafe_allow_html=True,
    )


def _query_rag(api_url, prompt):
    payload = json.dumps({"query": prompt}).encode("utf-8")
    req = request.Request(
        api_url,
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with request.urlopen(req, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            body = response.read().decode("utf-8")
    except error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="ignore")
        try:
            error_json = json.loads(body)
            detail = error_json.get("detail", body)
            raise RuntimeError(f"RAG API Error: {detail}") from exc
        except json.JSONDecodeError:
            raise RuntimeError(f"RAG API returned HTTP {exc.code}: {body}") from exc
    except error.URLError as exc:
        raise RuntimeError(f"Could not connect to API at {api_url}") from exc

    try:
        parsed = json.loads(body)
    except json.JSONDecodeError as exc:
        raise RuntimeError("RAG API returned invalid JSON.") from exc

    answer = parsed.get("answer")
    if not answer:
        raise RuntimeError("RAG API response did not include an answer.")
    return answer


def _init_state():
    if "messages" not in st.session_state:
        st.session_state.messages = [{"role": "assistant", "content": SYSTEM_GREETING}]
    if "api_url" not in st.session_state:
        st.session_state.api_url = DEFAULT_API_URL


def _render_sidebar():
    with st.sidebar:
        st.subheader("Settings")
        st.session_state.api_url = st.text_input(
            "RAG API endpoint",
            value=st.session_state.api_url,
            help="For Docker Compose use http://api:8000/query inside the container.",
        )

        if st.button("Clear chat", use_container_width=True):
            st.session_state.messages = [
                {"role": "assistant", "content": SYSTEM_GREETING}
            ]
            st.rerun()

        st.caption(
            "Tips: ask focused questions, mention chapter names, and request bullet "
            "summaries when needed."
        )


def _render_chat():
    for message in st.session_state.messages:
        with st.chat_message(message["role"]):
            st.markdown(message["content"])


def _handle_prompt(prompt):
    st.session_state.messages.append({"role": "user", "content": prompt})
    with st.chat_message("user"):
        st.markdown(prompt)

    with st.chat_message("assistant"):
        with st.spinner("Searching your documents..."):
            try:
                answer = _query_rag(st.session_state.api_url, prompt)
            except RuntimeError as exc:
                answer = f"Error: {exc}"
        st.markdown(answer)

    st.session_state.messages.append({"role": "assistant", "content": answer})


def main():
    st.set_page_config(page_title="Book RAG Assistant", page_icon="book", layout="wide")
    _inject_styles()

    _init_state()
    _render_sidebar()

    st.markdown(
        """
        <div class="hero">
            <div class="hero-title">Book RAG Assistant</div>
            <div class="hero-sub">
                Clean AI chat for your private PDF knowledge base. Ask focused questions for better retrieval quality.
            </div>
        </div>
        """,
        unsafe_allow_html=True,
    )

    _render_chat()

    prompt = st.chat_input("Ask a question about your books")
    if prompt:
        _handle_prompt(prompt)


if __name__ == "__main__":
    main()
