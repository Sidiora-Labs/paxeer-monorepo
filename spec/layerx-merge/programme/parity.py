from __future__ import annotations

import json
import os
import re
import shlex
import shutil
import subprocess
import tomllib
from collections import deque
from collections.abc import Mapping
from os import PathLike
from pathlib import Path, PurePosixPath

from .evidence import evidence_line
from .kvx import KvxValue, load

ROOT = Path(__file__).resolve().parents[3]
PARITY_PATH = Path(__file__).resolve().parent / "data/parity.kvx"
DOC_PATH = PARITY_PATH.parents[2] / "docs/parity-inventory.md"
TASK_PATHS = {
    "spec/layerx-merge/programme/data/parity.kvx",
    "spec/layerx-merge/programme/parity.py",
    "spec/layerx-merge/programme/tests/test_parity_run.py",
}
SELECTOR_ENVIRONMENT = {
    "apps/wallet/e2e/send.spec.ts::"
    "send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload": {
        "WALLET_TRANSFER_TRUTH_ANVIL_URL": "LOCAL_NODE_SEQUENCER_RPC",
        "WALLET_TRANSFER_TRUTH_SEQUENCER_RPC_URL": "LOCAL_NODE_SEQUENCER_RPC",
        "WALLET_TRANSFER_TRUTH_SENDER": "LOCAL_NODE_ADDRESS",
        "WALLET_TRANSFER_TRUTH_SEQUENCER_KEY": "LOCAL_NODE_KEY",
    }
}

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

    suffixes = {".ts" if path.suffix == ".tsx" else path.suffix for path, _ in parsed}
    if len(suffixes) != 1:
        raise ParityError("selectors must use one toolchain")
    suffix = next(iter(suffixes))
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
    if suffix == ".c":
        if any(name != "main" for _, name in parsed):
            raise ParityError("C selectors must name a test binary's main function")
        makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
        binaries = []
        for path, _ in dict.fromkeys(parsed):
            target = str(path.with_suffix(""))
            if not re.search(
                rf"^\$\(BUILD_DIR\)/{re.escape(target)}\s*:",
                makefile,
                re.MULTILINE,
            ):
                raise ParityError(f"no declared Makefile test binary for selector: {path}")
            binaries.append(PurePosixPath("build") / target)
        return " && ".join(
            f"{shlex.join(['make', str(binary)])} && {shlex.quote(str(binary))}"
            for binary in binaries
        )
    if suffix == ".exs":
        commands = []
        for path, name in dict.fromkeys(parsed):
            owner = next(
                (
                    parent
                    for parent in path.parents
                    if (ROOT / parent / "mix.exs").is_file()
                ),
                None,
            )
            if owner is None:
                raise ParityError(f"no owning mix.exs for selector: {path}")
            source = (ROOT / path).read_text(encoding="utf-8")
            if not re.search(rf'\btest\s+"{re.escape(name)}"\s*[,\n]', source):
                raise ParityError(f"no declared ExUnit test: {path}::{name}")
            umbrella = PurePosixPath("services/explorer/backend")
            if not path.is_relative_to(umbrella):
                raise ParityError(f"unsupported Elixir umbrella: {path}")
            commands.append(
                shlex.join(
                    [
                        "services/explorer/deploy/tools/mix-in-builder.sh",
                        "test",
                        str(path.relative_to(umbrella)),
                        "--only",
                        f"test:test {name}",
                    ]
                )
            )
        return " && ".join(commands)
    if suffix == ".ts":
        commands = []
        for path, name in dict.fromkeys(parsed):
            owner = next(
                (
                    parent
                    for parent in path.parents
                    if (ROOT / parent / "package.json").is_file()
                ),
                None,
            )
            if owner is None:
                raise ParityError(f"no owning package.json for selector: {path}")
            relative = str(path.relative_to(owner))
            pattern = rf"(?:^| ){re.escape(name)}$"
            if path.name.endswith(".spec.ts"):
                runner = [
                    "pnpm",
                    "exec",
                    "playwright",
                    "test",
                    "--config=playwright.wallet-app.config.ts",
                    relative,
                    "--grep",
                    pattern,
                    "--reporter=line",
                ]
            else:
                runner = [
                    "pnpm",
                    "exec",
                    "vitest",
                    "run",
                    relative,
                    "--testNamePattern",
                    pattern,
                    "--reporter=default",
                    "--no-color",
                ]
            if owner == PurePosixPath("apps/explorer"):
                runner = ["yarn", *runner[2:]]
            commands.append(
                "(" + shlex.join(["cd", str(owner)]) + " && " + shlex.join(runner) + ")"
            )
        return " && ".join(commands)
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


