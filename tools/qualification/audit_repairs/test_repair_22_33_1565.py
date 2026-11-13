"""Offline source proofs for audit 22.30.99's agent documentation and clock policy.

These tests read production inputs; they do not execute Rust or certify a node.
Out-of-scope clock violations remain violations and require named owner wiring.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import struct
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
AGENT = ROOT / "services/agent"
RECORD = Path(__file__).with_name("repair_22_33_1565.kvx")


def read(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(text, name):
    match = re.search(
        rf"^\[{re.escape(name)}\]\s*\n(.*?)(?=^\[|\Z)", text, re.M | re.S
    )
    assert match, f"missing section {name}"
    return match[1]


def value(text, name, key):
    body = section(text, name)
    match = re.search(rf"^{re.escape(key)}\s*=\s*(.+)$", body, re.M)
    assert match, f"missing {name}.{key}"
    return tomllib.loads("value = " + match[1])["value"]


def recipe(target):
    makefile = read("Makefile")
    match = re.search(
        rf"^{re.escape(target)}:[^\n]*\n((?:\t[^\n]*\n)+)", makefile, re.M
    )
    assert match, f"missing Makefile recipe for {target}"
    return match[1]


def test_agent_compatibility_major_matches_schema_manifest_and_publication():
    document = read("docs/agent/COMPATIBILITY.md")
    schema = read("services/agent/schema/agent-api/v1.kvx")
    executable = read("services/agent/crates/layerx-agentd/src/compat.rs")
    manifest = tomllib.loads(read("services/agent/Cargo.toml"))
    major = value(schema, "schema", "major")
    minor = value(schema, "schema", "minor")
    node_major = value(schema, "schema", "node_interface_major")
    version = manifest["workspace"]["package"]["version"]
    assert (major, minor, node_major) == (2, 0, 1)
    assert f"pub const CONTRACT_VERSION: u16 = {major};" in executable
    assert f'pub const SDK_VERSION: &str = "{version}";' in executable
    assert 'pub const DAEMON_VERSION: &str = env!("CARGO_PKG_VERSION");' in executable
    assert f"node_interface_major: {node_major}," in executable
    assert "minimum_node_interface_minor: 0," in executable
    assert "maximum_node_interface_minor: u16::MAX," in executable
    rows = re.findall(r"^\| (\d[^\n]+) \|$", document, re.M)
    assert rows == [f"{version} | {node_major}.0+ | {major} | {version}"]
    for carrier in ("request", "response"):
        encoded = bytes.fromhex(value(schema, f"golden.{carrier}.version", "encoded_hex"))
        assert struct.unpack_from(">HH", encoded) == (major, minor)
    assert "**2.0**" in document
    assert "historical additive changes to major 1" in document
    assert "SDKs reject an unsupported contract major" in document
    assert "web3_clientVersion" in document and "eth_chainId" in document
    assert "does not negotiate an independent\nnode protocol" in document
    assert "client_major(node_client_version)" in executable
    assert "line.trim() == expected" in executable


def test_agent_compatibility_signatures_rungs_and_unavailable_features():
    document = read("docs/agent/COMPATIBILITY.md")
    write = read("services/agent/schema/agent-api/write.kvx")
    errors = read("services/agent/schema/agent-api/errors.kvx")
    assert value(write, "module", "contract_major") == 2
    assert value(write, "operation.sign", "signature_encoding") == (
        "130 lowercase hex characters, 65 bytes"
    )
    signature = value(write, "type.McpExternalSignatureV1", "signature")
    for fact in ("65-byte secp256k1", "r || s || v", "130 lowercase hex characters"):
        assert fact in signature
        assert fact in document.replace("\n", " ")
    assert value(write, "operation.prepare", "response_required") == [
        "preparation_ref", "unsigned_canonical_bytes", "signing_preimage", "disclosure", "expiry"
    ]
    assert "unsigned canonical bytes, signing preimage and disclosure" in document
    assert "UnavailableCapability" in value(errors, "type.ErrorClass", "variants")
    assert "does not emulate" in value(
        errors, "type.ErrorClass", "UnavailableCapability.semantics"
    )
    assert "including for unavailable approval features" in document
    assert "must not emulate the capability or fall back to contract 1.0 or 1.1" in document
    rungs = value(errors, "type.Rung", "wire_names")
    assert rungs == ["submitted", "receipted", "committed"]
    assert " to ".join(f"`{rung}`" for rung in rungs) in document
    assert "Finality comes from the committed\nblock of the single network" in document
    assert "Approval holds are daemon-enforced" in document
    assert "confer no protocol authority" in document


def test_agent_compatibility_release_checks_match_real_make_consumers():
    document = read("docs/agent/COMPATIBILITY.md")
    commands = recipe("agent-test-agentd-migration")
    assert "make agent-test-agentd-migration" in document
    assert "--locked -p layerx-agentd --test migration" in commands
    assert "tools/runtime/run-with-clock.sh" in commands
    assert commands.index("--test migration") < commands.index("$(MAKE) agent-test-boundary")
    migration = read("services/agent/crates/layerx-agentd/tests/migration.rs")
    assert "assert_eq!(verify_published(), Ok(()));" in migration
    for refusal in ("UnsupportedNodeInterface", "UnsupportedContract", "UnsupportedSdk"):
        assert f"Err(CompatibilityError::{refusal})" in migration
    boundary = recipe("agent-test-boundary")
    assert 'LAYERX_QUALIFY_NODE_RPC_URL="$(AGENT_NODE_RPC_URL)"' in boundary
    assert "--check-report" in boundary
    assert "AGENT_NODE_RPC_URL ?= http://127.0.0.1:8545" in read("Makefile")
    assert "LAYERX_QUALIFY_NODE_RPC_URL" in document
    assert "http://127.0.0.1:8545" in document


def test_agent_readme_topology_crates_and_root_rpc_match_current_inputs():
    document = read("services/agent/README.md")
    assert "one BFT network, run by `paxd`" in document
    assert "A receipt is final when its block is committed" in document
    assert "Sub-chains are tenant\ndomains in the database" in document
    assert "Signed transactions are submitted as exact bytes" in " ".join(document.split())
    assert "EVM chain ID is **125** (`0x7d`)" in document
    endpoint = re.search(r"\]\((https://api-mainnet-beta[^)]+)\)", document)
    assert endpoint
    parsed = urlsplit(endpoint[1])
    assert (parsed.scheme, parsed.hostname, parsed.path) == (
        "https", "api-mainnet-beta.paxeer.network", "/"
    )
    assert not parsed.query and not parsed.fragment
    assert "api1.mainnet-beta.paxeer.network" in document
    assert "api14.mainnet-beta.paxeer.network" in document
    assert "`node.hyperpaxeer.com` is only the hpx library mirror" in document
    manifest = tomllib.loads(read("services/agent/Cargo.toml"))
    members = manifest["workspace"]["members"]
    table_names = re.findall(r"^\| `([^`]+)` \|", document, re.M)
    assert table_names == [Path(member).name for member in members]
    for member in members:
        package = tomllib.loads((AGENT / member / "Cargo.toml").read_text())
        assert package["package"]["name"] == Path(member).name
    host = tomllib.loads(read("services/agent/crates/layerx-agentd-host/Cargo.toml"))
    binary = next(entry for entry in host["bin"] if entry["name"] == "layerx-agentd")
    assert binary["path"] == "../layerx-agentd/src/main.rs"
    assert "Builds the `layerx-agentd` binary" in document
    toolchain = tomllib.loads(read("rust-toolchain.toml"))["toolchain"]["channel"]
    assert manifest["workspace"]["package"]["rust-version"] == toolchain
    assert f"Rust {toolchain}" in document
    assert manifest["workspace"]["lints"]["rust"]["unsafe_code"] == "deny"
    assert not re.search(r"\b(layerxd|LNI|stable-abi-allowlist|layerx-verifier)\b", document)


def test_agent_readme_prerequisites_follow_make_and_real_node_script():
    document = read("services/agent/README.md")
    script = read("services/agent/tools/run-real-node-tests.sh")
    invocation = re.search(r"```sh\n(sh services/agent/tools/run-real-node-tests.sh test)\n```", document)
    assert invocation
    assert shlex.split(invocation[1]) == [
        "sh", "services/agent/tools/run-real-node-tests.sh", "test"
    ]
    assert "sh services/agent/tools/run-real-node-tests.sh test" in document
    for mode, target in (("test", "agent-test"), ("sanitizers", "agent-test-sanitize")):
        assert f"{mode}) test_target={target} ;;" in script
    assert 'prepare) test_target= ;;' in script
    assert '[ "$(id -u)" -ne 0 ]' in script
    assert script.index('for executable in forge cast anvil go; do') < script.index('make "$test_target"')
    assert 'command -v "$executable" >/dev/null' in script
    assert 'python3 -c "from Crypto.Hash import keccak;' in script
    assert "requires root" in document
    for tool in ("Go", "forge", "cast", "anvil", "C compiler", "Make"):
        assert tool in document
    producer = re.search(r"python3 - <<'PY'\n(.*?)\nPY", script, re.S)
    assert producer
    tree = ast.parse(producer[1])
    assert any(isinstance(node, ast.FunctionDef) and node.name == "bounded_integer" for node in tree.body)
    for fact in ('manifest["chain_id"]', 'manifest["ports"][0]', 'manifest["chain_rpc"]', 'chain_id = 125'):
        assert fact in producer[1]
    assert "PAXEER_X_RUNTIME_MANIFEST" in document
    assert "LAYERX_TEST_PAXEER_RPC_PORT" in document
    assert "chain ID 125" in document
    for fixture in ("lxp_test_program_admission", "lxp_test_metered_allowance"):
        assert fixture in script
    assert "make paxeer-build" in script
    assert "run-real-node-tests.sh prepare" in recipe("agent-test-native-prerequisites")
    workspace_test = recipe("agent-test")
    assert "--locked --workspace" in workspace_test
    assert "run-with-clock.sh" in workspace_test
    assert "LAYERX_RUNTIME_CLOCK_BIN" in workspace_test
    assert "LAYERX_PAXEER_BOUNDARY_BIN" in workspace_test
    assert "layerx-runtime-clock" in recipe("public-tls-test-prerequisites")
    for target in ("agent-build", "agent-lint", "agent-check", "agent-test-mcp-scope", "agent-test-mcp-readonly", "agent-test-sdk-rust"):
        assert target in document
        assert recipe(target)


def test_agent_documentation_relative_links_resolve():
    for path in (ROOT / "docs/agent/COMPATIBILITY.md", AGENT / "README.md"):
        name = path.relative_to(ROOT).as_posix()
        document = path.read_text()
        links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", document)
        assert links
        for link in links:
            parsed = urlsplit(link)
            if parsed.scheme:
                assert parsed.scheme == "https"
                continue
            target = (path.parent / parsed.path).resolve()
            assert target.is_relative_to(ROOT)
            assert target.exists(), f"{name}: unresolved {link}"
            if parsed.fragment:
                headings = re.findall(r"^#+ (.+)$", target.read_text(), re.M)
                anchors = [re.sub(r"[^\w -]", "", title.lower()).replace(" ", "-") for title in headings]
                assert parsed.fragment in anchors, f"{name}: unresolved anchor {link}"


def test_clock_policy_preserves_exact_process_exception_and_scanner_scope():
    policy_text = read("services/agent/nondeterminism-allowlist.toml")
    policy = tomllib.loads(policy_text)
    assert policy["version"] == 1
    assert set(policy) == {"version", "exceptions"}
    assert len(policy["exceptions"]) == 1
    exception = policy["exceptions"][0]
    assert exception["path"] == "crates/layerx-agentd/src/main.rs"
    assert set(exception) == {"path", "reason", "safety"}
    assert "sole approved process boundary" in exception["reason"]
    assert "does not authorize ambient clock reads" in exception["safety"]
    scanner = read("services/agent/tools/boundary-check/src/main.rs")
    assert 'agent_root.join("crates")' in scanner
    assert 'name == "target"' in scanner
    assert 'extension == "rs"' in scanner
    assert "visit_files(&path, output)?;" in scanner
    assert 'allowlisted_paths(agent_root, "nondeterminism-allowlist.toml")' in scanner
    assert "clock_allowlist.contains(&path)" in scanner
    assert "body.contains(ambient)" in scanner
    # Match the production consumer's path-line grammar as well as the TOML parser.
    paths = [line.strip()[8:-1] for line in policy_text.splitlines() if line.strip().startswith('path = "')]
    assert paths == [exception["path"]]
    for fact in ("tests/", "host-tests/", "src/bin/", "target/", "comments"):
        assert fact in policy_text
    for adapter, function, clock_type in (
        ("wall_clock.rs", "wall_time", "SystemTime"),
        ("monotonic_clock.rs", "monotonic_time", "Instant"),
    ):
        path = AGENT / "tests/support" / adapter
        assert not path.is_relative_to(AGENT / "crates")
        assert f"pub fn {function}() -> std::time::{clock_type}" in path.read_text()
        assert f"std::time::{clock_type}::now()" in path.read_text()
        assert f"tests/support/{adapter}" in policy_text
    main = read("services/agent/crates/layerx-agentd/src/main.rs")
    assert "duration_since(UNIX_EPOCH)" in main
    assert "u64::try_from(value.as_millis()).map_err" in main


def test_sdk_wallet_deadlines_use_explicit_clock_without_silent_fallback():
    wallet = read("services/agent/crates/layerx-sdk/src/wallet.rs")
    rpc = read("services/agent/crates/layerx-sdk/src/rpc.rs")
    wait = read("services/agent/crates/layerx-sdk/src/rpc_verification.rs")
    for text in (wallet, rpc, wait):
        assert "Instant::now" not in text
        assert "SystemTime::now" not in text
    assert ".wait_for(activity, commitment, self.policy, timeout)" in wallet
    assert ".wait_for_with_commit(activity, self.policy, timeout, evidence)" in wallet
    assert "pub fn with_clock(mut self, clock: std::sync::Arc<dyn layerx_types::clock::Clock>)" in rpc
    assert "self.clock = Some(clock);" in rpc
    assert "self.clock.clone().ok_or(RpcError::Clock(" in rpc
    assert "layerx_types::clock::ClockError::Unavailable" in rpc
    assert "let clock = self.clock()?;" in wait
    assert "Deadline::start(clock.as_ref(), timeout).map_err(RpcError::Clock)?" in wait
    assert ".remaining(clock.as_ref())" in wait
    assert "self.call_bounded(method, params, remaining)" in wait
    assert "timeout > Duration::from_secs(300)" in wait


def test_clock_scanner_violations_have_exact_out_of_scope_owner_requests():
    record = tomllib.loads(RECORD.read_text())
    pins = json.loads(
        (Path(__file__).with_name("fixtures") / "repair_22_39_293_source_pins.json").read_text()
    )
    assert pins["source_revision"] == "2910889b640274b1b6f2bd14a2fba25019093bb2"
    requests = record["wiring"]["clock_requests"] + pins["clock_requests"]
    requested = set()
    for request in requests:
        path, owner, action = request.split(" | ")
        assert owner == "owning domain 9"
        assert action
        assert path not in requested
        assert (ROOT / path).is_file()
        requested.add(path)
    violations = set()
    allowed = AGENT / "crates/layerx-agentd/src/main.rs"
    for path in (AGENT / "crates").rglob("*.rs"):
        if "target" in path.relative_to(AGENT).parts or path == allowed:
            continue
        if any(token in path.read_text() for token in ("SystemTime::now", "Instant::now")):
            violations.add(path.relative_to(ROOT).as_posix())
    # Recording a violation does not allowlist it or claim boundary-check passes.
    assert requested == violations
    assert "services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs" in requested
    assert "services/agent/crates/layerx-sdk/tests/proof_sdk.rs" in requested
    assert record["finding"]["clock-policy-drift"]["runtime_status"] == "owner wiring required"


def test_repair_record_maps_each_audited_finding_to_executed_source_proofs():
    record = tomllib.loads(RECORD.read_text())
    audit = read("spec/layerx-merge/programme/audit/records/22.30.99.kvx")
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(record["finding"]) == {
        "agent-compatibility-major", "agent-readme-topology", "clock-policy-drift"
    }
    mapped = set()
    for finding, details in record["finding"].items():
        assert details["file"] == value(audit, f"finding.{finding}", "file")
        assert details["tests"] and set(details["tests"]) <= declared
        assert details["source_proof"] and details["review_only"]
        mapped.update(details["tests"])
    assert declared - {"test_repair_record_maps_each_audited_finding_to_executed_source_proofs"} == mapped
    assert record["task"]["base_revision"] == "aa5e0820b3652890f3812a220f4b91f4f8a45387"
    assert record["qualification"]["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1565.py"
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
