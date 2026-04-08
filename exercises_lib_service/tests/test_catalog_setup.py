import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest.mock import patch

import numpy as np


if "pymilvus" not in sys.modules:
    pymilvus = types.ModuleType("pymilvus")

    class DummyDataType:
        VARCHAR = "VARCHAR"
        INT64 = "INT64"
        FLOAT_VECTOR = "FLOAT_VECTOR"

    class DummyMilvusClient:
        pass

    pymilvus.DataType = DummyDataType
    pymilvus.MilvusClient = DummyMilvusClient
    sys.modules["pymilvus"] = pymilvus

import app


class FakeEmbeddingModel:
    def passage_embed(self, texts, batch_size=None, parallel=None):
        return [np.full(app.VECTOR_DIM, index + 1, dtype=np.float32) for index, _ in enumerate(texts)]

    def query_embed(self, texts):
        return [np.full(app.VECTOR_DIM, 0.5, dtype=np.float32) for _ in texts]


class CatalogSetupTests(unittest.TestCase):
    def setUp(self):
        self.original_embedding_model = app.embedding_model
        self.original_catalog = app.catalog
        self.original_catalog_by_id = app.catalog_by_exercise_id
        self.original_catalog_embeddings = app.catalog_embeddings
        self.original_catalog_meta = app.catalog_meta

    def tearDown(self):
        app.embedding_model = self.original_embedding_model
        app.catalog = self.original_catalog
        app.catalog_by_exercise_id = self.original_catalog_by_id
        app.catalog_embeddings = self.original_catalog_embeddings
        app.catalog_meta = self.original_catalog_meta

    def test_build_query_embedding_initializes_model_lazily(self):
        app.embedding_model = None
        with patch("app.TextEmbedding", return_value=FakeEmbeddingModel()) as model_factory:
            embedding = app.build_query_embedding("legs only")

        self.assertEqual(len(embedding), app.VECTOR_DIM)
        model_factory.assert_called_once_with(
            model_name=app.EMBEDDING_MODEL_NAME,
            threads=app.EMBEDDING_THREADS,
        )

    def test_initialize_catalog_reuses_cached_embeddings(self):
        items = app.load_exercise_catalog()[:3]
        cached_embeddings = np.vstack(
            [
                np.full(app.VECTOR_DIM, 0.1, dtype=np.float32),
                np.full(app.VECTOR_DIM, 0.2, dtype=np.float32),
                np.full(app.VECTOR_DIM, 0.3, dtype=np.float32),
            ]
        )

        with tempfile.TemporaryDirectory() as temp_dir:
            cache_path = Path(temp_dir) / "catalog-cache.npz"
            with patch("app.catalog_cache_path", return_value=cache_path):
                app.save_cached_catalog_embeddings(items, cached_embeddings)
                app.embedding_model = None
                with patch("app.load_exercise_catalog", return_value=items):
                    with patch(
                        "app.TextEmbedding",
                        side_effect=AssertionError("cache hit should not build the embedding model"),
                    ):
                        total = app.initialize_catalog()

        self.assertEqual(total, len(items))
        self.assertTrue(np.array_equal(app.catalog_embeddings, cached_embeddings))
        self.assertEqual(set(app.catalog_by_exercise_id), {item["exercise_id"] for item in items})


if __name__ == "__main__":
    unittest.main()
