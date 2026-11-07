"""Keep the public check configuration free of stale names and debug remnants."""

from pathlib import Path
import re
import tomllib

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = (
    ".github/workflows/paxeer-golangci.yml",
    ".github/workflows/paxeer-rocksdb-unit-tests.yml",
)
CONFIGS = (
    ".github/cliff.toml",
    ".github/codeql/codeql-config.yml",
    ".github/codecov.yml",
)


@pytest.mark.parametrize("relative", WORKFLOWS)
def test_workflows_have_no_todo(relative):
    text = (ROOT / relative).read_text(encoding="utf-8")
    findings = [
        f"{relative}:{number}: {line.strip()}"
        for number, line in enumerate(text.splitlines(), 1)
        if re.search(r"\bTODO\b", line, re.IGNORECASE)
    ]
    assert not findings, "Unresolved workflow TODOs:\n" + "\n".join(findings)


@pytest.mark.parametrize("relative", WORKFLOWS)
def test_workflows_have_no_commented_keys(relative):
    text = (ROOT / relative).read_text(encoding="utf-8")
    commented_key = re.compile(
        r"^\s*#\s*(?:-\s*)?[\"']?[a-z_][\w-]*[\"']?\s*:(?:\s|$)"
    )
    findings = [
        f"{relative}:{number}: {line.strip()}"
        for number, line in enumerate(text.splitlines(), 1)
        if commented_key.search(line)
    ]
    assert not findings, "Commented-out workflow keys:\n" + "\n".join(findings)
    workflow = yaml.load(text, Loader=yaml.BaseLoader)
    assert workflow["permissions"] == {"contents": "read"}
    assert workflow["jobs"], f"{relative} must retain its jobs"
    if relative.endswith("paxeer-rocksdb-unit-tests.yml"):
        steps = workflow["jobs"]["tests-rocksdb"]["steps"]
        assert any(
            "go test " in step.get("run", "")
            and "./storage/db_engine/rocksdb/..." in step["run"]
            for step in steps
        ), "The RocksDB workflow must still execute its package tests"


@pytest.mark.parametrize("relative", CONFIGS)
def test_ci_config_uses_current_project_name(relative):
    text = (ROOT / relative).read_text(encoding="utf-8")
    for stale_name in ("Paxeer X Network", "Paxeer-X-Network", "pax-chain"):
        assert stale_name.casefold() not in text.casefold(), (relative, stale_name)
    assert "paxeer-monorepo" in text, relative

    if relative.endswith(".toml"):
        config = tomllib.loads(text)
        assert "paxeer-monorepo" in config["changelog"]["header"]
    elif relative.endswith("codeql-config.yml"):
        config = yaml.load(text, Loader=yaml.BaseLoader)
        assert config["name"] == "paxeer-monorepo CodeQL config"
        assert {"uses": "security-extended"} in config["queries"]
    else:
        config = yaml.load(text, Loader=yaml.BaseLoader)
        flags = config["flags"]
        assert set(flags) == {"modules", "precompiles", "storage"}
        for name, flag in flags.items():
            assert flag["paths"] == [f"{name}/"]
            package = ROOT / name
            assert package.is_dir(), f"Coverage path does not exist: {name}"
            assert any(package.rglob("*.go")), f"No Go packages under {name}"
        projects = config["coverage"]["status"]["project"]
        assert projects["paxeer-monorepo"]["target"] == "40%"
        assert projects["paxeer-monorepo-pr"]["informational"] == "true"
        for name in ("paxeer-monorepo", "paxeer-monorepo-pr"):
            assert projects[name]["paths"] == ["**/*.go"]
            assert "flags" not in projects[name], (
                "Project checks must accept existing uploads"
            )
