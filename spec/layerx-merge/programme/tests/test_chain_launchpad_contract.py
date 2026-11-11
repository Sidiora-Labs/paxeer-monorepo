import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
CONTRACT = ROOT / "spec/layerx-merge/contracts/chain-launchpad.md"
SOLIDITY = ROOT / "precompiles/launchpad/Launchpad.sol"
INTERFACES = ROOT / "spec/layerx-merge/interfaces.kvx"

METHODS = (
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
)
EVENTS = (
    "ChainLaunched",
    "ChainRevenueEpochOpened",
    "ChainRevenueClaimed",
    "LaunchedChainRetired",
)
ERRORS = (
    "WrongChain",
    "NotLaunchedChain",
    "NotFeeRightHolder",
    "EpochTooEarly",
    "NothingToDistribute",
    "AlreadyClaimed",
)
KINDS = {
    "precompile_method",
    "precompile_event",
    "precompile_error",
    "storage_key",
    "go_exported",
    "rest_route",
    "ts_export",
}


def chainlaunch_rows() -> list[tuple[str, dict[str, str]]]:
    text = INTERFACES.read_text(encoding="utf-8")
    blocks = re.findall(
        r"(?ms)^\[interface\.22\.chainlaunch-([^\]]+)\]\n(.*?)(?=^\[|\Z)", text
    )
    rows = []
    for name, block in blocks:
        fields = {}
        for line in block.splitlines():
            if not line.strip():
                continue
            match = re.fullmatch(r"([a-z_]+) = (.+)", line)
            assert match, f"unparsed field in {name}: {line}"
            key, raw = match.groups()
            assert key not in fields, f"repeated field {key} in {name}"
            fields[key] = raw
        rows.append((name, fields))
    return rows


def test_chain_launchpad_document_declares_abi() -> None:
    document = CONTRACT.read_text(encoding="utf-8")
    solidity = SOLIDITY.read_text(encoding="utf-8")
    assert "0x0000000000000000000000000000000000001017" in document
    for kind, names in (("function", METHODS), ("event", EVENTS), ("error", ERRORS)):
        for name in names:
            if kind == "function":
                declaration = (
                    rf"\bfunction {name}\s*\([^;]*\)\s+external"
                    r"(?:\s+(?:view|payable))?(?:\s+returns\s*\([^;]*\))?;"
                )
            else:
                declaration = rf"\b{kind} {name}\s*\([^;]*\);"
            in_document = re.search(declaration, document)
            in_solidity = re.search(declaration, solidity)
            assert in_document, f"contract missing {kind} {name}"
            assert in_solidity, f"Solidity missing {kind} {name}"
            assert in_document.group() == in_solidity.group(), name
    for name in EVENTS:
        assert re.search(
            rf"event {name}\([^;]*uint256 timestamp, uint256 blockNumber\);",
            document,
        )
    for section in ("abi", "launch", "storage", "home-chain", "revenue", "api"):
        assert f"\n## {section}\n" in document
    for route in ("GET /v2/chains", "GET /v2/chains/ID", "GET /v2/chains/ID/revenue"):
        assert route in document


def test_chain_launchpad_interface_rows_parse_and_cover_contract() -> None:
    rows = chainlaunch_rows()
    assert rows
    names = [name for name, _ in rows]
    assert len(names) == len(set(names)), "duplicate chainlaunch interface id"
    document = CONTRACT.read_text(encoding="utf-8")
    for name, fields in rows:
        assert set(fields) == {
            "kind",
            "path",
            "line",
            "signature",
            "callers",
            "status",
            "contract",
            "notes",
        }, name
        for key in ("kind", "path", "signature", "status", "contract", "notes"):
            assert fields[key].startswith('"') and fields[key].endswith('"'), (name, key)
        kind = fields["kind"].strip('"')
        contract = fields["contract"].strip('"')
        assert kind in KINDS, name
        assert fields["line"] == "0", name
        assert fields["callers"] == "[]", name
        assert fields["status"] == '"new"', name
        assert contract.startswith("contracts/chain-launchpad.md#"), name
        assert f"\n## {contract.split('#', 1)[1]}\n" in document, name
        notes = fields["notes"].strip('"')
        assert re.search(r"\b22\.40\.(?:[1-9]|1[0-8])\b", notes), name
        assert "22.40.18" in notes, name
    for prefix, expected in (
        ("method-", METHODS),
        ("event-", EVENTS),
        ("error-", ERRORS),
    ):
        actual = {name for name in names if name.startswith(prefix)}
        assert actual == {prefix + value.lower() for value in expected}
    assert {fields["kind"].strip('"') for _, fields in rows} == KINDS
