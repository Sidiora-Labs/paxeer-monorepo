from pathlib import Path

import pytest

from programme.kvx import load
from programme.runcards import CardError, check_card, load_card

ROOT = Path(__file__).resolve().parents[4]
SCHEMA = ROOT / "spec/layerx-merge/programme/data/runcards/schema.kvx"


def test_schema_lists_required_keys() -> None:
    sections = load(SCHEMA)
    assert set(sections) == {"card"}
    schema = load_card(SCHEMA)
    assert schema == sections["card"]
    assert schema["required"] == [
        "role",
        "domain",
        "sandbox_profile",
        "anchors",
        "wave_gate",
        "max_parallel",
        "escalation",
        "limits",
    ]
    assert set(schema) == {"required", *schema["required"]}
    workflow = load(ROOT / "spec/workflow.kvx")
    assert workflow["limits"]["max_task_minutes"] == 30
    assert workflow["limits"]["max_gate_runs_per_task"] == 1
    assert schema["limits"] == [
        "max_task_minutes = 30",
        "max_gate_runs_per_task = 1",
        "source = spec/workflow.kvx:32-36",
    ]


def test_check_card_refuses_wrong_role(tmp_path: Path) -> None:
    path = tmp_path / "fm-markets.kvx"
    path.write_text(
        '[card]\nrole = "fm-markets"\ndomain = 3\nsandbox_profile = "go"\n'
        'anchors = ["3.0.1", "3.0.2", "3.0.3", "3.0.4", "3.0.9"]\n'
        'wave_gate = "scripts/gates/wave-0.sh"\nmax_parallel = 1\n'
        'escalation = "stop the wave and file a qualification finding"\n'
        'limits = ["max_task_minutes = 30", "max_gate_runs_per_task = 1"]\n',
        encoding="utf-8",
    )
    schema = load_card(SCHEMA)
    card = load_card(path)
    assert card["domain"] == 3
    assert card["role"] == "fm-markets"
    assert check_card(card, schema) is None
    with pytest.raises(CardError, match="role.*domain 3 requires fm-markets"):
        check_card({**card, "role": "fm-kernel"}, schema)

    for key in schema["required"]:
        incomplete = {name: value for name, value in card.items() if name != key}
        with pytest.raises(CardError, match=f"missing required card keys: {key}"):
            check_card(incomplete, schema)

    domains = {
        name.removeprefix("feature."): fields["name"]
        for name, fields in load(ROOT / "spec/layerx-merge/spec.kvx").items()
        if name.startswith("feature.")
    }
    assert set(domains) == {str(number) for number in range(1, 18)}
    for domain, slug in domains.items():
        valid = {**card, "domain": int(domain), "role": f"fm-{slug}"}
        assert check_card(valid, schema) is None
        assert check_card({**valid, "domain": domain}, schema) is None
        wrong_role = "fm-markets" if domain != "3" else "fm-kernel"
        with pytest.raises(CardError, match="role"):
            check_card({**valid, "role": wrong_role}, schema)

    for domain in (0, 18, True, "unknown", "03", ["3"]):
        with pytest.raises(CardError, match="domain"):
            check_card({**card, "domain": domain}, schema)

    path.write_text('[meta]\nrole = "fm-markets"\n', encoding="utf-8")
    with pytest.raises(CardError, match=r"missing \[card\] section"):
        load_card(path)
    path.write_text(
        '[card]\nrole = "fm-markets"\nrole = "fm-kernel"\n', encoding="utf-8"
    )
    with pytest.raises(CardError, match="duplicate key role"):
        load_card(path)
