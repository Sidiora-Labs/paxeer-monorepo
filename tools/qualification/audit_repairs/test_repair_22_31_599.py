from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "apps/explorer/deploy/scripts/collect_envs.sh"


def test_builder_collects_envs_from_consolidated_docs():
    dockerfile = (ROOT / "docker/explorer-frontend/Dockerfile").read_text()
    assert "./docs/ENVS.md" not in dockerfile
    builder = dockerfile.split(" AS builder\n", 1)[1].split("\nFROM ", 1)[0]
    copy = "COPY docs/explorer/ENVS.md ./docs/explorer/ENVS.md"
    collect = "./deploy/scripts/collect_envs.sh ./docs/explorer/ENVS.md"
    assert copy in builder
    assert collect in builder
    assert builder.index(copy) < builder.index(collect)


def test_collect_envs_writes_registry_from_consolidated_docs(tmp_path):
    result = subprocess.run(
        ["bash", str(SCRIPT), str(ROOT / "docs/explorer/ENVS.md")],
        cwd=tmp_path,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    registry = tmp_path / ".env.registry"
    assert registry.is_file()
    lines = registry.read_text().splitlines()
    assert lines
    assert all(line.startswith("NEXT_PUBLIC_") for line in lines)
    assert all(re.fullmatch(r"NEXT_PUBLIC_[A-Za-z0-9_]+=__", line) for line in lines)
    assert lines == sorted(set(lines))
    assert (tmp_path / ".env").is_file()


def test_collect_envs_rejects_missing_document(tmp_path):
    document = tmp_path / "missing.md"
    result = subprocess.run(
        ["bash", str(SCRIPT), str(document)],
        cwd=tmp_path,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert f"Environment documentation not found: {document}" in result.stderr
    assert not (tmp_path / ".env.registry").exists()