def count_go_tests(
    command: str,
    root: str | PathLike[str],
    *,
    log_path: str | PathLike[str] | None = None,
) -> int:
    arguments = shlex.split(command)
    if arguments[:2] != ["go", "test"]:
        raise ParityError("expected a go test command")
    if "-json" not in arguments:
        arguments.append("-json")
    count = 0
    with subprocess.Popen(
        arguments,
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
    ) as process:
        assert process.stdout is not None
        log = Path(log_path).open("a", encoding="utf-8") if log_path else None
        try:
            for line in process.stdout:
                if log is not None:
                    log.write(line)
                if not line.lstrip().startswith("{"):
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
        finally:
            if log is not None:
                log.close()
        exit_code = process.wait()
    if exit_code:
        raise subprocess.CalledProcessError(exit_code, arguments)
    return count


def _free_space(root: Path) -> str:
    return subprocess.run(
        ["df", "-k", str(root)],
        capture_output=True,
        text=True,
        check=True,
    ).stdout


def _row_output(
    log_path: Path,
) -> tuple[list[str], int, list[str], list[str], list[str], list[str]]:
    tail: deque[str] = deque(maxlen=200)
    total = 0
    executed: set[str] = set()
    passed: set[str] = set()
    rust_executed: set[str] = set()
    rust_passed: set[str] = set()
    with log_path.open(encoding="utf-8", errors="replace") as log:
        for line in log:
            total += 1
            tail.append(line.rstrip("\n"))
            rust_case = re.fullmatch(
                r"test ([A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)*)"
                r" \.\.\. (ok|FAILED)",
                re.sub(r"\x1b\[[0-?]*[ -/]*[@-~]", "", line).strip(),
            )
            if rust_case:
                rust_executed.add(rust_case[1])
                if rust_case[2] == "ok":
                    rust_passed.add(rust_case[1])
            if not line.lstrip().startswith("{"):
                continue
            try:
                event = json.loads(line)
            except ValueError:
                continue
            if not isinstance(event, dict):
                continue
            name = event.get("Test")
            if not isinstance(name, str) or "/" in name:
                continue
            if event.get("Action") == "run":
                executed.add(name)
            elif event.get("Action") == "pass":
                passed.add(name)
    return (
        list(tail),
        max(0, total - len(tail)),
        sorted(executed),
        sorted(passed),
        sorted(rust_executed),
        sorted(rust_passed),
    )


def _row_build_outputs(
    row: Mapping[str, KvxValue], root: Path, cargo_target: Path
) -> list[Path]:
    outputs: set[Path] = set()
    for selector in row["selectors"]:
        path = PurePosixPath(selector.partition("::")[0])
        if path.suffix == ".rs":
            outputs.add(cargo_target)
        elif path.is_relative_to("apps/wallet") and path.name.endswith(".spec.ts"):
            outputs.update(
                root / "apps/wallet" / name
                for name in ("playwright-report", "test-results", ".next")
            )
        elif path.is_relative_to("services/explorer/backend"):
            outputs.add(root / "services/explorer/backend/_build")
        elif path.is_relative_to("apps/explorer"):
            outputs.update(
                root / "apps/explorer" / name
                for name in (".next", "playwright-report", "test-results")
            )
    return sorted(outputs)


