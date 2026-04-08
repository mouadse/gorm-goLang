FROM python:3.12-slim AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1 \
    VIRTUAL_ENV=/opt/venv \
    PATH="/opt/venv/bin:${PATH}"

WORKDIR /app

RUN python -m venv "${VIRTUAL_ENV}"

COPY requirements.txt .
RUN pip install --upgrade pip wheel
RUN pip install --no-compile -r requirements.txt

COPY app.py .
COPY static ./static

EXPOSE 8000

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
