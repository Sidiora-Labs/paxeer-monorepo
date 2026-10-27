import json
import subprocess
from pathlib import Path

import pytest


CONTRACT_ROOT = Path(__file__).resolve().parent
REPOSITORY_ROOT = CONTRACT_ROOT.parents[4]


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


def abi_names(abi):
    return {
        kind: {entry["name"] for entry in abi if entry["type"] == kind}
        for kind in ("function", "event", "error")
    }


def test_noop_token_abi_matches_erc20_binding(compiled_contracts):
    artifact = json.loads(
        (compiled_contracts / "out/NoopToken.sol/NoopToken.json").read_text()
    )
    binding = json.loads((compiled_contracts / "bindings/erc20/abi.json").read_text())
    assert abi_names(artifact["abi"]) == abi_names(binding)


def test_my_nft_abi_matches_erc721_binding(compiled_contracts):
    artifact = json.loads(
        (compiled_contracts / "out/ERC721.sol/MyNFT.json").read_text()
    )
    binding = json.loads((compiled_contracts / "bindings/erc721/abi.json").read_text())
    assert abi_names(artifact["abi"]) == abi_names(binding)
