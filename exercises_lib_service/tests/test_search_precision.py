import asyncio
import math
import sys
import types
import unittest

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


LOWER_BODY_MUSCLES = {"quadriceps", "hamstrings", "glutes", "calves", "adductors", "abductors"}


def fake_embed_text(text: str) -> list[float]:
    vector = [0.0] * app.VECTOR_DIM
    for token in app.tokenize(text):
        slot = sum(ord(char) for char in token) % app.VECTOR_DIM
        vector[slot] += 1.0
    magnitude = math.sqrt(sum(value * value for value in vector)) or 1.0
    return [value / magnitude for value in vector]


class FakeEmbeddingModel:
    def passage_embed(self, texts, batch_size=None, parallel=None):
        return [fake_embed_text(text) for text in texts]

    def query_embed(self, texts):
        return [fake_embed_text(text) for text in texts]


class SearchPrecisionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.items = app.load_exercise_catalog()
        app.embedding_model = FakeEmbeddingModel()
        app.catalog = cls.items
        app.catalog_by_exercise_id = {
            item["exercise_id"]: item for item in cls.items
        }
        app.catalog_meta = app.build_catalog_meta(cls.items)
        for item in cls.items:
            item["_embedding"] = fake_embed_text(app.build_exercise_text(item))
        app.catalog_embeddings = np.array(
            [item["_embedding"] for item in cls.items], dtype=np.float32
        )

    def ranked_results(self, query: str, top_k: int = 8) -> list[dict[str, object]]:
        request = app.SearchRequest(query=query, top_k=top_k)
        query_signals = app.analyze_query(query)
        query_embedding = app.build_query_embedding(query, app.search_context_terms(request, query_signals))

        scored = []
        for item in self.items:
            if not app.matches_search_intent(item, request, query_signals):
                continue
            raw_score = sum(left * right for left, right in zip(query_embedding, item["_embedding"]))
            final_score = app.search_result_score(item, app.score_to_match_strength(raw_score), request, query_signals)
            scored.append((final_score, item))

        scored.sort(key=lambda pair: pair[0], reverse=True)
        return [item for _, item in scored[:top_k]]

    def test_leg_workout_only_requires_lower_body_primary(self):
        results = self.ranked_results("leg workout only")
        self.assertTrue(results)
        for item in results:
            self.assertTrue(LOWER_BODY_MUSCLES.intersection(item["primary_muscles"]))

    def test_lower_body_only_requires_lower_body_primary(self):
        results = self.ranked_results("lower body only")
        self.assertTrue(results)
        for item in results:
            self.assertTrue(LOWER_BODY_MUSCLES.intersection(item["primary_muscles"]))

    def test_legs_only_no_chest_excludes_chest_hits(self):
        results = self.ranked_results("legs only no chest")
        self.assertTrue(results)
        for item in results:
            all_muscles = set(item["primary_muscles"]) | set(item["secondary_muscles"])
            self.assertNotIn("chest", all_muscles)

    def test_chest_only_prefers_training_movements(self):
        results = self.ranked_results("chest only")
        self.assertTrue(results)
        self.assertNotEqual(results[0]["category"], "stretching")
        for item in results[:5]:
            self.assertIn("chest", item["primary_muscles"])

    def test_cycling_machine_surfaces_bike_movements(self):
        results = self.ranked_results("cycling machine")
        self.assertTrue(results)
        top_names = {item["name"] for item in results[:3]}
        self.assertTrue(
            {"Bicycling, Stationary", "Recumbent Bike"}.intersection(top_names)
        )

    def test_rowing_machine_surfaces_stationary_rowing(self):
        results = self.ranked_results("rowing machine")
        self.assertTrue(results)
        top_names = {item["name"] for item in results[:3]}
        self.assertIn("Rowing, Stationary", top_names)

    def test_running_machine_prefers_treadmill_results(self):
        results = self.ranked_results("running machine")
        self.assertTrue(results)
        top_names = {item["name"] for item in results[:3]}
        self.assertTrue(
            {"Running, Treadmill", "Walking, Treadmill", "Jogging, Treadmill"}.intersection(top_names)
        )

    def test_foam_rolling_finds_smr_and_recovery_results(self):
        results = self.ranked_results("foam rolling recovery")
        self.assertTrue(results)
        top_names = {item["name"] for item in results[:5]}
        self.assertTrue(any(name.endswith("-SMR") for name in top_names))

    def test_search_response_scores_follow_reranked_order(self):
        response = asyncio.run(
            app.search_exercises(app.SearchRequest(query="chest only", top_k=5))
        )
        self.assertGreater(len(response.results), 1)
        scores = [result.score for result in response.results]
        self.assertEqual(scores, sorted(scores, reverse=True))


if __name__ == "__main__":
    unittest.main()
