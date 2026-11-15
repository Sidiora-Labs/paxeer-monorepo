import json
import os
from pathlib import Path
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
WALLET = ROOT / "services/wallet"
TRADING = ROOT / "apps/perps"


def wallet_members():
    workspace = yaml.safe_load((WALLET / "pnpm-workspace.yaml").read_text())
    patterns = workspace["packages"]
    assert isinstance(patterns, list) and patterns
    members = []
    for pattern in patterns:
        paths = list(WALLET.glob(pattern))
        assert paths, f"Workspace member has no matching path: {pattern}"
        members.extend(path.resolve() for path in paths)
    assert len(members) == len(set(members)), "Workspace members must be unique"
    return members


def test_wallet_workspace_members_have_manifests():
    members = wallet_members()
    assert {WALLET / "gateway", WALLET / "demo", ROOT / "sdk/wallet"} <= set(members)
    for member in [WALLET, *members]:
        assert member.is_dir(), member
        manifest = member / "package.json"
        assert manifest.is_file(), manifest
        assert json.loads(manifest.read_text())["name"], manifest


def test_wallet_workspace_excludes_trading_workspace():
    for member in wallet_members():
        assert not member.is_relative_to(TRADING), member


def test_wallet_lockfile_importers_match_members():
    document = yaml.compose((WALLET / "pnpm-lock.yaml").read_text())
    sections = [value for key, value in document.value if key.value == "importers"]
    assert len(sections) == 1, "Lockfile must have one importers section"
    importers = [key.value for key, _ in sections[0].value]
    assert len(importers) == len(set(importers)), "Lockfile importers must be unique"
    expected = {".", *(os.path.relpath(member, WALLET) for member in wallet_members())}
    assert set(importers) == expected


def test_wallet_container_installs_exclude_trading_manifests():
    tracked = subprocess.check_output(
        ["git", "ls-files", "-z", "--", "*Dockerfile*", "*Containerfile*"],
        cwd=ROOT,
        text=True,
    ).split("\0")
    checked = set()
    for name in tracked:
        if not name or name.endswith(".dockerignore"):
            continue
        path = ROOT / name
        if not path.is_file():
            continue
        text = re.sub(r"\\\s*\n", " ", path.read_text())
        if "services/wallet" not in text or not re.search(r"\binstall\b", text):
            continue
        checked.add(name)
        for line in text.splitlines():
            instruction = re.match(r"\s*(COPY|ADD)\s+(.+)", line, re.IGNORECASE)
            if not instruction:
                continue
            arguments = instruction.group(2)
            if re.search(r"(?:^|\s)--from(?:=|\s)", arguments):
                continue
            arguments = re.sub(r"^\s*(?:--\S+\s+)*", "", arguments)
            words = (
                json.loads(arguments)
                if arguments.startswith("[")
                else shlex.split(arguments)
            )
            for source in words[:-1]:
                source_path = (ROOT / source.lstrip("/")).resolve()
                assert not source_path.is_relative_to(TRADING), (name, source)
    assert {"docker/web/Dockerfile", "docker/wallet-gateway/Dockerfile"} <= checked
