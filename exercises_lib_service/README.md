# Form Atlas

Local workout-program and exercise recommender built on:

- FastAPI
- FastEmbed (BAAI/bge-small-en-v1.5) for semantic embeddings
- Vendored data from [`yuhonas/free-exercise-db`](https://github.com/yuhonas/free-exercise-db)
- Local exercise images served from this repo
- Static frontend for weekly split generation and exercise discovery

The app uses **semantic embeddings** via FastEmbed's BGE model (384-dimensional vectors) to understand exercise descriptions and user queries in natural language. No external embedding API calls required - everything runs locally.

## What It Does

- embeds 873 local exercises using BERT-based semantic embeddings (BAAI/bge-small-en-v1.5)
- recommends exercises from natural-language prompts with semantic understanding
- generates a weekly workout split based on goal, experience, equipment, and focus muscles
- serves local exercise demo images directly from the app
- in-memory cosine similarity search (no external vector database required)

## Data Source

This project vendors the public-domain exercise dataset and images from:

- `yuhonas/free-exercise-db`

The upstream license copy is included at:

- [`exercise_data/FREE_EXERCISE_DB_LICENSE.md`](/home/mouad/Work/tries/2026-03-14-recommendation-system-milvus-opencode/exercise_data/FREE_EXERCISE_DB_LICENSE.md)

## Run Locally With Docker

```bash
docker compose up --build
```

The compose setup now keeps the large exercise dataset and embedding caches outside the image:

- `./exercise_data` is bind-mounted read-only into the container
- `./.catalog_cache` is bind-mounted so precomputed catalog embeddings survive rebuilds
- `fastembed_cache` is a named volume so the model download happens once and is reused

To keep embedding from consuming every CPU core, you can cap the container and FastEmbed threads:

```bash
APP_CPUS=2 EMBEDDING_THREADS=2 EMBEDDING_PARALLEL=1 docker compose up --build
```

Open:

```text
http://127.0.0.1:8000/
```

Stop:

```bash
docker compose down
```

## API Endpoints

- `GET /health`
- `GET /catalog/meta`
- `POST /init`
- `POST /search`
- `POST /program`

## Example Requests

Rebuild the local index:

```bash
curl -X POST http://127.0.0.1:8000/init
```

Search the exercise atlas:

```bash
curl -X POST http://127.0.0.1:8000/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "dumbbell push workout for chest and shoulders",
    "top_k": 6,
    "level": "beginner"
  }'
```

Generate a weekly split:

```bash
curl -X POST http://127.0.0.1:8000/program \
  -H "Content-Type: application/json" \
  -d '{
    "goal": "build_muscle",
    "days_per_week": 4,
    "session_minutes": 45,
    "level": "beginner",
    "equipment_profile": "home-bodyweight",
    "focus": ["shoulders", "abdominals"],
    "notes": "keep it joint-friendly"
  }'
```

## Verification

Suggested smoke test flow:

1. `docker compose up --build`
2. `curl http://127.0.0.1:8000/health`
3. `curl http://127.0.0.1:8000/catalog/meta`
4. `curl -X POST http://127.0.0.1:8000/search ...`
5. `curl -X POST http://127.0.0.1:8000/program ...`

## Notes

- **Image size and build context**: `exercise_data/` and `.catalog_cache/` are no longer copied into the image, which keeps rebuilds much faster and the image substantially smaller
- **First semantic request**: The embedding model (~33MB) downloads into the `fastembed_cache` Docker volume the first time the app needs query embeddings
- **Catalog cache**: Precomputed exercise embeddings stay in `.catalog_cache/`, so container rebuilds do not force a full catalog re-embed
- **Semantic embeddings**: Uses BAAI/bge-small-en-v1.5 via FastEmbed (ONNX Runtime - no PyTorch required)
- **CPU tuning**: Docker defaults now cap the service to `2` CPUs and set conservative embedding thread counts; raise `APP_CPUS`, `EMBEDDING_THREADS`, or `EMBEDDING_PARALLEL` if you want more throughput
- **In-memory search**: 873 exercises stored in memory with cosine similarity (no external database)
- **Fast restarts**: After the first model download, later restarts and rebuilds reuse the same cache volume
- **No API keys**: All embeddings computed locally using ONNX Runtime
