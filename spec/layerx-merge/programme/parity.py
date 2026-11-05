from __future__ import annotations

import json
import re
import shlex
import subprocess
import tomllib
from collections.abc import Mapping
from os import PathLike
from pathlib import Path, PurePosixPath

from .kvx import KvxValue, load

ROOT = Path(__file__).resolve().parents[3]
PARITY_PATH = Path(__file__).resolve().parent / "data/parity.kvx"
DOC_PATH = PARITY_PATH.parents[2] / "docs/parity-inventory.md"

ORACLE_SECTION = """## 1 Parity oracle

### What it replays

| Input | Path | Content |
|---|---|---|
| Kernel replay corpus | `tests/vectors/replay_corpus.lxb` | corpus read by the qualification replay (corpus header `tests/qualification/lxp_qual_replay.c:21`, activity type list `tests/qualification/lxp_qual_replay.c:28`) |
| Replay digest | `tests/vectors/qualification_replay_10m.digest` | expected kernel digest of the long replay |
| Cross-architecture activity list | `tests/test_replay_crossarch.c` | activity types 0x00010001 to 0x0005000d replayed by `make test-replay-crossarch` (`Makefile:1074`, list at `tests/test_replay_crossarch.c:9-26`) |
| Kernel goldens and fixtures | `tests/fixtures/`, `tests/replay/golden/history.lxl` (`make test-replay-golden-local`, `Makefile:381`) | receipts, grants, sessions, custody credits, genesis and module tables listed per construct in section 2 |
| Trading payload vectors | `tests/fixtures/trading-payloads/vectors.json` | 17 perps and spot payload vectors (producer `tests/modules/dump_trading_vectors.c`) |
| Program vectors | `programs/tests/vectors/`, `programs/fixtures/`, `programs/abi-frozen.sha256` | ABI v1 to v5 manifests, program fixtures, frozen ABI digest |
| Web preimage vectors | `modules/xweb/types/testdata/preimage-vectors.json`, `tests/fixtures/web/observation-activity.hex` | origin 1 and origin 2 attestation preimages and the canonical observation bytes |
| Client goldens | `agent/schema/agent-api/golden/`, `sdk/platform/conformance/fixtures/`, `human/schema/human-api/golden/` | wire and receipt goldens consumed by the agent, SDK and human clients in wave 5 |

Each corpus activity runs twice: once through liblayerx (the kernel execution function) and once as the mapped EVM transaction against the chain construct that replaces it (layerxledger 0x101A, layerxpay 0x101B, layerxauth 0x101C, layerxprograms 0x101D, layerxexchange 0x1015, xweb 0x1019).

### What it diffs

| Dimension | Kernel side | Chain side | Equality rule |
|---|---|---|---|
| Result | `lxp_result` code (`include/layerx/lxp_result.h`) | transaction status and the mapped error code of the precompile revert | same code for every activity, success and refusal alike |
| State | per-account, per-asset balances; asset supply; module records | bank balances at the derived addresses; denom supply; keeper store records | equal deltas per account and asset, equal supply deltas, equal record fields |
| Receipts | kernel receipt | sequencer-signed receipt and `eth_getTransactionReceipt` (`rpc/tx.go:106`) | status, gas used, logs and order equal |
| Events | kernel events | EVM logs of the construct | same events in the same order with equal fields |
| Conservation | `LXP_ERR_CONSERVATION` (`include/layerx/lxp_result.h:84`, raised at `src/ledger/lxp_apply.c:412`) | same refusal from the chain construct | a conservation refusal on one side is a conservation refusal on the other |

The comparison follows the pattern already used for parallel execution: `tests/occ/occ_test.go:198-232` runs the same transactions sequentially and under OCC and asserts equal result codes, events (`tests/occ/occ_test.go:55`), transaction results (`tests/occ/occ_test.go:83`) and store state (`tests/occ/occ_test.go:17`).

### Where and when it runs

The parity oracle runs once per wave on the merged wave revision, never per task. Task gates run the focused tests named in sections 2 to 5; the oracle then covers every construct landed up to that wave. Wave 2 adds ledger, pay and auth activities; wave 3 adds programs, origin-2 web and native trading; wave 6 adds the cut-over goldens of section 5.

| Wave | Constructs landed (the oracle starts at wave 2) |
|---|---|
| 0 | sequencer execution, signed receipts, pending reads, fee policy, address table |
| 1 | C-module host shim, account and asset maps, archive proofs |
| 2 | layerxledger, layerxpay, layerxauth, governance pause and fee parameters |
| 3 | layerxprograms, program-origin xweb, layerxexchange native perps and spot |
| 4 | AI and market guests, keeper and sidecar drivers |
| 5 | re-pointed callers: router, gateway, agent stack, wallet, explorer, bridge relayer |
| 6 | cut-over: freeze, balance export, upgrade handlers |
| 7 | retirement of the two-domain components |

### Command shape

| Gate | Command | Path |
|---|---|---|
| Parity oracle (planned) | `go test -count=1 ./tests/parity/kernel_chain/` | `tests/parity/kernel_chain/` (planned; same Go test layout as the existing `tests/chain/` package) |
| Kernel reference side | `make test-replay-crossarch` | `tests/test_replay_crossarch.c` |

"""


