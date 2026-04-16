# Book RAG MVP

A Dockerized RAG system that ingests PDFs, stores vectors in Qdrant, answers with OpenRouter, and includes a Streamlit chat UI.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- Optionally, [uv](https://github.com/astral-sh/uv) (for local testing without docker)
- An OpenRouter API Key ([Get one here](https://openrouter.ai/))

## Getting Started

1. **Setup Environment Variables:**
   ```bash
   cp .env.example .env
   # Edit .env and set OPENROUTER_API_KEY and HF_TOKEN
   ```
   Notes:
   - `OPENROUTER_MODEL` defaults to `google/gemini-3-flash-preview`
   - `QDRANT_COLLECTION` defaults to `books`
   - `HF_TOKEN` enables authenticated Hugging Face downloads (higher rate limits and faster model fetches)

2. **Add Books:**
   Drop your PDF files into the `books/` directory.

3. **Build images once (or when dependencies change):**
   ```bash
   TORCH_BACKEND=cpu make build
   make build-ui
   ```
   `make build` builds API + ingest only. `make build-ui` builds Streamlit only.
   For UI-only restarts, use `make up-ui`.

4. **Start services (no forced rebuild):**
   ```bash
   make up
   ```
   Endpoints:
   - API: `http://localhost:8080`
   - Streamlit UI: `http://localhost:8501`
   - Qdrant: `http://localhost:6333`

5. **Ingest Documents:**
   ```bash
   make ingest
   ```
   Ingestion is now idempotent:
   - If PDFs are unchanged and vectors already exist, ingestion is skipped.
   - If PDFs changed, chunking/embedding settings changed, or vector integrity is inconsistent, the collection is rebuilt automatically.
   - If `books/` is empty, existing vectors are cleared to avoid stale results.
   - To force full rebuild of vectors, run `make reingest`.

6. **Query via API (optional):**
   ```bash
   curl -X POST "http://localhost:8080/query" \
     -H "Content-Type: application/json" \
     -d '{"query": "What are the key concepts in the books?"}'
   ```

   Optional query controls:
   ```bash
   curl -X POST "http://localhost:8080/query" \
     -H "Content-Type: application/json" \
     -d '{
       "query": "What is Kamal and what does it do?",
       "mode": "hybrid",
       "exact_search": false,
       "include_sources": true,
       "similarity_top_k": 10,
       "sparse_top_k": 12
     }'
   ```
   Response shape now includes:
   - `answer`
   - `mode_used`
   - `sources` (optional, top retrieved passages with score/file/page/excerpt)

## Persistence and no-rebuild workflow

- Qdrant data is persisted in Docker volume `qdrant_data`.
- HuggingFace model cache is persisted in Docker volume `hf_cache_v2`.
- Ingest manifest/state is persisted in Docker volume `rag_state`.
- `make up` no longer forces `docker compose build`, so normal up/down cycles do not rebuild images.
- Streamlit is isolated in its own image/context (`./ui`), so UI work does not require rebuilding API/ingest.

## Retrieval/Chunking tuning

You can tune retrieval quality using environment variables:

- `HF_HUB_DISABLE_XET` (default: `1`, reduces Xet transfer errors in some environments)
- `RAG_EMBED_LOCAL_ONLY` (default: `0`; set to `1` to force using only pre-cached FastEmbed models and avoid outbound model downloads)
- `RAG_STATE_DIR` (default: `/app/state`, location of ingest manifest/state)
- `RAG_QUERY_MODE` (`hybrid`, `default`, `sparse`; `dense` is accepted as an alias for `default`)
- `RAG_SIMILARITY_TOP_K` (default: `8`)
- `RAG_SPARSE_TOP_K` (default: `8`, hybrid mode only)
- `RAG_SIMILARITY_CUTOFF` (default: `0.0`, filters weak matches before synthesis)
- `RAG_SEARCH_HNSW_EF` (default: `128`, query-time recall/latency tradeoff)
- `RAG_EXACT_SEARCH` (default: `0`, bypass HNSW for recall debugging)
- `RAG_INDEXED_ONLY` (default: `0`, useful when optimizer activity affects latency)
- `RAG_HYBRID_FUSION` (`rrf` or `relative_score`, default: `rrf`)
- `RAG_HYBRID_ALPHA` (default: `0.5`, dense weight when using weighted fusion)
- `RAG_HYBRID_RRF_K` (default: `60`, RRF damping factor)
- `RAG_QUANTIZATION_RESCORE` (default: `1`)
- `RAG_QUANTIZATION_IGNORE` (default: `0`)
- `RAG_QUANTIZATION_OVERSAMPLING` (default: `3.0`)
- `RAG_CHUNK_SIZE` (default: `512`)
- `RAG_CHUNK_OVERLAP` (default: `64`)
- `RAG_EMBED_DIMENSION` (default: `768`, override only for custom FastEmbed-compatible models)

Collection and indexing controls are also configurable:

- `QDRANT_SHARD_NUMBER` (default: `1`)
- `QDRANT_REPLICATION_FACTOR` (default: `1`)
- `QDRANT_WRITE_CONSISTENCY_FACTOR` (default: `1`)
- `QDRANT_ON_DISK_PAYLOAD` (default: `1`)
- `QDRANT_DENSE_ON_DISK` (default: `0`)
- `QDRANT_SPARSE_ON_DISK` (default: `0`)
- `QDRANT_HNSW_M` (default: `32`)
- `QDRANT_HNSW_EF_CONSTRUCT` (default: `200`)
- `QDRANT_FULL_SCAN_THRESHOLD` (default: `10000`)
- `QDRANT_MAX_INDEXING_THREADS` (default: `0`)
- `QDRANT_DEFAULT_SEGMENT_NUMBER` (default: `2`)
- `QDRANT_INDEXING_THRESHOLD` (default: `10000`)
- `QDRANT_FLUSH_INTERVAL_SEC` (default: `5`)
- `QDRANT_PREVENT_UNOPTIMIZED` (default: `0`)
- `QDRANT_ENABLE_QUANTIZATION` (default: `1`)
- `QDRANT_QUANTIZATION_ALWAYS_RAM` (default: `1`)

If you see TLS hostname mismatch errors against model hosts (for example `huggingface.co`), your network may be intercepting HTTPS traffic. Authenticate/fix network egress first, or pre-populate `hf_cache_v2` and set `RAG_EMBED_LOCAL_ONLY=1`.

When ingest, chunking, or collection settings change, the manifest fingerprint changes too. Running `make ingest` will detect the mismatch and rebuild the collection automatically.

## Architecture

- **`api` (FastAPI)**: answers queries using LlamaIndex + OpenRouter, with hybrid dense+sparse retrieval, weighted RRF fusion, cutoff filtering, and optional source traces.
- **`ingest` (job container)**: builds vectors from PDFs, enriches chunk metadata, and prepares the Qdrant collection with explicit HNSW/optimizer/quantization settings.
- **`qdrant`**: persistent vector database.
- **`ui` (Streamlit)**: chat interface for user-friendly RAG Q&A.