def _explorer_build_volume(root: Path) -> str:
    configured = os.environ.get("MIX_BUILD_VOLUME")
    if configured:
        return configured
    backend = str(root / "services/explorer/backend")
    checksum = subprocess.run(
        ["cksum"], input=backend, capture_output=True, text=True, check=True
    ).stdout.split()[0]
    basename = re.sub(r"[^A-Za-z0-9_.-]", "-", root.name)
    return f"explorer-mix-build-{basename}-{checksum}"


def _remove_row_outputs(outputs: list[Path]) -> None:
    for output in outputs:
        if output.is_symlink():
            output.unlink()
        elif output.exists():
            shutil.rmtree(output)


def _tracked_outside_task(root: Path) -> dict[str, bytes | None]:
    status = subprocess.run(
        ["git", "status", "--porcelain=v1", "-z", "--untracked-files=no"],
        cwd=root,
        capture_output=True,
        check=True,
    ).stdout.split(b"\0")
    changed = {}
    entries = iter(status)
    for entry in entries:
        if not entry:
            continue
        paths = [os.fsdecode(entry[3:])]
        if b"R" in entry[:2] or b"C" in entry[:2]:
            paths.append(os.fsdecode(next(entries)))
        for path in paths:
            if path not in TASK_PATHS:
                file = root / path
                changed[path] = file.read_bytes() if file.exists() else None
    return changed


def _restore_row_files(root: Path, before: dict[str, bytes | None]) -> list[str]:
    after = _tracked_outside_task(root)
    changed = sorted(
        path
        for path in before.keys() | after.keys()
        if path not in before or path not in after or before[path] != after[path]
    )
    if changed:
        subprocess.run(["git", "checkout", "--", *changed], cwd=root, check=True)
        for path in changed:
            if path in before:
                if before[path] is None:
                    (root / path).unlink(missing_ok=True)
                else:
                    (root / path).write_bytes(before[path])
    return changed


