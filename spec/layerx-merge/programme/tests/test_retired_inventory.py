import json
from pathlib import Path

from programme.retired import diff_against_json, load_components

ROOT = Path(__file__).resolve().parents[4]
RETIRED_CALLERS = ROOT / "tools/paxeer-x/retired_callers.json"


def test_inventory_matches_retired_callers_json(tmp_path: Path) -> None:
    assert diff_against_json(RETIRED_CALLERS) == (set(), set())

    source = json.loads(RETIRED_CALLERS.read_text(encoding="utf-8"))
    components = load_components()
    assert len(components) == len(source["components"]) == 47
    for name, component in components.items():
        original = source["components"][name]
        assert component["req"] == original["req"], name
        assert component["symbols"] == original["symbols"], name
        assert component["deleted_trees"] == original["deleted_trees"], name
        assert component["replacement"] == original["replacement"], name
        assert isinstance(component["refusal"], str), name
        assert component["refusal"], name
        if original["deleted_trees"]:
            assert component["refusal"] == "deleted", name

    assert components["verify"]["refusal"] == "LXP_MODULE_DISABLED"
    assert components["verify"]["address"] == "0x1012"
    assert components["anchor"]["refusal"] == "ErrModuleRetired"
    assert components["anchor"]["address"] == "0x1014"
    assert components["exchange-intents"]["refusal"] == "ErrRetired"
    assert components["exchange-intents"]["refusal_code"] == 14
    assert components["exchange-intents"]["refusal_reason"] == "LXP_EXCHANGE_RETIRED"
    assert components["exchange-intents"]["retired_since"] == "v6.12.0"

    source["components"]["json-only-verify"] = source["components"].pop("verify")
    changed = tmp_path / "retired_callers.json"
    changed.write_text(json.dumps(source), encoding="utf-8")
    assert diff_against_json(changed) == ({"json-only-verify"}, {"verify"})


def test_every_component_names_owner_domain() -> None:
    components = load_components()
    assert components
    for name, component in components.items():
        owner_domain = component["owner_domain"]
        assert type(owner_domain) is int, name
        assert 1 <= owner_domain <= 16, name

    for name, owner_domain in {
        "verify": 6,
        "anchor": 6,
        "exchange-intents": 3,
        "lni": 9,
        "paxeer-verifier": 9,
    }.items():
        assert components[name]["owner_domain"] == owner_domain, name
