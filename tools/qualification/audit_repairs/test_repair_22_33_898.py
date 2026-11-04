"""Offline source proof of the production web image's wallet dependency wiring."""

import ast
import json
from pathlib import Path, PurePosixPath
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
WORKSPACE = "services/wallet"
SDK = "sdk/wallet"
RECORD = Path(__file__).with_name("repair_22_33_898.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def manifest(path):
    return json.loads(source(f"{path}/package.json"))


def build_instructions():
    text = re.sub(r"\\\n\s*", " ", source("docker/web/Dockerfile"))
    instructions = []
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        op, arguments = line.split(maxsplit=1)
        if op.upper() == "FROM" and instructions:
            break
        instructions.append((op.upper(), shlex.split(arguments)))
    assert instructions[0] == (
        "FROM", ["node:24.11.0-bookworm-slim", "AS", "build"]
    )
    return instructions


def image_copies(instructions):
    copies = {}
    cwd = PurePosixPath("/")
    for index, (op, args) in enumerate(instructions):
        if op == "WORKDIR":
            cwd = cwd / args[0]
        elif op == "COPY":
            assert not any(arg.startswith("--") for arg in args)
            destination = cwd / args[-1]
            for origin in args[:-1]:
                path = ROOT / origin
                assert path.exists(), origin
                if path.is_dir():
                    for child in path.rglob("*"):
                        if child.is_file() and "node_modules" not in child.parts:
                            relative = child.relative_to(path)
                            copies[str(destination / relative)] = (
                                str(child.relative_to(ROOT)), index
                            )
                else:
                    target = destination / path.name if args[-1].endswith("/") else destination
                    copies[str(target)] = (origin, index)
    return copies


def wallet_install(instructions):
    installs = [
        (i, args) for i, (op, args) in enumerate(instructions)
        if op == "RUN" and "pnpm" in args and "install" in args
    ]
    assert len(installs) == 1
    return installs[0]


def test_wallet_workspace_inputs_reach_original_image_paths():
    instructions = build_instructions()
    install_index, _ = wallet_install(instructions)
    copies = image_copies(instructions)
    workspace = yaml.safe_load(source(f"{WORKSPACE}/pnpm-workspace.yaml"))
    members = {
        (ROOT / WORKSPACE / member).resolve().relative_to(ROOT).as_posix()
        for member in workspace["packages"]
    }
    assert SDK in members
    inputs = {
        f"{WORKSPACE}/package.json", f"{WORKSPACE}/pnpm-workspace.yaml",
        f"{WORKSPACE}/pnpm-lock.yaml", "services/agent/sdk/typescript/package.json",
        *(f"{member}/package.json" for member in members),
    }
    for path in inputs:
        origin, copied_at = copies[f"/src/{path}"]
        assert origin == path
        assert copied_at < install_index
    assert copies["/src/sdk/wallet/src/index.ts"][0] == "sdk/wallet/src/index.ts"
    ignored = source("docker/web/Dockerfile.dockerignore").splitlines()
    assert "**/node_modules" in ignored
    assert not any(line.startswith("!") for line in ignored)
    assert not any(line.startswith("services/") or line.startswith("sdk/") for line in ignored)


def test_pinned_frozen_wallet_install_precedes_web_build():
    instructions = build_instructions()
    index, command = wallet_install(instructions)
    assert command == [
        "npm", "exec", "--yes", "--package=pnpm@9.15.9", "--", "pnpm",
        "--dir", "/src/services/wallet", "install", "--frozen-lockfile",
        "--ignore-scripts", "--filter", "@paxeer/wallet",
    ]
    runs = [(i, args) for i, (op, args) in enumerate(instructions) if op == "RUN"]
    web_install = next(i for i, args in runs if args == [
        "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"
    ])
    web_build = next(i for i, args in runs if args[-3:] == ["npm", "run", "build"])
    agent_build = next(i for i, args in runs if args == [
        "npm", "--prefix", "/src/services/agent/sdk/typescript", "run", "build"
    ])
    assert agent_build < index < web_install < web_build
    assert [(op, args) for op, args in instructions if op == "WORKDIR"] == [
        ("WORKDIR", ["/src/apps/web"])
    ]
    assert "next build --webpack" in manifest("apps/web")["scripts"]["build"]


def test_wallet_manifest_dependencies_have_locked_producers():
    lock = yaml.safe_load(source(f"{WORKSPACE}/pnpm-lock.yaml"))
    assert lock["lockfileVersion"] == "9.0"
    sdk = manifest(SDK)
    importer = lock["importers"]["../../sdk/wallet"]
    for group in ("dependencies", "devDependencies"):
        assert set(sdk[group]) == set(importer[group])
        for name, specifier in sdk[group].items():
            dependency = importer[group][name]
            assert dependency["specifier"] == specifier
            version = dependency["version"]
            if specifier.startswith("link:"):
                assert name == "@sidiora/layerx-sdk"
                assert version == "link:../../services/agent/sdk/typescript"
                assert manifest("services/agent/sdk/typescript")["name"] == name
                continue
            assert re.match(r"^\d+\.\d+\.\d+", version), (name, version)
            package = f"{name}@{version.split('(', 1)[0]}"
            assert lock["packages"][package]["resolution"]["integrity"].startswith("sha512-")
            assert f"{name}@{version}" in lock["snapshots"]
    assert {"viem", "@supabase/supabase-js"} <= set(importer["dependencies"])


def test_wallet_source_dependencies_reach_web_consumer():
    web = manifest("apps/web")
    assert web["dependencies"]["@paxeer/wallet"] == "file:../../sdk/wallet"
    paths = json.loads(source("apps/web/tsconfig.json"))["compilerOptions"]["paths"]
    assert paths["@paxeer/wallet"] == ["../../sdk/wallet/src/index.ts"]
    assert paths["@paxeer/wallet/provider"] == ["../../sdk/wallet/src/provider.ts"]
    sdk = manifest(SDK)
    assert sdk["name"] == "@paxeer/wallet"
    for alias in ("@paxeer/wallet", "@paxeer/wallet/provider"):
        path = (ROOT / "apps/web" / paths[alias][0]).resolve()
        assert path.is_relative_to(ROOT / SDK / "src")
        imports = set(re.findall(r"\bfrom\s+['\"]([^'\"]+)['\"]", path.read_text()))
        assert "viem" in imports
        if alias == "@paxeer/wallet":
            assert "@supabase/supabase-js" in imports
        for package in imports:
            if package.startswith("."):
                continue
            name = "/".join(package.split("/")[:2]) if package.startswith("@") else package.split("/")[0]
            assert name in sdk["dependencies"]
        # TypeScript walks the source ancestors; apps/web/node_modules is not one.
        resolution_paths = {parent / "node_modules" for parent in path.parents}
        assert ROOT / SDK / "node_modules" in resolution_paths
        assert ROOT / "apps/web/node_modules" not in resolution_paths
    assert "@sidiora/layerx-sdk/browser" in source("sdk/wallet/src/receipt.ts")


def test_repair_record_maps_finding_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    finding = record["finding"]["web-image-wallet-dependencies"]
    assert repair["task"] == "22.33.898"
    assert repair["audit"] == "22.30.185"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert finding["file"] == "docker/web/Dockerfile"
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.185.kvx"))
    original = audit["finding"]["web-image-wallet-dependencies"]
    assert original["file"] == finding["file"]
    assert original["class"] == finding["class"] == "wiring-gap"
    assert original["severity"] == finding["severity"] == "medium"
    functions = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == functions
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_898.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "exact verified candidate revision" in repair["verified_revision_source"]
    assert "No pre-change verification was run" in repair["baseline"]
    assert "No Docker image build" in repair["review_only"]
    requests = repair["wiring_requests"]
    assert any("services/wallet/pnpm-lock.yaml" in request and "domain 12" in request for request in requests)
    assert any("docker build -f docker/web/Dockerfile ." in request and "domain 16" in request for request in requests)
