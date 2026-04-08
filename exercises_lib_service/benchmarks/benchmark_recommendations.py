import asyncio
import gc
import sys
import time
from pathlib import Path
from statistics import median

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import app


SEARCH_REQUESTS = [
    app.SearchRequest(query="chest only", top_k=5),
    app.SearchRequest(query="lower body only", top_k=6, level="beginner"),
    app.SearchRequest(query="cycling machine", top_k=5),
    app.SearchRequest(query="foam rolling recovery", top_k=5),
    app.SearchRequest(
        query="push workout for chest and shoulders with dumbbells",
        top_k=6,
        level="beginner",
    ),
    app.SearchRequest(
        query="core work with bodyweight only",
        top_k=6,
        level="beginner",
    ),
]

PROGRAM_REQUESTS = [
    app.ProgramRequest(
        goal="build_muscle",
        days_per_week=4,
        session_minutes=45,
        level="beginner",
        equipment_profile="home-bodyweight",
        focus=["shoulders", "abdominals"],
        notes="keep it joint-friendly",
    ),
    app.ProgramRequest(
        goal="lose_fat",
        days_per_week=2,
        session_minutes=45,
        level="beginner",
        equipment_profile="full-gym",
        focus=[],
        notes="cycling machine rowing treadmill intervals",
    ),
    app.ProgramRequest(
        goal="build_muscle",
        days_per_week=3,
        session_minutes=45,
        level="beginner",
        equipment_profile="kettlebell-kit",
        focus=["quadriceps", "hamstrings", "calves"],
        notes="",
    ),
    app.ProgramRequest(
        goal="move_better",
        days_per_week=4,
        session_minutes=40,
        level="beginner",
        equipment_profile="mobility-reset",
        focus=["hamstrings", "lower back"],
        notes="posture and mobility",
    ),
]


class FakeEmbeddingModel:
    def passage_embed(self, texts, batch_size=None, parallel=None):
        return [fake_embed_text(text) for text in texts]

    def query_embed(self, texts):
        return [fake_embed_text(text) for text in texts]


def fake_embed_text(text: str) -> list[float]:
    vector = np.zeros(app.VECTOR_DIM, dtype=np.float32)
    for token in app.tokenize(text):
        slot = sum(ord(char) for char in token) % app.VECTOR_DIM
        vector[slot] += 1.0
    magnitude = float(np.linalg.norm(vector)) or 1.0
    return (vector / magnitude).tolist()


def initialize_fake_catalog() -> None:
    items = app.load_exercise_catalog()
    app.embedding_model = FakeEmbeddingModel()
    app.catalog = items
    app.catalog_by_exercise_id = {item["exercise_id"]: item for item in items}
    app.catalog_meta = app.build_catalog_meta(items)
    app.catalog_embeddings = np.array(
        [fake_embed_text(app.build_exercise_text(item)) for item in items],
        dtype=np.float32,
    )


def run_search_batch() -> None:
    for request in SEARCH_REQUESTS:
        response = asyncio.run(app.search_exercises(request))
        if not response.results:
            raise RuntimeError(f"search produced no results for {request.query!r}")



def run_program_batch() -> None:
    for request in PROGRAM_REQUESTS:
        response = asyncio.run(app.recommend_program(request))
        if not response.days:
            raise RuntimeError(f"program produced no days for {request.goal!r}")



def benchmark(label: str, fn, loops: int) -> tuple[float, float, float]:
    samples_ms: list[float] = []
    for _ in range(loops):
        started = time.perf_counter_ns()
        fn()
        ended = time.perf_counter_ns()
        samples_ms.append((ended - started) / 1_000_000)
    return sum(samples_ms), median(samples_ms), min(samples_ms)


if __name__ == "__main__":
    initialize_fake_catalog()
    run_search_batch()
    run_program_batch()

    gc.disable()
    search_total_ms, search_median_ms, search_min_ms = benchmark("search", run_search_batch, loops=40)
    program_total_ms, program_median_ms, program_min_ms = benchmark("program", run_program_batch, loops=25)
    gc.enable()

    total_ms = search_total_ms + program_total_ms
    print(f"METRIC total_ms={total_ms:.3f}")
    print(f"METRIC search_total_ms={search_total_ms:.3f}")
    print(f"METRIC program_total_ms={program_total_ms:.3f}")
    print(f"METRIC search_median_batch_ms={search_median_ms:.3f}")
    print(f"METRIC program_median_batch_ms={program_median_ms:.3f}")
    print(f"METRIC search_min_batch_ms={search_min_ms:.3f}")
    print(f"METRIC program_min_batch_ms={program_min_ms:.3f}")
