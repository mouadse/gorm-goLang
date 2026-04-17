import json
import os
import tempfile
import types
import unittest
from pathlib import Path
from unittest.mock import patch

from test_support import load_app_module


class FakeCount:
    def __init__(self, count):
        self.count = count


class FakeClient:
    def __init__(self, initial_points=0, exists=True):
        self._points = initial_points
        self._exists = exists
        self.deleted_collections = []
        self.created_collections = []
        self.updated_collections = []
        self.created_payload_indexes = []

    def get_collections(self):
        return {}

    def collection_exists(self, _collection_name):
        return self._exists

    def count(self, collection_name, exact=True):
        return FakeCount(self._points)

    def delete_collection(self, collection_name):
        self.deleted_collections.append(collection_name)
        self._exists = False
        self._points = 0

    def create_collection(self, **kwargs):
        self.created_collections.append(kwargs)
        self._exists = True

    def update_collection(self, **kwargs):
        self.updated_collections.append(kwargs)

    def create_payload_index(self, **kwargs):
        self.created_payload_indexes.append(kwargs)


class IngestLogicTests(unittest.TestCase):
    def setUp(self):
        self.ingest = load_app_module("ingest")

    def test_empty_books_clears_existing_vectors_and_manifest(self):
        with tempfile.TemporaryDirectory() as data_dir:
            manifest_path = Path(data_dir) / "state" / "manifest.json"
            manifest_path.parent.mkdir(parents=True, exist_ok=True)
            manifest_path.write_text("{}", encoding="utf-8")

            fake_client = FakeClient(initial_points=5, exists=True)
            fake_vector_store = types.SimpleNamespace(client=fake_client)

            with (
                patch.dict(os.environ, {"RAG_STATE_DIR": str(manifest_path.parent)}),
                patch.object(
                    self.ingest, "get_qdrant_vector_store", return_value=fake_vector_store
                ),
                patch.object(self.ingest, "get_collection_name", return_value="books"),
                patch.object(self.ingest, "_wait_for_qdrant_ready"),
                patch.object(self.ingest, "setup_settings"),
                patch.object(self.ingest, "_get_manifest_path", return_value=str(manifest_path)),
                patch.object(self.ingest.VectorStoreIndex, "from_documents") as from_docs,
            ):
                self.ingest.ingest_books(data_dir=data_dir)

            self.assertEqual(fake_client.deleted_collections, ["books"])
            self.assertFalse(manifest_path.exists())
            from_docs.assert_not_called()

    def test_missing_manifest_for_existing_vectors_forces_rebuild(self):
        with tempfile.TemporaryDirectory() as data_dir:
            pdf_path = Path(data_dir) / "sample.pdf"
            pdf_path.write_bytes(b"fake-pdf-content")

            fake_client = FakeClient(initial_points=9, exists=True)
            fake_vector_store = types.SimpleNamespace(client=fake_client)
            manifest_path = Path(data_dir) / "state" / "manifest.json"
            manifest_path.parent.mkdir(parents=True, exist_ok=True)

            with (
                patch.dict(os.environ, {"RAG_STATE_DIR": str(manifest_path.parent)}),
                patch.object(
                    self.ingest, "get_qdrant_vector_store", return_value=fake_vector_store
                ),
                patch.object(self.ingest, "get_collection_name", return_value="books"),
                patch.object(self.ingest, "_wait_for_qdrant_ready"),
                patch.object(self.ingest, "_read_manifest", return_value=None),
                patch.object(self.ingest, "setup_settings"),
                patch.object(self.ingest, "_get_manifest_path", return_value=str(manifest_path)),
                patch.object(
                    self.ingest,
                    "_get_collection_points",
                    side_effect=[9, 21],
                ),
                patch.object(
                    self.ingest.VectorStoreIndex, "from_documents"
                ) as from_documents,
            ):
                self.ingest.ingest_books(data_dir=data_dir)

            self.assertEqual(fake_client.deleted_collections, ["books"])
            self.assertTrue(from_documents.called)

            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            self.assertEqual(manifest["point_count"], 21)

    def test_manifest_fingerprint_changes_when_ingest_config_changes(self):
        with tempfile.TemporaryDirectory() as data_dir:
            pdf_path = Path(data_dir) / "sample.pdf"
            pdf_path.write_bytes(b"fake-pdf-content")

            manifest_a = self.ingest._build_manifest(
                data_dir,
                ["sample.pdf"],
                ingest_config={
                    "embed_model": "model-a",
                    "chunk_size": 512,
                    "chunk_overlap": 64,
                },
            )
            manifest_b = self.ingest._build_manifest(
                data_dir,
                ["sample.pdf"],
                ingest_config={
                    "embed_model": "model-b",
                    "chunk_size": 512,
                    "chunk_overlap": 64,
                },
            )

            self.assertNotEqual(manifest_a["fingerprint"], manifest_b["fingerprint"])

    def test_document_enrichment_adds_source_stem_and_page_number(self):
        document = types.SimpleNamespace(
            metadata={"file_name": "Book.pdf", "page_label": "7"},
            excluded_embed_metadata_keys=[],
            excluded_llm_metadata_keys=[],
        )

        enriched = self.ingest._enrich_documents([document])[0]

        self.assertEqual(enriched.metadata["source_stem"], "Book")
        self.assertEqual(enriched.metadata["page_number"], 7)
        self.assertIn("source_stem", enriched.excluded_embed_metadata_keys)
        self.assertIn("page_number", enriched.excluded_llm_metadata_keys)

    def test_ensure_collection_schema_creates_dense_only_unnamed_vector(self):
        fake_client = FakeClient(initial_points=0, exists=False)

        self.ingest._ensure_collection_schema(fake_client, "books")

        self.assertEqual(len(fake_client.created_collections), 1)
        vectors_config = fake_client.created_collections[0]["vectors_config"]
        self.assertNotIsInstance(vectors_config, dict)
        self.assertEqual(vectors_config.size, 1536)


if __name__ == "__main__":
    unittest.main()
