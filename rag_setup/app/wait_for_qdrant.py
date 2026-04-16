import os
import time

import qdrant_client


def wait_for_qdrant(timeout_seconds=60):
    qdrant_url = os.getenv("QDRANT_URL", "http://localhost:6333")
    client = qdrant_client.QdrantClient(url=qdrant_url, timeout=5)
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


if __name__ == "__main__":
    timeout = int(os.getenv("QDRANT_WAIT_TIMEOUT_SECONDS", "60"))
    wait_for_qdrant(timeout_seconds=timeout)
