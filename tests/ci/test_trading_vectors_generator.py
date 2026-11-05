import json
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def test_generator_matches_fixture():
    subprocess.run(["make", "build/tools/dump_trading_vectors"], cwd=ROOT, check=True)
    result = subprocess.run(
        [str(ROOT / "build/tools/dump_trading_vectors")],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    generated = json.loads(result.stdout)
    fixture = json.loads(
        (ROOT / "tests/fixtures/trading-payloads/vectors.json").read_text()
    )

    assert generated["source"] == "tests/modules/dump_trading_vectors.c"
    vectors = generated["vectors"]
    expected_vectors = fixture["vectors"]
    assert len(vectors) == len(expected_vectors) == 17
    assert [(v["name"], v["activity_type"]) for v in vectors] == [
        (v["name"], v["activity_type"]) for v in expected_vectors
    ]
    for vector, expected in zip(vectors, expected_vectors):
        assert vector["bytes"] == expected["bytes"], (
            f"Payload bytes differ for vector {vector['name']}"
        )
        assert vector == expected, f"Vector {vector['name']} differs from fixture"
