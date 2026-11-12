"""Check the served chain launch declarations against their consumers."""

from __future__ import annotations

import ast
import hashlib
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
INTERFACE_FILE = ROOT / "spec/layerx-merge/interfaces.kvx"
ABI_FILE = ROOT / "precompiles/launchpad/abi.json"
CHAIN_ABI_FILE = ROOT / "precompiles/launchpad/abi/ChainLaunch.json"
RECORD_FILE = ROOT / "tools/qualification/audit_repairs/repair_22_40_18.kvx"
EVENTS = {"ChainLaunched", "ChainRevenueEpochOpened", "ChainRevenueClaimed", "LaunchedChainRetired"}
ERRORS = {"WrongChain", "NotLaunchedChain", "NotFeeRightHolder", "EpochTooEarly", "NothingToDistribute", "AlreadyClaimed"}
SERIES_TEST_FILES = {
    "22.40.8": ("node/chainlaunch_block_test.go",),
    "22.40.9": ("services/launchpad/indexer/internal/decode/chainlaunch_test.go", "services/launchpad/indexer/internal/processor/chainlaunch_test.go"),
    "22.40.10": ("apps/launchpad/web/src/widgets/create-chain/chain-launch.test.ts",),
    "22.40.11": ("apps/explorer/ui/paxeerX/chains/launch.spec.ts",),
    "22.40.12": ("spec/layerx-merge/programme/tests/test_chain_launch_docs.py",),
    "22.40.13": ("services/launchpad/api/internal/rest/chains_test.go",),
    "22.40.14": ("apps/launchpad/web/src/widgets/trade/home-chain.test.ts", "apps/launchpad/web/src/widgets/holder-rewards/chain-revenue.test.tsx"),
    "22.40.15": ("sdk/launchpad/packages/sdk/test/chains.test.mjs",),
    "22.40.16": ("apps/launchpad/mobile/test/vitest/chain-launch.test.ts",),
    "22.40.17": ("tools/qualification/paxeer-x/chain_launch_journey.py",),
}


def chain_rows() -> dict[str, dict[str, object]]:
    source = INTERFACE_FILE.read_text()
    rows = {}
    pattern = r"(?ms)^\[(interface\.22\.chainlaunch-[^\]]+)\]\n(.*?)(?=^\[|\Z)"
    for name, body in re.findall(pattern, source):
        row = {}
        for key, raw in re.findall(r"(?m)^(\w+) = (.*)$", body):
            row[key] = json.loads(raw)
        rows[name] = row
    return rows


def test_chain_launch_interfaces_are_served_at_their_declarations() -> None:
    rows = chain_rows()
    assert len(rows) == 51
    for name, row in rows.items():
        assert row["status"] == "served", name
        path = ROOT / str(row["path"])
        assert path.is_file(), name
        line = int(row["line"])
        lines = path.read_text().splitlines()
        assert 1 <= line <= len(lines), name
        signature = str(row["signature"])
        if "-route-" in name:
            symbol = signature.split(" ", 1)[1]
        elif "-store-" in name:
            symbol = signature.split()[0]
        elif "-keeper-" in name:
            symbol = re.search(r"\bfunc\s+\([^)]*\)\s*(\w+)", signature).group(1)
        elif "-ts-" in name:
            symbol = re.search(r"\b(list|get|revenue)\(", signature).group(1)
        else:
            symbol = signature.split("(", 1)[0]
        assert symbol in lines[line - 1], (name, path, line, symbol)
        assert row["callers"], name
        assert all((ROOT / caller).is_file() for caller in row["callers"]), name


def test_chain_launch_documents_cover_the_abi() -> None:
    abi = json.loads(ABI_FILE.read_text())
    chain_abi = json.loads(CHAIN_ABI_FILE.read_text())
    functions = {entry["name"] for entry in chain_abi if entry["type"] == "function"}
    rows = chain_rows()
    for kind, names in (("method", functions), ("event", EVENTS), ("error", ERRORS)):
        declared = {str(row["signature"]).split("(", 1)[0] for key, row in rows.items() if f"-{kind}-" in key}
        assert declared == names
        abi_kind = "function" if kind == "method" else kind
        assert names <= {entry["name"] for entry in abi if entry.get("type") == abi_kind}
    for path in (ROOT / "spec/layerx-merge/contracts/chain-launchpad.md", ROOT / "docs/launchpad/chain-launch.md"):
        document = path.read_text()
        for kind, names in (("function", functions), ("event", EVENTS), ("error", ERRORS)):
            for name in names:
                assert re.search(rf"\b{kind}\s+{re.escape(name)}\s*\(", document), (path, name)


def test_chain_launch_series_files_and_record() -> None:
    record = RECORD_FILE.read_text()
    task_line = re.search(r'(?m)^tasks = (\[.*\])$', record)
    assert task_line is not None
    tasks = json.loads(task_line.group(1))
    assert tasks == [f"22.40.{number}" for number in range(1, 21)]
    assert set(SERIES_TEST_FILES) == {f"22.40.{number}" for number in range(8, 18)}
    for task, paths in SERIES_TEST_FILES.items():
        assert all((ROOT / path).is_file() for path in paths), task


def test_wallet_launchpad_abi_matches_precompile_bytes() -> None:
    assert (ROOT / "services/wallet/attestor/internal/policy/evm/abi/launchpad.json").read_bytes() == ABI_FILE.read_bytes()


def test_native_source_hash_tables_match_the_tree() -> None:
    directory = ROOT / "sdk/launchpad/protocol/bindings/testdata/repair_m948"
    for name in ("native-execution.json", "native-node-rpc.json"):
        hashes = json.loads((directory / name).read_text())["SourceHashes"]
        assert hashes, name
        for path, expected in hashes.items():
            assert hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == expected, (name, path)


def test_agent_sdk_write_tables_cover_abi_writes() -> None:
    abi = json.loads(ABI_FILE.read_text())
    writes = {entry["name"] for entry in abi if entry.get("type") == "function" and entry.get("stateMutability") not in ("view", "pure")}
    python_source = ROOT / "services/agent/sdk/python/layerx_sdk/generated/precompile_writes.py"
    tree = ast.parse(python_source.read_text())
    table = next(node.value for node in tree.body if isinstance(node, ast.AnnAssign) and isinstance(node.target, ast.Name) and node.target.id == "LAUNCHPAD_WRITES")
    python_names = {key.value.split("(", 1)[0] for key in table.keys}
    typescript_source = (ROOT / "services/agent/sdk/typescript/src/generated/precompile-writes.ts").read_text()
    section = typescript_source.split("export const LAUNCHPAD_WRITES = {", 1)[1].split("} as const satisfies", 1)[0]
    typescript_names = {name.split("(", 1)[0] for name in re.findall(r'^\s*"([^"]+)":', section, re.M)}
    assert writes <= python_names
    assert writes <= typescript_names
