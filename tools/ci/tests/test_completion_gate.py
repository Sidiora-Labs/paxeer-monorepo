import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import unittest

import pytest


ROOT = Path(__file__).resolve().parents[3]

RETIRED_MAKE_TARGETS = (
    (
        "interface.16.mk-authority-grant-fixtures",
        "authority-grant-fixtures",
    ),
    (
        "interface.16.mk-authority-grant-fixtures-check",
        "authority-grant-fixtures-check",
    ),
    (
        "interface.16.mk-builddir-bin-layerx-guarantor",
        "$(BUILD_DIR)/bin/layerx-guarantor",
    ),
    (
        "interface.16.mk-builddir-bin-layerx-handover",
        "$(BUILD_DIR)/bin/layerx-handover",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-core",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_core",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-exchange",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_exchange",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-feedback",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_feedback",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-integration",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_integration",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-receipt",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_receipt",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-runtime",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_runtime",
    ),
    (
        "interface.16.mk-builddir-tests-lxp-test-guarantor-terminal-rejection",
        "$(BUILD_DIR)/tests/lxp_test_guarantor_terminal_rejection",
    ),
    (
        "interface.16.mk-builddir-tests-relay-archive-sign",
        "$(BUILD_DIR)/tests/relay-archive-sign",
    ),
    (
        "interface.16.mk-layerx-guarantor",
        "layerx-guarantor",
    ),
    (
        "interface.16.mk-layerx-handover",
        "layerx-handover",
    ),
    (
        "interface.16.mk-layerx-verify",
        "layerx-verify",
    ),
    (
        "interface.16.mk-layerxd",
        "layerxd",
    ),
    (
        "interface.16.mk-paxeer-x-finality-recordings-build",
        "paxeer-x-finality-recordings-build",
    ),
    (
        "interface.16.mk-platform-hosted-guarantor-topology-check",
        "platform-hosted-guarantor-topology-check",
    ),
    (
        "interface.16.mk-relay-archive-build",
        "relay-archive-build",
    ),
    (
        "interface.16.mk-relay-archive-e2e",
        "relay-archive-e2e",
    ),
    (
        "interface.16.mk-test-bridge-maintenance-publication",
        "test-bridge-maintenance-publication",
    ),
    (
        "interface.16.mk-test-daemon-allowance",
        "test-daemon-allowance",
    ),
    (
        "interface.16.mk-test-daemon-grant-issuance",
        "test-daemon-grant-issuance",
    ),
    (
        "interface.16.mk-test-daemon-guarantor",
        "test-daemon-guarantor",
    ),
    (
        "interface.16.mk-test-daemon-guarantor-feedback",
        "test-daemon-guarantor-feedback",
    ),
    (
        "interface.16.mk-test-daemon-guarantor-integration",
        "test-daemon-guarantor-integration",
    ),
    (
        "interface.16.mk-test-daemon-guarantor-sanitize",
        "test-daemon-guarantor-sanitize",
    ),
    (
        "interface.16.mk-test-daemon-guarantor-settlement",
        "test-daemon-guarantor-settlement",
    ),
    (
        "interface.16.mk-test-daemon-guarantor-unit",
        "test-daemon-guarantor-unit",
    ),
    (
        "interface.16.mk-test-daemon-handover",
        "test-daemon-handover",
    ),
    (
        "interface.16.mk-test-daemon-handover-consumers",
        "test-daemon-handover-consumers",
    ),
    (
        "interface.16.mk-test-daemon-handover-crash",
        "test-daemon-handover-crash",
    ),
    (
        "interface.16.mk-test-daemon-handover-peers",
        "test-daemon-handover-peers",
    ),
    (
        "interface.16.mk-test-daemon-maintenance-crash",
        "test-daemon-maintenance-crash",
    ),
    (
        "interface.16.mk-test-daemon-maintenance-publication",
        "test-daemon-maintenance-publication",
    ),
    (
        "interface.16.mk-test-daemon-metered-allowance",
        "test-daemon-metered-allowance",
    ),
    (
        "interface.16.mk-test-daemon-module-maintenance",
        "test-daemon-module-maintenance",
    ),
    (
        "interface.16.mk-test-daemon-native-onboarding",
        "test-daemon-native-onboarding",
    ),
    (
        "interface.16.mk-test-daemon-owner-rotation",
        "test-daemon-owner-rotation",
    ),
    (
        "interface.16.mk-test-daemon-withdrawal-replica-recovery",
        "test-daemon-withdrawal-replica-recovery",
    ),
    (
        "interface.16.mk-test-guarantor-receipt",
        "test-guarantor-receipt",
    ),
    (
        "interface.16.mk-test-guarantor-terminal-rejection",
        "test-guarantor-terminal-rejection",
    ),
    (
        "interface.16.mk-test-layerxd",
        "test-layerxd",
    ),
    (
        "interface.16.mk-test-lni-version-parity",
        "test-lni-version-parity",
    ),
    (
        "interface.16.mk-test-program-admission",
        "test-program-admission",
    ),
    (
        "interface.16.mk-test-program-simulate",
        "test-program-simulate",
    ),
    (
        "interface.16.mk-test-tools",
        "test-tools",
    ),
    (
        "interface.16.mk-test-verify-cli",
        "test-verify-cli",
    ),
)

