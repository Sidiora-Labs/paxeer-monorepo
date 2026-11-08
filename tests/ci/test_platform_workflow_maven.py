from pathlib import Path
import re
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]


def test_platform_job_installs_maven_before_mvn():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/platform.yml").read_text(encoding="utf-8")
    )
    job = workflow["jobs"]["build-lint-test"]
    # The release workflow calls must preserve the primary job's Maven setup.
    assert "uses" not in job
    assert "if" not in job
    steps = job["steps"]
    updates = []
    installs = []
    versions = []
    invocations = []
    builds = []
    for step_index, step in enumerate(steps):
        for command_index, command in enumerate(
            re.split(r"&&|\n", step.get("run", ""))
        ):
            tokens = shlex.split(command, comments=True)
            position = (step_index, command_index)
            if tokens == ["sudo", "apt-get", "update"]:
                updates.append(position)
            if tokens[:3] == ["sudo", "apt-get", "install"] and "maven" in tokens[3:]:
                assert "if" not in step, "Maven installation must run unconditionally"
                assert not step.get("continue-on-error", False)
                installs.append(position)
            if tokens[:1] == ["mvn"]:
                invocations.append(position)
                if tokens[1:] == ["-v"]:
                    versions.append(position)
                else:
                    builds.append(position)

    assert installs, "The platform job must install Maven, not just a JDK"
    assert updates and min(updates) < min(installs)
    assert invocations and min(installs) < min(invocations)
    assert versions and min(installs) < min(versions)
    assert builds and min(versions) < min(builds)
    assert min(installs)[0] < min(builds)[0], "Install Maven before the sample step"