class ParityError(ValueError):
    pass


def _rust_target(path: PurePosixPath) -> tuple[PurePosixPath, str, str]:
    if path.parent.name != "tests":
        raise ParityError(f"Rust selector must name an integration test: {path}")
    crate = path.parent.parent
    manifest = ROOT / crate / "Cargo.toml"
    try:
        with manifest.open("rb") as source:
            package = tomllib.load(source)["package"]
        name = package["name"]
        if not isinstance(name, str) or not name:
            raise ParityError(f"missing Rust crate name: {manifest}")
        if "workspace" in package:
            workspace_manifest = manifest.parent / package["workspace"] / "Cargo.toml"
            with workspace_manifest.open("rb") as source:
                if "workspace" not in tomllib.load(source):
                    raise ParityError(f"missing Rust workspace: {workspace_manifest}")
            manifest = workspace_manifest.resolve()
        else:
            for parent in manifest.parents:
                candidate = parent / "Cargo.toml"
                if candidate.is_file():
                    with candidate.open("rb") as source:
                        if "workspace" in tomllib.load(source):
                            manifest = candidate
                            break
                if parent == ROOT:
                    break
        return PurePosixPath(manifest.relative_to(ROOT).as_posix()), name, path.stem
    except (OSError, KeyError, TypeError, ValueError) as error:
        raise ParityError(f"cannot resolve Rust selector {path}: {error}") from error


