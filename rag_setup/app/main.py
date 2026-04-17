from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from llama_index.core import VectorStoreIndex, PromptTemplate
from llama_index.core.postprocessor import SimilarityPostprocessor
from threading import Lock
import time
import os
from shared import (
    setup_settings,
    get_qdrant_vector_store,
    get_retrieval_settings,
)

# Initialize FastAPI application
app = FastAPI(title="Book RAG MVP API")

class QueryRequest(BaseModel):
    query: str
    mode: str | None = None
    exact_search: bool | None = None
    similarity_top_k: int | None = None
    include_sources: bool = False


class QuerySource(BaseModel):
    score: float | None = None
    file_name: str | None = None
    source_stem: str | None = None
    page_label: str | None = None
    page_number: int | None = None
    excerpt: str | None = None


class QueryResponse(BaseModel):
    answer: str
    mode_used: str
    sources: list[QuerySource] | None = None


# Global variable to cache the query engine
_query_engine_cache: dict = {}
_settings_initialized = False
_settings_lock = Lock()

# Query result cache with TTL support
_query_cache_lock = Lock()
_query_cache: dict = {}
_query_cache_max_size = 100
_query_cache_ttl_seconds = int(os.getenv("RAG_CACHE_TTL_SECONDS", "3600"))  # Default 1 hour

# Cache statistics for observability
_cache_hits = 0
_cache_misses = 0


def ensure_settings_initialized():
    """Initialize global LlamaIndex settings only when the API needs to serve queries."""
    global _settings_initialized
    if _settings_initialized:
        return

    with _settings_lock:
        if _settings_initialized:
            return
        setup_settings()
        _settings_initialized = True




def _normalize_query_mode(query_mode):
    return "default" if query_mode else None


def _build_retrieval_settings(request: QueryRequest | None = None, overrides=None):
    retrieval_settings = dict(get_retrieval_settings())
    if request is not None:
        request_mode = _normalize_query_mode(request.mode)
        if request_mode is not None:
            retrieval_settings["query_mode"] = request_mode
        if request.exact_search is not None:
            retrieval_settings["exact_search"] = bool(request.exact_search)
        if request.similarity_top_k is not None:
            retrieval_settings["similarity_top_k"] = max(1, int(request.similarity_top_k))
    if overrides:
        retrieval_settings.update(overrides)
    return retrieval_settings


def _get_retrieval_signature(retrieval_settings):
    return (
        retrieval_settings["query_mode"],
        retrieval_settings["similarity_top_k"],
        retrieval_settings["similarity_cutoff"],
        retrieval_settings["search_hnsw_ef"],
        retrieval_settings["exact_search"],
        retrieval_settings["indexed_only"],
        retrieval_settings["quantization_rescore"],
        retrieval_settings["quantization_ignore"],
        retrieval_settings["quantization_oversampling"],
    )


def _create_query_engine(retrieval_settings=None):
    """
    Retrieve the vector index and instantiate a query engine.
    """
    ensure_settings_initialized()
    active_retrieval_settings = retrieval_settings or get_retrieval_settings()
    vector_store = get_qdrant_vector_store(retrieval_settings=active_retrieval_settings)

    # Check if the collection exists before loading the index
    client = vector_store.client
    collection_name = vector_store.collection_name
    has_quantization = False
    try:
        if not client.collection_exists(collection_name):
            raise HTTPException(
                status_code=404,
                detail=(
                    f"RAG system is not ready: collection '{collection_name}' not found. "
                    "Please run the ingestion process (e.g., 'make ingest') before querying."
                )
            )
        collection_info = client.get_collection(collection_name)
        has_quantization = collection_info.config.quantization_config is not None
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(
            status_code=503,
            detail=f"Failed to connect to vector database: {str(exc)}"
        )

    # We load the index from the existing vector store
    # It assumes the ingestion has already been run.
    index = VectorStoreIndex.from_vector_store(vector_store=vector_store)

    # Custom prompt to enforce concise answers
    qa_prompt_tmpl_str = (
        "Context information is below.\n"
        "---------------------\n"
        "{context_str}\n"
        "---------------------\n"
        "Given the context information and not prior knowledge, "
        "answer the query in a concise, brief, and direct manner. Keep the answer short (2-3 sentences max). "
        "If the answer is not contained within the context, say 'I don't have enough information'.\n"
        "Query: {query_str}\n"
        "Answer: "
    )
    qa_prompt_tmpl = PromptTemplate(qa_prompt_tmpl_str)

    from qdrant_client import models
    search_params = models.SearchParams(
        hnsw_ef=active_retrieval_settings["search_hnsw_ef"],
        exact=active_retrieval_settings["exact_search"],
        indexed_only=active_retrieval_settings["indexed_only"],
    )
    if has_quantization:
        search_params.quantization = models.QuantizationSearchParams(
            ignore=active_retrieval_settings["quantization_ignore"],
            rescore=active_retrieval_settings["quantization_rescore"],
            oversampling=active_retrieval_settings["quantization_oversampling"],
        )

    vector_store_kwargs = {
        "search_params": search_params
    }
    node_postprocessors = []
    if active_retrieval_settings["similarity_cutoff"] > 0:
        node_postprocessors.append(
            SimilarityPostprocessor(
                similarity_cutoff=active_retrieval_settings["similarity_cutoff"]
            )
        )

    query_engine_kwargs = {
        "similarity_top_k": active_retrieval_settings["similarity_top_k"],
        "vector_store_query_mode": active_retrieval_settings["query_mode"],
        "text_qa_template": qa_prompt_tmpl,
        "vector_store_kwargs": vector_store_kwargs,
    }
    if node_postprocessors:
        query_engine_kwargs["node_postprocessors"] = node_postprocessors

    return index.as_query_engine(**query_engine_kwargs)


