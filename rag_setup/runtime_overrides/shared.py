import os
from llama_index.core import Settings
from llama_index.llms.openrouter import OpenRouter
from llama_index.embeddings.fastembed import FastEmbedEmbedding
from llama_index.vector_stores.qdrant import QdrantVectorStore
from fastembed import TextEmbedding
import qdrant_client

DEFAULT_COLLECTION_NAME = "books"
DEFAULT_EMBED_MODEL = "BAAI/bge-base-en-v1.5"
DEFAULT_CHUNK_SIZE = 512
DEFAULT_CHUNK_OVERLAP = 64
DEFAULT_SIMILARITY_TOP_K = 8
DEFAULT_SPARSE_TOP_K = 8
DEFAULT_QUERY_MODE = "hybrid"
DEFAULT_SEARCH_HNSW_EF = 128
DEFAULT_HYBRID_FUSION = "rrf"
DEFAULT_HYBRID_ALPHA = 0.5
DEFAULT_HYBRID_RRF_K = 60
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
DEFAULT_QDRANT_SPARSE_ON_DISK = False
DEFAULT_QDRANT_QUANTIZATION_ENABLED = True
DEFAULT_QDRANT_QUANTIZATION_ALWAYS_RAM = True
DEFAULT_QDRANT_QUANTIZATION_OVERSAMPLING = 3.0
DEFAULT_QDRANT_QUANTIZATION_RESCORE = True
DEFAULT_QDRANT_QUANTIZATION_IGNORE = False
DEFAULT_QDRANT_EXACT_SEARCH = False
DEFAULT_QDRANT_INDEXED_ONLY = False
DEFAULT_EMBED_DIMENSION = 768
CHUNK_METADATA_VERSION = 2
DENSE_VECTOR_NAME = "text-dense"
SPARSE_VECTOR_NAME = "text-sparse-new"


def _read_int_env(name, default_value, minimum=0):
    raw_value = os.getenv(name)
    if raw_value is None:
        return default_value

    try:
        parsed = int(raw_value)
    except ValueError:
        return default_value
    return max(minimum, parsed)


def _read_optional_int_env(name, minimum=None):
    raw_value = os.getenv(name)
    if raw_value is None:
        return None

    value = raw_value.strip()
    if value == "":
        return None

    try:
        parsed = int(value)
    except ValueError:
        return None

    if minimum is not None:
        parsed = max(minimum, parsed)
    return parsed


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


def _normalize_query_mode(query_mode):
    normalized = (query_mode or DEFAULT_QUERY_MODE).strip().lower()
    if normalized == "dense":
        normalized = "default"
    if normalized not in {"default", "sparse", "hybrid"}:
        normalized = DEFAULT_QUERY_MODE
    return normalized


def get_embedding_model_name():
    return os.getenv("EMBED_MODEL", DEFAULT_EMBED_MODEL)


