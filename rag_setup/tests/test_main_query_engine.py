import types
import unittest
from unittest.mock import patch

from test_support import load_app_module


class MainQueryEngineTests(unittest.TestCase):
    def setUp(self):
        self.main = load_app_module("main")
        self.main._query_engine_cache = {}
        self.main._query_cache = {}
        self.main._cache_hits = 0
        self.main._cache_misses = 0

    def test_query_engine_uses_configurable_topk(self):
        captured = {}

        class FakeIndex:
            def as_query_engine(self, **kwargs):
                captured.update(kwargs)
                return types.SimpleNamespace(name="engine")

        class FakeClient:
            def collection_exists(self, collection_name):
                return True
            def get_collection(self, collection_name):
                import types
                # Return a mock collection_info with no quantization_config
                return types.SimpleNamespace(config=types.SimpleNamespace(quantization_config=None))

        fake_vector_store = types.SimpleNamespace(client=FakeClient(), collection_name="test")

        with (
            patch.object(self.main, "get_qdrant_vector_store", return_value=fake_vector_store),
            patch.object(
                self.main.VectorStoreIndex,
                "from_vector_store",
                return_value=FakeIndex(),
            ),
            patch.object(self.main, "setup_settings", return_value=None),
            patch.object(
                self.main,
                "get_retrieval_settings",
                return_value={
                    "query_mode": "default",
                    "similarity_top_k": 11,
                    "similarity_cutoff": 0.25,
                    "search_hnsw_ef": 96,
                    "exact_search": True,
                    "indexed_only": False,
                    "quantization_rescore": True,
                    "quantization_ignore": False,
                    "quantization_oversampling": 2.0,
                },
            ),
        ):
            engine = self.main.get_query_engine()

        self.assertEqual(engine.name, "engine")
        self.assertEqual(captured["vector_store_query_mode"], "default")
        self.assertEqual(captured["similarity_top_k"], 11)
        self.assertEqual(captured["vector_store_kwargs"]["search_params"].hnsw_ef, 96)
        self.assertTrue(captured["vector_store_kwargs"]["search_params"].exact)
        self.assertEqual(
            captured["node_postprocessors"][0].similarity_cutoff,
            0.25,
        )

    def test_query_cache_preserves_query_case(self):
        class EchoEngine:
            def __init__(self):
                self.queries = []

            def query(self, query_text):
                self.queries.append(query_text)
                return f"answer for {query_text}"

        engine = EchoEngine()
        retrieval_settings = {
            "query_mode": "default",
            "similarity_top_k": 8,
            "similarity_cutoff": 0.0,
            "search_hnsw_ef": 128,
            "exact_search": False,
            "indexed_only": False,
            "quantization_rescore": True,
            "quantization_ignore": False,
            "quantization_oversampling": 3.0,
        }

        with patch.object(
            self.main,
            "get_query_engine",
            return_value=engine,
        ) as get_query_engine_mock, patch.object(
            self.main,
            "get_retrieval_settings",
            return_value=retrieval_settings,
        ):
            upper_response = self.main.query_rag(self.main.QueryRequest(query="US"))
            lower_response = self.main.query_rag(self.main.QueryRequest(query="us"))
            cached_upper_response = self.main.query_rag(self.main.QueryRequest(query="US"))

        self.assertEqual(upper_response.answer, "answer for US")
        self.assertEqual(lower_response.answer, "answer for us")
        self.assertEqual(cached_upper_response.answer, "answer for US")
        self.assertEqual(engine.queries, ["US", "us"])
        self.assertEqual(get_query_engine_mock.call_count, 2)
        self.assertEqual(self.main._cache_hits, 1)
        self.assertEqual(self.main._cache_misses, 2)


if __name__ == "__main__":
    unittest.main()
