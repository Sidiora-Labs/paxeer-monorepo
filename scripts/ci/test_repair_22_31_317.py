import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

import pytest


CI = Path(__file__).resolve().parent


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
        key: Path(first[key]).read_bytes() for key in ("evidence", "source_evidence")
    }
    assert release(source).returncode == 0
    emitted = records(source)
    assert set(emitted) == {"gate.4.1.1", "gate.4.1.2"}
    assert emitted["gate.4.1.1"] == first
    for key, contents in retained.items():
        assert first[key] != emitted["gate.4.1.2"][key]
        assert Path(first[key]).read_bytes() == contents
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
    assert "release source input" in Path(record["evidence"]).read_text()
    result = consume(source)
    assert result.returncode == 1
    assert "command_failed" in result.stderr


def test_dirty_source_is_retained_but_not_release_eligible(source):
    (source[0] / "input.txt").write_text("modified source input\n")
    result = release(source)
    assert result.returncode == 0, result.stdout + result.stderr
    assert json.loads(result.stdout)["release_eligible"] is False
    record = records(source)["gate.4.1.1"]
    assert Path(record["evidence"]).read_text() == "modified source input\n"
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
    path = Path(record[artifact])
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
