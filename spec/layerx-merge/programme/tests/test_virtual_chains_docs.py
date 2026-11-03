import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DOCUMENTS = (
    "docs/wiki/VirtualChains.md",
    "spec/layerx-merge/contracts/virtual-chains.md",
)
METHODS = (
    "createChain",
    "fundChain",
    "pauseChain",
    "resumeChain",
    "retireChain",
    "governancePause",
    "setOperator",
    "getChain",
    "chainByName",
    "chainCount",
    "chainIdOf",
    "params",
)
EVENTS = (
    "ChainCreated",
    "ChainStatusChanged",
    "ChainFunded",
    "ChainOperatorChanged",
)


def test_virtual_chains_docs_name_every_contract() -> None:
    for name in DOCUMENTS:
        document = (ROOT / name).read_text(encoding="utf-8")
        for required in (
            "0x000000000000000000000000000000000000101E",
            "VirtualChainIDBase = 125000000",
            "ChainIDOf(id) = VirtualChainIDBase + id",
            "DomainOfChainID(125) = 0",
            "DomainOfChainID(VirtualChainIDBase + id) = id for id >= 1",
            "/chain/<id>",
        ):
            assert required in document, f"{name}: missing {required}"
        for method in METHODS:
            assert re.search(rf"\bfunction {method}\s*\(", document), (
                f"{name}: missing ABI method {method}"
            )
        for event in EVENTS:
            assert re.search(rf"\bevent {event}\s*\(", document), (
                f"{name}: missing ABI event {event}"
            )

    contract = (ROOT / DOCUMENTS[1]).read_text(encoding="utf-8")
    for section in (
        "registry",
        "wiring",
        "abi",
        "precompile",
        "admission",
        "fees",
        "rpc",
        "indexer",
        "explorer",
        "wallet",
        "launch",
        "sdk",
        "closure",
    ):
        assert f"\n## {section}\n" in contract, f"missing contract anchor #{section}"

    index = (ROOT / "spec/layerx-merge/contracts/README.md").read_text(
        encoding="utf-8"
    )
    assert "| [virtual-chains.md](virtual-chains.md) | 20 | 31 |" in index

    home = (ROOT / "docs/wiki/Home.md").read_text(encoding="utf-8")
    assert "[Virtual chains](VirtualChains.md)" in home
    network = (ROOT / "docs/paxeer-network.md").read_text(encoding="utf-8")
    assert "## Virtual chains\n" in network
    assert "[Virtual chains](wiki/VirtualChains.md)" in network
