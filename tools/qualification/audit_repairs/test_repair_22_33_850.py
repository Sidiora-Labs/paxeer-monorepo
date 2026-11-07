"""Source qualification of retained gates; never impersonate a native run."""

import ast
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
GATES = ROOT / "tools/paxeer-x/gates"
SELECTORS = ("11.2", "111.6.11", "12.1", "18.3", "24.14", "24.16", "24.18")


def source(selector):
    return (GATES / (selector + ".sh")).read_text()


def embedded(selector):
    return source(selector).split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]


def declarations(selector):
    namespace = {"json": json, "re": re}
    tree = ast.parse(embedded(selector))
    nodes = [node for node in tree.body if isinstance(node, (ast.Assign, ast.FunctionDef))]
    exec(compile(ast.Module(body=nodes, type_ignores=[]), selector, "exec"), namespace)
    return namespace


def catalogue():
    # Execute the real parser without candidate.py's unrelated Go policy startup.
    tree = ast.parse((GATES.parent / "candidate.py").read_text())
    names = {"Invalid", "require", "keys", "catalogue"}
    nodes = [node for node in tree.body if isinstance(node, (ast.ClassDef, ast.FunctionDef))
             and node.name in names]
    namespace = {"Path": Path, "re": re, "json": json}
    exec(compile(ast.Module(body=nodes, type_ignores=[]), "candidate.py", "exec"), namespace)
    return namespace["catalogue"](ROOT / "spec/layerx-merge/spec.kvx")


def bindings(selector):
    match = re.search(r"^# paxeer-x-services: (.+)$", source(selector), re.M)
    assert match
    names = match[1].split()
    assert len(set(names)) == len(names)
    assert set(names) <= catalogue().keys()
    return set(names)


@pytest.mark.parametrize("selector", SELECTORS)
def test_every_repaired_shell_is_parsed(selector):
    path = GATES / (selector + ".sh")
    assert path.stat().st_mode & 0o111, "registered gate must be executable"
    result = subprocess.run(["bash", "-n", str(path)],
                            text=True, capture_output=True, check=False)
    assert result.returncode == 0, result.stderr


@pytest.mark.parametrize("selector,package,target", [
    ("11.2", "layerx-human-identity-provider", "assertion"),
    ("12.1", "layerx-human-kms", "provider"),
])
def test_f019_f020_current_human_workspace_retains_real_provider_corpus(selector, package, target):
    text = source(selector)
    command = re.search(r"sh -c '([^']+)'", text)[1]
    assert command.startswith("cd services/human && ")
    assert "timeout 15m tools/runtime/run-with-clock.sh" in text
    assert f"cargo test -p {package} --test {target}" in command
    if selector == "12.1":
        assert "RUST_TEST_THREADS=1" in command
    manifest = tomllib.loads((ROOT / f"services/human/crates/{package}/Cargo.toml").read_text())
    assert manifest["package"]["name"] == package
    workspace = tomllib.loads((ROOT / "services/human/Cargo.toml").read_text())
    assert f"crates/{package}" in workspace["workspace"]["members"]
    corpus = ROOT / f"services/human/crates/{package}/tests/{target}.rs"
    assert corpus.is_file() and "#[test]" in corpus.read_text()
    assert "code=$?" in text and 'exit "$code"' in text
    assert "PAXEER_X_GATE tests=" in text and "skipped=" in text
    assert "passed" in text and "failed" in text and "ignored" in text
    dispatcher = (GATES.parent / "verify-task.sh").read_text()
    assert "elif not counts or tests == 0:" in dispatcher
    assert "elif skipped:" in dispatcher


def test_f036_current_paxd_recipes_and_actual_container_checks():
    text = source("111.6.11")
    selected = shlex.split(re.search(r"timeout 20m tools/containers/check.sh (.+) 2>&1", text)[1])
    assert "platform-node" not in selected
    assert len(selected) == len(set(selected))
    assert set(selected) == {
        "platform-agent-boundary", "platform-authority", "platform-core", "platform-dashboard",
        "platform-dashboard-web", "platform-faucet", "platform-gateway", "platform-identity",
        "platform-internal", "paxeer", "platform-registry", "platform-registry-builder",
        "platform-testnet", "platform-webhooks", "ramps", "hpx-registry",
    }
    checker = (ROOT / "tools/containers/check.sh").read_text()
    rows = checker.split("SERVICES='\n", 1)[1].split("\n'", 1)[0].splitlines()
    recipes = [row.split("|") for row in rows]
    assert set(selected) <= {row[0] for row in recipes}
    paxd = [row for row in recipes if row[0] == "paxeer" and row[1].endswith("Dockerfile.paxd")]
    assert len(paxd) == 1
    assert (ROOT / paxd[0][1]).is_file()
    assert "docker build --check" in checker and "paxd_check_recipe" in checker
    assert "containers-check: docker\\/.* check ok" in text
    assert "n++" in text and '"$tests"' in text
    assert "chmod 700" in text and 'exit "$code"' in text
    assert "docker/platform-node/Dockerfile" in checker.split("# Retired service name:", 1)[1]