class _SelectorEnvironment:
    def __init__(
        self,
        root: Path,
        directory: Path,
        constructs: Mapping[str, Mapping[str, KvxValue]],
    ) -> None:
        self.root = root
        self.directory = directory
        self.remaining = sum(
            any(selector in SELECTOR_ENVIRONMENT for selector in row["selectors"])
            for row in constructs.values()
        )
        self.temporary = directory / "node-tmp"
        self.base = self.temporary / "lx-localnode"
        self.env_path = self.base / "wallet.env"
        self.environment = dict(os.environ, TMPDIR=str(self.temporary))
        self.values: dict[str, str] = {}
        self.record = {"starts": 0, "stops": 0, "homes": [], "removed": False}
        self.start_exit: int | None = None
        self.start_output = ""

    def __enter__(self) -> _SelectorEnvironment:
        return self

    def __exit__(self, *_args: object) -> None:
        self.close()

    def for_row(self, row: Mapping[str, KvxValue], log_path: Path) -> dict[str, str]:
        sources = {
            target: source
            for selector in row["selectors"]
            for target, source in SELECTOR_ENVIRONMENT.get(selector, {}).items()
        }
        if not sources:
            return dict(os.environ)
        if self.start_exit is None:
            self.temporary.mkdir(mode=0o700, parents=True)
            self.record["starts"] += 1
            process = subprocess.run(
                [
                    "bash",
                    "tools/qualification/localnode/v6120-node.sh",
                    "--sequencer",
                    "--env-out",
                    str(self.env_path),
                ],
                cwd=self.root,
                env=self.environment,
                capture_output=True,
                text=True,
                check=False,
            )
            self.start_exit = process.returncode
            startup_output = process.stdout + process.stderr
            if self.start_exit and "bootstrap-peer did not stop" in startup_output:
                for peer_log in sorted(self.base.glob("node.*/bootstrap-peer.log")):
                    with peer_log.open(encoding="utf-8", errors="replace") as log:
                        peer_tail = "".join(deque(log, maxlen=40))
                    startup_output += (
                        f"\n--- last forty lines of {peer_log} ---\n" + peer_tail
                    )
            # The bootstrap failure diagnostics must not disclose a signing key.
            self.start_output = re.sub(
                r"(?i)(?:0x)?[0-9a-f]{64}",
                "[redacted]",
                startup_output,
            )
            if self.start_exit == 0:
                for line in self.env_path.read_text(encoding="utf-8").splitlines():
                    assignment = shlex.split(line)
                    if len(assignment) != 1 or "=" not in assignment[0]:
                        raise ParityError("invalid local node environment assignment")
                    key, value = assignment[0].split("=", 1)
                    self.values[key] = value
        with log_path.open("a", encoding="utf-8") as log:
            log.write(self.start_output)
        if self.start_exit:
            raise subprocess.CalledProcessError(
                self.start_exit, "local sequencer bootstrap"
            )
        if any(not self.values.get(source) for source in sources.values()):
            raise ParityError(
                "local node environment is missing a declared selector value"
            )
        return dict(
            os.environ,
            **{target: self.values[source] for target, source in sources.items()},
        )

    def finish_row(self, row: Mapping[str, KvxValue]) -> None:
        if any(selector in SELECTOR_ENVIRONMENT for selector in row["selectors"]):
            self.remaining -= 1
            if self.remaining == 0:
                self.close()

    def close(self) -> None:
        for home in sorted(self.base.glob("node.*")):
            subprocess.run(
                [
                    "bash",
                    "tools/qualification/localnode/v6120-node.sh",
                    "--stop",
                    str(home),
                ],
                cwd=self.root,
                env=self.environment,
                capture_output=True,
                check=True,
            )
            self.record["stops"] += 1
            self.record["homes"].append(str(home))
            shutil.rmtree(home)
        if self.temporary.exists():
            shutil.rmtree(self.temporary)
            self.record["removed"] = True
        self.values.clear()


def _run_row(
    command: str, root: Path, environment: dict[str, str], log_path: Path
) -> int:
    secret = environment.get("WALLET_TRANSFER_TRUTH_SEQUENCER_KEY")
    with log_path.open("a", encoding="utf-8") as log, subprocess.Popen(
        ["bash", "-c", command],
        cwd=root,
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        encoding="utf-8",
        errors="replace",
    ) as process:
        assert process.stdout is not None
        for line in process.stdout:
            if secret:
                line = line.replace(secret, "[redacted]").replace(
                    secret.removeprefix("0x"), "[redacted]"
                )
            log.write(line)
        return process.wait()


