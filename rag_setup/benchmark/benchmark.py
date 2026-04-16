#!/usr/bin/env python3
"""
Benchmark script for RAG query latency and quality evaluation.
Outputs METRIC lines for autoresearch.
"""
import json
import os
import sys
import time
import urllib.request
import urllib.error

# Configuration
API_URL = os.getenv("API_URL", "http://localhost:8080/query")
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY")
OPENROUTER_MODEL = os.getenv("JUDGE_MODEL", "google/gemini-3-flash-preview")
QUERIES_FILE = os.path.dirname(os.path.abspath(__file__)) + "/queries.json"

# Number of times to run each query for latency averaging
LATENCY_RUNS = int(os.getenv("LATENCY_RUNS", "3"))


def load_queries():
    """Load test queries from JSON file."""
    with open(QUERIES_FILE, "r") as f:
        return json.load(f)


def query_rag(query_text):
    """Send a query to the RAG API and return the answer with timing."""
    payload = json.dumps({"query": query_text}).encode("utf-8")
    headers = {"Content-Type": "application/json"}
    
    start_time = time.perf_counter()
    try:
        req = urllib.request.Request(API_URL, data=payload, headers=headers, method="POST")
        with urllib.request.urlopen(req, timeout=60) as response:
            result = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        raise RuntimeError(f"HTTP {e.code}: {body}")
    elapsed_ms = (time.perf_counter() - start_time) * 1000
    
    return result.get("answer", ""), elapsed_ms


def judge_answer_quality(query, answer, expected_topics):
    """
    Use LLM-as-judge to evaluate answer quality.
    Returns a score from 1-5.
    """
    if not OPENROUTER_API_KEY:
        # If no API key, return a neutral score
        print("Warning: OPENROUTER_API_KEY not set, skipping quality evaluation", file=sys.stderr)
        return 0
    
    judge_prompt = f"""You are an expert evaluator for a RAG (Retrieval-Augmented Generation) system.

Evaluate the following answer based on:
1. Relevance: Does the answer address the question?
2. Accuracy: Is the information correct and helpful?
3. Completeness: Does it provide sufficient detail?

Question: {query}

Expected topics that should be covered: {', '.join(expected_topics)}

Answer to evaluate: {answer}

Rate the answer quality on a scale of 1-5:
1 = Completely irrelevant or wrong
2 = Mostly irrelevant or has significant errors
3 = Somewhat relevant but incomplete or has minor errors
4 = Relevant and accurate with good detail
5 = Excellent, comprehensive, and highly accurate

Respond with ONLY a single number (1, 2, 3, 4, or 5). No explanation."""

    payload = json.dumps({
        "model": OPENROUTER_MODEL,
        "messages": [{"role": "user", "content": judge_prompt}],
        "max_tokens": 10
    }).encode("utf-8")
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {OPENROUTER_API_KEY}",
    }
    
    try:
        req = urllib.request.Request(
            "https://openrouter.ai/api/v1/chat/completions",
            data=payload,
            headers=headers,
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=30) as response:
            result = json.loads(response.read().decode("utf-8"))
            content = result.get("choices", [{}])[0].get("message", {}).get("content", "3").strip()
            # Extract just the number
            score = int(''.join(c for c in content if c.isdigit()))
            return max(1, min(5, score))
    except Exception as e:
        print(f"Judge error: {e}", file=sys.stderr)
        return 0


def run_benchmark():
    """Run the full benchmark and output metrics."""
    queries = load_queries()
    
    if not queries:
        print("ERROR: No queries loaded", file=sys.stderr)
        sys.exit(1)
    
    print(f"Running benchmark with {len(queries)} queries, {LATENCY_RUNS} runs each...", file=sys.stderr)
    
    all_latencies = []
    all_quality_scores = []
    query_results = []
    
    for q in queries:
        query_id = q["id"]
        query_text = q["query"]
        expected_topics = q.get("expected_topics", [])
        
        print(f"\nQuery: {query_id}: {query_text[:50]}...", file=sys.stderr)
        
        # Run multiple times for latency averaging
        latencies = []
        answer = ""
        for run in range(LATENCY_RUNS):
            try:
                answer, latency_ms = query_rag(query_text)
                latencies.append(latency_ms)
                print(f"  Run {run+1}: {latency_ms:.0f}ms", file=sys.stderr)
            except Exception as e:
                print(f"  Run {run+1}: ERROR - {e}", file=sys.stderr)
                continue
        
        if not latencies:
            print(f"ERROR: All runs failed for query {query_id}", file=sys.stderr)
            continue
        
        avg_latency = sum(latencies) / len(latencies)
        all_latencies.append(avg_latency)
        
        # Judge quality (only once per query)
        quality_score = judge_answer_quality(query_text, answer, expected_topics)
        if quality_score > 0:
            all_quality_scores.append(quality_score)
        
        query_results.append({
            "id": query_id,
            "avg_latency_ms": avg_latency,
            "quality_score": quality_score,
        })
        
        print(f"  Avg latency: {avg_latency:.0f}ms, Quality: {quality_score}/5", file=sys.stderr)
    
    if not all_latencies:
        print("ERROR: No successful queries", file=sys.stderr)
        sys.exit(1)
    
    # Calculate aggregate metrics
    total_latency = sum(all_latencies)
    avg_latency = total_latency / len(all_latencies)
    avg_quality = sum(all_quality_scores) / len(all_quality_scores) if all_quality_scores else 0
    
    # Output metrics for autoresearch
    print(f"\n=== RESULTS ===", file=sys.stderr)
    print(f"METRIC avg_latency_ms={avg_latency:.2f}")
    print(f"METRIC total_latency_ms={total_latency:.2f}")
    print(f"METRIC avg_quality_score={avg_quality:.2f}")
    
    # Also print summary
    print(f"\nSummary:", file=sys.stderr)
    print(f"  Queries: {len(all_latencies)}/{len(queries)}", file=sys.stderr)
    print(f"  Avg Latency: {avg_latency:.0f}ms", file=sys.stderr)
    print(f"  Avg Quality: {avg_quality:.2f}/5", file=sys.stderr)
    
    return 0


if __name__ == "__main__":
    sys.exit(run_benchmark())