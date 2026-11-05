"""Build and qualify the retained Programs transaction, precompile and explorer path."""

import importlib.util
import hashlib
import json
import os
from pathlib import Path
import re
import ssl
import subprocess
import sys
import time
from urllib.parse import urlsplit
from urllib.request import Request, urlopen

_spec = importlib.util.spec_from_file_location(
    "program_build_evidence", Path(__file__).with_name("hosted_testnet_build.py")
)
evidence = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(evidence)
require = evidence.require
SCHEMA = "layerx.native-program-operations-build.v1"
ADDRESS = "0x000000000000000000000000000000000000101d"
SOURCES = [
    "tools/platform/cli/Cargo.toml", "tools/platform/cli/src/programs.rs",
    "tools/platform/cli/tests/program_evm_calls.rs", "tools/platform/cli/tests/program_evm_registry.rs",
    "services/platform/hosted/gateway/src/lib.rs", "services/platform/hosted/gateway/src/main.rs",
    "precompiles/layerxprograms/method_call.go", "precompiles/layerxprograms/replay_test.go",
    "rpc/raw_tx.go", "rpc/sequencer_receipt.go",
    "services/platform/examples/agent-program-call/verify-receipt.mjs",
    "services/platform/examples/support/runtime.mjs",
    "services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/native_settlement.ex",
    "services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/sequencer_receipt.ex",
    "services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/committed_vectors_test.exs",
    "apps/web/src/explorer/client.ts", "apps/web/src/explorer/model.ts",
    "apps/web/src/explorer/program-contract.test.ts", "tools/paxeer-x/gates/104.33.7.sh",
    "tools/paxeer-x/gates/program_operations.py", "tools/paxeer-x/gates/hosted_profile.py",
    "tools/paxeer-x/gates/compiler_manifest.py",
]
RUST_TARGETS = {
    "gateway_lib": ("services/platform/hosted/gateway/Cargo.toml", "layerx_platform_gateway", True),
    "gateway_bin": ("services/platform/hosted/gateway/Cargo.toml", "layerx-gateway", True),
    "cli_calls": ("tools/platform/cli/Cargo.toml", "program_evm_calls", True),
    "cli_registry": ("tools/platform/cli/Cargo.toml", "program_evm_registry", True),
    "cli": ("tools/platform/cli/Cargo.toml", "layerx", False),
}
EXPLORER = "services/explorer/backend"
EXPLORER_TEST = "apps/explorer/test/explorer/chain/paxeer_x/committed_vectors_test.exs"


def commands(directory):
    return {
        "gateway": ["cargo", "test", "--locked", "--manifest-path", "services/platform/Cargo.toml",
                    "-p", "layerx-platform-gateway", "--lib", "--bin", "layerx-gateway",
                    "--no-run", "--message-format=json"],
        "cli": ["cargo", "test", "--locked", "--manifest-path", "tools/platform/Cargo.toml",
                "-p", "layerx-platform-cli", "--test", "program_evm_calls", "--test", "program_evm_registry",
                "--no-run", "--message-format=json"],
        "precompile": ["go", "test", "-c", "-o", str(directory / "layerxprograms.test"),
                       "./precompiles/layerxprograms"],
        "explorer": ["mix", "compile", "--warnings-as-errors"],
    }


def rust_artifacts(root, records):
    selected = {}
    for name, (manifest, target, test) in RUST_TARGETS.items():
        rows = [row for row in records if row.get("reason") == "compiler-artifact"
                and row.get("manifest_path") == str(root / manifest)
                and row.get("target", {}).get("name") == target
                and row.get("profile", {}).get("test") is test and row.get("executable")]
        require(len(rows) == 1, "exact compiler artifact required: " + name)
        selected[name] = rows[0]
    return selected


def beams(directory):
    result = {str(path.relative_to(directory)): evidence.digest(path)
              for path in sorted(directory.rglob("*.beam")) if path.is_file() and not path.is_symlink()}
    require(result, "actual compiled native explorer artifacts required")
    return result


