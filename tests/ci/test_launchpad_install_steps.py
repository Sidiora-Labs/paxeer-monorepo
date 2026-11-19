"""System package installs refresh the index and retry before failing a job."""

from pathlib import Path
import re
import shlex

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
    form = re.fullmatch(
        r'#!/usr/bin/env bash\nset -euo pipefail\n\n'
        r'for attempt in \{1\.\.(\d+)\}; do\n'
        r'  if sudo apt-get update && sudo apt-get install "\$@"; then\n'
        r'    exit 0\n'
        r'  fi\n'
        r'  if \[ "\$attempt" -lt \1 \]; then\n'
        r'    sleep (\d+)\n'
        r'  fi\n'
        r'done\nexit 1\n',
        source,
    )
    assert form, "Each attempt must refresh the index, install, and pause on retry"
    assert int(form[1]) >= 3, "System package installs need at least three attempts"
    assert int(form[2]) > 0, "System package retries need a positive pause"