def assert_rust_corpus(selector, expected):
    namespace = declarations(selector)
    assert namespace["CASES"] == expected
    text = embedded(selector)
    for fact in ("'cargo', 'test', '--locked'", "'services/interop/Cargo.toml'",
                 "'-p', 'layerx-mirror'", "[name, '--', '--exact', '--nocapture']",
                 "if result.returncode:", "total += executed_counts(result.stdout, name)",
                 "PAXEER_X_GATE tests={total} skipped=0", "timeout=1200"):
        assert fact in text
    manifest = tomllib.loads((ROOT / "services/interop/crates/layerx-mirror/Cargo.toml").read_text())
    assert manifest["package"]["name"] == "layerx-mirror"
    workspace = tomllib.loads((ROOT / "services/interop/Cargo.toml").read_text())
    assert "crates/layerx-mirror" in workspace["workspace"]["members"]
    for case in expected:
        module, tests, name = case.split("::")
        assert tests == "tests"
        retained = (ROOT / f"services/interop/crates/layerx-mirror/src/{module}.rs").read_text()
        assert re.search(r"#\[test\]\s+fn " + name + r"\(", retained)


def test_f047_committed_block_freshness_and_retained_service_contract():
    assert bindings("18.3") == {"mirror-publisher", "mirror-signer", "sequencer"}
    # The catalogue documents this external dependency but does not register it.
    # Bind the retained node receipt producer; keep unknown-service refusal intact.
    assert "archive-node" in catalogue()["mirror-publisher"]["dependencies"]
    assert "archive-node" not in catalogue()
    assert "pax_sequencerReceipt" in catalogue()["sequencer"]["route"]
    dispatcher = (GATES.parent / "verify-task.sh").read_text()
    assert "if name not in services:" in dispatcher
    assert "gate declares service " in dispatcher
    assert "interop-archive-contract.py" not in source("18.3")
    assert_rust_corpus("18.3", (
        "publisher::tests::a_v4_archive_round_trips_a_committed_block",
        "runtime::tests::the_runtime_spools_committed_archives_in_height_order",
        "publisher::tests::a_fresh_gate_withholds_unreconciled_journal_progress",
    ))
    integration = declarations("18.3")["INTEGRATION_CASES"]
    assert integration == (
        ("committed_freshness", "freshness_reports_committed_lag_in_blocks"),
        ("committed_source", "the_finalized_tag_gives_the_latest_committed_coordinate"),
        ("committed_source", "a_committed_block_is_acquired_with_every_receipt_verified"),
    )
    assert "['--test', target, name, '--', '--exact', '--nocapture']" in embedded("18.3")
    for target, name in integration:
        retained = (ROOT / f"services/interop/crates/layerx-mirror/tests/{target}.rs").read_text()
        assert re.search(r"#\[test\]\s+fn " + name + r"\(.*\)", retained)
    freshness = (ROOT / "services/interop/crates/layerx-mirror/tests/committed_freshness.rs").read_text()
    for fact in ("CommittedFreshness::Behind { block_lag: 7 }", "CommittedFreshness::Current",
                 "CommittedFreshness::DifferentAtHeight", "CommittedFreshness::NodeHasNoCommittedBlock",
                 "MirrorVerificationFreshness::relative(freshness)"):
        assert fact in freshness
    transport = (ROOT / "services/interop/crates/layerx-mirror/tests/committed_source.rs").read_text()
    for fact in ("CommittedBlockSource::new(&rpc.config)", ".acquire(height)",
                 "verify_sequencer_signature(bytes, ReceiptKeys::HeightRanges(&pins))",
                 'json!(["finalized", false])', '"pax_sequencerReceipt"'):
        assert fact in transport
    runtime = (ROOT / "services/interop/crates/layerx-mirror/src/runtime.rs").read_text()
    for fact in ('include_str!("../tests/fixtures/committed_blocks.json")',
                 "CommittedHeader::from_block_json", "super::spool_committed(",
                 "super::ordered_archives(&spool)", "value.node.latest_batch_mirrored, Some(second.header.height)",
                 "value.latest_committed_height, Some(second.header.height)", "value.committed_ready"):
        assert fact in runtime
    fixture = json.loads((ROOT / "services/interop/crates/layerx-mirror/tests/fixtures/committed_blocks.json").read_text())
    assert fixture["committedBlock"] and fixture["receipts"]
    publisher = (ROOT / "services/interop/crates/layerx-mirror/src/publisher.rs").read_text()
    for fact in ("Archive::from_committed(&block, 125, head)", "ArchiveData::decode(archive.bytes())",
                 "Archive::from_spool(expected_bytes, head)", "ReconcileRefusal::ArchiveUnavailable",
                 "progress.verified, MirrorCursor::default()"):
        assert fact in publisher