RETIRED_PATHS = (
    (
        "interface.16.c-lxp-real-replay-activity-j59",
        "tests/support/lxp_real_replay.h",
    ),
    (
        "interface.16.c-lxp-real-replay-build-j59",
        "tests/support/lxp_real_replay.h",
    ),
    (
        "interface.16.c-lxp-real-replay-init-j59",
        "tests/support/lxp_real_replay.h",
    ),
    (
        "interface.16.c-lxp-real-replay-prepare-j59",
        "tests/support/lxp_real_replay.h",
    ),
    (
        "interface.16.c-pay1-guarantor-replay-j59",
        "tests/support/lxp_pay1_replay.h",
    ),
    (
        "interface.16.c-pay1-replay-activities-j59",
        "tests/support/lxp_pay1_replay.h",
    ),
    (
        "interface.16.cli-scr-ci-lni-version-parity",
        "scripts/ci/lni-version-parity.sh",
    ),
)

PLANNED_PATHS = (
    (
        "16.2.4",
        "tools/ci/tests/test_makefile_wave6.py",
    ),
    (
        "16.2.4",
        "tests/ci/test_retired_make_rules.py",
    ),
    (
        "16.3.2",
        "tools/containers/test_kernel_image.py",
    ),
    (
        "16.3.3",
        "tools/containers/test_kernel_init.py",
    ),
    (
        "16.3.4",
        "tools/containers/test_platform_node_retired.py",
    ),
    (
        "16.3.6",
        "tools/containers/test_human_service_image.py",
    ),
    (
        "16.5.2",
        "tools/ci/tests/test_xweb_workflow_check.py",
    ),
    (
        "16.5.5",
        "tools/ci/tests/test_program_http_producer.py",
    ),
    (
        "16.7.2",
        "tools/bringup/tests/test_kernel_genesis_retired.py",
    ),
    (
        "16.7.3",
        "tools/bringup/tests/test_bringup_fixtures_retired.py",
    ),
    (
        "16.9.3",
        "tools/paxeer-x/tests/test_route_catalogue_lx.py",
    ),
    (
        "16.11.11",
        "tools/qualification/test_retired_callers_11.py",
    ),
    (
        "16.11.15",
        "tools/qualification/test_retired_callers_15.py",
    ),
    (
        "16.28.1",
        "tools/repotest/reachability_test.go",
    ),
    (
        "16.28.2",
        "tools/repotest/selectors_test.go",
    ),
    (
        "16.28.3",
        "tools/repotest/pytest_reachability_test.go",
    ),
    (
        "16.31.2",
        "tests/ci/test_retired_sequencer.py",
    ),
    (
        "16.31.3",
        "tests/ci/test_retired_batch_publication.py",
    ),
    (
        "16.31.4",
        "tests/ci/test_retired_replay_crossarch.py",
    ),
    (
        "16.31.6",
        "tests/ci/test_retired_qual_faults.py",
    ),
    (
        "16.32.2",
        "tests/ci/test_retired_equivocation_bond.py",
    ),
    (
        "16.32.9",
        "tests/ci/test_retired_batch_wal.py",
    ),
    (
        "16.32.10",
        "tests/ci/test_retired_batch_identity.py",
    ),
    (
        "16.32.11",
        "tests/ci/test_retired_finality_evidence.py",
    ),
    (
        "16.33.4",
        "tests/ci/test_retired_web.py",
    ),
    (
        "16.33.5",
        "tests/ci/test_retired_programs_batch.py",
    ),
    (
        "16.33.6",
        "tests/ci/test_retired_replay_support.py",
    ),
    (
        "16.34.1",
        "tests/ci/test_retired_daemon_lni.py",
    ),
    (
        "16.34.2",
        "tests/ci/test_retired_layerxd.py",
    ),
    (
        "16.34.3",
        "tests/ci/test_retired_verify_cli.py",
    ),
    (
        "16.34.4",
        "tests/ci/test_retired_relay_archive.py",
    ),
    (
        "16.97.2",
        "tests/ci/test_requested_suites_absent.py",
    ),
    (
        "16.97.3",
        "tests/ci/test_retired_requested_retirements_absent.py",
    ),
    (
        "16.97.5",
        "tests/ci/test_programs_suite_references.py",
    ),
)


