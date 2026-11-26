"""System package installs refresh the index and retry before failing a job."""

import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/launchpad-test.yml"
INSTALL_SCRIPT = ROOT / "scripts/ci/install-system-packages.sh"
INSTALL_COMMAND = 'bash "$GITHUB_WORKSPACE/scripts/ci/install-system-packages.sh"'
APT_INSTALL = re.compile(r"\bapt(?:-get)?\b[^\n;&|]*\binstall\b")


def launchpad_install_steps():
    workflow = yaml.load(WORKFLOW.read_text(), Loader=yaml.BaseLoader)
    installs = []
    for job_name, job in workflow["jobs"].items():
        for position, step in enumerate(job.get("steps", [])):
            for line in step.get("run", "").replace("\\\n", " ").splitlines():
                if APT_INSTALL.search(line) or "install-system-packages.sh" in line:
                    installs.append((job_name, job, position, step, line.strip()))
    return installs


def test_launchpad_system_package_install_inventory_is_not_empty():
    installs = launchpad_install_steps()
    assert installs, "The workflow must declare system package installs"
    assert len({(name, step["name"], line) for name, _, _, step, line in installs}) >= 6


@pytest.mark.parametrize(
    ("job_name", "job", "position", "step", "command"),
    launchpad_install_steps(),
    ids=lambda value: value if isinstance(value, str) else None,
)
def test_launchpad_system_package_installs_use_shared_retries(
    job_name, job, position, step, command
):
    context = (job_name, step["name"], command)
    assert command.startswith(INSTALL_COMMAND + " "), context
    tokens = shlex.split(command)
    assert tokens[:2] == [
        "bash", "$GITHUB_WORKSPACE/scripts/ci/install-system-packages.sh",
    ], context
    assert len(tokens) > 2, context
    assert re.fullmatch(
        re.escape(INSTALL_COMMAND) + r"(?: [\w.-]+)+", command
    ), context
    assert "continue-on-error" not in job, context
    assert "continue-on-error" not in step, context
    assert "GITHUB_WORKSPACE" not in job.get("env", {}), context
    assert "GITHUB_WORKSPACE" not in step.get("env", {}), context
    assert any(
        previous.get("uses", "").startswith("actions/checkout@")
        and "if" not in previous
        and "continue-on-error" not in previous
        and "path" not in previous.get("with", {})
        for previous in job["steps"][:position]
    ), context


def test_launchpad_system_package_retries_refresh_and_propagate_failure():
    source = INSTALL_SCRIPT.read_text()
    attempts = re.search(r'^for attempt in \{1\.\.(\d+)\}; do$', source, re.M)
    pause = re.search(r'^\s+sleep (\d+)$', source, re.M)
    update_bound = re.search(
        r'^readonly APT_UPDATE_TIMEOUT_SECONDS=(\d+)$', source, re.M
    )
    install_bound = re.search(
        r'^readonly APT_INSTALL_TIMEOUT_SECONDS=(\d+)$', source, re.M
    )
    assert attempts and pause and update_bound and install_bound, (
        "Each attempt must refresh the index, install, and pause on retry"
    )
    assert int(attempts[1]) >= 3, (
        "System package installs need at least three attempts"
    )
    assert int(pause[1]) > 0, "System package retries need a positive pause"
    update_timeout = int(update_bound[1])
    install_timeout = int(install_bound[1])
    assert 30 <= update_timeout <= 300, "Package index reads need a bounded timeout"
    assert update_timeout <= install_timeout <= 900, (
        "Package installs need a bounded timeout"
    )
    assert source.startswith('#!/usr/bin/env bash\nset -euo pipefail\n')
    for variable, command in [
        ("APT_UPDATE_TIMEOUT_SECONDS", "update"),
        ("APT_INSTALL_TIMEOUT_SECONDS", 'install "$@"'),
    ]:
        assert (
            'timeout --kill-after=10s "${' + variable + '}s" '
            'sudo apt-get -o Acquire::Retries=3 ' + command
        ) in source