def test_f026_archive_producer_and_real_restart_recovery_consumers_are_selected():
    assert bindings("24.14") == {"mirror-publisher", "mirror-signer", "sequencer"}
    assert_rust_corpus("24.14", (
        "publisher::tests::a_v4_archive_round_trips_a_committed_block",
        "publisher::tests::batch_era_archive_versions_are_refused",
        "runtime::tests::the_runtime_spools_committed_archives_in_height_order",
        "runtime::tests::clean_restart_resumes_after_the_contiguous_spool_prefix",
        "runtime::tests::startup_refuses_corrupt_archive_bytes",
        "runtime::tests::startup_refuses_gapped_batch_metadata",
        "runtime::tests::checkpoint_refusal_latches_until_verified_evidence",
    ))
    runtime = (ROOT / "services/interop/crates/layerx-mirror/src/runtime.rs").read_text()
    recovery = runtime.split("fn the_runtime_spools_committed_archives_in_height_order()", 1)[1]
    recovery = recovery.split("\n    #[test]", 1)[0]
    assert recovery.index("drop(spool)") < recovery.index("reopen spool")
    for fact in ("ArchiveSpool::open(&directory)", "super::recover_committed_coordinate",
                 "assert_eq!(recovered, Some(previous))", 'Err("committed_parent_mismatch")',
                 "committed.receipts, block.receipts", "second.header.height + 1"):
        assert fact in recovery
    assert "PAXEER_X_RUNTIME_NATIVE_REVISION" not in source("24.14")


def assert_go_corpora(selector, expected):
    assert declarations(selector)["CORPORA"] == expected
    text = embedded(selector)
    assert "['go', 'test', '-json', '-count=1', '-run', pattern, package]" in text
    assert "'^(' + '|'.join(re.escape(name) for name in names) + ')$'" in text
    assert "if result.returncode:" in text
    assert "total += executed_counts(result.stdout, names)" in text
    assert "PAXEER_X_GATE tests={total} skipped=0" in text
    assert "timeout=900" in text
    for package, names in expected:
        retained = "\n".join(path.read_text() for path in (ROOT / package).glob("*_test.go"))
        for name in names:
            assert re.search(r"func " + name + r"\(t \*testing.T\)", retained)


def test_f022_f027_native_transfer_committed_bank_proofs_and_genuine_counts():
    text = source("24.16")
    assert bindings("24.16") == {"sequencer"}
    assert "make -j5 paxd-build" in text
    assert "paxd-build: paxd-native-libs" in (ROOT / "Makefile").read_text()
    for retired in ("BetaUsdl", "WETH", "forge", "layerx-platform-paxeer-boundary", "artifacts/"):
        assert retired not in text
    assert_go_corpora("24.16", (
        ("./node/sequencer", ("TestStandalonePrimaryReceiptsTransferThroughSendAPI",)),
        ("./custodyproof", ("TestNativeBankCommittedStateAndRecovery", "TestNativeBankMalformedProofRefusal")),
    ))
    transfer = (ROOT / "node/sequencer/standalone_primary_test.go").read_text()
    for fact in ("sequencer.NewNode(cfg, h.app)", "evmrpc.NewSendAPI", "api.SendRawTransaction(ctx, raw)",
                 "n.ReceiptSource.SequencerSend(hash)", "verify.ReceiptSignature",
                 "h.app.EvmKeeper.GetNonce(snapshot, sender.EvmAddress)"):
        assert fact in transfer
    bank = (ROOT / "custodyproof/verify_test.go").read_text()
    for fact in ("bank.SendCoins(ctx, owner, recipient", "store.Commit(true)", "Prove: true",
                 "nativeTransport(t, f.request)", "v.Close()", "OpenVerifier(nativeTrustDB(t, dir)",
                 "require.Equal(t, result.Amount, recovered.Amount)", 'require.Equal(t, "500", advanced.Amount)'):
        assert fact in bank
    refusals = (ROOT / "custodyproof/repair_m856_test.go").read_text()
    for fact in ('"wrong_account"', '"wrong_denom"', '"bad_membership"', '"bad_signature"',
                 '"refused proof advanced persistent trust"'):
        assert fact in refusals