def build(root, directory):
    require(directory.is_absolute() and not directory.resolve().is_relative_to(root),
            "private compiler evidence outside checkout required")
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    evidence.private(directory, True)
    before = evidence.source_identity(root)
    logs, records = {}, []
    for name, argv in commands(directory).items():
        path = directory / (name + ".log")
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w") as stream:
            result = subprocess.run(argv, cwd=root / EXPLORER if name == "explorer" else root,
                                    env=dict(os.environ, CARGO_TARGET_DIR=str(directory / "target"),
                                             MIX_ENV="test", MIX_BUILD_PATH=str(directory / "explorer-build")),
                                    stdin=subprocess.DEVNULL, stdout=stream, stderr=subprocess.STDOUT,
                                    timeout=1800)
        require(result.returncode == 0, "native Programs compiler failed; see " + str(path))
        logs[name] = {"path": str(path), "sha256": evidence.digest(path), "command": argv, "exit": result.returncode}
        if name in {"gateway", "cli"}:
            records.extend(json.loads(line) for line in path.read_text().splitlines() if line.startswith("{"))
    selected = rust_artifacts(root, records)
    artifacts = {name: evidence.artifact(row["executable"]) for name, row in selected.items()}
    artifacts["precompile"] = evidence.artifact(directory / "layerxprograms.test")
    compiled = beams(directory / "explorer-build")
    require(evidence.source_identity(root) == before, "source changed during native Programs build")
    manifest = directory / "program-operations-build.json"
    evidence.save(manifest, {"schema": SCHEMA, "identity": before, "logs": logs,
                             "compiler_artifacts": selected, "artifacts": artifacts,
                             "explorer_beams": compiled})
    return manifest


def verify(root, manifest):
    require(manifest.is_absolute() and not manifest.resolve().is_relative_to(root),
            "private compiler manifest outside checkout required")
    built = evidence.load(manifest)
    require(built["schema"] == SCHEMA and built["identity"] == evidence.source_identity(root),
            "current revision/tree/source closure required")
    directory = manifest.parent
    require(set(built["logs"]) == set(commands(directory)), "complete compiler command set required")
    records = []
    for name, argv in commands(directory).items():
        log = built["logs"][name]
        evidence.private(Path(log["path"]))
        require(type(log["exit"]) is int and log["exit"] == 0 and log["command"] == argv
                and evidence.digest(log["path"]) == log["sha256"], "compiler command/transcript changed")
        if name in {"gateway", "cli"}:
            records.extend(json.loads(line) for line in Path(log["path"]).read_text().splitlines()
                           if line.startswith("{"))
    selected = rust_artifacts(root, records)
    require(selected == built["compiler_artifacts"], "compiler artifact records changed")
    require(set(built["artifacts"]) == {*RUST_TARGETS, "precompile"}, "complete native artifact set required")
    for name, row in selected.items():
        evidence.checked_artifact(built["artifacts"][name], row["executable"])
    evidence.checked_artifact(built["artifacts"]["precompile"], directory / "layerxprograms.test")
    require(beams(directory / "explorer-build") == built["explorer_beams"], "native explorer compiler artifacts changed")
    require(all((root / name).is_file() for name in SOURCES), "retained source closure required")
    return built


def loopback_origin(value):
    origin = urlsplit(value)
    require(origin.scheme in {"http", "https"} and origin.hostname in {"localhost", "127.0.0.1", "::1"}
            and origin.path in {"", "/"} and not origin.username and not origin.password
            and not origin.query and not origin.fragment, "disposable root node JSON-RPC required")
    return value.rstrip("/")


def rpc(origin, method, params):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    request = Request(origin, data=body, headers={"Content-Type": "application/json"}, method="POST")
    context = ssl.create_default_context(cafile=os.environ.get("LAYERX_GATEWAY_CA_FILE"))
    with urlopen(request, context=context, timeout=30) as response:
        raw = response.read(2097153)
        require(len(raw) <= 2097152, "bounded node response required")
    value = json.loads(raw)
    require(value.get("jsonrpc") == "2.0" and type(value.get("id")) is int and value["id"] == 1
            and (("error" in value) != ("result" in value)), "matching JSON-RPC response required")
    return value


def result(origin, method, params):
    value = rpc(origin, method, params)
    require("error" not in value, "node refused " + method)
    return value["result"]


def committed(origin, tx_hash, transaction):
    require(isinstance(transaction, dict) and transaction.get("hash", "").lower() == tx_hash.lower()
            and transaction.get("to", "").lower() == ADDRESS
            and transaction.get("input", "").startswith("0x54bd2b48"), "actual Programs EVM transaction required")
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        receipt = result(origin, "eth_getTransactionReceipt", [tx_hash])
        if receipt is not None:
            require(receipt.get("transactionHash", "").lower() == tx_hash.lower()
                    and receipt.get("to", "").lower() == ADDRESS and receipt.get("status") == "0x1",
                    "successful matching native transaction receipt required")
            height = receipt.get("blockNumber")
            require(isinstance(height, str) and int(height, 16) > 0, "committed block height required")
            block = result(origin, "eth_getBlockByNumber", [height, False])
            require(block is not None and block.get("hash") == receipt.get("blockHash")
                    and block.get("number") == height and tx_hash in block.get("transactions", [])
                    and int(result(origin, "eth_blockNumber", []), 16) >= int(height, 16),
                    "canonical committed block membership required")
            return receipt
        time.sleep(0.2)
    raise RuntimeError("transaction has no committed receipt; qualification refused")


