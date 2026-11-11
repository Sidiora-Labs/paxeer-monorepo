import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def canonical_type(parameter):
    kind = parameter["type"]
    if not kind.startswith("tuple"):
        return kind
    components = ",".join(canonical_type(component) for component in parameter["components"])
    return f"({components}){kind[5:]}"


def test_write_tables_carry_the_chain_launch():
    abi = json.loads((ROOT / "precompiles/launchpad/abi.json").read_text())
    typescript = (
        ROOT / "services/agent/sdk/typescript/src/generated/precompile-writes.ts"
    ).read_text()
    python = (
        ROOT / "services/agent/sdk/python/layerx_sdk/generated/precompile_writes.py"
    ).read_text()
    explorer = json.loads(
        (
            ROOT
            / "services/explorer/services/sig-provider/sig-provider/src/sources/precompiles.json"
        ).read_text()
    )["abi"]

    typescript_writes = typescript.split("export const LAUNCHPAD_WRITES = {", 1)[1].split(
        "export const LAUNCHPAD_EVENTS = [", 1
    )[0]
    typescript_events = typescript.split("export const LAUNCHPAD_EVENTS = [", 1)[1]
    python_writes = python.split("LAUNCHPAD_WRITES: dict[str, bool] = {", 1)[1].split(
        "LAUNCHPAD_EVENT_INPUTS:", 1
    )[0]
    python_events = python.split("LAUNCHPAD_EVENT_INPUTS:", 1)[1]

    writes = [
        entry
        for entry in abi
        if entry["type"] == "function"
        and entry["stateMutability"] not in ("view", "pure")
    ]
    events = [entry for entry in abi if entry["type"] == "event"]
    assert writes
    assert events

    for entry in writes:
        signature = f"{entry['name']}({','.join(canonical_type(arg) for arg in entry['inputs'])})"
        assert json.dumps(signature) in typescript_writes, signature
        assert json.dumps(signature) in python_writes, signature

    for entry in events:
        name = entry["name"]
        assert f'"name":"{name}"' in typescript_events, name
        assert f'("{name}", (' in python_events, name

    for entry in abi:
        if entry["type"] in ("function", "event", "error"):
            assert entry in explorer, (entry["type"], entry["name"])
