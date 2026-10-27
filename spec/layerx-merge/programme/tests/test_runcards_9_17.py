from collections import Counter
from pathlib import Path

from programme.kvx import load
from programme.runcards import check_card, load_card

ROOT = Path(__file__).resolve().parents[4]
CARDS = ROOT / "spec/layerx-merge/programme/data/runcards"
DOMAINS = {
    9: ("agent", "rust"),
    10: ("human", "rust"),
    11: ("interop", "rust"),
    12: ("wallet", "mixed"),
    13: ("programs", "rust"),
    14: ("web-explorer-bridge", "node"),
    15: ("cosmos-fork", "go"),
    16: ("tools-ops", "mixed"),
    17: ("programme", "mixed"),
}


def test_cards_9_to_17_pass_schema() -> None:
    schema = load_card(CARDS / "schema.kvx")
    spec = load(ROOT / "spec/layerx-merge/spec.kvx")
    workflow = load(ROOT / "spec/workflow.kvx")

    assert set(DOMAINS) == set(range(9, 18))
    for domain, (slug, profile) in DOMAINS.items():
        card = load_card(CARDS / f"fm-{slug}.kvx")
        assert check_card(card, schema) is None, slug
        assert spec[f"feature.{domain}"]["name"] == slug
        assert card["role"] == f"fm-{slug}"
        assert card["domain"] == domain
        assert card["sandbox_profile"] == profile

        anchors = [f"{domain}.0.{suffix}" for suffix in (1, 2, 3, 4, 9)]
        assert card["anchors"] == anchors, slug
        assert all(f"task.{anchor}" in spec for anchor in anchors), slug

        waves = Counter(
            fields["wave"]
            for name, fields in spec.items()
            if name.startswith(f"task.{domain}.")
        )
        assert waves, slug
        assert card["wave_gate"] == [
            f"scripts/gates/wave-{wave}.sh" for wave in sorted(waves)
        ], slug
        assert card["max_parallel"] == max(waves.values()), slug
        assert card["escalation"] == "stop the wave and file a qualification finding"
        assert card["limits"] == schema["limits"], slug
        for key in ("max_task_minutes", "max_gate_runs_per_task"):
            assert f"{key} = {workflow['limits'][key]}" in card["limits"], slug
        assert card["no_false_success"] == workflow["principle"]["no_false_success"]