def run_all(
    rows: Mapping[str, Mapping[str, KvxValue]],
    root: str | PathLike[str],
    log_dir: str | PathLike[str],
) -> dict[str, bool]:
    constructs = {
        name: row for name, row in rows.items() if name.startswith("construct.")
    }
    if not constructs:
        raise ParityError("parity inventory must contain at least one construct")
    last_go_row = next(
        (
            name
            for name, row in reversed(constructs.items())
            if any(
                selector.partition("::")[0].endswith(".go")
                for selector in row["selectors"]
            )
        ),
        None,
    )
    for name in constructs:
        if not re.fullmatch(r"construct\.[A-Za-z0-9_-]+", name):
            raise ParityError(f"invalid parity row name: {name}")
    root = Path(root).resolve()
    directory = (root / log_dir).resolve()
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    results: dict[str, bool] = {}
    native_results: dict[str, int] = {}
    cargo_target = directory / "cargo-target"
    run_record = {"revision": revision, "free_space_start": _free_space(root)}
    record_path = directory / "run.json"
    record_path.write_text(json.dumps(run_record) + "\n", encoding="utf-8")
    print(f"Parity run start:\n{run_record['free_space_start']}", flush=True)
    with (
        (directory / "evidence.log").open("w", encoding="utf-8") as ledger,
        (directory / "report.jsonl").open("w", encoding="utf-8") as report,
        (directory / "prerequisites.log").open("w", encoding="utf-8") as prerequisites,
        _SelectorEnvironment(root, directory, constructs) as selector_environment,
    ):
        for name, row in constructs.items():
            tracked_before = _tracked_outside_task(root)
            log_path = directory / f"{name}.log"
            command = f"command_for({name})"
            count = 0
            exit_code = 1
            prerequisite = ""
            free_space = _free_space(root)
            log_path.write_text(
                "# df -k of repository root\n" + free_space, encoding="utf-8"
            )
            outputs: list[Path] = []
            build_volumes: list[str] = []
            try:
                command = command_for(row)
                outputs = _row_build_outputs(row, root, cargo_target)
                selectors = row["selectors"]
                suffix = PurePosixPath(selectors[0].partition("::")[0]).suffix
                environment = selector_environment.for_row(row, log_path)
                if suffix == ".rs":
                    environment["CARGO_TARGET_DIR"] = str(cargo_target)
                if suffix == ".exs":
                    build_volumes = [_explorer_build_volume(root)]
                required = []
                if suffix == ".go":
                    required.append(
                        (
                            "native-libraries",
                            [
                                "make",
                                "build/liblayerx.a",
                                "programs/target/debug/liblayerx_programs_sandbox.a",
                            ],
                        )
                    )
                    if any(selector.startswith("node/") for selector in selectors):
                        required.append(("evmone-lib", ["make", "evmone-lib"]))
                for prerequisite_name, arguments in required:
                    native_log = directory / f"{prerequisite_name}.log"
                    native_command = shlex.join(arguments)
                    if prerequisite_name not in native_results:
                        with native_log.open("w", encoding="utf-8") as log:
                            native_results[prerequisite_name] = subprocess.run(
                                arguments,
                                cwd=root,
                                stdout=log,
                                stderr=subprocess.STDOUT,
                                check=False,
                            ).returncode
                        prerequisites.write(
                            evidence_line(
                                revision,
                                native_command,
                                native_results[prerequisite_name],
                                native_log,
                            )
                            + "\n"
                        )
                        prerequisites.flush()
                    native_exit = native_results[prerequisite_name]
                    if native_exit != 0:
                        command = native_command + " && " + command
                        with native_log.open(encoding="utf-8", errors="replace") as log:
                            native_output = "".join(deque(log, maxlen=200))
                        prerequisite = " prerequisite=" + json.dumps(
                            evidence_line(
                                revision, native_command, native_exit, native_log
                            )
                        )
                        prerequisite += " prerequisite_tail=" + json.dumps(
                            "\n".join(native_output.splitlines()[-20:])
                        )
                        with log_path.open("a", encoding="utf-8") as log:
                            log.write(
                                "Native prerequisite failed; row did not run:\n"
                                + evidence_line(
                                    revision, native_command, native_exit, native_log
                                )
                                + "\n"
                                + native_output
                            )
                        raise subprocess.CalledProcessError(
                            native_exit, native_command
                        )
                if shlex.split(command)[:2] == ["go", "test"]:
                    command = shlex.join([*shlex.split(command), "-json"])
                    count = count_go_tests(command, root, log_path=log_path)
                    exit_code = 0
                else:
                    exit_code = _run_row(command, root, environment, log_path)
                    output = re.sub(
                        r"\x1b\[[0-?]*[ -/]*[@-~]",
                        "",
                        log_path.read_text(encoding="utf-8"),
                    ).replace("\r", "\n")
                    if suffix == ".c":
                        count = len(set(selectors)) if exit_code == 0 else 0
                    elif suffix == ".exs":
                        count = sum(
                            int(tests) - int(excluded or 0) - int(skipped or 0)
                            for tests, _, excluded, skipped in re.findall(
                                r"(\d+) tests?, (\d+) failures?"
                                r"(?:, (\d+) excluded)?(?:, (\d+) skipped)?",
                                output,
                            )
                        )
                    elif suffix in {".ts", ".tsx"}:
                        count = sum(
                            int(match)
                            for match in re.findall(
                                r"^\s*(?:Tests\s+)?(\d+) passed\b", output, re.MULTILINE
                            )
                        )
                    elif suffix == ".py":
                        count = sum(
                            int(match)
                            for match in re.findall(r"\b(\d+) passed\b", output)
                        )
                    else:
                        count = sum(
                            int(match)
                            for match in re.findall(
                                r"^test result: ok\. (\d+) passed;",
                                output,
                                re.MULTILINE,
                            )
                        )
            except subprocess.CalledProcessError as error:
                exit_code = error.returncode
            except (ParityError, OSError, ValueError) as error:
                exit_code = 1
                with log_path.open("a", encoding="utf-8") as log:
                    log.write(f"{type(error).__name__}: {error}\n")
            finally:
                selector_environment.finish_row(row)
                restored_files = _restore_row_files(root, tracked_before)
            output, omitted, executed, passed, rust_executed, rust_passed = _row_output(
                log_path
            )
            expected_go = {
                selector.partition("::")[2]
                for selector in row["selectors"]
                if selector.partition("::")[0].endswith(".go")
            }
            expected_rust = {
                selector.partition("::")[2]
                for selector in row["selectors"]
                if selector.partition("::")[0].endswith(".rs")
            }
            missing_go = sorted(expected_go - set(executed))
            missing_rust = sorted(expected_rust - set(rust_executed))
            unpassed_go = sorted(expected_go - set(passed))
            unpassed_rust = sorted(expected_rust - set(rust_passed))
            results[name] = (
                exit_code == 0
                and count > 0
                and not missing_go
                and not missing_rust
                and not unpassed_go
                and not unpassed_rust
            )
            line = evidence_line(revision, command, exit_code, log_path)
            line += prerequisite
            if missing_go or missing_rust:
                line += " missing_tests=" + json.dumps(missing_go + missing_rust)
            if unpassed_go or unpassed_rust:
                line += " unpassed_tests=" + json.dumps(unpassed_go + unpassed_rust)
            line += f" tests={count}"
            report.write(
                json.dumps(
                    {
                        "row": name,
                        "selectors": row["selectors"],
                        "revision": revision,
                        "command": command,
                        "exit": exit_code,
                        "log": str(log_path),
                        "tests": count,
                        "passed": results[name],
                        "evidence": line,
                        "output": output,
                        "omitted_lines": omitted,
                        "go_tests_run": executed,
                        "go_tests_passed": passed,
                        "rust_tests_run": rust_executed,
                        "rust_tests_passed": rust_passed,
                        "missing_go_tests": missing_go,
                        "missing_rust_tests": missing_rust,
                        "unpassed_go_tests": unpassed_go,
                        "unpassed_rust_tests": unpassed_rust,
                        "free_space": free_space,
                        "removed_build_outputs": [str(path) for path in outputs],
                        "removed_build_volumes": build_volumes,
                        "restored_files": restored_files,
                        "environment_sources": {
                            selector: SELECTOR_ENVIRONMENT[selector]
                            for selector in row["selectors"]
                            if selector in SELECTOR_ENVIRONMENT
                        },
                    }
                )
                + "\n"
            )
            report.flush()
            ledger.write(line + "\n")
            ledger.flush()
            log_path.unlink()
            _remove_row_outputs(outputs)
            if name == last_go_row:
                cleanup_command = ["go", "clean", "-cache"]
                cleanup_log = directory / "go-cache-clean.log"
                with cleanup_log.open("w", encoding="utf-8") as log:
                    cleanup = subprocess.run(
                        cleanup_command,
                        cwd=root,
                        stdout=log,
                        stderr=subprocess.STDOUT,
                        check=False,
                    )
                run_record["go_cache_cleanup"] = {
                    "after_row": name,
                    "evidence": evidence_line(
                        revision,
                        shlex.join(cleanup_command),
                        cleanup.returncode,
                        cleanup_log,
                    ),
                }
                record_path.write_text(json.dumps(run_record) + "\n", encoding="utf-8")
                cleanup.check_returncode()
            for volume in build_volumes:
                subprocess.run(
                    ["docker", "volume", "rm", volume],
                    capture_output=True,
                    text=True,
                    check=True,
                )
            print(f"{name}: {line} report={directory / 'report.jsonl'}", flush=True)
    run_record["free_space_end"] = _free_space(root)
    run_record["selector_environment"] = selector_environment.record
    record_path.write_text(json.dumps(run_record) + "\n", encoding="utf-8")
    print(f"Parity run end:\n{run_record['free_space_end']}", flush=True)
    return results


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


