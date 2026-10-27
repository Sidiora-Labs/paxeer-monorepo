from collections import Counter
from pathlib import Path

from programme.kvx import load
from programme.runcards import check_card, load_card

ROOT = Path(__file__).resolve().parents[4]
CARDS = ROOT / "spec/layerx-merge/programme/data/runcards"
DOMAINS = (
    ("kernel", "go-cgo"),
    ("ledger-auth-pay", "go-cgo"),
    ("markets", "go"),
    ("launchpad", "mixed"),
    ("node-runtime", "go-cgo"),
    ("chain-modules", "go"),
    ("rpc-api", "go"),
    ("platform", "rust"),
)


def test_cards_1_to_8_pass_schema() -> None:
    schema = load_card(CARDS / "schema.kvx")
    spec = load(ROOT / "spec/layerx-merge/spec.kvx")
    workflow = load(ROOT / "spec/workflow.kvx")

    for domain, (slug, profile) in enumerate(DOMAINS, start=1):
        assert spec[f"feature.{domain}"]["name"] == slug
        card = load_card(CARDS / f"fm-{slug}.kvx")
        assert check_card(card, schema) is None
        assert card["role"] == f"fm-{slug}"
        assert type(card["domain"]) is int
        assert card["domain"] == domain
        assert card["sandbox_profile"] == profile

        anchors = [f"{domain}.0.{suffix}" for suffix in (1, 2, 3, 4, 9)]
        assert card["anchors"] == anchors
        assert all(f"task.{anchor}" in spec for anchor in anchors)
        assert spec[f"feature.{domain}"]["completion_task"] == anchors[-1]

        wave_counts = Counter(
            fields["wave"]
            for name, fields in spec.items()
            if name.startswith(f"task.{domain}.")
        )
        assert wave_counts
        assert card["wave_gate"] == [
            f"scripts/gates/wave-{wave}.sh" for wave in sorted(wave_counts)
        ]
        assert type(card["max_parallel"]) is int
        assert card["max_parallel"] == max(wave_counts.values())
        assert card["escalation"] == "stop the wave and file a qualification finding"
        assert card["limits"] == schema["limits"]
        for key in ("max_task_minutes", "max_gate_runs_per_task"):
            assert f"{key} = {workflow['limits'][key]}" in card["limits"]
        assert card["no_false_success"] == workflow["principle"]["no_false_success"]
