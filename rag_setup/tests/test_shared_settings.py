import os
import unittest
from unittest.mock import patch

from test_support import load_app_module


class SharedSettingsTests(unittest.TestCase):
    def setUp(self):
        self.shared = load_app_module("shared")

    def test_retrieval_settings_are_read_from_environment(self):
        with patch.dict(
            os.environ,
            {
                "RAG_QUERY_MODE": "dense",
                "RAG_SIMILARITY_TOP_K": "12",
                "RAG_SEARCH_HNSW_EF": "77",
                "RAG_EXACT_SEARCH": "true",
            },
            clear=False,
        ):
            settings = self.shared.get_retrieval_settings()

        self.assertEqual(settings["query_mode"], "default")
        self.assertEqual(settings["similarity_top_k"], 12)
        self.assertEqual(settings["search_hnsw_ef"], 77)
        self.assertTrue(settings["exact_search"])

    def test_chunk_overlap_is_clamped_when_it_exceeds_chunk_size(self):
        with patch.dict(
            os.environ,
            {"RAG_CHUNK_SIZE": "128", "RAG_CHUNK_OVERLAP": "300"},
            clear=False,
        ):
            chunk_size, chunk_overlap = self.shared.get_chunk_settings()

        self.assertEqual(chunk_size, 128)
        self.assertEqual(chunk_overlap, 16)

    def test_collection_settings_include_embedding_dimension_and_qdrant_tuning(self):
        with patch.dict(
            os.environ,
            {"QDRANT_HNSW_M": "32"},
            clear=False,
        ):
            settings = self.shared.get_qdrant_collection_settings()

        self.assertEqual(settings["embed_dimension"], 1536)
        self.assertEqual(settings["hnsw_m"], 32)
        self.assertTrue(settings["quantization_enabled"])

    def test_openai_embedding_model_uses_openai_api_key_and_dimension(self):
        with patch.dict(
            os.environ,
            {"OPENAI_API_KEY": "test-key", "RAG_EMBED_DIMENSION": "512"},
            clear=False,
        ):
            self.shared.setup_settings()

        self.assertEqual(
            self.shared.Settings.embed_model.kwargs["model"], "text-embedding-3-small"
        )
        self.assertEqual(self.shared.Settings.embed_model.kwargs["api_key"], "test-key")
        self.assertEqual(self.shared.Settings.embed_model.kwargs["dimensions"], 512)

    def test_openai_embedding_model_omits_dimension_for_unsupported_model(self):
        with patch.dict(
            os.environ,
            {
                "OPENAI_API_KEY": "test-key",
                "EMBED_MODEL": "text-embedding-ada-002",
                "RAG_EMBED_DIMENSION": "512",
            },
            clear=False,
        ):
            self.shared.setup_settings()
            collection_settings = self.shared.get_qdrant_collection_settings()

        self.assertEqual(
            self.shared.Settings.embed_model.kwargs["model"], "text-embedding-ada-002"
        )
        self.assertNotIn("dimensions", self.shared.Settings.embed_model.kwargs)
        self.assertEqual(collection_settings["embed_dimension"], 1536)

    def test_unknown_embedding_model_omits_dimension_but_uses_configured_schema_size(self):
        with patch.dict(
            os.environ,
            {
                "OPENAI_API_KEY": "test-key",
                "EMBED_MODEL": "custom-embedding-model",
                "RAG_EMBED_DIMENSION": "2048",
            },
            clear=False,
        ):
            self.shared.setup_settings()
            collection_settings = self.shared.get_qdrant_collection_settings()

        self.assertEqual(
            self.shared.Settings.embed_model.kwargs["model"], "custom-embedding-model"
        )
        self.assertNotIn("dimensions", self.shared.Settings.embed_model.kwargs)
        self.assertEqual(collection_settings["embed_dimension"], 2048)

    def test_dense_only_qdrant_vector_store_does_not_use_named_dense_vector(self):
        fake_client = object()

        vector_store = self.shared.get_qdrant_vector_store(client=fake_client)

        self.assertFalse(vector_store.kwargs["enable_hybrid"])
        self.assertNotIn("dense_vector_name", vector_store.kwargs)


if __name__ == "__main__":
    unittest.main()