def get_query_engine(retrieval_settings=None, use_cache=True):
    """
    Retrieve the vector index and instantiate a query engine.
    """
    global _query_engine_cache

    active_retrieval_settings = retrieval_settings or get_retrieval_settings()
    cache_key = _get_retrieval_signature(active_retrieval_settings)
    if use_cache and cache_key in _query_engine_cache:
        return _query_engine_cache[cache_key]

    query_engine = _create_query_engine(retrieval_settings=active_retrieval_settings)
    if use_cache:
        _query_engine_cache[cache_key] = query_engine
    return query_engine


def _build_sources(response, limit=5):
    source_nodes = getattr(response, "source_nodes", None) or []
    sources = []
    for source_node in source_nodes[:limit]:
        node = getattr(source_node, "node", source_node)
        metadata = dict(getattr(node, "metadata", {}) or {})
        text = getattr(node, "text", None)
        if text is None and hasattr(node, "get_content"):
            text = node.get_content()
        text = (text or "").strip().replace("\n", " ")
        if len(text) > 280:
            text = f"{text[:277]}..."
        sources.append(
            QuerySource(
                score=getattr(source_node, "score", None),
                file_name=metadata.get("file_name"),
                source_stem=metadata.get("source_stem"),
                page_label=metadata.get("page_label"),
                page_number=metadata.get("page_number"),
                excerpt=text or None,
            )
        )
    return sources or None


@app.post("/query", response_model=QueryResponse)
def query_rag(request: QueryRequest):
    """
    Endpoint to query the RAG system based on ingested PDFs.
    Implements caching with TTL for repeated queries.
    """
    global _cache_hits, _cache_misses
    
    # Trim only: query capitalization can be semantically meaningful.
    base_query = request.query.strip()
    
    try:
        retrieval_settings = _build_retrieval_settings(request=request)
        retrieval_signature = _get_retrieval_signature(retrieval_settings)
        cache_key = f"{base_query}:{retrieval_signature}"
        
        current_time = time.time()
        
        with _query_cache_lock:
            if cache_key in _query_cache:
                entry = _query_cache[cache_key]
                if current_time - entry["timestamp"] < _query_cache_ttl_seconds:
                    _cache_hits += 1
                    return QueryResponse(
                        answer=entry["answer"],
                        mode_used=entry["mode_used"],
                        sources=entry["sources"] if request.include_sources else None,
                    )
                else:
                    del _query_cache[cache_key]
        
        _cache_misses += 1
        
        query_engine = get_query_engine(retrieval_settings=retrieval_settings)
        used_retrieval_settings = dict(retrieval_settings)
        response = query_engine.query(request.query)

        answer = str(response)
        sources = _build_sources(response)
        used_mode = used_retrieval_settings["query_mode"]
        
        final_cache_key = f"{base_query}:{_get_retrieval_signature(used_retrieval_settings)}"
        with _query_cache_lock:
            if len(_query_cache) >= _query_cache_max_size:
                oldest_key = min(_query_cache.keys(), key=lambda k: _query_cache[k]["timestamp"])
                del _query_cache[oldest_key]
            _query_cache[final_cache_key] = {
                "answer": answer,
                "mode_used": used_mode,
                "sources": sources,
                "timestamp": current_time,
            }
        
        return QueryResponse(
            answer=answer,
            mode_used=used_mode,
            sources=sources if request.include_sources else None,
        )
    except HTTPException:
        raise
    except Exception as e:
        error_message = str(e)
        if (
            "CERTIFICATE_VERIFY_FAILED" in error_message
            and "Hostname mismatch" in error_message
        ) or "HTTPS traffic is being intercepted" in error_message:
            raise HTTPException(
                status_code=503,
                detail=(
                    "Query failed: TLS hostname verification failed for an external dependency. "
                    "This usually indicates captive-portal/proxy interception. "
                    "Authenticate/fix network egress and retry."
                ),
            )
        raise HTTPException(status_code=500, detail=f"Query failed: {error_message}")


@app.get("/health")
def health_check():
    """Simple health check endpoint."""
    return {"status": "ok"}


class CacheStatsResponse(BaseModel):
    size: int
    max_size: int
    ttl_seconds: int
    hits: int
    misses: int


class CacheClearResponse(BaseModel):
    cleared: int


@app.get("/cache/stats", response_model=CacheStatsResponse)
def get_cache_stats():
    """Get query cache statistics including TTL and hit rate."""
    with _query_cache_lock:
        return CacheStatsResponse(
            size=len(_query_cache),
            max_size=_query_cache_max_size,
            ttl_seconds=_query_cache_ttl_seconds,
            hits=_cache_hits,
            misses=_cache_misses
        )


@app.post("/cache/clear", response_model=CacheClearResponse)
def clear_cache():
    """Clear the query cache. Useful when documents are updated."""
    global _cache_hits, _cache_misses, _query_engine_cache
    with _query_cache_lock:
        cleared = len(_query_cache)
        _query_cache.clear()
        _query_engine_cache.clear()
        _cache_hits = 0
        _cache_misses = 0
        return CacheClearResponse(cleared=cleared)
