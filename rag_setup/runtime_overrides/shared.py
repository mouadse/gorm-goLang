import os
from llama_index.core import Settings
from llama_index.llms.openrouter import OpenRouter
from llama_index.embeddings.openai import OpenAIEmbedding
from llama_index.vector_stores.qdrant import QdrantVectorStore
import qdrant_client

DEFAULT_COLLECTION_NAME = "books"
DEFAULT_EMBED_MODEL = "text-embedding-3-small"
DEFAULT_CHUNK_SIZE = 512
DEFAULT_CHUNK_OVERLAP = 64
DEFAULT_SIMILARITY_TOP_K = 8
DEFAULT_QUERY_MODE = "default"
DEFAULT_SEARCH_HNSW_EF = 128
DEFAULT_QDRANT_SHARD_NUMBER = 1
DEFAULT_QDRANT_REPLICATION_FACTOR = 1
DEFAULT_QDRANT_WRITE_CONSISTENCY_FACTOR = 1
DEFAULT_QDRANT_ON_DISK_PAYLOAD = True
DEFAULT_QDRANT_HNSW_M = 32
DEFAULT_QDRANT_HNSW_EF_CONSTRUCT = 200
DEFAULT_QDRANT_FULL_SCAN_THRESHOLD = 10_000
DEFAULT_QDRANT_MAX_INDEXING_THREADS = 0
DEFAULT_QDRANT_DEFAULT_SEGMENT_NUMBER = 2
DEFAULT_QDRANT_INDEXING_THRESHOLD = 10_000
DEFAULT_QDRANT_FLUSH_INTERVAL_SEC = 5
DEFAULT_QDRANT_PREVENT_UNOPTIMIZED = True
DEFAULT_QDRANT_DENSE_ON_DISK = False
DEFAULT_QDRANT_QUANTIZATION_ENABLED = True
DEFAULT_QDRANT_QUANTIZATION_ALWAYS_RAM = True
DEFAULT_QDRANT_QUANTIZATION_OVERSAMPLING = 3.0
DEFAULT_QDRANT_QUANTIZATION_RESCORE = True
DEFAULT_QDRANT_QUANTIZATION_IGNORE = False
DEFAULT_QDRANT_EXACT_SEARCH = False
DEFAULT_QDRANT_INDEXED_ONLY = False
DEFAULT_EMBED_DIMENSION = 1536
CHUNK_METADATA_VERSION = 2
DENSE_VECTOR_NAME = "text-dense"
OPENAI_EMBEDDING_MODELS_WITH_DIMENSIONS = {
    "text-embedding-3-small",
    "text-embedding-3-large",
}
OPENAI_EMBEDDING_MODEL_NATIVE_DIMENSIONS = {
    "text-embedding-ada-002": 1536,
    "text-embedding-3-small": 1536,
    "text-embedding-3-large": 3072,
}


def _read_int_env(name, default_value, minimum=0):
    raw_value = os.getenv(name)
    if raw_value is None:
        return default_value
    try:
        parsed = int(raw_value)
    except ValueError:
        return default_value
    return max(minimum, parsed)


def _read_float_env(name, default_value, minimum=0.0):
    raw_value = os.getenv(name)
    if raw_value is None:
        return default_value
    try:
        parsed = float(raw_value)
    except ValueError:
        return default_value
    return max(minimum, parsed)


def _read_bool_env(name, default_value=False):
    raw_value = os.getenv(name)
    if raw_value is None:
        return default_value
    return raw_value.strip().lower() in {"1", "true", "yes", "on"}


def _read_str_env(name, default_value):
    raw_value = os.getenv(name)
    if raw_value is None:
        return default_value
    value = raw_value.strip()
    return value or default_value


def get_embedding_model_name():
    return _read_str_env("EMBED_MODEL", DEFAULT_EMBED_MODEL)


def _normalize_embedding_model_name(model_name):
    return (model_name or "").strip().lower()


def embedding_model_supports_dimensions(model_name=None):
    normalized_model = _normalize_embedding_model_name(
        model_name or get_embedding_model_name()
    )
    return normalized_model in OPENAI_EMBEDDING_MODELS_WITH_DIMENSIONS


def get_embedding_dimension():
    model_name = get_embedding_model_name()
    normalized_model = _normalize_embedding_model_name(model_name)
    if embedding_model_supports_dimensions(model_name):
        return _read_int_env("RAG_EMBED_DIMENSION", DEFAULT_EMBED_DIMENSION, minimum=1)
    if normalized_model in OPENAI_EMBEDDING_MODEL_NATIVE_DIMENSIONS:
        return OPENAI_EMBEDDING_MODEL_NATIVE_DIMENSIONS[normalized_model]
    return _read_int_env("RAG_EMBED_DIMENSION", DEFAULT_EMBED_DIMENSION, minimum=1)