def get_embedding_dimension():
    configured_dimension = _read_int_env(
        "RAG_EMBED_DIMENSION", DEFAULT_EMBED_DIMENSION, minimum=1
    )
    model_name = get_embedding_model_name()
    for model_description in TextEmbedding.list_supported_models():
        if model_description.get("model") == model_name:
            return int(model_description["dim"])
    if model_name == DEFAULT_EMBED_MODEL:
        return DEFAULT_EMBED_DIMENSION
    return configured_dimension


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
        "query_mode": _normalize_query_mode(
            os.getenv("RAG_QUERY_MODE", DEFAULT_QUERY_MODE)
        ),
        "similarity_top_k": _read_int_env(
            "RAG_SIMILARITY_TOP_K", DEFAULT_SIMILARITY_TOP_K, minimum=1
        ),
        "sparse_top_k": _read_int_env(
            "RAG_SPARSE_TOP_K", DEFAULT_SPARSE_TOP_K, minimum=1
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
        "hybrid_fusion": _read_str_env(
            "RAG_HYBRID_FUSION", DEFAULT_HYBRID_FUSION
        ).lower(),
        "hybrid_alpha": _read_float_env(
            "RAG_HYBRID_ALPHA", DEFAULT_HYBRID_ALPHA, minimum=0.0
        ),
        "hybrid_rrf_k": _read_int_env(
            "RAG_HYBRID_RRF_K", DEFAULT_HYBRID_RRF_K, minimum=1
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
        "sparse_vector_name": SPARSE_VECTOR_NAME,
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
        "sparse_on_disk": _read_bool_env(
            "QDRANT_SPARSE_ON_DISK", DEFAULT_QDRANT_SPARSE_ON_DISK
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


def build_qdrant_sparse_config(collection_settings=None):
    settings = collection_settings or get_qdrant_collection_settings()
    return qdrant_client.models.SparseVectorParams(
        index=qdrant_client.models.SparseIndexParams(
            on_disk=settings["sparse_on_disk"]
        )
    )


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


def _result_tuples(result):
    nodes = getattr(result, "nodes", None) or []
    similarities = getattr(result, "similarities", None) or []
    pairs = list(zip(similarities, nodes))
    pairs.sort(key=lambda pair: pair[0], reverse=True)
    return pairs


def _empty_result_like(example_result):
    return example_result.__class__(nodes=None, similarities=None, ids=None)


def _relative_score_fusion(dense_result, sparse_result, alpha=0.5, top_k=2):
    dense_pairs = _result_tuples(dense_result)
    sparse_pairs = _result_tuples(sparse_result)

    if not dense_pairs and not sparse_pairs:
        return _empty_result_like(dense_result)
    if not sparse_pairs:
        return dense_result
    if not dense_pairs:
        return sparse_result

    all_nodes = {node.node_id: node for _, node in dense_pairs}
    all_nodes.update({node.node_id: node for _, node in sparse_pairs})

    def normalize(pairs):
        scores = [score for score, _ in pairs]
        if not scores:
            return {}
        max_score = max(scores)
        min_score = min(scores)
        if max_score == min_score:
            return {node.node_id: max_score for _, node in pairs}
        return {
            node.node_id: (score - min_score) / (max_score - min_score)
            for score, node in pairs
        }

    dense_scores = normalize(dense_pairs)
    sparse_scores = normalize(sparse_pairs)

    fused = []
    for node_id, node in all_nodes.items():
        dense_score = dense_scores.get(node_id, 0.0)
        sparse_score = sparse_scores.get(node_id, 0.0)
        fused.append(((alpha * dense_score) + ((1 - alpha) * sparse_score), node))

    fused.sort(key=lambda item: item[0], reverse=True)
    fused = fused[:top_k]
    return dense_result.__class__(
        nodes=[node for _, node in fused],
        similarities=[score for score, _ in fused],
        ids=[node.node_id for _, node in fused],
    )


def _rrf_score(rank, rrf_k):
    return 1.0 / (rrf_k + rank)


def _reciprocal_rank_fusion(
    dense_result,
    sparse_result,
    alpha=0.5,
    top_k=2,
    rrf_k=DEFAULT_HYBRID_RRF_K,
):
    dense_pairs = _result_tuples(dense_result)
    sparse_pairs = _result_tuples(sparse_result)

    if not dense_pairs and not sparse_pairs:
        return _empty_result_like(dense_result)
    if not sparse_pairs:
        return dense_result
    if not dense_pairs:
        return sparse_result

    all_nodes = {node.node_id: node for _, node in dense_pairs}
    all_nodes.update({node.node_id: node for _, node in sparse_pairs})

    dense_ranks = {
        node.node_id: index for index, (_, node) in enumerate(dense_pairs, start=1)
    }
    sparse_ranks = {
        node.node_id: index for index, (_, node) in enumerate(sparse_pairs, start=1)
    }

    fused = []
    for node_id, node in all_nodes.items():
        score = 0.0
        if node_id in dense_ranks:
            score += alpha * _rrf_score(dense_ranks[node_id], rrf_k)
        if node_id in sparse_ranks:
            score += (1 - alpha) * _rrf_score(sparse_ranks[node_id], rrf_k)
        fused.append((score, node))

    fused.sort(key=lambda item: item[0], reverse=True)
    fused = fused[:top_k]
    return dense_result.__class__(
        nodes=[node for _, node in fused],
        similarities=[score for score, _ in fused],
        ids=[node.node_id for _, node in fused],
    )


def get_hybrid_fusion_fn(retrieval_settings=None):
    settings = retrieval_settings or get_retrieval_settings()
    alpha = min(max(settings["hybrid_alpha"], 0.0), 1.0)
    fusion_mode = settings["hybrid_fusion"]
    if fusion_mode == "relative_score":
        return lambda dense_result, sparse_result, alpha=alpha, top_k=2: _relative_score_fusion(
            dense_result, sparse_result, alpha=alpha, top_k=top_k
        )
    return lambda dense_result, sparse_result, alpha=alpha, top_k=2: _reciprocal_rank_fusion(
        dense_result,
        sparse_result,
        alpha=alpha,
        top_k=top_k,
        rrf_k=settings["hybrid_rrf_k"],
    )


def _is_tls_hostname_mismatch(exception):
    message = str(exception)
    return "CERTIFICATE_VERIFY_FAILED" in message and "Hostname mismatch" in message


class TunedFastEmbedEmbedding(FastEmbedEmbedding):
    def __init__(self, *args, embed_call_batch_size=None, embed_call_parallel=None, **kwargs):
        super().__init__(*args, **kwargs)
        self._embed_call_batch_size = embed_call_batch_size
        self._embed_call_parallel = embed_call_parallel

    def _embed_with_runtime_settings(self, texts):
        embed_kwargs = {}
        if self._embed_call_batch_size is not None:
            embed_kwargs["batch_size"] = self._embed_call_batch_size
        if self._embed_call_parallel is not None:
            embed_kwargs["parallel"] = self._embed_call_parallel

        if self.doc_embed_type == "passage":
            embeddings = list(self._model.passage_embed(texts, **embed_kwargs))
        else:
            embeddings = list(self._model.embed(texts, **embed_kwargs))
        return [embedding.tolist() for embedding in embeddings]

    def _get_text_embeddings(self, texts):
        return self._embed_with_runtime_settings(texts)


def get_embedding_runtime_settings():
    threads = _read_optional_int_env("RAG_EMBED_THREADS", minimum=0)
    if threads is None:
        threads = _read_optional_int_env("RAG_CPU_THREADS", minimum=0)
    if threads == 0:
        threads = None

    return {
        "threads": threads,
        "batch_size": _read_optional_int_env("RAG_EMBED_BATCH_SIZE", minimum=1),
        "parallel": _read_optional_int_env("RAG_EMBED_PARALLEL", minimum=0),
        "embed_batch_size": _read_int_env("RAG_EMBED_BATCH_SIZE", 10, minimum=1),
    }


def _build_fastembed_model():
    model_name = get_embedding_model_name()
    local_files_only = _read_bool_env("RAG_EMBED_LOCAL_ONLY", False)
    runtime_settings = get_embedding_runtime_settings()

    init_kwargs = {
        "model_name": model_name,
        "local_files_only": local_files_only,
        "threads": runtime_settings["threads"],
        "embed_batch_size": runtime_settings["embed_batch_size"],
        "embed_call_batch_size": runtime_settings["batch_size"],
        "embed_call_parallel": runtime_settings["parallel"],
    }

    try:
        return TunedFastEmbedEmbedding(**init_kwargs)
    except Exception as exc:
        if local_files_only:
            raise RuntimeError(
                "Embedding model cache is required but missing/corrupted. "
                "RAG_EMBED_LOCAL_ONLY=1 is set and no local FastEmbed model is available. "
                "Populate the shared Hugging Face cache first, then retry."
            ) from exc

        if _is_tls_hostname_mismatch(exc):
            try:
                return TunedFastEmbedEmbedding(
                    **{**init_kwargs, "local_files_only": True}
                )
            except Exception as local_only_exc:
                raise RuntimeError(
                    "Embedding model download failed due TLS hostname mismatch while reaching external model hosts. "
                    "This usually means HTTPS traffic is being intercepted (for example captive portal/proxy). "
                    "Authenticate/fix network egress and retry, or pre-populate the model cache and set "
                    "RAG_EMBED_LOCAL_ONLY=1."
                ) from local_only_exc
        raise


def setup_settings():
    """
    Configure global LlamaIndex settings to use:
    - HuggingFace embedding model (BAAI/bge-base-en-v1.5)
    - OpenRouter for LLM (google/gemini-3-flash-preview as default)
    """
    # Use FastEmbed local embedding model for faster installation without huge PyTorch dependency
    Settings.embed_model = _build_fastembed_model()
    chunk_size, chunk_overlap = get_chunk_settings()
    Settings.chunk_size = chunk_size
    Settings.chunk_overlap = chunk_overlap

    # Configure OpenRouter for LLM with API key from environment
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
    """
    Initialize Qdrant Vector Store connected to the containerized DB.
    """
    if collection_name is None:
        collection_name = get_collection_name()

    if client is None:
        client = get_qdrant_client()

    collection_settings = get_qdrant_collection_settings()
    active_retrieval_settings = retrieval_settings or get_retrieval_settings()

    return QdrantVectorStore(
        client=client,
        collection_name=collection_name,
        enable_hybrid=True,
        dense_config=build_qdrant_dense_config(collection_settings),
        sparse_config=build_qdrant_sparse_config(collection_settings),
        quantization_config=build_qdrant_quantization_config(collection_settings),
        hybrid_fusion_fn=get_hybrid_fusion_fn(active_retrieval_settings),
        dense_vector_name=collection_settings["dense_vector_name"],
        sparse_vector_name=collection_settings["sparse_vector_name"],
        shard_number=collection_settings["shard_number"],
        replication_factor=collection_settings["replication_factor"],
        write_consistency_factor=collection_settings["write_consistency_factor"],
        payload_indexes=get_qdrant_payload_indexes(),
    )
