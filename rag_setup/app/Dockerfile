# syntax=docker/dockerfile:1.7
FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    RAG_STATE_DIR=/app/state

# Set working directory
WORKDIR /app

# Default to CPU-only torch wheels unless explicitly overridden at build time
ARG TORCH_BACKEND=cpu
ARG UV_VERSION=0.7.8
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-cache-dir "uv==${UV_VERSION}"

# Copy dependency files
COPY pyproject.toml requirements.lock ./

# Install dependencies via a locked requirements file
# Force CPU torch backend to avoid pulling NVIDIA/CUDA libraries on non-GPU hosts
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --system -r requirements.lock --torch-backend ${TORCH_BACKEND}

# Copy application code
COPY . .

RUN groupadd --gid 1000 appuser \
    && useradd --uid 1000 --gid 1000 --create-home --shell /usr/sbin/nologin appuser \
    && mkdir -p /app/state \
    && chown -R appuser:appuser /app /home/appuser

USER appuser

# Expose FastAPI port
EXPOSE 8000

# Default command (used by the api service)
CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
