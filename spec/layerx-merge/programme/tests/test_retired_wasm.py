from pathlib import Path

from programme.kvx import load
from programme.retired import load_components, owners

ROOT = Path(__file__).resolve().parents[4]


def test_requires_sei_component_names_callers() -> None:
    component = load_components()["requires-sei-wasm"]
    assert component["req"] == "17029"
    assert component["symbols"] == ["saturn.wasm", "venus.wasm"]
    assert type(component["owner_domain"]) is int
    assert component["owner_domain"] == 16
    assert component["refusal"] == "not deployable after the merge"
    assert component["deleted_trees"] == []
    assert component["callers"] == [
        "tests/integration/contracts/deploy_wasm_contracts.sh:36",
        "tests/integration/contracts/deploy_wasm_contracts.sh:50",
        "tools/benchmark/loadtest/contracts/deploy_ten_contracts.sh:34-35",
    ]
    assert component["wiring"] == ["wiring.J60.10"]
    for term in ("requires_sei", "cw20_base.wasm", "16.38.1", "mars.wasm"):
        assert term in component["replacement"], term

    caller_paths = {location.rsplit(":", 1)[0] for location in component["callers"]}
    assert owners(caller_paths) == {path: 16 for path in caller_paths}
    for path in caller_paths:
        assert (ROOT / path).is_file(), path

    specification = load(ROOT / "spec/layerx-merge/spec.kvx")
    assert specification["task.17.2.4"]["wiring"] == component["wiring"]
    request = specification[component["wiring"][0]]
    assert request["requester"] == "17.2.4"
    assert request["domain"] == component["owner_domain"]
    assert request["path"] == "tests/integration/contracts/deploy_wasm_contracts.sh"
    for path in caller_paths:
        assert path in request["what"], path
    for term in ("saturn.wasm", "venus.wasm", "cw20_base.wasm", "16.38.1"):
        assert term in request["what"], term
