import json
import subprocess
from pathlib import Path

import pytest

from programme.retired import callers, owners

ROOT = Path(__file__).resolve().parents[4]


def test_owners_maps_prefixes() -> None:
    expected = {
        "services/agent/x.rs": 9,
        "node/app.go": 5,
        "spec/layerx-merge/spec.kvx": 17,
        "src/ledger/account.c": 1,
        "modules/lxphost/host.go": 1,
        "tests/crypto/hash.c": 1,
        "modules/layerxpay/module.go": 2,
        "precompiles/layerxledger/ledger.go": 2,
        "api/layerxauth/types.go": 2,
        "services/platform/perps-data/src/main.rs": 3,
        "precompiles/spot/spot.go": 3,
        "sdk/launchpad/client.go": 4,
        "tools/launchpad/build.py": 4,
        "contracts/launchpad/Token.sol": 4,
        "go.mod": 5,
        "tests/daemon/start.py": 5,
        "precompiles/setup.go": 6,
        "contracts/src/BatchCallAndSponsor.sol": 6,
        "api/layerxbridge/types.go": 6,
        "rpc/server.go": 7,
        "sdk/proto/messages.proto": 7,
        "api/eth/service.go": 7,
        "api/openapi.yaml": 7,
        "api/README.md": 17,
        "buf.yaml": 7,
        "services/platform/gateway/src/main.rs": 8,
        "tools/platform/build.rs": 8,
        "sdk/platform/client.rs": 8,
        "services/human/src/main.rs": 10,
        "services/interop/src/main.rs": 11,
        "sdk/wallet/client.ts": 12,
        "programs/runtime/src/lib.rs": 13,
        "precompiles/layerxprograms/programs.go": 13,
        "sdk/js/client.ts": 14,
        "tests/explorer-api/check.py": 14,
        "sdk/baseapp/abci.go": 15,
        "tools/paxeer-x/tests/retired_callers_test.py": 16,
        "tests/codec-extra/check.py": 16,
        "Makefile": 16,
        "foundry.local.toml": 16,
        "AGENTS.md": 16,
        "README.md": 17,
        "GOTCHA.kvx": 17,
    }
    assert owners(expected) == expected
    assert owners(f"{path}:42" for path in expected) == expected
    assert owners(["node/app.go:1", "node/app.go:2"]) == {"node/app.go": 5}
    assert owners([]) == {}
    for path in (
        "services/agent-other/x.rs",
        "node-other/app.go",
        "unknown/file.go",
        "../node/app.go",
        "/node/app.go",
        "node/../app.go",
    ):
        with pytest.raises(ValueError):
            owners([path])


def test_callers_runs_the_scan_for_one_component() -> None:
    found = callers("verify", ROOT)
    assert found == []
    catalogue = json.loads(
        (ROOT / "tools/paxeer-x/retired_callers.json").read_text(encoding="utf-8")
    )
    symbols = catalogue["components"]["verify"]["symbols"]
    node_allowances = [
        entry
        for entry in catalogue["allowed_references"]
        if entry["path"] == "node"
    ]
    assert len(node_allowances) == 1
    assert "layerxverify" in node_allowances[0]["symbols"]
    upgrade_source = (ROOT / "node/upgrades.go").read_text(
        encoding="utf-8"
    ).splitlines()
    imports = {
        f"node/upgrades.go:{number}"
        for number, line in enumerate(upgrade_source, 1)
        if line.strip().endswith(
            '"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"'
        )
    }
    assert imports
    for location in imports:
        _, number = location.rsplit(":", 1)
        line = upgrade_source[int(number) - 1]
        assert any(symbol in line for symbol in symbols), location
    assert imports.isdisjoint(found)
    registration_source = (ROOT / "precompiles/setup.go").read_text(
        encoding="utf-8"
    ).splitlines()
    allowed_imports = {
        f"precompiles/setup.go:{number}"
        for number, line in enumerate(registration_source, 1)
        if line.strip()
        == '"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"'
    }
    assert allowed_imports
    assert allowed_imports.isdisjoint(found)
    assert owners(imports) == {"node/upgrades.go": 5}
    with pytest.raises(subprocess.CalledProcessError) as refusal:
        callers("unknown-retired-component", ROOT)
    assert refusal.value.returncode == 2
    assert refusal.value.cmd == [
        "python3",
        "tools/paxeer-x/tests/retired_callers_test.py",
        "--component",
        "unknown-retired-component",
    ]
    assert "retired-callers: refusal: unknown component" in refusal.value.stderr


def test_callers_reports_locations_in_a_temporary_tracked_tree(tmp_path: Path) -> None:
    catalogue_path = "tools/paxeer-x/retired_callers.json"
    catalogue = json.loads((ROOT / catalogue_path).read_text(encoding="utf-8"))
    allowances = [
        entry for entry in catalogue["allowed_references"] if entry["path"] != "node"
    ]
    assert len(catalogue["allowed_references"]) - len(allowances) == 1
    temporary_catalogue = {
        "components": catalogue["components"],
        "allowed_references": allowances,
    }
    checker_path = "tools/paxeer-x/tests/retired_callers_test.py"
    node_path = "node/upgrades.go"
    for path in (checker_path, node_path):
        target = tmp_path / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes((ROOT / path).read_bytes())
    (tmp_path / catalogue_path).write_text(
        json.dumps(temporary_catalogue), encoding="utf-8"
    )
    subprocess.run(
        ["git", "init"],
        cwd=tmp_path,
        capture_output=True,
        check=True,
    )
    subprocess.run(
        ["git", "add", "--", checker_path, node_path, catalogue_path],
        cwd=tmp_path,
        capture_output=True,
        check=True,
    )

    found = callers("verify", tmp_path)
    assert found
    assert len(found) == len(set(found))
    symbols = catalogue["components"]["verify"]["symbols"]
    upgrade_source = (tmp_path / node_path).read_text(encoding="utf-8").splitlines()
    for location in found:
        path, number = location.rsplit(":", 1)
        assert path == node_path, location
        line_number = int(number)
        assert 1 <= line_number <= len(upgrade_source), location
        assert any(
            symbol in upgrade_source[line_number - 1] for symbol in symbols
        ), location
    imports = {
        f"{node_path}:{number}"
        for number, line in enumerate(upgrade_source, 1)
        if line.strip().endswith(
            '"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"'
        )
    }
    assert imports
    assert imports.issubset(found)
    assert owners(found) == {node_path: 5}
    assert callers("verify", ROOT) == []
