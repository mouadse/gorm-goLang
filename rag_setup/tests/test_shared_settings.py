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
                "RAG_SPARSE_TOP_K": "7",
                "RAG_SEARCH_HNSW_EF": "77",
                "RAG_EXACT_SEARCH": "true",
                "RAG_HYBRID_FUSION": "relative_score",
                "RAG_HYBRID_RRF_K": "45",
            },
            clear=False,
        ):
            settings = self.shared.get_retrieval_settings()

        self.assertEqual(settings["query_mode"], "default")
        self.assertEqual(settings["similarity_top_k"], 12)
        self.assertEqual(settings["sparse_top_k"], 7)
        self.assertEqual(settings["search_hnsw_ef"], 77)
        self.assertTrue(settings["exact_search"])
        self.assertEqual(settings["hybrid_fusion"], "relative_score")
        self.assertEqual(settings["hybrid_rrf_k"], 45)

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
        settings = self.shared.get_qdrant_collection_settings()

        self.assertEqual(settings["embed_dimension"], 768)
        self.assertEqual(settings["hnsw_m"], 32)
        self.assertTrue(settings["quantization_enabled"])


if __name__ == "__main__":
    unittest.main()
