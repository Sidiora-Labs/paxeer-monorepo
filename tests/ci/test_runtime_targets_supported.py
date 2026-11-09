from datetime import date, datetime, timezone
import json
from pathlib import Path
import re
import xml.etree.ElementTree as ET

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
DOTNET_PROJECTS = (
    "sdk/platform/dotnet/LayerX.Sdk.csproj",
    "sdk/platform/dotnet/tests/LayerX.Sdk.Tests/LayerX.Sdk.Tests.csproj",
    "sdk/platform/dotnet/tests/Repair138/Repair138.csproj",
    "services/platform/examples/samples/first-payment-csharp/FirstPayment.csproj",
)


def _assert_dotnet_supported(version, location):
    end_of_life = json.loads(
        (ROOT / "tests/ci/data/runtime_end_of_life.json").read_text(encoding="utf-8")
    )["dotnet"]
    assert tuple(map(int, version.split("."))) >= (10, 0), (
        f"{location}: .NET {version} is below the .NET 10 floor"
    )
    assert version in end_of_life, f"{location}: unknown .NET release {version}"
    expires = date.fromisoformat(end_of_life[version])
    today = datetime.now(timezone.utc).date()
    assert today < expires, (
        f"{location}: .NET {version} reaches end of life on {expires} (today: {today})"
    )


@pytest.mark.parametrize("relative", DOTNET_PROJECTS)
def test_dotnet_target_frameworks_supported(relative):
    project = ET.parse(ROOT / relative).getroot()
    frameworks = project.findall("./PropertyGroup/TargetFramework")
    frameworks += project.findall("./PropertyGroup/TargetFrameworks")
    assert frameworks, f"{relative}: missing target framework"
    for element in frameworks:
        for framework in (element.text or "").split(";"):
            match = re.fullmatch(
                r"net(\d+\.\d+)(?:-[a-zA-Z0-9.]+)?", framework.strip()
            )
            assert match is not None, f"{relative}: invalid framework {framework!r}"
            _assert_dotnet_supported(match[1], relative)


def test_workflow_dotnet_version_supported():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/platform.yml").read_text(encoding="utf-8")
    )
    installs = []
    for job_name, job in workflow["jobs"].items():
        for index, step in enumerate(job.get("steps", [])):
            location = f"platform.yml:{job_name}:{step.get('name', index)}"
            if step.get("uses", "").partition("@")[0] == "actions/setup-dotnet":
                versions = step.get("with", {}).get("dotnet-version", "").split()
                assert versions, f"{location}: missing explicit .NET SDK version"
                for version in versions:
                    match = re.fullmatch(r"(\d+\.\d+)\.(?:x|\d+)", version)
                    assert match is not None, f"{location}: invalid SDK {version!r}"
                    _assert_dotnet_supported(match[1], location)
                installs.append((job_name, index))
            for framework in re.findall(
                r"--framework\s+(\S+)", step.get("run", "")
            ):
                match = re.fullmatch(r"net(\d+\.\d+)", framework)
                assert match is not None, f"{location}: invalid framework {framework!r}"
                _assert_dotnet_supported(match[1], location)

    assert installs, "Platform workflow must explicitly install the .NET SDK"
    steps = workflow["jobs"]["build-lint-test"]["steps"]
    public_installs = [index for job, index in installs if job == "build-lint-test"]
    assert public_installs, "The public check must install the .NET SDK"
    for index in public_installs:
        assert "if" not in steps[index], "Public .NET setup must run unconditionally"
        assert not steps[index].get("continue-on-error", False)
    consumers = [
        index
        for index, step in enumerate(steps)
        if step.get("run")
        in {
            "make platform-dependencies-install",
            "make platform-verify-sdks",
        }
    ]
    assert len(consumers) == 2, (
        "Public check must install dependencies and build the SDKs"
    )
    assert max(public_installs) < min(consumers), "Install .NET before its consumers"
    for index in consumers:
        assert "if" not in steps[index], "Public SDK checks must run unconditionally"
        assert not steps[index].get("continue-on-error", False)
