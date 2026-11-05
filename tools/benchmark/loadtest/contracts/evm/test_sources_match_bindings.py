import copy
import sys
import json
import subprocess
from pathlib import Path

import pytest


CONTRACT_ROOT = Path(__file__).resolve().parent
REPOSITORY_ROOT = CONTRACT_ROOT.parents[4]
sys.path.insert(0, str(CONTRACT_ROOT.parents[1] / "scripts"))
from operations_cli import request


@pytest.fixture(scope="module")
def compiled_contracts():
    subprocess.run(
        [
            "forge",
            "build",
            "--offline",
            "--root",
            str(CONTRACT_ROOT.relative_to(REPOSITORY_ROOT)),
        ],
        cwd=REPOSITORY_ROOT,
        check=True,
    )
    return CONTRACT_ROOT


def canonical_abi(abi):
    completed = request("canonical-abi", json.dumps(abi))
    if completed.returncode:
        raise ValueError(completed.stderr or "invalid ABI")
    return json.loads(completed.stdout)


def test_changed_parameter_is_rejected():
    original = json.loads((CONTRACT_ROOT / "bindings/erc20/abi.json").read_text())
    changed = copy.deepcopy(original)
    entry = next(item for item in changed if item.get("name") == "transfer")
    entry["inputs"][1]["type"] = "uint128"
    assert canonical_abi(original) != canonical_abi(changed)


def test_noop_token_abi_matches_erc20_binding(compiled_contracts):
    artifact = json.loads(
        (compiled_contracts / "out/NoopToken.sol/NoopToken.json").read_text()
    )
    binding = json.loads((compiled_contracts / "bindings/erc20/abi.json").read_text())
    assert canonical_abi(artifact["abi"]) == canonical_abi(binding)


def test_my_nft_abi_matches_erc721_binding(compiled_contracts):
    artifact = json.loads(
        (compiled_contracts / "out/ERC721.sol/MyNFT.json").read_text()
    )
    binding = json.loads((compiled_contracts / "bindings/erc721/abi.json").read_text())
    assert canonical_abi(artifact["abi"]) == canonical_abi(binding)
