import argparse
import hashlib
import json
import os
import time
from pathlib import Path
from llama_index.core import SimpleDirectoryReader, VectorStoreIndex, StorageContext
from shared import (
    setup_settings,
    get_collection_name,
    get_qdrant_collection_settings,
    get_qdrant_payload_indexes,
    get_qdrant_vector_store,
    get_ingest_config,
    build_qdrant_vectors_config,
    build_qdrant_quantization_config,
    build_qdrant_hnsw_config,
    build_qdrant_optimizer_config,
)

MANIFEST_NAME = ".rag_ingest_manifest.json"
MANIFEST_VERSION = 3


def _get_manifest_path(data_dir, collection_name):
    state_dir = os.getenv("RAG_STATE_DIR", "/app/state")
    os.makedirs(state_dir, exist_ok=True)

    normalized_dir = os.path.abspath(data_dir)
    scope = {"collection": collection_name, "data_dir": normalized_dir}
    scope_hash = hashlib.sha256(
        json.dumps(scope, sort_keys=True).encode("utf-8")
    ).hexdigest()[:12]
    return os.path.join(state_dir, f"{collection_name}_{scope_hash}_{MANIFEST_NAME}")


def _sha256_file(file_path):
    hasher = hashlib.sha256()
    with open(file_path, "rb") as source_file:
        for chunk in iter(lambda: source_file.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def _list_pdf_files(data_dir):
    return sorted(
        [
            entry
            for entry in os.listdir(data_dir)
            if entry.lower().endswith(".pdf")
            and os.path.isfile(os.path.join(data_dir, entry))
        ]
    )


def _read_manifest(manifest_path):
    if not os.path.exists(manifest_path):
        return None
    try:
        with open(manifest_path, "r", encoding="utf-8") as manifest_file:
            return json.load(manifest_file)
    except (OSError, json.JSONDecodeError):
        return None


def _write_manifest(manifest_path, manifest_data):
    try:
        with open(manifest_path, "w", encoding="utf-8") as manifest_file:
            json.dump(manifest_data, manifest_file, indent=2)
    except OSError as exc:
        print(f"Warning: failed to write ingest manifest: {exc}")


def _build_manifest(data_dir, pdf_files, ingest_config):
    file_entries = []
    for file_name in pdf_files:
        file_path = os.path.join(data_dir, file_name)
        stats = os.stat(file_path)
        file_entries.append(
            {
                "name": file_name,
                "size_bytes": stats.st_size,
                "mtime_ns": stats.st_mtime_ns,
                "sha256": _sha256_file(file_path),
            }
        )

    manifest_body = {
        "files": file_entries,
        "ingest_config": ingest_config,
    }
    fingerprint_input = json.dumps(manifest_body, sort_keys=True).encode("utf-8")
    return {
        "manifest_version": MANIFEST_VERSION,
        "fingerprint": hashlib.sha256(fingerprint_input).hexdigest(),
        "files": file_entries,
        "ingest_config": ingest_config,
    }


def _get_collection_points(client, collection_name):
    if not client.collection_exists(collection_name):
        return 0
    return client.count(collection_name=collection_name, exact=True).count


def _wait_for_qdrant_ready(client, timeout_seconds=60):
    deadline = time.monotonic() + timeout_seconds
    last_error = None

    while time.monotonic() < deadline:
        try:
            client.get_collections()
            return
        except Exception as exc:
            last_error = exc
            time.sleep(1)

    raise RuntimeError(f"Qdrant did not become ready in {timeout_seconds}s: {last_error}")


def _manifest_matches_current_state(previous_manifest, current_manifest, point_count):
    if previous_manifest.get("fingerprint") != current_manifest["fingerprint"]:
        return False

    previous_point_count = previous_manifest.get("point_count")
    if not isinstance(previous_point_count, int) or previous_point_count <= 0:
        return False
    return previous_point_count == point_count


def _append_unique(sequence, value):
    if value not in sequence:
        sequence.append(value)


def _enrich_documents(documents):
    for document in documents:
        metadata = dict(getattr(document, "metadata", {}) or {})

        file_name = metadata.get("file_name")
        if file_name:
            metadata["source_stem"] = Path(file_name).stem

        page_label = metadata.get("page_label")
        if page_label is not None:
            metadata["page_label"] = str(page_label)
            try:
                metadata["page_number"] = int(str(page_label))
            except ValueError:
                metadata.pop("page_number", None)

        document.metadata = metadata

        excluded_embed_keys = list(
            getattr(document, "excluded_embed_metadata_keys", []) or []
        )
        excluded_llm_keys = list(
            getattr(document, "excluded_llm_metadata_keys", []) or []
        )
        for metadata_key in ("source_stem", "page_number"):
            _append_unique(excluded_embed_keys, metadata_key)
            _append_unique(excluded_llm_keys, metadata_key)
        document.excluded_embed_metadata_keys = excluded_embed_keys
        document.excluded_llm_metadata_keys = excluded_llm_keys

    return documents


def _ensure_collection_schema(client, collection_name):
    from qdrant_client import models

    collection_settings = get_qdrant_collection_settings()
    vectors_config = build_qdrant_vectors_config(collection_settings)
    quantization_config = build_qdrant_quantization_config(collection_settings)
    hnsw_config = build_qdrant_hnsw_config(collection_settings)
    optimizer_config = build_qdrant_optimizer_config(collection_settings)
    payload_indexes = get_qdrant_payload_indexes()

    if not client.collection_exists(collection_name):
        client.create_collection(
            collection_name=collection_name,
            vectors_config=vectors_config,
            shard_number=collection_settings["shard_number"],
            replication_factor=collection_settings["replication_factor"],
            write_consistency_factor=collection_settings["write_consistency_factor"],
            on_disk_payload=collection_settings["on_disk_payload"],
            hnsw_config=hnsw_config,
            optimizers_config=optimizer_config,
            quantization_config=quantization_config,
        )
    else:
        client.update_collection(
            collection_name=collection_name,
            hnsw_config=hnsw_config,
            optimizers_config=optimizer_config,
            quantization_config=quantization_config or models.Disabled(),
        )

    for payload_index in payload_indexes:
        try:
            client.create_payload_index(
                collection_name=collection_name,
                field_name=payload_index["field_name"],
                field_schema=payload_index["field_schema"],
            )
        except Exception as exc:
            if "already exists" not in str(exc).lower():
                raise


def ingest_books(data_dir="/books", reset=False):
    """
    Ingest PDFs from the books directory into Qdrant using LlamaIndex.
    """
    print(f"Starting ingestion from {data_dir}...")

    if not os.path.exists(data_dir):
        print(f"No files found in {data_dir}. Please add some PDFs.")
        return

    collection_name = get_collection_name()
    vector_store = get_qdrant_vector_store(collection_name=collection_name)
    client = vector_store.client
    print("Waiting for Qdrant connection...")
    _wait_for_qdrant_ready(client)

    pdf_files = _list_pdf_files(data_dir)
    manifest_path = _get_manifest_path(data_dir=data_dir, collection_name=collection_name)
    point_count = _get_collection_points(client, collection_name)

    if not pdf_files:
        print(f"No PDF files found in {data_dir}.")
        if point_count > 0 and client.collection_exists(collection_name):
            print(
                f"Clearing collection '{collection_name}' to avoid serving stale vectors."
            )
            client.delete_collection(collection_name)
        if os.path.exists(manifest_path):
            try:
                os.remove(manifest_path)
            except OSError as exc:
                print(f"Warning: failed to remove ingest manifest: {exc}")
        return

    previous_manifest = _read_manifest(manifest_path)
    ingest_config = get_ingest_config()
    current_manifest = _build_manifest(data_dir, pdf_files, ingest_config=ingest_config)

    if not reset and point_count > 0:
        if previous_manifest is None:
            print("Collection has vectors but manifest is missing, rebuilding collection.")
            reset = True

        elif _manifest_matches_current_state(
            previous_manifest, current_manifest, point_count
        ):
            print(
                "PDF files are unchanged and vectors already exist; "
                "skipping ingestion."
            )
            return

        else:
            print(
                "Detected source/config/vector-count mismatch, rebuilding collection..."
            )
            reset = True

    if reset and client.collection_exists(collection_name):
        print(f"Resetting existing collection '{collection_name}'...")
        client.delete_collection(collection_name)
        _wait_for_qdrant_ready(client)

    # 1. Setup global settings (LLM, Embeddings)
    setup_settings()
    _ensure_collection_schema(client, collection_name)
    vector_store = get_qdrant_vector_store(
        collection_name=collection_name,
        client=client,
    )

    # 2. Load documents
    print("Reading PDF documents...")
    reader = SimpleDirectoryReader(input_dir=data_dir, required_exts=[".pdf"])
    documents = reader.load_data()
    documents = _enrich_documents(documents)
    print(f"Loaded {len(documents)} document chunks from PDFs.")

    # 3. Setup Qdrant vector store
    print("Connecting to Vector DB...")
    storage_context = StorageContext.from_defaults(vector_store=vector_store)

    # 4. Build index
    print("Building index and generating embeddings (this may take a while)...")
    VectorStoreIndex.from_documents(
        documents, storage_context=storage_context, show_progress=True
    )

    current_manifest["point_count"] = _get_collection_points(client, collection_name)
    _write_manifest(manifest_path, current_manifest)
    print("Ingestion complete!")

    try:
        import requests
        api_url = os.getenv("API_URL", "http://api:8000")
        if api_url.endswith("/query"):
            api_url = api_url[:-6]
        requests.post(f"{api_url}/cache/clear", timeout=2)
        print("Query cache cleared.")
    except Exception as exc:
        print(f"Failed to clear query cache: {exc}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Ingest PDF files into Qdrant for the RAG system."
    )
    parser.add_argument(
        "--data-dir", default="/books", help="Directory containing source PDF files."
    )
    parser.add_argument(
        "--reset",
        action="store_true",
        help="Delete existing vectors before re-ingesting all PDFs.",
    )
    args = parser.parse_args()
    ingest_books(data_dir=args.data_dir, reset=args.reset)