@pytest.fixture
def system_package_runner(tmp_path):
    apt_root = tmp_path / "apt config"
    sources_dir = apt_root / "sources.list.d"
    sources_dir.mkdir(parents=True)
    originals = {
        "sources.list": (
            "# Ubuntu archives\n"
            "deb http://us.archive.ubuntu.com/ubuntu noble main\n"
            "deb https://security.ubuntu.com/ubuntu noble-security main\n"
        ),
        "sources.list.d/ubuntu.sources": (
            "Types: deb\n"
            "URIs: https://eu.west.archive.ubuntu.com/ubuntu/ "
            "http://gb.archive.ubuntu.com:80/ubuntu\n"
            "Suites: noble noble-updates\nComponents: main universe\n\n"
            "Types: deb\nURIs: http://security.ubuntu.com/ubuntu\n"
            "Suites: noble-security\nComponents: main\n"
        ),
        "sources.list.d/ubuntu.list": (
            "deb https://de.archive.ubuntu.com/ubuntu noble-updates main\n"
        ),
        "apt-mirrors.txt": (
            "http://us.archive.ubuntu.com/ubuntu\n"
            "https://gb.archive.ubuntu.com/ubuntu/\n"
            "https://archive.ubuntu.com/ubuntu\n"
            "https://example.org/ubuntu\n"
        ),
        "sources.list.d/other.sources": (
            "Types: deb\nURIs: https://example.org/ubuntu\n"
            "Suites: stable\nComponents: main\n"
            "# https://us.archive.ubuntu.com.example.org/ubuntu\n"
            "# ftp://us.archive.ubuntu.com/ubuntu\n"
            "# https://notarchive.ubuntu.com/ubuntu\n"
            "# https://example.org/us.archive.ubuntu.com/ubuntu\n"
        ),
        "sources.list.d/ubuntu.list.save": (
            "deb http://us.archive.ubuntu.com/ubuntu noble main\n"
        ),
    }
    for name, content in originals.items():
        path = apt_root / name
        path.write_text(content)
        os.utime(path, ns=(1_000_000_000, 1_000_000_000))

    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    command_source = f"#!{sys.executable}\n" + '''\
import json
import os
from pathlib import Path
import sys

command = Path(sys.argv[0]).name
args = sys.argv[1:]
if command == "sudo":
    if args[0] == "python3" and os.environ.get("ARCHIVE_SWITCH_FAIL"):
        sys.exit(1)
    os.execvp(args[0], args)

root = Path(os.environ["APT_CONFIG_ROOT"])
sources = {
    str(path.relative_to(root)): {
        "content": path.read_text(),
        "mtime": path.stat().st_mtime_ns,
        "inode": path.stat().st_ino,
    }
    for path in root.rglob("*") if path.is_file()
}
log = Path(os.environ["COMMAND_LOG"])
events = [json.loads(line) for line in log.read_text().splitlines()]
with log.open("a") as stream:
    stream.write(json.dumps({
        "command": command, "args": args, "sources": sources,
    }) + "\\n")
if command == "apt-get":
    if args[2] == "update":
        sys.exit(int(os.environ.get("INDEX_READ_FAIL", "0")))
    installs = sum(
        event["command"] == "apt-get" and event["args"][2] == "install"
        for event in events
    )
    sys.exit(1 if installs < int(os.environ["INSTALL_FAILURES"]) else 0)
if command == "dpkg":
    sys.exit(int(os.environ.get("DPKG_EXIT", "0")))
'''
    for name in ["sudo", "apt-get", "dpkg", "sleep"]:
        executable = bin_dir / name
        executable.write_text(command_source)
        executable.chmod(0o755)
    log = tmp_path / "commands.jsonl"
    log.write_text("")

    def run(failures=0, **extra_env):
        arguments = ["--yes", "--no-install-recommends", "package name", "", "a*b"]
        result = subprocess.run(
            ["bash", str(INSTALL_SCRIPT), *arguments],
            env={
                **os.environ,
                "PATH": str(bin_dir) + os.pathsep + os.environ["PATH"],
                "APT_CONFIG_ROOT": str(apt_root),
                "COMMAND_LOG": str(log),
                "INSTALL_FAILURES": str(failures),
                **extra_env,
            },
            capture_output=True,
            text=True,
            timeout=10,
        )
        events = [json.loads(line) for line in log.read_text().splitlines()]
        for event in events:
            if event["command"] == "apt-get":
                assert event["args"][:2] == ["-o", "Acquire::Retries=3"]
                if event["args"][2] == "install":
                    assert event["args"][3:] == arguments
        return result, events

    return apt_root, originals, run