class CompletionGateTest(unittest.TestCase):
    def test_tools_ops_completion_retired_surfaces_absent(self):
        makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
        declared_targets = set()
        for line in makefile.replace("\\\n", " ").splitlines():
            if line.startswith("\t"):
                continue
            targets, separator, prerequisites = line.split("#", 1)[0].partition(":")
            if separator:
                declared_targets.update(targets.split())
                if targets.strip() == ".PHONY":
                    declared_targets.update(prerequisites.split())

        for interface, target in RETIRED_MAKE_TARGETS:
            with self.subTest(interface=interface, path="Makefile", target=target):
                self.assertNotIn(
                    target,
                    declared_targets,
                    f"Retired surface {interface} remains in Makefile: {target}",
                )

        for interface, relative_path in RETIRED_PATHS:
            with self.subTest(interface=interface, path=relative_path):
                path = ROOT / relative_path
                self.assertFalse(
                    path.exists() or path.is_symlink(),
                    f"Retired surface {interface} remains at {relative_path}",
                )

        for task, relative_path in PLANNED_PATHS:
            with self.subTest(task=task, path=relative_path):
                self.assertTrue(
                    (ROOT / relative_path).is_file(),
                    f"Task {task} is missing planned file: {relative_path}",
                )


CI = ROOT / "scripts/ci"


def execute(argv, cwd, env):
    return subprocess.run(
        [str(arg) for arg in argv],
        cwd=cwd,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=30,
    )


@pytest.fixture
def source(tmp_path):
    root = (tmp_path / "source").resolve()
    root.mkdir()
    ci = root / "scripts" / "ci"
    ci.mkdir(parents=True)
    for name in ("release-gate.sh", "beta-ledger-check.sh"):
        shutil.copy2(CI / name, ci / name)
    (root / "input.txt").write_text("release source input\n")
    (root / "Makefile").write_text(
        ".PHONY: acceptance failed transient\n"
        "acceptance:\n\t@cat input.txt\n"
        "failed:\n\t@cat input.txt\n\t@exit 7\n"
        "transient:\n\t@cat input.txt\n"
        "\t@python3 -c 'from pathlib import Path; "
        'p = Path("transient-source"); p.write_text("mutation"); p.unlink()' + "'\n"
    )
    spec = root / "release.kvx"
    spec.write_text(
        '[req.8]\nac_5 = "Run the declared gate on its actual source"\n'
        '[task.4.1]\nreqs = ["8.5"]\nverify_cmd = "make acceptance"\n'
        '[task.4.2]\nreqs = ["8.5"]\nverify_cmd = "make failed"\n'
        '[task.4.3]\nreqs = ["8.5"]\nverify_cmd = "make transient"\n'
    )
    env = os.environ.copy()
    for key in tuple(env):
        if key.startswith("GIT_") or key.startswith("PAXEER_X_BETA_"):
            del env[key]
    env["GIT_CONFIG_NOSYSTEM"] = "1"
    env["GIT_CONFIG_GLOBAL"] = os.devnull
    env["GIT_AUTHOR_NAME"] = env["GIT_COMMITTER_NAME"] = "Release gate test"
    env["GIT_AUTHOR_EMAIL"] = env["GIT_COMMITTER_EMAIL"] = "gate@example.invalid"
    for command in (
        ["git", "init", "-q"],
        ["git", "add", "."],
        ["git", "-c", "core.hooksPath=/dev/null", "commit", "-qm", "Release source"],
    ):
        result = execute(command, root, env)
        assert result.returncode == 0, result.stderr
    revision = execute(["git", "rev-parse", "HEAD"], root, env).stdout.strip()
    private = (tmp_path / "private").resolve()
    private.mkdir(mode=0o700)
    return root, private, spec, revision, env


