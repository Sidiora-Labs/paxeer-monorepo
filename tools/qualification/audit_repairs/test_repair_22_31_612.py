"""Keep first-party licence fields aligned and vendored licences attributed."""

import json
from pathlib import Path
import re
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
FIRST_PARTY_MANIFESTS = (
    "contracts/launchpad/package.json",
    "contracts/package.json",
    "sdk/launchpad/packages/candles-sdk/package.json",
    "sdk/launchpad/packages/indexer-sdk/package.json",
    "sdk/launchpad/packages/ranking-algo-sdk/package.json",
    "sdk/names/package.json",
    "sdk/wallet/package.json",
    "services/platform/perf-db/Cargo.toml",
    "services/wallet/ceremony/legacy-import/package.json",
    "services/wallet/package.json",
    "tests/integration/dapp_tests/package.json",
    "tests/integration/rpc_tests/package.json",
    "tools/names/package.json",
)
VENDORED_TREES = {
    "apps/explorer": ("blockscout/frontend", "GPL-3.0-only", "ISC", "MIT"),
    "services/explorer/backend": ("blockscout/blockscout", "GPL-3.0"),
    "services/explorer/services/libs": ("blockscout/blockscout-rs", "MIT"),
    "tools/benchmark/loadtest/contracts/evm/lib/openzeppelin-contracts": (
        "OpenZeppelin/openzeppelin-contracts",
        "MIT",
    ),
    "apps/inference-peer/engine": ("Nehanth/pooled", "MIT", "Apache-2.0 AND MIT"),
    "tools/hyperfide": ("Sidiora-Labs/hyperfide", "Apache-2.0", "MIT"),
}


def notice_tree_entries():
    notice = (ROOT / "NOTICE").read_text(encoding="utf-8")
    return {
        match[1]: match[2]
        for match in re.finditer(
            r"^([^\s:]+/[^\s:]+):([^\n]*(?:\n[ \t]+[^\n]*)*)", notice, re.MULTILINE
        )
    }


def manifest_licence_fields(relative_path):
    path = ROOT / relative_path
    text = path.read_text(encoding="utf-8")
    if path.name == "package.json":
        manifest = json.loads(text)
        return {"license": manifest["license"]} if "license" in manifest else {}

    manifest = tomllib.loads(text)
    fields = {}
    package = manifest.get("package", {})
    if "license" in package:
        licence = package["license"]
        if licence == {"workspace": True}:
            if "workspace" in package:
                workspace_paths = [path.parent / package["workspace"] / "Cargo.toml"]
            else:
                workspace_paths = [path.parent / "Cargo.toml"]
                workspace_paths.extend(
                    parent / "Cargo.toml"
                    for parent in path.parent.parents
                    if parent == ROOT or ROOT in parent.parents
                )
            for workspace_path in workspace_paths:
                if not workspace_path.is_file():
                    continue
                workspace_manifest = tomllib.loads(
                    workspace_path.read_text(encoding="utf-8")
                )
                if "workspace" in workspace_manifest:
                    licence = workspace_manifest["workspace"]["package"]["license"]
                    break
            else:
                raise AssertionError(f"No licence workspace found for {relative_path}")
        fields["package.license"] = licence
    workspace_package = manifest.get("workspace", {}).get("package", {})
    if "license" in workspace_package:
        fields["workspace.package.license"] = workspace_package["license"]
    return fields


@pytest.mark.parametrize("manifest", FIRST_PARTY_MANIFESTS)
def test_first_party_manifest_licence_fields_are_apache_2_0(manifest):
    fields = manifest_licence_fields(manifest)
    assert fields, f"Missing licence field in {manifest}"
    for field, licence in fields.items():
        assert licence == "Apache-2.0", f"{manifest}: {field} = {licence!r}"


def test_notice_records_every_vendored_tree_with_its_licence():
    entries = notice_tree_entries()
    for tree, required_terms in VENDORED_TREES.items():
        assert tree in entries, f"NOTICE omits {tree}"
        for term in required_terms:
            assert term in entries[tree], f"NOTICE entry for {tree} omits {term}"
    assert "apps/explorer/toolkit" in entries["apps/explorer"]
    assert "apps/explorer/deploy/tools" in entries["apps/explorer"]


def test_no_tracked_manifest_outside_notice_drifts_from_the_repository_licence():
    tracked = subprocess.run(
        [
            "git",
            "ls-files",
            "-z",
            "--",
            "**/Cargo.toml",
            "**/package.json",
            "Cargo.toml",
            "package.json",
        ],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    trees = notice_tree_entries()
    manifests = [name for name in tracked.stdout.split("\0") if name]
    assert manifests, "Git returned no tracked manifests"
    violations = []
    for manifest in manifests:
        path = Path(manifest)
        if {"node_modules", "vendor"}.intersection(path.parts[:-1]):
            continue
        if any(manifest.startswith(tree + "/") for tree in trees):
            continue
        expected = (
            "(Apache-2.0 AND MIT)"
            if manifest == "apps/inference-peer/package.json"
            else "Apache-2.0"
        )
        for field, licence in manifest_licence_fields(manifest).items():
            if licence != expected:
                violations.append(f"{manifest}: {field} = {licence!r}, expected {expected}")
    assert not violations, "Unattributed manifest licence drift:\n" + "\n".join(violations)
