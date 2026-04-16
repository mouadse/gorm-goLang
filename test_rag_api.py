#!/usr/bin/env python3
import argparse
import json
import sys
import urllib.error
import urllib.parse
import urllib.request


def request_json(method, url, payload=None, token=None):
    body = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"

    request = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=120) as response:
            raw = response.read().decode("utf-8")
            return response.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        detail = raw
        try:
            detail = json.loads(raw)
        except json.JSONDecodeError:
            pass
        raise RuntimeError(f"{method} {url} failed with {exc.code}: {detail}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"{method} {url} failed: {exc.reason}") from exc


def login(base_url, email, password):
    _, payload = request_json(
        "POST",
        f"{base_url}/v1/auth/login",
        {"email": email, "password": password},
    )
    token = payload.get("access_token")
    if not token:
        raise RuntimeError("login response did not include access_token")
    return token


def main():
    parser = argparse.ArgumentParser(description="Smoke test the Go RAG API endpoints.")
    parser.add_argument("--base-url", default="http://localhost:8082", help="Base URL for the Go API.")
    parser.add_argument("--email", default="alex@example.com", help="Admin email for protected RAG endpoints.")
    parser.add_argument("--password", default="password123", help="Admin password for protected RAG endpoints.")
    parser.add_argument(
        "--query",
        default="What is Kamal and what does it do?",
        help="RAG query to send to /v1/rag/query.",
    )
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")
    print(f"Using API base URL: {base_url}")

    print("1. Checking public RAG health endpoint...")
    _, health = request_json("GET", f"{base_url}/v1/rag/health")
    if health.get("status") != "ok":
        raise RuntimeError(f"unexpected RAG health payload: {health}")
    print("   OK")

    print("2. Checking public RAG cache stats endpoint...")
    _, cache_stats = request_json("GET", f"{base_url}/v1/rag/cache/stats")
    for key in ("size", "max_size", "ttl_seconds", "hits", "misses"):
        if key not in cache_stats:
            raise RuntimeError(f"missing {key!r} in cache stats payload: {cache_stats}")
    print("   OK")

    print("3. Running RAG query through the Go API...")
    _, query_payload = request_json(
        "POST",
        f"{base_url}/v1/rag/query",
        {"query": args.query, "include_sources": True},
    )
    answer = (query_payload.get("answer") or "").strip()
    if not answer:
        raise RuntimeError(f"empty RAG answer payload: {query_payload}")
    print(f"   OK, received answer length={len(answer)}")

    print("4. Logging in as admin for protected cache-clear endpoint...")
    token = login(base_url, args.email, args.password)
    print("   OK")

    print("5. Clearing the RAG cache through the Go API...")
    _, clear_payload = request_json("POST", f"{base_url}/v1/rag/cache/clear", {}, token=token)
    if "cleared" not in clear_payload:
        raise RuntimeError(f"unexpected cache-clear payload: {clear_payload}")
    print(f"   OK, cleared={clear_payload['cleared']}")

    print("RAG API smoke test passed.")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:  # noqa: BLE001
        print(f"RAG API smoke test failed: {exc}", file=sys.stderr)
        sys.exit(1)