def release(source, task="4.1", command=("make", "acceptance"), extra=()):
    root, private, spec, _, env = source
    return execute(
        [
            "bash", root / "scripts/ci/release-gate.sh",
            "--job", "acceptance", "--spec", spec, "--task", task,
            "--ledger", private / "ledger.kvx", "--evidence-root", private,
            *extra, "--", *command,
        ],
        root,
        env,
    )


def consume(source):
    root, private, spec, revision, env = source
    return execute(
        [
            "bash", root / "scripts/ci/beta-ledger-check.sh",
            "--ledger", private / "ledger.kvx", "--spec", spec,
            "--candidate", revision, "--evidence-root", private,
        ],
        root,
        env,
    )


def release_log(record):
    sidecar = json.loads(Path(record["source_evidence"]).read_text())
    kind = sidecar["evidence"]["kind"]
    assert kind in ("file", "directory"), sidecar
    evidence = Path(record["evidence"])
    return evidence / "command.log" if kind == "directory" else evidence


def records(source):
    result = {}
    section = None
    for line in (source[1] / "ledger.kvx").read_text().splitlines():
        if line.startswith("["):
            section = result[line[1:-1]] = {}
        elif " = " in line:
            key, value = line.split(" = ", 1)
            section[key] = json.loads(value)
    return result


@pytest.mark.parametrize("kind", ["file", "directory"])
def test_release_record_is_accepted_unchanged(source, kind):
    root, private, _, revision, _ = source
    result = release(source, extra=("--evidence-kind", kind))
    assert result.returncode == 0, result.stdout + result.stderr
    summary = json.loads(result.stdout)
    assert summary["release_eligible"] is True
    assert summary["command_exit_status"] == 0
    ledger_before = (private / "ledger.kvx").read_bytes()
    record = records(source)["gate.4.1.1"]
    assert set(record) == {
        "task", "reqs", "revision", "command", "environment", "started_at",
        "outcome", "evidence", "source_evidence", "note",
    }
    assert record["revision"] == revision
    assert record["command"] == "make acceptance"
    assert record["outcome"] == "pass"
    assert record["reqs"] == ["8.5"]
    sidecar = Path(record["source_evidence"])
    assert str(sidecar) == summary["source_evidence"]
    sidecar_before = sidecar.read_bytes()
    value = json.loads(sidecar_before)
    evidence = Path(record["evidence"])
    log = evidence / "command.log" if kind == "directory" else evidence
    assert log.read_text() == (root / "input.txt").read_text()
    assert value["evidence"]["kind"] == kind
    assert value["source_start"] == value["source_end"]
    assert value["source_start"]["clean"] is True
    assert value["source_start"]["revision"] == revision
    assert value["source_start"]["source_identity"] == hashlib.sha256(
        revision.encode() + b"\0"
    ).hexdigest()
    assert value["argv"] == ["make", "acceptance"]
    assert value["mutation_observation"] == {
        "method": "linux-inotify-source-and-git-v1", "complete": True, "changed": False,
    }
    for path in (private, private / "ledger.kvx", evidence, log, sidecar):
        assert path.stat().st_mode & 0o077 == 0
        assert root not in path.parents
    accepted = consume(source)
    assert accepted.returncode == 0, accepted.stdout + accepted.stderr
    assert "1 gate record(s)" in accepted.stdout
    assert revision in accepted.stdout
    assert (private / "ledger.kvx").read_bytes() == ledger_before
    assert sidecar.read_bytes() == sidecar_before
    assert not (root / "spec/layerx-beta/evidence").exists()


