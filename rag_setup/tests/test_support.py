import importlib
import sys
import types
from pathlib import Path

APP_DIR = Path(__file__).resolve().parents[1] / "app"
if str(APP_DIR) not in sys.path:
    sys.path.insert(0, str(APP_DIR))


def install_external_stubs():
    llama_index_pkg = types.ModuleType("llama_index")
    core_mod = types.ModuleType("llama_index.core")
    llms_pkg = types.ModuleType("llama_index.llms")
    openrouter_mod = types.ModuleType("llama_index.llms.openrouter")
    embeddings_pkg = types.ModuleType("llama_index.embeddings")
    openai_embedding_mod = types.ModuleType("llama_index.embeddings.openai")
    fastembed_mod = types.ModuleType("llama_index.embeddings.fastembed")
    postprocessor_mod = types.ModuleType("llama_index.core.postprocessor")
    vector_stores_pkg = types.ModuleType("llama_index.vector_stores")
    qdrant_vs_mod = types.ModuleType("llama_index.vector_stores.qdrant")
    qdrant_client_mod = types.ModuleType("qdrant_client")
    qdrant_client_models_mod = types.ModuleType("qdrant_client.models")
    fastapi_mod = types.ModuleType("fastapi")
    pydantic_mod = types.ModuleType("pydantic")
    fastembed_pkg = types.ModuleType("fastembed")

    class DummySettings:
        pass

    class DummyPromptTemplate:
        def __init__(self, template):
            self.template = template

    class DummySearchParams:
        def __init__(self, **kwargs):
            for k, v in kwargs.items():
                setattr(self, k, v)
                
    class DummyQuantizationSearchParams:
        def __init__(self, **kwargs):
            for k, v in kwargs.items():
                setattr(self, k, v)

    class DummyQdrantModel:
        def __init__(self, **kwargs):
            for k, v in kwargs.items():
                setattr(self, k, v)

    class DummyDisabled:
        pass

    class DummyPayloadSchemaType:
        KEYWORD = "keyword"
        INTEGER = "integer"

    class DummyDistance:
        COSINE = "cosine"

    class DummyScalarType:
        INT8 = "int8"

    qdrant_client_models_mod.SearchParams = DummySearchParams
    qdrant_client_models_mod.QuantizationSearchParams = DummyQuantizationSearchParams
    qdrant_client_models_mod.VectorParams = DummyQdrantModel
    qdrant_client_models_mod.ScalarQuantizationConfig = DummyQdrantModel
    qdrant_client_models_mod.ScalarQuantization = DummyQdrantModel
    qdrant_client_models_mod.HnswConfigDiff = DummyQdrantModel
    qdrant_client_models_mod.OptimizersConfigDiff = DummyQdrantModel
    qdrant_client_models_mod.SparseVectorParams = DummyQdrantModel
    qdrant_client_models_mod.SparseIndexParams = DummyQdrantModel
    qdrant_client_models_mod.Disabled = DummyDisabled
    qdrant_client_models_mod.PayloadSchemaType = DummyPayloadSchemaType
    qdrant_client_models_mod.Distance = DummyDistance
    qdrant_client_models_mod.ScalarType = DummyScalarType

    class DummySimpleDirectoryReader:
        def __init__(self, input_dir, required_exts=None):
            self.input_dir = input_dir
            self.required_exts = required_exts or []

        def load_data(self):
            return [
                types.SimpleNamespace(
                    metadata={
                        "file_name": "stub.pdf",
                        "page_label": "1",
                    },
                    excluded_embed_metadata_keys=[],
                    excluded_llm_metadata_keys=[],
                )
            ]

    class DummyStorageContext:
        @classmethod
        def from_defaults(cls, vector_store=None):
            return {"vector_store": vector_store}

    class DummyVectorStoreIndex:
        @classmethod
        def from_documents(cls, documents, storage_context=None, show_progress=False):
            return {"documents": documents, "storage_context": storage_context}

        @classmethod
        def from_vector_store(cls, vector_store=None):
            return types.SimpleNamespace(
                as_query_engine=lambda **kwargs: types.SimpleNamespace(kwargs=kwargs)
            )

    class DummyOpenRouter:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

    class DummyOpenAIEmbedding:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

    class DummyFastEmbedEmbedding:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

    class DummyTextEmbedding:
        @staticmethod
        def list_supported_models():
            return [{"model": "BAAI/bge-base-en-v1.5", "dim": 768}]

    class DummyQdrantClient:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

    class DummyQdrantVectorStore:
        def __init__(self, **kwargs):
            self.kwargs = kwargs
            self.client = kwargs.get("client")
            self.collection_name = kwargs.get("collection_name")

    class DummySimilarityPostprocessor:
        def __init__(self, **kwargs):
            for key, value in kwargs.items():
                setattr(self, key, value)

    class DummyFastAPI:
        def __init__(self, **kwargs):
            self.kwargs = kwargs

        def post(self, *_args, **_kwargs):
            def decorator(func):
                return func

            return decorator

        def get(self, *_args, **_kwargs):
            def decorator(func):
                return func

            return decorator

    class DummyHTTPException(Exception):
        def __init__(self, status_code, detail):
            super().__init__(detail)
            self.status_code = status_code
            self.detail = detail

    class DummyBaseModel:
        def __init__(self, **kwargs):
            for key, value in kwargs.items():
                setattr(self, key, value)

    core_mod.Settings = DummySettings()
    core_mod.PromptTemplate = DummyPromptTemplate
    core_mod.SimpleDirectoryReader = DummySimpleDirectoryReader
    core_mod.StorageContext = DummyStorageContext
    core_mod.VectorStoreIndex = DummyVectorStoreIndex
    openrouter_mod.OpenRouter = DummyOpenRouter
    openai_embedding_mod.OpenAIEmbedding = DummyOpenAIEmbedding
    fastembed_mod.FastEmbedEmbedding = DummyFastEmbedEmbedding
    postprocessor_mod.SimilarityPostprocessor = DummySimilarityPostprocessor
    qdrant_vs_mod.QdrantVectorStore = DummyQdrantVectorStore
    qdrant_client_mod.QdrantClient = DummyQdrantClient
    fastapi_mod.FastAPI = DummyFastAPI
    fastapi_mod.HTTPException = DummyHTTPException
    pydantic_mod.BaseModel = DummyBaseModel
    fastembed_pkg.TextEmbedding = DummyTextEmbedding

    sys.modules["llama_index"] = llama_index_pkg
    sys.modules["llama_index.core"] = core_mod
    sys.modules["llama_index.core.postprocessor"] = postprocessor_mod
    sys.modules["llama_index.llms"] = llms_pkg
    sys.modules["llama_index.llms.openrouter"] = openrouter_mod
    sys.modules["llama_index.embeddings"] = embeddings_pkg
    sys.modules["llama_index.embeddings.openai"] = openai_embedding_mod
    sys.modules["llama_index.embeddings.fastembed"] = fastembed_mod
    sys.modules["llama_index.vector_stores"] = vector_stores_pkg
    sys.modules["llama_index.vector_stores.qdrant"] = qdrant_vs_mod
    sys.modules["qdrant_client"] = qdrant_client_mod
    sys.modules["qdrant_client.models"] = qdrant_client_models_mod
    qdrant_client_mod.models = qdrant_client_models_mod
    sys.modules["fastapi"] = fastapi_mod
    sys.modules["pydantic"] = pydantic_mod
    sys.modules["fastembed"] = fastembed_pkg


def load_app_module(module_name):
    install_external_stubs()
    for cached_name in ("shared", "ingest", "main"):
        if cached_name in sys.modules:
            del sys.modules[cached_name]
    importlib.invalidate_caches()
    return importlib.import_module(module_name)
