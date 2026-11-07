"""Source qualification and real protected-evidence transports; no native build.

The only subprocess in this module is the requested shell syntax parser. Native
corpora, publisher recovery and paxd execution remain explicit review limits.
"""

import ast
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
GATES = ROOT / "tools/paxeer-x/gates"
sys.dont_write_bytecode = True
sys.path.insert(0, str(GATES))
import compiler_manifest as compiler
import hosted_profile as hosted
import profile2_native as profile2


SHELLS = ("104.17.4", "104.24.1", "104.24.2", "104.25.1", "104.25.2",
          "104.30.3", "104.30.5", "104.33.7")


def source(gate):
    return (GATES / (gate + ".sh")).read_text()


def python_blocks(text):
    return re.findall(r"<<'([A-Z]+)'\n(.*?)\n\1(?:\n|$)", text, re.S)


def assignment(text, name):
    for node in ast.walk(ast.parse(text)):
        if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name)
                                               and target.id == name for target in node.targets):
            return ast.literal_eval(node.value)
    raise AssertionError("missing declared assignment: " + name)


def embedded(gate):
    blocks = python_blocks(source(gate))
    assert len(blocks) == 1
    return blocks[0][1]


def test_all_changed_shells_parse_and_embedded_consumers_are_real_python():
    result = subprocess.run(["bash", "-n", *[str(GATES / (gate + ".sh")) for gate in SHELLS]],
                            capture_output=True, text=True, timeout=15)
    assert result.returncode == 0, result.stderr
    for gate in SHELLS:
        text = source(gate)
        assert "set -euo pipefail" in text
        for _, code in python_blocks(text):
            ast.parse(code, filename=gate + ".sh")
    for name in ("compiler_manifest.py", "hosted_profile.py", "profile2_native.py"):
        ast.parse((GATES / name).read_text(), filename=name)


def test_f014_manifest_precedes_gateway_inventory_and_preserves_corpus():
    text = embedded("104.17.4")
    assert 'require_manifest(root, "104.17.4", binaries)' in text
    assert text.index("require_manifest(root,") < text.index("inventory = execute(")
    assert "LAYERX_GATEWAY_LIB_TEST_BINARY" in text
    for fact in ("LAYERX_GATEWAY_TEST_BINARY", "'--list'", "'--nocapture'",
                 "'--test-threads=1'", "int(results[0][0]) != len(declared)",
                 "results[0][1:] != ('0', '0')", "gateway/tests/key-store.py",
                 "record.get('skipped') != 0"):
        assert fact in text
    manifest = tomllib.loads((ROOT / "services/platform/hosted/gateway/Cargo.toml").read_text())
    assert manifest["package"]["name"] == compiler.PLANS["104.17.4"][1]
    assert compiler.PLANS["104.17.4"][2] == {"layerx_platform_gateway": 1, "layerx_gateway": 1}


def test_f015_manifest_binds_entire_selected_migration_corpus_before_discovery():
    text = embedded("104.24.1")
    assert assignment(text, "families") == compiler.PLANS["104.24.1"][2]
    assert 'require_manifest(root, "104.24.1",' in text
    assert "[str(path) for paths in executables.values() for path in paths]" in text
    assert text.index("require_manifest(root,") < text.index("listing = subprocess.run(")
    for fact in ("--include-ignored", "--test-threads=1", "int(summaries[0].group(1)) == len(names)",
                 "family_count == 6", "MIGRATION_V2_CONNECTED_COMPLETE ethereum=1 solana=1 funded=2 skipped=0",
                 "MIGRATION_FUNDED_OPERATOR_COMPLETE ethereum=1 solana=1 funded=2 pending=1 skipped=0",
                 "count += connected_history()", "contract.interop_child.terminate()",
                 "contract.start_interop()", "snapshot() == before"):
        assert fact in text
    manifest = tomllib.loads((ROOT / "services/interop/crates/layerx-migrate/Cargo.toml").read_text())
    assert manifest["package"]["name"] == compiler.PLANS["104.24.1"][1]


def test_f016_manifest_precedes_exact_connected_case_without_relaxing_counts():
    text = source("104.24.2")
    assert 'require_manifest(root, "104.24.2", [sys.argv[2]])' in text
    assert text.index('require_manifest(root,') < text.index('"${candidates[0]}" "$test_name" --exact')
    for fact in ("exactly one genuine compiled test executable required", "--nocapture --test-threads=1",
                 "running 1 test", "1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out;",
                 "migration_v2_connected_runs_authenticated_mapping_and_funded_settlement",
                 "MIGRATION_V2_CONNECTED_COMPLETE ethereum=1 solana=1 funded=2 skipped=0"):
        assert fact in text
    assert compiler.PLANS["104.24.2"][2] == {"migration_v2_connected": 1}


