from datetime import date, datetime, timezone
import json
from pathlib import Path
import re
import tomllib
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


def _assert_runtime_floor(version, minimum, location):
    assert re.fullmatch(r"\d+(?:\.\d+){0,2}", version), (
        f"{location}: invalid runtime version {version!r}"
    )
    parts = tuple(map(int, version.split(".")))
    parts += (0,) * (3 - len(parts))
    floor = minimum + (0,) * (3 - len(minimum))
    assert parts >= floor, f"{location}: {version} is below the {minimum} floor"


def test_node_engine_floors_supported():
    packages = (
        "services/explorer/backend/apps/explorer/package.json",
        "services/explorer/backend/apps/block_scout_web/assets/package.json",
        "sdk/names/package.json",
        "services/wallet/package.json",
    )
    for relative in packages:
        package = json.loads((ROOT / relative).read_text(encoding="utf-8"))
        engine = package["engines"]["node"]
        assert isinstance(engine, str), f"{relative}: Node engine must be a string"
        match = re.fullmatch(r">=\s*(\d+(?:\.\d+){0,2})", engine.strip())
        assert match is not None, f"{relative}: invalid Node floor {engine!r}"
        _assert_runtime_floor(match[1], (22,), relative)

    relative = "services/explorer/backend/.tool-versions"
    declarations = [
        line.partition("#")[0].strip()
        for line in (ROOT / relative).read_text(encoding="utf-8").splitlines()
        if re.match(r"\s*nodejs\b", line)
    ]
    assert len(declarations) == 1, f"{relative}: expected one Node declaration"
    match = re.fullmatch(r"nodejs\s+(\d+\.\d+\.\d+)", declarations[0])
    assert match is not None, f"{relative}: invalid Node version {declarations[0]!r}"
    _assert_runtime_floor(match[1], (22,), relative)


def test_requires_python_supported():
    relative = "sdk/chainflow/python/pyproject.toml"
    project = tomllib.loads((ROOT / relative).read_text(encoding="utf-8"))
    requirement = project["project"]["requires-python"]
    assert isinstance(requirement, str), f"{relative}: requires-python must be a string"
    match = re.fullmatch(r">=\s*(\d+\.\d+(?:\.\d+)?)", requirement.strip())
    assert match is not None, f"{relative}: invalid Python floor {requirement!r}"
    _assert_runtime_floor(match[1], (3, 11), relative)

    relative = ".github/workflows/paxeer-integration-test.yml"
    workflow = yaml.safe_load((ROOT / relative).read_text(encoding="utf-8"))
    steps = workflow["jobs"]["integration-tests"]["steps"]
    installs = [
        step
        for step in steps
        if step.get("uses", "").partition("@")[0] == "actions/setup-python"
    ]
    assert installs, f"{relative}: integration tests must explicitly install Python"
    for step in installs:
        version = step.get("with", {}).get("python-version")
        assert isinstance(version, str), f"{relative}: missing explicit Python version"
        assert re.fullmatch(r"\d+\.\d+(?:\.\d+)?", version), (
            f"{relative}: invalid Python interpreter version {version!r}"
        )
        _assert_runtime_floor(version, (3, 12), relative)


def test_go_directives_supported():
    paths = (
        "sdk/platform/go/go.mod",
        "services/platform/examples/samples/first-payment-go/go.mod",
        "sdk/chainflow/go/go.mod",
        "sdk/chainflow/go/go.work",
        "tools/hpx/registry/go.mod",
    )
    for relative in paths:
        declarations = [
            line.partition("//")[0].strip()
            for line in (ROOT / relative).read_text(encoding="utf-8").splitlines()
            if re.match(r"\s*go\b", line)
        ]
        assert len(declarations) == 1, f"{relative}: expected one Go directive"
        match = re.fullmatch(r"go\s+(\d+\.\d+(?:\.\d+)?)", declarations[0])
        assert match is not None, f"{relative}: invalid Go directive {declarations[0]!r}"
        _assert_runtime_floor(match[1], (1, 25), relative)


def test_nvmrc_matches_image_node():
    relative = "apps/wallet/.nvmrc"
    version = (ROOT / relative).read_text(encoding="utf-8").strip()
    assert re.fullmatch(r"\d+\.\d+\.\d+", version), (
        f"{relative}: expected an explicit Node version, got {version!r}"
    )
    relative = "docker/wallet-pwa/Dockerfile"
    stages = {}
    for index, line in enumerate(
        (ROOT / relative).read_text(encoding="utf-8").splitlines(), start=1
    ):
        if not re.match(r"\s*FROM\b", line, re.IGNORECASE):
            continue
        location = f"{relative}:{index}"
        match = re.fullmatch(
            r"\s*FROM\s+node:(\d+\.\d+\.\d+)"
            r"(?:-[a-zA-Z0-9_.-]+)?(?:@sha256:[0-9a-f]{64})?"
            r"\s+AS\s+(\w+)\s*",
            line,
            re.IGNORECASE,
        )
        assert match is not None, f"{location}: expected a versioned Node image"
        assert match[2] not in stages, f"{location}: duplicate stage {match[2]}"
        assert match[1] == version, (
            f"{location}: Node {match[1]} differs from apps/wallet/.nvmrc {version}"
        )
        stages[match[2]] = match[1]
    assert set(stages) == {"sdk", "builder", "runner"}, (
        f"{relative}: expected sdk, builder and runner Node images"
    )
