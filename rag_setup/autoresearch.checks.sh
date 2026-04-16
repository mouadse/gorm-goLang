#!/bin/bash
set -euo pipefail

# Run tests to ensure changes don't break existing functionality
cd "$(dirname "$0")"
python -m pytest tests/ -v --tb=short 2>&1 | tail -20