def test_compiler_producer_and_consumer_bind_revision_complete_closure_log_and_executables():
    text = (GATES / "compiler_manifest.py").read_text()
    for fact in ('git("ls-files", "--stage", "-z")', 'git("rev-parse", "HEAD")',
                 'git("rev-parse", "HEAD^{tree}")', 'files[name] = digest(path)',
                 '"gitlink": object_id.decode()', '"symlink": os.readlink(path)',
                 'document["source"] == source', 'source_identity(root) == source',
                 'document["executables"] == rows', 'set(map(str, executables)) == set(rows)',
                 'hashlib.sha256(data).hexdigest() == log["sha256"]',
                 '"profile", {}).get("test")', 'finished == [True]', 'b"\\x7fELF"',
                 'os.O_EXCL | os.O_NOFOLLOW', 'os.fsync(stream.fileno())'):
        assert fact in text
    for gate in compiler.PLANS:
        command = compiler.command_for(gate)
        assert command[:3] == ["cargo", "test", "--locked"]
        assert command[command.index("--manifest-path") + 1] == compiler.PLANS[gate][0]
        assert "--no-run" in command and "--message-format=json" in command
    assert compiler.command_for("104.24.2")[-2:] == ["--test", "migration_v2_connected"]


@pytest.mark.parametrize("gate", ["104.17.4", "104.24.1", "104.24.2"])
def test_compiler_consumer_refuses_absent_failed_and_empty_compiler_transcripts(gate):
    # Negative inputs exercise the production parser, without a made-up passing
    # compiler event or a substitute executable.
    for transcript in (b"", b'{"reason":"build-finished","success":false}\n',
                       b'{"reason":"build-finished","success":true}\n'):
        with pytest.raises(ValueError):
            compiler.compiler_outputs(transcript, gate)


def test_compiler_consumer_refuses_stale_revision_or_incomplete_source_before_io():
    # An invalid document must be refused before reading its unavailable log.
    current = {"revision": "current", "tree": "tree", "files": {"src": "digest"}}
    for stale in ({**current, "revision": "old"}, {**current, "files": {}}):
        document = {"schema": compiler.SCHEMA, "gate": "104.24.2", "source": stale,
                    "command": compiler.command_for("104.24.2"), "exit_code": 0,
                    "compiler_log": {}, "executables": {}}
        with pytest.raises(ValueError, match="current source"):
            compiler.verify_document(document, ROOT, "104.24.2", [], current)


def test_private_evidence_roundtrip_reopen_and_refusals(tmp_path):
    path = tmp_path / "evidence.json"
    # Drive the actual private writer and reader across a close/reopen boundary.
    value = {"schema": hosted.SCHEMA, "source_revision": "current", "inputs": {}}
    compiler.write_private(path, value)
    first = compiler.private_json(path, ROOT)
    del first
    assert compiler.private_json(path, ROOT) == value
    assert path.stat().st_mode & 0o077 == 0
    with pytest.raises(FileExistsError):
        compiler.write_private(path, value)
    link = tmp_path / "link.json"
    link.symlink_to(path)
    with pytest.raises(ValueError, match="canonical"):
        compiler.private_json(link, ROOT)
    path.chmod(0o644)
    with pytest.raises(ValueError, match="private owner"):
        compiler.private_json(path, ROOT)
    path.chmod(0o600)
    alias = tmp_path / "alias.json"
    os.link(path, alias)
    with pytest.raises(ValueError, match="private owner"):
        compiler.private_json(path, ROOT)
    duplicate = tmp_path / "duplicate.json"
    duplicate.write_text('{"source_revision":"one","source_revision":"two"}')
    duplicate.chmod(0o600)
    with pytest.raises(ValueError, match="duplicate"):
        compiler.private_json(duplicate, ROOT)


