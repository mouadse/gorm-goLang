#!/bin/bash
set -euo pipefail

# Autoresearch benchmark script for RAG query latency and quality
# This script runs the benchmark and outputs METRIC lines

cd "$(dirname "$0")"

# Load environment variables from .env if present
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Check if API is healthy
echo "Checking API health..." >&2
if ! curl -s -f "http://localhost:8080/health" > /dev/null 2>&1; then
    echo "ERROR: API is not healthy. Run 'make up' first." >&2
    exit 1
fi

echo "API is healthy" >&2

# Run the benchmark
python3 benchmark/benchmark.py