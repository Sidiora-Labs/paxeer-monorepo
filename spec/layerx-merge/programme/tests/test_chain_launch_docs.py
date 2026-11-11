import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
CONTRACT = ROOT / "spec/layerx-merge/contracts/chain-launchpad.md"
GUIDE = ROOT / "docs/launchpad/chain-launch.md"
WIKI = ROOT / "docs/wiki/VirtualChains.md"
SOLIDITY = ROOT / "precompiles/launchpad/Launchpad.sol"
ABI = ROOT / "precompiles/launchpad/abi.json"

FUNCTIONS = {
    "launchChain",
    "launchChainWithPermit",
    "openChainRevenueEpoch",
    "claimChainRevenue",
    "claimChainRevenueBatch",
    "pauseLaunchedChain",
    "resumeLaunchedChain",
    "retireLaunchedChain",
    "launchedChain",
    "launchedChainOfPool",
    "homeChainOf",
    "chainRevenueEpoch",
    "chainRevenueClaimable",
}
EVENTS = {
    "ChainLaunched",
    "ChainRevenueEpochOpened",
    "ChainRevenueClaimed",
    "LaunchedChainRetired",
}
ERRORS = {
    "WrongChain",
    "NotLaunchedChain",
    "NotFeeRightHolder",
    "EpochTooEarly",
    "NothingToDistribute",
    "AlreadyClaimed",
}
EXPECTED = {"function": FUNCTIONS, "event": EVENTS, "error": ERRORS}
DECLARATION = re.compile(r"^\s*(function|event|error)\s+(\w+)\([^;\n]*;", re.MULTILINE)
LINK = re.compile(r"\[[^\]]+\]\(([^)]+)\)")


def declarations(document: str) -> dict[str, dict[str, str]]:
    found: dict[str, dict[str, str]] = {kind: {} for kind in EXPECTED}
    for match in DECLARATION.finditer(document):
        kind, name = match.group(1), match.group(2)
        found[kind][name] = match.group(0).strip()
    return found


def test_chain_launch_declarations_match_contract_solidity_and_abi() -> None:
    contract = CONTRACT.read_text(encoding="utf-8")
    guide = GUIDE.read_text(encoding="utf-8")
    solidity = SOLIDITY.read_text(encoding="utf-8")
    abi = json.loads(ABI.read_text(encoding="utf-8"))
    contract_block = re.search(r"```solidity\n(.*?)\n```", contract, re.DOTALL)
    guide_block = re.search(r"```solidity\n(.*?)\n```", guide, re.DOTALL)
    assert contract_block is not None
    assert guide_block is not None

    contract_declarations = declarations(contract_block.group(1))
    guide_declarations = declarations(guide_block.group(1))
    source_declarations = declarations(solidity)
    for kind, expected_names in EXPECTED.items():
        assert set(contract_declarations[kind]) == expected_names
        assert set(guide_declarations[kind]) == expected_names
        abi_names = {entry["name"] for entry in abi if entry["type"] == kind}
        for name in expected_names:
            assert name in abi_names, f"ABI missing {kind} {name}"
            assert guide_declarations[kind][name] == contract_declarations[kind][name]
            assert guide_declarations[kind][name] == source_declarations[kind][name]


def test_chain_launch_pages_have_resolving_relative_links() -> None:
    for page in (GUIDE, WIKI, CONTRACT):
        links = LINK.findall(page.read_text(encoding="utf-8"))
        assert links, f"{page} has no links"
        for link in links:
            target = link.split("#", 1)[0]
            if not target or target.startswith("/") or "://" in target:
                continue
            assert (page.parent / target).is_file(), f"{page}: missing {link}"

    assert "[chain launch guide](../launchpad/chain-launch.md)" in WIKI.read_text(
        encoding="utf-8"
    )
    assert "[chain launch guide](../../../docs/launchpad/chain-launch.md)" in (
        CONTRACT.read_text(encoding="utf-8")
    )