def test_f023_standard_paxd_foundation_manifest_and_retained_consensus():
    text = source("24.18")
    assert bindings("24.18") == {"sequencer", "platform-receipt-authority"}
    for retired in ("layerxd", "layerx-genesis-build", "LAYERX_CUSTODY_ARTIFACT_MANIFEST",
                    "layerx-core-boundary", "24.16.sh build"):
        assert retired not in text
    assert "umask 077" in text
    assert '== manifest.json' in text
    assert "python3 tools/bringup/tests/foundation-artifacts.py --build" in text
    assert '--output "$(dirname -- "$PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST")"' in text
    plan = json.loads((ROOT / "tools/bringup/runtime-bundle.json").read_text())
    assert plan["version"] == 2 and plan["bundle"] == "merged-runtime"
    assert plan["names"] == ["paxd", "attestor", "bringup-inspect"]
    producer = (ROOT / "tools/bringup/tests/foundation-artifacts.py").read_text()
    for fact in ('["make", "-f", "chain.mk", "build"]', 'source = ROOT / "build/paxd"',
                 '"vcs.modified=false"', 'source_binding(selected_revision, PATHS[name]) == binding',
                 'record.get("sha256") == sha256(path)', 'regular_private(path, executable=True)',
                 'os.chmod(stage / "manifest.json", 0o600)', 'os.rename(stage, destination)'):
        assert fact in producer
    consumer = (ROOT / "tools/bringup/tests/paxeer-x-fixture-foundation.py").read_text()
    assert "ART.validate(manifest, path.parent, revision)" in consumer
    assert "exec /artifacts/paxd start" in consumer
    assert_go_corpora("24.18", (
        ("./custodyproof", ("TestRealStateCreditAndClockBounds", "TestRealStateCreditRefusesMissingAuthority",
                            "TestNativeBankCommittedStateAndRecovery", "TestNativeBankMalformedProofRefusal")),
        ("./consensus/internal/consensus/types", ("TestLeaderPinRoundRobinFromActivation",)),
        ("./modules/sequencer/keeper", ("TestReceiptKeysGovernanceOnly", "TestReceiptKeysRotationBoundary")),
        ("./node/sequencer", ("TestStandbyAckBeforeRelease", "TestStandbyNoAckNoReceipt")),
    ))


@pytest.mark.parametrize("selector", ["18.3", "24.14"])
def test_rust_count_protocol_requires_exact_case_and_refuses_empty_failed_or_skipped(selector):
    count = declarations(selector)["executed_counts"]
    name = "count_protocol::metacharacters[0]"
    # Parser unit inputs are protocol examples, never native execution evidence.
    valid = f"test {name} ... ok\ntest result: ok. 1 passed; 0 failed; 0 ignored; 0 measured;\n"
    assert count(valid, name) == 1
    for invalid in ("", "unrelated compiler output", valid.replace(name, "different_case"),
                    valid.replace("1 passed", "0 passed"), valid.replace("0 ignored", "1 ignored"),
                    valid.replace("0 failed", "1 failed"), valid + valid):
        with pytest.raises(ValueError):
            count(invalid, name)


@pytest.mark.parametrize("selector", ["24.16", "24.18"])
def test_go_count_protocol_requires_actual_cases_and_refuses_skip_failure_duplicate_or_empty(selector):
    count = declarations(selector)["executed_counts"]
    names = ("TestCountProtocol",)
    rows = [{"Action": "run", "Test": names[0]}, {"Action": "pass", "Test": names[0]},
            {"Action": "pass"}]
    encode = lambda events: "\n".join(json.dumps(row) for row in events)
    assert count(encode(rows), names) == 1
    subtests = [rows[0], {"Action": "run", "Test": names[0] + "/child"},
                {"Action": "pass", "Test": names[0] + "/child"}, *rows[1:]]
    assert count(encode(subtests), names) == 2
    for invalid in ([], [rows[-1]], rows[:-1], [*rows, {"Action": "fail"}],
                    [rows[0], {"Action": "skip", "Test": names[0]}, rows[-1]],
                    [rows[0], rows[0], *rows[1:]], [rows[1], rows[-1]],
                    [{"Action": "run", "Test": "TestUnexpected"},
                     {"Action": "pass", "Test": "TestUnexpected"}, rows[-1]]):
        with pytest.raises(ValueError):
            count(encode(invalid), names)


def test_repair_record_maps_all_findings_to_tests_and_discloses_limits_and_wiring():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_850.kvx").read_text())
    assert set(record["finding"]) == {"f019", "f036", "f020", "f047", "f026", "f022", "f027", "f023"}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body
                if isinstance(node, ast.FunctionDef)}
    for row in record["finding"].values():
        assert row["tests"] and set(row["tests"]) <= declared
        assert row["source_proof"] and row["review_only"]
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["task"]["baseline"].lower()
    assert record["wiring"]["core_consumers"]["owner_domain"]
    assert record["wiring"]["core_consumers"]["path"]
    assert record["wiring"]["candidate_build"]["path"]