def submission_refusal(value, kind):
    error = value.get("error")
    require(isinstance(error, dict) and type(error.get("code")) is int
            and error["code"] in {-32000, -32602} and "result" not in value,
            kind + " must have a genuine transaction refusal, not an unavailable RPC method")
    pattern = r"nonce|already|duplicate|replay" if kind == "duplicate" else r"rlp|short|length|decode|input|transaction"
    require(isinstance(error.get("message"), str) and re.search(pattern, error["message"], re.I),
            kind + " transaction refusal reason required")


def run(argv, root, timeout=600, env=None):
    output = subprocess.run([str(arg) for arg in argv], cwd=root, capture_output=True, text=True,
                            stdin=subprocess.DEVNULL, timeout=timeout, env=env)
    sys.stdout.write(output.stdout)
    sys.stderr.write(output.stderr)
    require(output.returncode == 0, "actual Programs acceptance command failed")
    return output.stdout


def rust_suite(binary, selector, root):
    names = set(re.findall(r"^(.+): test$", run([binary, selector, "--list"], root, 30), re.M))
    require(names, "real Programs scoped corpus empty")
    output = run([binary, selector, "--nocapture", "--test-threads=1"], root)
    summary = re.findall(r"test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;", output)
    require(len(summary) == 1 and tuple(map(int, summary[0])) == (len(names), 0, 0),
            "complete scoped Programs corpus required without ignored tests")
    return len(names)


def qualify(root, inputs):
    manifest = Path(os.environ["PAXEER_X_HOSTED_PROGRAM_ARTIFACTS"])
    built = verify(root, manifest)
    artifacts = {name: Path(row["path"]) for name, row in built["artifacts"].items()}
    tests = 0
    for name, selector in (("gateway_lib", "tests::production_route_refuses_every_program_and_activity_route"),
                           ("gateway_bin", "programs_wire_tests::"), ("cli_calls", ""), ("cli_registry", "")):
        tests += rust_suite(artifacts[name], selector, root)
    names = set(run([artifacts["precompile"], "-test.list", "^TestPrograms|^TestProofLayerxprograms"], root, 30).splitlines())
    require({"TestProgramsReplayedActivityReturnsStoredReceipt", "TestProgramsSubmitKernelRefusalReverts",
             "TestProgramsCallReturnsTerminalOutcome", "TestProofLayerxprograms0x101d"} <= names,
            "real precompile call, replay, refusal and proof cases required")
    output = run([artifacts["precompile"], "-test.v", "-test.run", "^TestPrograms|^TestProofLayerxprograms"], root)
    passed = set(re.findall(r"^--- PASS: (\S+)", output, re.M))
    require(passed == names and "--- SKIP" not in output, "complete real precompile corpus required")
    tests += len(names)
    output = run(["mix", "test", "--no-compile", EXPLORER_TEST], root / EXPLORER,
                 env=dict(os.environ, MIX_ENV="test", MIX_BUILD_PATH=str(manifest.parent / "explorer-build")))
    summary = re.findall(r"(\d+) tests?, (\d+) failures?", output)
    require(len(summary) == 1 and summary[0] == ("1", "0") and not re.search(r"excluded|skipped", output, re.I),
            "real committed native explorer consumer required without skips")
    tests += 1
    output = run(["node", "--test", "apps/web/src/explorer/program-contract.test.ts"], root, 120)
    count = re.findall(r"^# tests (\d+)$", output, re.M)
    require(count == ["4"] and "# fail 0" in output and "# skipped 0" in output,
            "complete explorer program consumer corpus required")
    tests += 4
    supplied = evidence.load(Path(os.environ["PAXEER_X_PROGRAM_BINDINGS_RESULT"]))
    require(supplied["schema"] == "paxeer-x.program-bindings-result.v1" and supplied["tests"] > 0
            and supplied["skipped"] == 0 and supplied["source"]["revision"] == built["identity"]["revision"],
            "existing genuine current seven-language binding result required")
    plan_path = Path(supplied["manifest"]["path"])
    plan = evidence.load(plan_path)
    require(evidence.digest(plan_path) == supplied["manifest"]["sha256"], "binding result provenance changed")
    require({"rust", "typescript", "python", "go", "java", "swift", "csharp"} <= set(plan["languages"]),
            "all seven SDK languages required")
    for row in plan["sdk_outputs"] + plan["consumer_sources"]:
        require(evidence.digest(row["path"]) == row["sha256"], "binding source/artifact changed")
    tests += supplied["tests"]
    tests += connected(root, artifacts["cli"], inputs)
    verify(root, manifest)
    print("PAXEER_X_GATE tests=" + str(tests) + " skipped=0")