@pytest.mark.parametrize("failures", [0, 1, 2, 3])
def test_launchpad_system_package_attempts_use_main_archive(
    system_package_runner, failures
):
    apt_root, originals, run = system_package_runner
    expected = dict(originals)
    expected["sources.list"] = (
        "# Ubuntu archives\n"
        "deb http://archive.ubuntu.com/ubuntu noble main\n"
        "deb https://security.ubuntu.com/ubuntu noble-security main\n"
    )
    expected["sources.list.d/ubuntu.sources"] = (
        "Types: deb\n"
        "URIs: https://archive.ubuntu.com/ubuntu/ "
        "http://archive.ubuntu.com:80/ubuntu\n"
        "Suites: noble noble-updates\nComponents: main universe\n\n"
        "Types: deb\nURIs: http://security.ubuntu.com/ubuntu\n"
        "Suites: noble-security\nComponents: main\n"
    )
    expected["sources.list.d/ubuntu.list"] = (
        "deb https://archive.ubuntu.com/ubuntu noble-updates main\n"
    )
    expected["apt-mirrors.txt"] = (
        "http://archive.ubuntu.com/ubuntu\n"
        "https://archive.ubuntu.com/ubuntu/\n"
        "https://archive.ubuntu.com/ubuntu\n"
        "https://example.org/ubuntu\n"
    )
    result, events = run(failures)
    attempts = min(failures + 1, 3)
    assert result.returncode == (1 if failures == 3 else 0), result.stderr
    assert [event["command"] for event in events] == (
        ["apt-get", "apt-get"]
        + ["sleep", "dpkg", "apt-get", "apt-get"] * (attempts - 1)
    )
    updates = [
        event for event in events
        if event["command"] == "apt-get" and event["args"][2] == "update"
    ]
    for index, event in enumerate(updates):
        contents = originals if index == 0 else expected
        assert {
            name: source["content"] for name, source in event["sources"].items()
        } == contents
        for name, source in event["sources"].items():
            if index == 0 or originals[name] == expected[name]:
                assert source["mtime"] == 1_000_000_000
    if attempts == 3:
        assert updates[1]["sources"] == updates[2]["sources"]
    installs = [
        event for event in events
        if event["command"] == "apt-get" and event["args"][2] == "install"
    ]
    assert len(installs) == attempts
    for update, install in zip(updates, installs):
        assert install["sources"] == update["sources"]
    repairs = [event for event in events if event["command"] == "dpkg"]
    for previous, repair in zip(installs, repairs):
        assert repair["sources"] == previous["sources"]
    for event in events:
        if event["command"] == "sleep":
            assert event["args"] == ["5"]
        elif event["command"] == "dpkg":
            assert event["args"] == ["--configure", "-a"]
    lines = result.stdout.splitlines()
    assert len(lines) == attempts
    assert lines[0] == "System package attempt 1: runner sources"
    for attempt in range(2, attempts + 1):
        assert lines[attempt - 1] == (
            f"System package attempt {attempt}: main host archive.ubuntu.com sources"
        )
    for name, content in (originals if attempts == 1 else expected).items():
        assert (apt_root / name).read_text() == content
        if attempts == 1 or originals[name] == expected[name]:
            assert (apt_root / name).stat().st_mtime_ns == 1_000_000_000


def test_launchpad_system_package_retries_with_missing_source_files(
    system_package_runner,
):
    apt_root, originals, run = system_package_runner
    for name in originals:
        (apt_root / name).unlink()
    (apt_root / "sources.list.d").rmdir()
    result, events = run(1)
    assert result.returncode == 0, result.stderr
    assert len([event for event in events if event["command"] == "apt-get"]) == 4
    assert list(apt_root.iterdir()) == []


def test_launchpad_system_package_continues_after_archive_switch_error(
    system_package_runner,
):
    apt_root, originals, run = system_package_runner
    result, events = run(1, ARCHIVE_SWITCH_FAIL="1", DPKG_EXIT="1")
    assert result.returncode == 0
    assert "Could not switch archive sources; continuing." in result.stderr
    assert len([event for event in events if event["command"] == "apt-get"]) == 4
    for name, content in originals.items():
        assert (apt_root / name).read_text() == content


def test_launchpad_system_package_failed_index_reads_propagate_failure(
    system_package_runner,
):
    _, _, run = system_package_runner
    result, events = run(INDEX_READ_FAIL="1")
    assert result.returncode == 1
    apt_calls = [event for event in events if event["command"] == "apt-get"]
    assert len(apt_calls) == 3
    assert all(event["args"][2:] == ["update"] for event in apt_calls)