def _coverage_selectors(value: KvxValue) -> list[str]:
    if not isinstance(value, str):
        raise ParityError("acceptance and task selectors must be strings")
    return [
        re.sub(r" \((?:planned|existing)(?:, [^)]*)?\)$", "", selector.strip())
        for selector in re.split(r";\s+(?=[A-Za-z0-9_./-]+::)", value)
    ]


def _is_test_source(path: Path) -> bool:
    return (
        path.name.endswith("_test.go")
        or (path.suffix == ".rs" and not {"tests", "src"}.isdisjoint(path.parts))
        or path.suffix == ".exs"
        or path.name.endswith((".test.ts", ".test.tsx", ".test.mjs", ".test.js"))
        or (path.name.startswith("test_") and path.suffix == ".py")
        or path.name.endswith("Tests.swift")
    )


def _selector_present(path: Path, name: str, text: str) -> bool:
    if not name:
        return False
    if path.suffix == ".py" and "." in name:
        return all(
            component and re.search(rf"(?<!\w){re.escape(component)}(?!\w)", text)
            for component in name.split(".")
        )
    if path.suffix in {".exs", ".ts", ".tsx", ".mjs", ".js"}:
        if name in text:
            return True
        descriptions: list[str] = []
        for match in re.finditer(
            r"\b(?P<kind>describe|test|it)(?:\.\w+)*\s*\(?\s*"
            r"(?P<quote>[\"'`])(?P<title>(?:\\.|(?!(?P=quote))[^\\])*)(?P=quote)",
            text,
        ):
            title = match["title"]
            if match["kind"] == "describe":
                descriptions.append(title)
            elif any(f"{description} {title}" == name for description in descriptions):
                return True
        return False
    if "." in name:
        return all(
            component and re.search(rf"(?<!\w){re.escape(component)}(?!\w)", text)
            for component in name.split(".")
        )
    return re.search(rf"(?<!\w){re.escape(name)}(?!\w)", text) is not None