def get_chunk_settings():
    chunk_size = _read_int_env("RAG_CHUNK_SIZE", DEFAULT_CHUNK_SIZE, minimum=32)
    chunk_overlap = _read_int_env(
        "RAG_CHUNK_OVERLAP", DEFAULT_CHUNK_OVERLAP, minimum=0
    )
    if chunk_overlap >= chunk_size:
        chunk_overlap = max(0, chunk_size // 8)
    return chunk_size, chunk_overlap


def get_ingest_config():
    chunk_size, chunk_overlap = get_chunk_settings()
    return {
        "embed_model": get_embedding_model_name(),
        "embed_dimension": get_embedding_dimension(),
        "chunk_size": chunk_size,
        "chunk_overlap": chunk_overlap,
        "chunk_metadata_version": CHUNK_METADATA_VERSION,
        "collection": get_qdrant_collection_settings(),
    }


def get_retrieval_settings():
    return {
        "query_mode": "default",
        "similarity_top_k": _read_int_env(
            "RAG_SIMILARITY_TOP_K", DEFAULT_SIMILARITY_TOP_K, minimum=1
        ),
        "similarity_cutoff": _read_float_env("RAG_SIMILARITY_CUTOFF", 0.0, minimum=0.0),
        "search_hnsw_ef": _read_int_env(
            "RAG_SEARCH_HNSW_EF", DEFAULT_SEARCH_HNSW_EF, minimum=1
        ),
        "exact_search": _read_bool_env(
            "RAG_EXACT_SEARCH", DEFAULT_QDRANT_EXACT_SEARCH
        ),
        "indexed_only": _read_bool_env(
            "RAG_INDEXED_ONLY", DEFAULT_QDRANT_INDEXED_ONLY
        ),
        "quantization_rescore": _read_bool_env(
            "RAG_QUANTIZATION_RESCORE", DEFAULT_QDRANT_QUANTIZATION_RESCORE
        ),
        "quantization_ignore": _read_bool_env(
            "RAG_QUANTIZATION_IGNORE", DEFAULT_QDRANT_QUANTIZATION_IGNORE
        ),
        "quantization_oversampling": _read_float_env(
            "RAG_QUANTIZATION_OVERSAMPLING",
            DEFAULT_QDRANT_QUANTIZATION_OVERSAMPLING,
            minimum=1.0,
        ),
    }


def get_qdrant_collection_settings():
    return {
        "dense_vector_name": DENSE_VECTOR_NAME,
        "embed_dimension": get_embedding_dimension(),
        "shard_number": _read_int_env(
            "QDRANT_SHARD_NUMBER", DEFAULT_QDRANT_SHARD_NUMBER, minimum=1
        ),
        "replication_factor": _read_int_env(
            "QDRANT_REPLICATION_FACTOR",
            DEFAULT_QDRANT_REPLICATION_FACTOR,
            minimum=1,
        ),
        "write_consistency_factor": _read_int_env(
            "QDRANT_WRITE_CONSISTENCY_FACTOR",
            DEFAULT_QDRANT_WRITE_CONSISTENCY_FACTOR,
            minimum=1,
        ),
        "on_disk_payload": _read_bool_env(
            "QDRANT_ON_DISK_PAYLOAD", DEFAULT_QDRANT_ON_DISK_PAYLOAD
        ),
        "dense_on_disk": _read_bool_env(
            "QDRANT_DENSE_ON_DISK", DEFAULT_QDRANT_DENSE_ON_DISK
        ),
        "hnsw_m": _read_int_env("QDRANT_HNSW_M", DEFAULT_QDRANT_HNSW_M, minimum=4),
        "hnsw_ef_construct": _read_int_env(
            "QDRANT_HNSW_EF_CONSTRUCT", DEFAULT_QDRANT_HNSW_EF_CONSTRUCT, minimum=8
        ),
        "full_scan_threshold": _read_int_env(
            "QDRANT_FULL_SCAN_THRESHOLD",
            DEFAULT_QDRANT_FULL_SCAN_THRESHOLD,
            minimum=1,
        ),
        "max_indexing_threads": _read_int_env(
            "QDRANT_MAX_INDEXING_THREADS",
            DEFAULT_QDRANT_MAX_INDEXING_THREADS,
            minimum=0,
        ),
        "default_segment_number": _read_int_env(
            "QDRANT_DEFAULT_SEGMENT_NUMBER",
            DEFAULT_QDRANT_DEFAULT_SEGMENT_NUMBER,
            minimum=1,
        ),
        "indexing_threshold": _read_int_env(
            "QDRANT_INDEXING_THRESHOLD",
            DEFAULT_QDRANT_INDEXING_THRESHOLD,
            minimum=1,
        ),
        "flush_interval_sec": _read_int_env(
            "QDRANT_FLUSH_INTERVAL_SEC",
            DEFAULT_QDRANT_FLUSH_INTERVAL_SEC,
            minimum=1,
        ),
        "prevent_unoptimized": _read_bool_env(
            "QDRANT_PREVENT_UNOPTIMIZED", DEFAULT_QDRANT_PREVENT_UNOPTIMIZED
        ),
        "quantization_enabled": _read_bool_env(
            "QDRANT_ENABLE_QUANTIZATION", DEFAULT_QDRANT_QUANTIZATION_ENABLED
        ),
        "quantization_always_ram": _read_bool_env(
            "QDRANT_QUANTIZATION_ALWAYS_RAM",
            DEFAULT_QDRANT_QUANTIZATION_ALWAYS_RAM,
        ),
    }


def get_qdrant_payload_indexes():
    models = qdrant_client.models
    return [
        {"field_name": "doc_id", "field_schema": models.PayloadSchemaType.KEYWORD},
        {"field_name": "document_id", "field_schema": models.PayloadSchemaType.KEYWORD},
        {"field_name": "file_name", "field_schema": models.PayloadSchemaType.KEYWORD},
        {"field_name": "source_stem", "field_schema": models.PayloadSchemaType.KEYWORD},
        {"field_name": "page_label", "field_schema": models.PayloadSchemaType.KEYWORD},
        {"field_name": "page_number", "field_schema": models.PayloadSchemaType.INTEGER},
    ]


def build_qdrant_dense_config(collection_settings=None):
    settings = collection_settings or get_qdrant_collection_settings()
    return qdrant_client.models.VectorParams(
        size=settings["embed_dimension"],
        distance=qdrant_client.models.Distance.COSINE,
        on_disk=settings["dense_on_disk"],
    )


def build_qdrant_vectors_config(collection_settings=None):
    return build_qdrant_dense_config(collection_settings)


def build_qdrant_quantization_config(collection_settings=None):
    settings = collection_settings or get_qdrant_collection_settings()
    if not settings["quantization_enabled"]:
        return None
    return qdrant_client.models.ScalarQuantization(
        scalar=qdrant_client.models.ScalarQuantizationConfig(
            type=qdrant_client.models.ScalarType.INT8,
            always_ram=settings["quantization_always_ram"],
        )
    )


def build_qdrant_hnsw_config(collection_settings=None):
    settings = collection_settings or get_qdrant_collection_settings()
    return qdrant_client.models.HnswConfigDiff(
        m=settings["hnsw_m"],
        ef_construct=settings["hnsw_ef_construct"],
        full_scan_threshold=settings["full_scan_threshold"],
        max_indexing_threads=settings["max_indexing_threads"],
    )


def build_qdrant_optimizer_config(collection_settings=None):
    settings = collection_settings or get_qdrant_collection_settings()
    return qdrant_client.models.OptimizersConfigDiff(
        default_segment_number=settings["default_segment_number"],
        indexing_threshold=settings["indexing_threshold"],
        flush_interval_sec=settings["flush_interval_sec"],
        prevent_unoptimized=settings["prevent_unoptimized"],
    )


def _build_openai_embedding_model():
    model_name = get_embedding_model_name()
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise RuntimeError(
            "OPENAI_API_KEY environment variable is required for embeddings."
        )

    embedding_kwargs = {
        "model": model_name,
        "api_key": api_key,
        "embed_batch_size": _read_int_env("RAG_EMBED_BATCH_SIZE", 64, minimum=1),
    }
    if embedding_model_supports_dimensions(model_name):
        embedding_kwargs["dimensions"] = get_embedding_dimension()

    return OpenAIEmbedding(**embedding_kwargs)


def setup_settings():
    Settings.embed_model = _build_openai_embedding_model()
    chunk_size, chunk_overlap = get_chunk_settings()
    Settings.chunk_size = chunk_size
    Settings.chunk_overlap = chunk_overlap

    api_key = os.getenv("OPENROUTER_API_KEY")
    model = os.getenv("OPENROUTER_MODEL", "google/gemini-3-flash-preview")
    try:
        max_tokens = int(os.getenv("OPENROUTER_MAX_TOKENS", "512"))
    except ValueError:
        max_tokens = 512
    if not api_key:
        print("Warning: OPENROUTER_API_KEY environment variable is not set!")

    Settings.llm = OpenRouter(api_key=api_key, model=model, max_tokens=max_tokens)


def get_collection_name():
    return os.getenv("QDRANT_COLLECTION", DEFAULT_COLLECTION_NAME)


def get_qdrant_client():
    qdrant_url = os.getenv("QDRANT_URL", "http://localhost:6333")
    timeout_seconds = _read_float_env("QDRANT_TIMEOUT_SECONDS", 10.0, minimum=0.1)
    return qdrant_client.QdrantClient(url=qdrant_url, timeout=timeout_seconds)


def get_qdrant_vector_store(collection_name=None, client=None, retrieval_settings=None):
    if collection_name is None:
        collection_name = get_collection_name()

    if client is None:
        client = get_qdrant_client()

    collection_settings = get_qdrant_collection_settings()

    vector_store_kwargs = {
        "client": client,
        "collection_name": collection_name,
        "enable_hybrid": False,
        "dense_config": build_qdrant_dense_config(collection_settings),
        "quantization_config": build_qdrant_quantization_config(collection_settings),
        "shard_number": collection_settings["shard_number"],
        "replication_factor": collection_settings["replication_factor"],
        "write_consistency_factor": collection_settings["write_consistency_factor"],
        "payload_indexes": get_qdrant_payload_indexes(),
    }

    return QdrantVectorStore(**vector_store_kwargs)