def connected(root, cli, inputs):
    # Only the closed, revision-bound private profile supplies connected inputs.
    os.environ.update(inputs)
    origin = loopback_origin(inputs["LAYERX_PROGRAM_RPC_URL"])
    require(result(origin, "eth_chainId", []) == "0x7d", "Paxeer chain identity required")
    fixture = evidence.load(Path(inputs["LAYERX_PROGRAM_CALL_ARGUMENTS"]))
    program = fixture["program"]
    require(re.fullmatch("[0-9a-f]{64}", program), "canonical program id required")
    key_file = Path(fixture["evm_key_file"])
    evidence.private(key_file)
    options = fixture["options"]
    allowed = {"--fuel", "--calldata", "--fee-limit", "--capability", "--access-declaration",
               "--abi-version", "--entrypoint", "--response-capacity", "--memory-bytes",
               "--storage-read-bytes", "--storage-write-bytes", "--output-values", "--output-bytes", "--table-elements"}
    require(isinstance(options, list) and len(options) % 2 == 0
            and all(isinstance(value, str) for value in options)
            and all(options[index] in allowed and not options[index + 1].startswith("--")
                    for index in range(0, len(options), 2)), "explicit bounded native call options required")
    base = [cli, "--json", "--rpc", origin, "program"]
    # These CLI commands call the retained layerxprograms views and EVM producer.
    discovery = json.loads(run(base + ["registry", "get", program], root))["data"]
    interface = json.loads(run(base + ["interface", "get", program], root))["data"]
    require(discovery["program_id"] == program and discovery["active"] is True
            and interface["program_id"] == program and interface["code_hash"] == discovery["code_hash"],
            "code-bound retained program reads required")
    require(hashlib.sha256(bytes.fromhex(interface["interface"])).hexdigest() == interface["interface_digest"],
            "interface digest mismatch")
    simulated = json.loads(run(base + ["simulate", program, *options], root))["data"]
    require(simulated["outcome"]["terminal_result"] == 0 and "tx_hash" not in simulated,
            "successful noncommitting eth_call simulation required")
    called = json.loads(run(base + ["call", program, "--evm-key-file", key_file, *options], root))["data"]
    require(called["rung"] == "submitted" and re.fullmatch(r"0x[0-9a-fA-F]{64}", called["tx_hash"]),
            "actual EVM transaction producer result required")
    tx_hash = called["tx_hash"]
    transaction = result(origin, "eth_getTransactionByHash", [tx_hash])
    require(isinstance(transaction, dict) and transaction.get("input", "")[10:74] == program,
            "committed transaction must name the requested program")
    receipt = committed(origin, tx_hash, transaction)
    # The retained SDK consumer authenticates the signed receipt. The canonical
    # block check above separately establishes commitment on this BFT node.
    keys = fixture["receipt_keys"]
    require(isinstance(keys, list) and keys, "independently pinned receipt key roster required")
    verified = run(["node", "services/platform/examples/agent-program-call/verify-receipt.mjs", tx_hash], root,
                   env=dict(os.environ, LAYERX_RPC_URL=origin, LAYERX_RECEIPT_KEYS_JSON=json.dumps(keys)))
    require(verified.strip() == "receipted", "independent genuine signed receipt verification required")
    raw = result(origin, "eth_getRawTransactionByHash", [tx_hash])
    require(isinstance(raw, str) and raw.startswith("0x02") and len(raw) > 4, "actual signed EIP-1559 bytes required")
    replay = rpc(origin, "eth_sendRawTransaction", [raw])
    require("error" in replay and type(replay["error"].get("code")) is int,
            "duplicate committed transaction must refuse")
    submission_refusal(replay, "duplicate")
    # Truncating the genuine signed envelope cannot encode a valid transaction.
    tampered = rpc(origin, "eth_sendRawTransaction", [raw[:-2]])
    require("error" in tampered and type(tampered["error"].get("code")) is int,
            "tampered signed envelope must refuse")
    submission_refusal(tampered, "tampered")
    require(committed(origin, tx_hash, transaction) == receipt, "duplicate/tamper changed committed receipt")
    evidence_dir = Path(os.environ["PAXEER_X_EVIDENCE_DIR"])
    evidence.save(evidence_dir / "104.33.7-committed-receipt.json", receipt)
    return 9


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[3]
    try:
        if len(sys.argv) == 3 and sys.argv[1] == "--build":
            print(build(root, Path(sys.argv[2])))
        elif len(sys.argv) == 1:
            from hosted_profile import load_native
            revision = evidence.git(root, "rev-parse", "HEAD").decode().strip()
            inputs = load_native(os.environ["PAXEER_X_HOSTED_PROGRAM_PROFILE"], revision, root)
            qualify(root, inputs)
        else:
            raise RuntimeError("usage: program_operations.py [--build PRIVATE_DIRECTORY]")
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("native Programs qualification refused: " + str(error), file=sys.stderr)
        sys.exit(78)