def test_repeated_release_retains_both_source_sidecars(source):
    assert release(source).returncode == 0
    first = records(source)["gate.4.1.1"]
    retained = {
        key: (release_log(first) if key == "evidence" else Path(first[key])).read_bytes()
        for key in ("evidence", "source_evidence")
    }
    assert release(source).returncode == 0
    emitted = records(source)
    assert set(emitted) == {"gate.4.1.1", "gate.4.1.2"}
    assert emitted["gate.4.1.1"] == first
    for key, contents in retained.items():
        assert first[key] != emitted["gate.4.1.2"][key]
        path = release_log(first) if key == "evidence" else Path(first[key])
        assert path.read_bytes() == contents
    result = consume(source)
    assert result.returncode == 0, result.stderr
    assert "2 gate record(s)" in result.stdout


def test_command_failure_is_bound_to_source_and_rejected(source):
    result = release(source, "4.2", ("make", "failed"))
    assert result.returncode == 1, result.stdout + result.stderr
    summary = json.loads(result.stdout)
    assert summary["outcome"] == "fail"
    assert summary["command_exit_status"] == 2
    assert summary["release_eligible"] is False
    record = records(source)["gate.4.2.1"]
    assert record["outcome"] == "fail"
    value = json.loads(Path(record["source_evidence"]).read_text())
    assert value["command_exit_status"] == 2
    assert value["ineligibility_reasons"] == ["command_failed"]
    assert "release source input" in release_log(record).read_text()
    result = consume(source)
    assert result.returncode == 1
    assert "command_failed" in result.stderr


def test_dirty_source_is_retained_but_not_release_eligible(source):
    (source[0] / "input.txt").write_text("modified source input\n")
    result = release(source)
    assert result.returncode == 0, result.stdout + result.stderr
    assert json.loads(result.stdout)["release_eligible"] is False
    record = records(source)["gate.4.1.1"]
    assert release_log(record).read_text() == "modified source input\n"
    value = json.loads(Path(record["source_evidence"]).read_text())
    assert value["source_start"]["clean"] is False
    assert value["source_end"]["clean"] is False
    result = consume(source)
    assert result.returncode == 1
    assert "dirty_start" in result.stderr
    assert "dirty_end" in result.stderr


def test_transient_source_mutation_cannot_receive_gate_credit(source):
    result = release(source, "4.3", ("make", "transient"))
    assert result.returncode == 0, result.stdout + result.stderr
    assert json.loads(result.stdout)["release_eligible"] is False
    record = records(source)["gate.4.3.1"]
    value = json.loads(Path(record["source_evidence"]).read_text())
    assert value["source_start"] == value["source_end"]
    assert value["source_start"]["clean"] is True
    assert value["mutation_observation"]["complete"] is True
    assert value["mutation_observation"]["changed"] is True
    result = consume(source)
    assert result.returncode == 1
    assert "source_changed" in result.stderr


def test_command_must_match_selected_task_before_execution(source):
    result = release(source, command=("make", "transient"))
    assert result.returncode == 1
    assert "focused commands differ from selected task spec" in result.stdout
    assert list(source[1].iterdir()) == []


@pytest.mark.parametrize("artifact", ["evidence", "source_evidence"])
def test_consumer_rejects_missing_source_or_changed_log(source, artifact):
    result = release(source)
    assert result.returncode == 0, result.stdout + result.stderr
    record = records(source)["gate.4.1.1"]
    path = release_log(record) if artifact == "evidence" else Path(record[artifact])
    if artifact == "source_evidence":
        path.unlink()
    else:
        with path.open("a") as stream:
            stream.write("altered after execution\n")
    result = consume(source)
    assert result.returncode == 1
    assert "source evidence is not release eligible" in result.stderr


def test_legacy_unbound_cli_is_rejected_without_source_tree_evidence(source):
    root, private, _, _, env = source
    result = execute(
        ["bash", root / "scripts/ci/release-gate.sh", "--job", "acceptance",
         "--ordinal", "1", "--", "make", "acceptance"],
        root,
        env,
    )
    assert result.returncode == 2
    assert not (root / "spec/layerx-beta/evidence").exists()
    assert list(private.iterdir()) == []


def test_release_default_evidence_is_directory(source):
    result = release(source)
    assert result.returncode == 0, result.stdout + result.stderr
    record = records(source)["gate.4.1.1"]
    assert Path(record["evidence"]).is_dir()
    sidecar = json.loads(Path(record["source_evidence"]).read_text())
    assert sidecar["evidence"]["kind"] == "directory"
    assert release_log(record).read_text() == (source[0] / "input.txt").read_text()


if __name__ == "__main__":
    unittest.main()