def _coverage_dispositions() -> dict[tuple[str, str], str | None]:
    dispositions: dict[tuple[str, str], str | None] = {}
    for section, entry in load(Path(__file__).with_name("parity-stale.kvx")).items():
        if not section.startswith("stale."):
            continue
        acceptance, file, test = (entry[key] for key in ("acceptance", "file", "test"))
        if not all(isinstance(value, str) for value in (acceptance, file, test)):
            raise ParityError(f"[{section}] disposition identity must contain strings")
        selector = file + (f"::{test}" if test else "")
        normalized = _coverage_selectors(selector)
        if len(normalized) != 1:
            raise ParityError(f"[{section}] disposition must name one selector")
        identity = (acceptance, normalized[0])
        if identity in dispositions:
            raise ParityError(f"[{section}] duplicate disposition for {identity}")
        if entry["disposition"] == "retired":
            reason = entry.get("reason")
            if not isinstance(reason, str) or not reason.strip():
                raise ParityError(f"[{section}] retired selector needs a reason")
            dispositions[identity] = None
        elif entry["disposition"] == "replacement":
            replacement = entry.get("replacement")
            if not isinstance(replacement, str) or not replacement.strip():
                raise ParityError(f"[{section}] replacement selector must be nonempty")
            selectors = _coverage_selectors(replacement)
            if len(selectors) != 1:
                raise ParityError(f"[{section}] replacement must name one selector")
            dispositions[identity] = selectors[0]
        else:
            raise ParityError(f"[{section}] unknown selector disposition")
    return dispositions


