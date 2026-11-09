import json
from pathlib import Path
import re
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
    loops = re.findall(
        r"^for service in ([^;\n]+); do\n(.*?)^done$",
        source, flags=re.M | re.S,
    )
    workspace_loops = [(shlex.split(services), body) for services, body in loops
                       if 'run_leg ' in body]
    assert len(workspace_loops) == 1
    services, body = workspace_loops[0]
    assert services == ["libs", "smart-contract-verifier", "sig-provider"]
    workspace_legs = [shlex.split(line, comments=True)
                      for line in body.splitlines() if line.strip()]
    assert len(workspace_legs) == 1
    leg = workspace_legs[0]
    assert leg[:4] == [
        "run_leg", "$service", "$service workspace tests",
        "$repo_root/services/explorer/services/$service",
    ]
    assert leg[4:] == [
        "cargo",
        "test",
        "--locked",
        "--workspace",
    ]
    frontend = next(item for item in legs if item[1] == "frontend-vitest")
    assert legs[-1] == frontend
    assert re.search(
        r"^run_leg frontend-vitest[^\n]*\nfor service in "
        r"libs smart-contract-verifier sig-provider; do\n", source, flags=re.M,
    )
    for service in services:
        manifest = ROOT / "services/explorer/services" / service / "Cargo.toml"
        assert manifest.is_file(), manifest
