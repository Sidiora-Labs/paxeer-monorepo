import json
from pathlib import Path
import shlex
import subprocess


ROOT = Path(__file__).resolve().parents[3]


def test_catalogue_lists_signature_route():
    catalogue = json.loads(
        (ROOT / "tools/paxeer-x/route-catalogue.json").read_text(encoding="utf-8")
    )
    routes = catalogue["routes"]
    transaction_path = "/api/v2/transactions/{transaction_hash_param}"
    status_id = f"explorer-backend:GET:{transaction_path}/status"
    signature_id = f"explorer-backend:GET:{transaction_path}/signature"
    status_indices = [i for i, route in enumerate(routes) if route["id"] == status_id]
    signature_indices = [
        i for i, route in enumerate(routes) if route["id"] == signature_id
    ]
    assert len(status_indices) == 1
    assert signature_indices == [status_indices[0] + 1]
    signature = routes[signature_indices[0]]
    assert signature["method"] == "GET"
    assert signature["path"] == f"/explorer/backend{transaction_path}/signature"
    assert signature["upstream_path"] == f"{transaction_path}/signature"
    expected = dict(routes[status_indices[0]])
    expected.update(
        id=signature_id,
        path=f"/explorer/backend{transaction_path}/signature",
        upstream_path=f"{transaction_path}/signature",
    )
    assert signature == expected


def test_explorer_gate_runs_sig_provider():
    script = ROOT / "scripts/explorer/gate-test.sh"
    result = subprocess.run(
        ["bash", "-n", str(script)],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    source = script.read_text(encoding="utf-8").replace("\\\n", "")
    legs = [
        shlex.split(line, comments=True)
        for line in source.splitlines()
        if line.startswith("run_leg ")
    ]
    manifest = "services/explorer/services/sig-provider/Cargo.toml"
    signature_legs = [leg for leg in legs if manifest in leg]
    assert len(signature_legs) == 1
    leg = signature_legs[0]
    assert leg[3] == "$repo_root"
    assert leg[4:] == [
        "cargo",
        "test",
        "--locked",
        "--manifest-path",
        manifest,
        "--workspace",
    ]
    frontend_index = next(i for i, item in enumerate(legs) if item[1] == "frontend-vitest")
    assert legs.index(leg) == frontend_index + 1