def acceptance_coverage(
    spec_path: str | PathLike[str],
) -> tuple[list[str], list[str]]:
    sections = load(spec_path)
    selector_owners: dict[str, set[str]] = {}
    requirement_owners: dict[str, set[str]] = {}
    statuses: dict[str, KvxValue] = {}
    for section, task in sections.items():
        if not section.startswith("task."):
            continue
        statuses[section] = task["status"]
        reqs = task["reqs"]
        if not isinstance(reqs, list):
            raise ParityError(f"[{section}] reqs must be a list of requirements")
        for req in reqs:
            requirement_owners.setdefault(req, set()).add(section)
        tests = task.get("tests")
        if not isinstance(tests, list):
            raise ParityError(f"[{section}] tests must be a list of selectors")
        for test in tests:
            for selector in _coverage_selectors(test):
                selector_owners.setdefault(selector, set()).add(section)

    dispositions = _coverage_dispositions()
    missing: list[str] = []
    stale: list[str] = []
    sources: dict[Path, str] = {}
    for section, acceptance in sections.items():
        if not section.startswith("acceptance."):
            continue
        for selector in _coverage_selectors(acceptance["test"]):
            identity = (section, selector)
            effective = dispositions.get(identity, selector)
            if effective is None:
                continue
            held = requirement_owners.get(acceptance.get("req"), set())
            named = selector_owners.get(effective, set())
            owners = named | selector_owners.get(selector, set()) | held
            ownership = ", ".join(
                f"{owner} ({statuses[owner]})" for owner in sorted(owners)
            )
            description = selector
            if effective != selector:
                description += f" -> {effective}"
            diagnostic = f"[{section}] {description}; owners: {ownership or 'none'}"
            command = (
                re.match(r"^(?:make|cargo|grep|go)(?:\s|$)", effective) is not None
                or re.search(r"\s+\([^()]*\bgate\b[^()]*\)$", effective) is not None
            )
            path, separator, name = effective.partition("::")
            path = re.sub(r"\s+\([^()]*\)$", "", path)
            source = ROOT / path
            present = source.exists()
            completed = bool(held) and all(
                statuses[owner] in {"done", "superseded"} for owner in held
            )
            if not (
                named
                or (completed and (present or command))
                or (identity in dispositions and present)
            ):
                missing.append(diagnostic)
            if (
                not separator
                or command
                or any(statuses[owner] != "done" for owner in held)
            ):
                continue
            if source.is_file() and _is_test_source(source):
                if source not in sources:
                    sources[source] = source.read_text(encoding="utf-8")
                test_name = name.rsplit("::", 1)[-1]
                present = _selector_present(source, test_name, sources[source])
            if not present:
                stale.append(diagnostic)
    return list(dict.fromkeys(missing)), stale


def uncovered(spec_path: str | PathLike[str]) -> list[str]:
    missing, stale = acceptance_coverage(spec_path)
    return list(dict.fromkeys([*missing, *stale]))


def _doc_text(value: KvxValue) -> str:
    return str(value).replace("|", "\\|").replace("\n", "<br>")


def _doc_refs(values: KvxValue) -> str:
    if not isinstance(values, list):
        raise ParityError("document references must be a list of strings")
    return "; ".join(f"`{_doc_text(value)}`" for value in values) or "none"


def render_doc(rows: Mapping[str, Mapping[str, KvxValue]]) -> str:
    """Render the parity inventory from the KVX rows in source order."""
    entries = [
        row for name, row in rows.items() if name.startswith(("construct.", "dropped."))
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