def catalogue():
    spec = importlib.util.spec_from_file_location("repair_848_candidate", GATES.parent / "candidate.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.catalogue(ROOT / module.SERVICE_CATALOGUE)


def bindings(gate):
    matched = re.search(r"^# paxeer-x-services: (.+)$", source(gate), re.M)
    assert matched
    names = matched[1].split()
    assert len(names) == len(set(names))
    assert set(names) <= catalogue().keys()
    dispatcher = (GATES.parent / "verify-task.sh").read_text()
    assert "if name not in services:" in dispatcher
    assert "gate declares service " in dispatcher
    return set(names)


def test_f043_current_mirror_bindings_preserve_both_archive_recovery_lanes():
    assert bindings("104.25.1") == {"mirror-publisher", "mirror-signer"}
    assert "mirror-signer" in catalogue()["mirror-publisher"]["dependencies"]
    text = embedded("104.25.1")
    assert assignment(text, "TESTS") == [
        "runtime::tests::worker_degradation_survives_progress_application_per_lane",
        "runtime::tests::checkpoint_refusal_latches_until_verified_evidence",
        "runtime::tests::clean_restart_resumes_after_the_contiguous_spool_prefix",
    ]
    runtime = (ROOT / "services/interop/crates/layerx-mirror/src/runtime.rs").read_text()
    for name in assignment(text, "TESTS"):
        assert "fn " + name.split("::")[-1] + "(" in runtime
    for lane in ("ethereum", "solana"):
        for phase in ("stall", "restored", "reorg", "recovered"):
            assert "'" + lane + "_" + phase + "'" in text
    for fact in ("recovered(phases[lane + '_restored'])", "recovered(phases[lane + '_recovered'])",
                 "stalled[other]['latest_batch_mirrored'] <= origin[other]['latest_batch_mirrored']",
                 "live.get('revision') != git('rev-parse', 'HEAD')", "private_json(Path(raw))"):
        assert fact in text


def test_f044_binds_real_mirror_and_retained_verifier_consumers():
    assert bindings("104.25.2") == {"mirror-publisher", "mirror-signer", "platform-gateway",
                                    "explorer", "human-explorer-index"}
    text = embedded("104.25.2")
    assert assignment(text, "TESTS") == ["verify::tests::native_fixture_receipt_verifies_and_altered_bytes_are_refused"]
    manifest = tomllib.loads((ROOT / "services/interop/crates/layerx-mirror/Cargo.toml").read_text())
    assert manifest["package"]["name"] == "layerx-mirror"
    assert manifest["package"].get("autobins", True) is True
    binary_source = ROOT / "services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs"
    assert binary_source.is_file() and "fn main(" in binary_source.read_text()
    verifier = (ROOT / "services/interop/crates/layerx-mirror/src/verify.rs").read_text()
    assert "fn native_fixture_receipt_verifies_and_altered_bytes_are_refused(" in verifier
    for fact in ("'--bin', 'layerx-mirror-verify'", "scripts/qualify-mirror-verification-live.sh",
                 "'LAYERX_MIRROR_STATE_TAMPER_REQUEST'", "'LAYERX_MIRROR_EXPLORER_VERIFY_URL'",
                 "'LAYERX_MIRROR_TS_CONFORMANCE'", "'LAYERX_MIRROR_PYTHON_CONFORMANCE'",
                 "'LAYERX_MIRROR_GO_CONFORMANCE'", "'LAYERX_MIRROR_JVM_CONFORMANCE'",
                 "'LAYERX_MIRROR_SWIFT_CONFORMANCE'", "'LAYERX_MIRROR_DOTNET_CONFORMANCE'",
                 "artifacts['layerx-mirror-verify']['path']", "1 passed; 0 failed;"):
        assert fact in text


def test_f017_retained_native_interface_transport_preserves_positive_and_refusal_criteria():
    text = embedded("104.30.3")
    assert "test_interface_protocol" not in text
    expected = ("canonical_abi_domains_and_legacy_vectors", "exact_capabilities_exports_and_upgrade_policy",
                "canonical_encoding_refuses_malformed_bounds", "node_transaction_result_and_interface_transport")
    assert assignment(text, "INTERFACE_CASES") == expected
    assert '"--test", "interface_protocol", "--no-run", "--message-format=json"' in text
    assert '"interface_consumer": artifact(interface_executables[0])' in text
    assert 'for name in INTERFACE_CASES:' in text
    assert 'manifest["interface_consumer"]["path"], "--exact", name' in text
    assert "1 passed; 0 failed; 0 ignored;" in text
    assert "6 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out;" in text
    path = ROOT / "programs/crates/layerx-programs-registry/tests/interface_protocol.rs"
    retained = path.read_text()
    for name in expected:
        assert "fn " + name + "(" in retained
    assert "repair_22_33_221::native_interface_transport();" in retained
    support = (path.parent / "support/repair_22_33_221.rs").read_text()
    for fact in ('include_str!("../../src/repair_22_31_256.hex")', "DeploymentProof::decode(&bytes)",
                 "verify_deployment(&decoded, input.now)", "VerifiedInterfaceRead::verify_at_head(",
                 'node_producer()["deploy"]', 'node_producer()["failed"]', "for field in 0..6"):
        # The producer selects deploy/upgrade through a named key.
        assert fact in support
    for fact in ("abi2-dynamic", "abi3-dynamic", "abi4-dynamic", "authorize_upgrade", "is_err()"):
        assert fact in retained


def test_f018_native_profile2_uses_paxd_funding_committed_proofs_and_real_restart():
    shell = source("104.30.5")
    assert 'exec python3 -B "$root/tools/paxeer-x/gates/profile2_native.py" "$@"' in shell
    text = (GATES / "profile2_native.py").read_text()
    for retired in ("layerxd", "sign-credit", "test-credit", "CUSTODY_FIXTURE",
                    "LAYERX_TEST_NATIVE_BIN_DIR", "paxeer-x-profile2-native"):
        assert retired not in shell + text
    assert profile2.REQUIRED == {
        "two-assets-funded", "complete-exits", "missing-exit-refused", "current-proof-read",
        "proof-substitution-refused", "native-restart", "tombstone-history", "exit-conservation",
        "historical-replay", "wrong-asset-refused", "wrong-owner-refused",
    }
    for fact in ('["make", "paxd-build"]', 'ROOT / "build/paxd"',
                 'arguments[1] == 2', 'method == "transfer"', 'len({arguments[2]',
                 'session.node.program_send(', 'session.node.committed(', 'session.restart()',
                 'session.observations(row["after"]) == after', '"--test", "winddown"',
                 '"retained registry corpus empty or skipped"', '"PAXEER_X_PROFILE2_NATIVE_FIXTURE"',
                 'module.PaxdFixture(profile, artifact["path"], evidence)', 'fixture.stop()'):
        assert fact in text
    merged = (ROOT / "tools/qualification/paxeer-x/programs_accounts_winddown.py").read_text()
    restart = merged.split("    def restart(self):", 1)[1].split("\n\ndef ", 1)[0]
    assert "self.stop()" in restart and "self.start()" in restart
    assert "str(self.home)" in merged and "shutil.copytree(seed, self.home)" in merged
    chain = (ROOT / "tools/qualification/paxeer-x/fixtures/merged_chain.py").read_text()
    for fact in ("'pax_sequencerReceipt'", "'rung') == 'committed'", "_EXECUTION.verified_execution(",
                 "'eth_getTransactionReceipt'", "'eth_getBlockByNumber'", "tx.data == self.program_call"):
        assert fact in chain
    abi = json.loads((ROOT / "precompiles/layerxprograms/abi.json").read_text())
    functions = {row["name"] for row in abi if row["type"] == "function"}
    assert {"registerAccount", "transfer", "exitValue", "tombstone", "getStateHead"} <= functions


def test_f018_inventory_and_account_conservation_reject_missing_duplicate_and_wrong_asset():
    rows = [{"id": name} for name in sorted(profile2.REQUIRED)]
    assert profile2.inventory(rows) is rows
    for bad in (rows[:-1], rows + rows[:1], rows[:-1] + rows[:1]):
        with pytest.raises(ValueError, match="exact profile-2"):
            profile2.inventory(bad)
    reads = [{"method": "getValueAccount", "arguments": ["program", name]} for name in ("asset-a", "asset-b")]
    encoded = ["0x" + amount.to_bytes(32, "big").hex() + bytes(32).hex() for amount in (7, 9)]
    assert profile2.balances(reads, encoded) == {("program", "asset-a"): 7, ("program", "asset-b"): 9}
    with pytest.raises(ValueError, match="duplicate"):
        profile2.balances([reads[0], reads[0]], encoded)
    with pytest.raises(ValueError, match="both native asset"):
        profile2.balances(reads[:1], encoded[:1])
    with pytest.raises(ValueError, match="canonical account"):
        profile2.balances(reads, ["0x00", encoded[1]])


def test_f062_services_and_protected_profile_keep_all_connected_acceptance_cases():
    assert bindings("104.33.7") == {"platform-gateway", "human-explorer-index", "platform-program-registry",
                                    "sequencer", "platform-receipt-authority"}
    text = embedded("104.33.7")
    assert "inputs=load_profile(profile_path,revision,root)" in text
    assert "os.environ['LAYERX_" not in text
    for fact in ("PAXEER_X_HOSTED_PROGRAM_PROFILE", "inputs['LAYERX_GATEWAY_KEY_SECRET']",
                 "inputs['LAYERX_PROGRAM_CALL_ACTIVITY_FILE']", "inputs['LAYERX_RECEIPT_VERIFY_BIN']",
                 "actual discovery failed", "actual code-bound interface failed", "interface digest mismatch",
                 "actual noncommitting simulation failed", "missing idempotency must refuse", "actual call failed",
                 "independent real receipt verification failed", "exact idempotent receipt replay failed",
                 "tampered signed activity must refuse", "tuple(map(int,summary[0]))==(len(names),0,0)",
                 "'rust','typescript','python','go','java','swift','csharp'"):
        assert fact in text
    assert "tools/paxeer-x/gates/hosted_profile.py" in assignment(text, "sources")
    assert "tools/paxeer-x/gates/compiler_manifest.py" in assignment(text, "sources")


def test_f062_real_profile_producer_private_transport_consumer_and_reopen(tmp_path):
    # These are inert inputs to the real profile transport. No server, receipt
    # verification or successful native execution is substituted or claimed.
    files = {}
    for key, relative in (
        ("LAYERX_GATEWAY_CA_FILE", "tools/paxeer-x/gates/104.33.7.sh"),
        ("LAYERX_PROGRAM_CALL_ACTIVITY_FILE", "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"),
        ("LAYERX_RECEIPT_VERIFY_BIN", "tools/paxeer-x/gates/104.33.7.sh"),
    ):
        path = tmp_path / key
        path.write_bytes((ROOT / relative).read_bytes())
        path.chmod(0o700 if key == "LAYERX_RECEIPT_VERIFY_BIN" else 0o600)
        files[key] = str(path)
    secret = tmp_path / "secret"
    secret.write_text("inert-qualification-profile-secret")
    secret.chmod(0o600)
    inputs = {**files, "LAYERX_GATEWAY_KEY_SECRET_FILE": str(secret),
              "LAYERX_PROGRAM_GATEWAY_URL": "https://127.0.0.1:12345",
              "LAYERX_GATEWAY_KEY_ID": "qualification", "LAYERX_PROGRAM_ID": "01" * 32,
              "LAYERX_PROGRAM_IDEMPOTENCY_KEY": "02" * 32}
    path = tmp_path / "profile.json"
    hosted.publish(path, "current", inputs, ROOT)
    accepted = hosted.load(path, "current", ROOT)
    assert accepted == {**inputs, "LAYERX_GATEWAY_KEY_SECRET": secret.read_text()}
    del accepted
    assert hosted.load(path, "current", ROOT)["LAYERX_PROGRAM_ID"] == inputs["LAYERX_PROGRAM_ID"]
    with pytest.raises(ValueError, match="current private"):
        hosted.load(path, "old", ROOT)
    secret.chmod(0o644)
    with pytest.raises(ValueError, match="private owner"):
        hosted.load(path, "current", ROOT)


@pytest.mark.parametrize("change", [
    {"LAYERX_PROGRAM_GATEWAY_URL": "https://outside.invalid:443"},
    {"LAYERX_PROGRAM_GATEWAY_URL": "http://127.0.0.1:12345"},
    {"LAYERX_GATEWAY_KEY_SECRET": "inline-secret"},
    {"LAYERX_PROGRAM_ID": "wrong"},
])
def test_f062_profile_consumer_refuses_external_plaintext_inline_secret_and_wrong_identity(change):
    inputs = {key: "/private/file" for key in hosted.INPUTS}
    inputs.update(LAYERX_PROGRAM_GATEWAY_URL="https://127.0.0.1:12345", LAYERX_GATEWAY_KEY_ID="qualification",
                  LAYERX_PROGRAM_ID="01" * 32, LAYERX_PROGRAM_IDEMPOTENCY_KEY="02" * 32)
    with pytest.raises(ValueError):
        hosted.validate({"schema": hosted.SCHEMA, "source_revision": "current", "inputs": {**inputs, **change}}, "current")


def test_record_maps_every_finding_to_executed_tests_and_honest_evidence_limits():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_848.kvx").read_text())
    assert set(record["finding"]) == {"f014", "f015", "f016", "f043", "f044", "f017", "f018", "f062"}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= declared
        assert finding["source_proof"] and finding["review_only"]
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["task"]["baseline"].lower()
    assert record["wiring"]["profile2"]["owner_domain"]
    assert record["wiring"]["profile2"]["path"].startswith("tools/qualification/")