def command_for(row: Mapping[str, KvxValue]) -> str:
    selectors = row.get("selectors")
    if not isinstance(selectors, list) or not selectors:
        raise ParityError("selectors must be a nonempty list")
    parsed: list[tuple[PurePosixPath, str]] = []
    for selector in selectors:
        if not isinstance(selector, str):
            raise ParityError("each selector must be a path::test string")
        path_text, separator, name = selector.partition("::")
        if (
            not separator
            or not re.fullmatch(r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*", path_text)
            or {".", ".."}.intersection(path_text.split("/"))
            or not re.fullmatch(
                r"[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*", name
            )
        ):
            raise ParityError(f"invalid exact selector: {selector}")
        parsed.append((PurePosixPath(path_text), name))

    suffixes = {path.suffix for path, _ in parsed}
    if len(suffixes) != 1:
        raise ParityError("selectors must use one toolchain")
    suffix = parsed[0][0].suffix
    if suffix == ".go":
        packages = {path.parent for path, _ in parsed}
        if len(packages) != 1:
            raise ParityError("Go selectors span two packages")
        if any(
            not path.name.endswith("_test.go")
            or not re.fullmatch(r"Test[A-Za-z0-9_]+", name)
            for path, name in parsed
        ):
            raise ParityError("Go selectors must name declared Test functions")
        package = parsed[0][0].parent.as_posix()
        target = "./" if package == "." else f"./{package}/"
        pattern = "^(" + "|".join(dict.fromkeys(name for _, name in parsed)) + ")$"
        return shlex.join(["go", "test", "-count=1", target, "-run", pattern])
    if suffix == ".py":
        return shlex.join(["python3", "-m", "pytest", *selectors, "-q"])
    if suffix == ".rs":
        targets = [(_rust_target(path), name) for path, name in parsed]
        crates = {(manifest, crate) for (manifest, crate, _), _ in targets}
        if len(crates) != 1:
            raise ParityError("Rust selectors span two crates")
        manifest, crate = targets[0][0][:2]
        tests: dict[str, list[str]] = {}
        for (_, _, file), name in targets:
            tests.setdefault(file, []).append(name)
        return " && ".join(
            shlex.join(
                [
                    "cargo",
                    "test",
                    "--locked",
                    "--manifest-path",
                    str(manifest),
                    "-p",
                    crate,
                    "--test",
                    file,
                    "--",
                    "--exact",
                    *dict.fromkeys(names),
                ]
            )
            for file, names in tests.items()
        )
    raise ParityError(f"unsupported selector toolchain: {suffix}")


def count_go_tests(command: str, root: str | PathLike[str]) -> int:
    arguments = shlex.split(command)
    if arguments[:2] != ["go", "test"]:
        raise ParityError("expected a go test command")
    result = subprocess.run(
        [*arguments, "-json"],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    )
    count = 0
    for line in result.stdout.splitlines():
        if not line.strip():
            continue
        event = json.loads(line)
        name = event.get("Test")
        if (
            event.get("Action") == "run"
            and isinstance(name, str)
            and name.startswith("Test")
            and "/" not in name
        ):
            count += 1
    return count


def zero_test_commands(
    spec_path: str | PathLike[str], root: str | PathLike[str]
) -> list[str]:
    task_ids: list[str] = []
    for section, task in load(spec_path).items():
        if not section.startswith("task."):
            continue
        command = task.get("verify_cmd")
        if not isinstance(command, str) or shlex.split(command)[:2] != ["go", "test"]:
            continue
        if count_go_tests(command, root) == 0:
            task_ids.append(section.removeprefix("task."))
    return task_ids


def _doc_text(value: KvxValue) -> str:
    return str(value).replace("|", "\\|").replace("\n", "<br>")


def _doc_refs(values: KvxValue) -> str:
    if not isinstance(values, list):
        raise ParityError("document references must be a list of strings")
    return "; ".join(f"`{_doc_text(value)}`" for value in values) or "none"


def render_doc(rows: Mapping[str, Mapping[str, KvxValue]]) -> str:
    """Render the parity inventory from the KVX rows in source order."""
    entries = [
        row
        for name, row in rows.items()
        if name.startswith(("construct.", "dropped."))
    ]
    sections = {
        number: [row for row in entries if row["source_section"] == number]
        for number in (2, 3, 5)
    }
    dropped = [row for row in entries if "dropped" in row]
    dropped_count = sum(len(row["dropped"]) for row in dropped)
    lines = [
        "# Parity inventory",
        "",
        "Requirement 1 (feature parity): every kernel construct folded into Paxeer X "
        "Network keeps its behaviour, shown by the kernel's own tests and goldens "
        "re-targeted at the chain and by a differential harness, the parity oracle.",
        "",
        "Generated from `programme/data/parity.kvx`. Source paths retain the "
        "original inventory; exact selectors name the current test targets. "
        "Planned-by IDs identify the tasks declaring selectors. "
        "An inventory entry is not evidence that its tests passed.",
        "Regenerate from the repository root with:",
        "",
        "```sh",
        "PYTHONPATH=spec/layerx-merge python3 -m programme.parity",
        "```",
        "",
        f"Counts: {len(sections[2])} existing construct rows (section 2), "
        f"{len(sections[3])} new target rows (section 3), "
        f"{dropped_count} dropped references across {len(dropped)} rows (section 4), "
        f"{len(sections[5])} cut-over goldens (section 5). "
        "Dropped reference counts include repeated references across rows.",
        "",
        ORACLE_SECTION.rstrip("\n"),
    ]

    def append_targets(number: int, title: str) -> None:
        lines.extend(
            [
                "",
                f"## {number} {title}",
                "",
                "Source paths are historical references. Run the exact selectors; "
                "rows without selectors are listed as dropped in section 4.",
                "",
                "| Construct | Source paths | Exact selectors | Owner domain | Wave | Planned by |",
                "|---|---|---|---|---|---|",
            ]
        )
        for row in sections[number]:
            selectors = (
                _doc_refs(row["selectors"])
                if "selectors" in row
                else "Dropped; see section 4"
            )
            planned_by = _doc_refs(row["planned_by"]) if "planned_by" in row else "none"
            cells = [
                _doc_text(row["title"]),
                _doc_refs(row["source_paths"]),
                selectors,
                _doc_text(row["owner_domain"]),
                _doc_text(row["wave"]),
                planned_by,
            ]
            lines.append("| " + " | ".join(cells) + " |")

    append_targets(2, "Existing targets")
    append_targets(3, "New targets")
    lines.extend(
        [
            "",
            "## 4 Dropped selectors and targets",
            "",
            "These references are excluded from the exact selectors. "
            "Rows with replacements retain their current selectors in sections 2, 3 or 5. "
            "The parity oracle description in section 1 is preserved verbatim; "
            "its planned harness is recorded below as dropped.",
            "",
            "| Construct | Dropped references | Source section | Owner domain | Wave | Disposition |",
            "|---|---|---|---|---|---|",
        ]
    )
    for row in dropped:
        disposition = (
            _doc_text(row["reason"])
            if "reason" in row
            else f"Superseded; current exact selectors are in section {row['source_section']}."
        )
        cells = [
            _doc_text(row["title"]),
            _doc_refs(row["dropped"]),
            _doc_text(row["source_section"]),
            _doc_text(row["owner_domain"]),
            _doc_text(row["wave"]),
            disposition,
        ]
        lines.append("| " + " | ".join(cells) + " |")
    append_targets(5, "Cut-over goldens")
    return "\n".join(lines) + "\n"


if __name__ == "__main__":
    DOC_PATH.write_bytes(render_doc(load(PARITY_PATH)).encode("utf-8"))
