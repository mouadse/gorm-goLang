import math
import sys
import types
import unittest
from unittest.mock import patch


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


LOWER_BODY_PRIMARY_MUSCLES = {"quadriceps", "hamstrings", "calves", "glutes", "lower back"}
UPPER_BODY_PRIMARY_MUSCLES = {"chest", "shoulders", "triceps", "lats", "middle back", "biceps", "neck"}


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


class ProgramFilterTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.items = app.load_exercise_catalog()
        app.embedding_model = FakeEmbeddingModel()
        app.catalog = cls.items
        app.catalog_by_exercise_id = {item["exercise_id"]: item for item in cls.items}
        app.catalog_meta = app.build_catalog_meta(cls.items)
        for item in cls.items:
            item["_embedding"] = fake_embed_text(app.build_exercise_text(item))

    def fake_vector_search(self, query: str, *, top_k: int, level=None, equipment=None, category=None, muscles=None, extra_context=None):
        query_embedding = app.build_query_embedding(query, extra_context)
        hits = []
        for item in self.items:
            if level and item["level"] != level:
                continue
            if equipment and item["equipment"] != equipment:
                continue
            if category and item["category"] != category:
                continue
            if muscles and not muscles.intersection(
                set(item["primary_muscles"]) | set(item["secondary_muscles"])
            ):
                continue
            raw_score = sum(left * right for left, right in zip(query_embedding, item["_embedding"]))
            hits.append((item["_catalog_index"], raw_score))

        hits.sort(key=lambda hit: hit[1], reverse=True)
        return hits[:top_k]

    def build_program_offline(self, request: app.ProgramRequest) -> app.ProgramResponse:
        with patch("app.vector_search", new=self.fake_vector_search):
            return app.build_program(request)

    def ranked_program_candidates(
        self, request: app.ProgramRequest, blueprint_index: int
    ) -> list[tuple[float, dict[str, object]]]:
        blueprint = app.program_blueprints(request)[blueprint_index]
        query = f"{blueprint['query']} {request.notes}".strip()
        query_terms = set(app.expanded_terms(app.tokenize(query)))
        query_signals = app.analyze_query(query)
        hits = self.fake_vector_search(
            query,
            top_k=48,
            muscles=set(blueprint["emphasis"]) | set(request.focus) if request.focus else None,
            extra_context=list(set(blueprint["emphasis"]) | set(request.focus)),
        )

        ranked = []
        for index, score in hits:
            item = app.catalog[index]
            ranked.append(
                (
                    app.candidate_score(
                        item,
                        app.score_to_match_strength(score),
                        blueprint,
                        request,
                        query_terms,
                        query_signals,
                    ),
                    item,
                )
            )

        ranked.sort(key=lambda pair: pair[0], reverse=True)
        return ranked

    def test_lower_body_focus_uses_focus_driven_days(self):
        request = app.ProgramRequest(
            goal="build_muscle",
            days_per_week=3,
            session_minutes=45,
            level="beginner",
            equipment_profile="kettlebell-kit",
            focus=["quadriceps", "hamstrings", "calves"],
        )

        blueprints = app.program_blueprints(request)
        self.assertEqual(len(blueprints), 3)
        self.assertTrue(all(blueprint["strict_focus"] for blueprint in blueprints))
        self.assertTrue(
            all(any(muscle.title() in blueprint["title"] for muscle in request.focus) for blueprint in blueprints)
        )

    def test_lower_body_focus_blocks_unrelated_program_exercises(self):
        request = app.ProgramRequest(
            goal="build_muscle",
            days_per_week=3,
            session_minutes=45,
            level="beginner",
            equipment_profile="kettlebell-kit",
            focus=["quadriceps", "hamstrings", "calves"],
        )

        program = self.build_program_offline(request)
        allowed_equipment = app.EQUIPMENT_PROFILES[request.equipment_profile]
        blueprints = app.program_blueprints(request)

        self.assertEqual(len(program.days), 3)
        total_priority_hits = 0
        for day, blueprint in zip(program.days, blueprints):
            self.assertTrue(any(muscle.title() in day.title for muscle in request.focus))
            self.assertGreaterEqual(len(day.exercises), 2)
            total_priority_hits += sum(
                1 for exercise in day.exercises if set(exercise.primary_muscles).intersection(blueprint["priority"])
            )
            for exercise in day.exercises:
                self.assertIn(exercise.equipment, allowed_equipment)
                self.assertTrue(set(exercise.primary_muscles).intersection(LOWER_BODY_PRIMARY_MUSCLES))
                self.assertFalse(set(exercise.primary_muscles).intersection(UPPER_BODY_PRIMARY_MUSCLES))
        self.assertGreaterEqual(total_priority_hits, 4)

    def test_single_focus_can_expand_with_related_support_without_upper_body_leakage(self):
        request = app.ProgramRequest(
            goal="build_muscle",
            days_per_week=2,
            session_minutes=45,
            level="beginner",
            equipment_profile="home-bodyweight",
            focus=["calves"],
        )

        program = self.build_program_offline(request)
        for day in program.days:
            self.assertGreaterEqual(len(day.exercises), 4)
            for exercise in day.exercises:
                self.assertFalse(set(exercise.primary_muscles).intersection(UPPER_BODY_PRIMARY_MUSCLES))
                self.assertTrue(set(exercise.primary_muscles).intersection(LOWER_BODY_PRIMARY_MUSCLES))

    def test_focus_program_does_not_repeat_exercises_across_days(self):
        request = app.ProgramRequest(
            goal="build_muscle",
            days_per_week=3,
            session_minutes=45,
            level="beginner",
            equipment_profile="kettlebell-kit",
            focus=["quadriceps", "hamstrings", "calves"],
        )

        program = self.build_program_offline(request)
        exercise_ids = [
            exercise.exercise_id
            for day in program.days
            for exercise in day.exercises
        ]
        self.assertEqual(len(exercise_ids), len(set(exercise_ids)))

    def test_constrained_focus_backfill_stays_unique_across_days(self):
        request = app.ProgramRequest(
            goal="build_muscle",
            days_per_week=2,
            session_minutes=45,
            level="beginner",
            equipment_profile="home-bodyweight",
            focus=["calves"],
        )

        program = self.build_program_offline(request)
        exercise_ids = [
            exercise.exercise_id
            for day in program.days
            for exercise in day.exercises
        ]
        self.assertEqual(len(exercise_ids), len(set(exercise_ids)))

    def test_program_notes_boost_machine_cardio_candidates(self):
        request = app.ProgramRequest(
            goal="lose_fat",
            days_per_week=2,
            session_minutes=45,
            level="beginner",
            equipment_profile="full-gym",
            focus=[],
            notes="cycling machine rowing treadmill intervals",
        )

        ranked = self.ranked_program_candidates(request, blueprint_index=0)
        top_names = {item["name"] for _, item in ranked[:5]}
        self.assertTrue(
            {
                "Running, Treadmill",
                "Walking, Treadmill",
                "Jogging, Treadmill",
                "Bicycling, Stationary",
                "Recumbent Bike",
                "Rowing, Stationary",
            }.intersection(top_names)
        )


if __name__ == "__main__":
    unittest.main()
