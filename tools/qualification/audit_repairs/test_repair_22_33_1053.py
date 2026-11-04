"""Prove finding 27 against committed Solidity, role ABI and package source bytes.

This gate parses the published data path; it does not compile Solidity or execute
TypeScript, deploy a contract or infer deployment addresses.
"""

import hashlib
import json
import re
from pathlib import Path, PurePosixPath

import pytest


ROOT = Path(__file__).resolve().parents[3]
PACKAGE = ROOT / "sdk/launchpad/packages/sidiora-launchpad-v3"
ROLES = (
    ("factoryAbi", "core", "SidioraFactory"),
    ("routerAbi", "periphery", "Router"),
    ("quoterAbi", "periphery", "Quoter"),
    ("protocolConfigAbi", "protocol", "ProtocolConfig"),
    ("erc20Abi", "core", "SidioraERC20"),
)
LICENSE = "// SPDX-License-Identifier: LicenseRef-Paxlabs-HyperPax-OS-Protocol"


def contained_file(path):
    assert isinstance(path, str) and path
    relative = PurePosixPath(path)
    assert not relative.is_absolute()
    assert ".." not in relative.parts
    assert "\\" not in path and ":" not in path
    assert relative.as_posix() == path
    resolved = (ROOT / path).resolve(strict=True)
    assert resolved.is_relative_to(ROOT)
    assert resolved.is_file()
    return resolved


def provenance():
    return json.loads((PACKAGE / "provenance.json").read_bytes())


def exported_abis():
    """Decode the actual JSON literal initializers without a JS toolchain."""
    remaining = (PACKAGE / "src/abis.ts").read_text()
    exports = {}
    while remaining.strip():
        declaration = re.match(r"\s*export const (\w+) =\s*", remaining)
        assert declaration is not None, "ABI source must contain literal exports"
        name = declaration.group(1)
        assert name not in exports, f"duplicate ABI export: {name}"
        value, end = json.JSONDecoder().raw_decode(remaining, declaration.end())
        suffix = re.match(r"\s+as const;", remaining[end:])
        assert suffix is not None, f"{name}: expected an immutable literal ABI"
        assert isinstance(value, list) and value, name
        exports[name] = value
        remaining = remaining[end + suffix.end() :]
    return exports


def test_finding_27_complete_provenance_without_deployment_defaults():
    document = provenance()
    assert set(document) == {"abi_sources", "deployment_defaults", "app_imports"}
    assert document["deployment_defaults"] is False
    assert document["app_imports"] == []
    names = [entry["export"] for entry in document["abi_sources"]]
    expected = [name for name, _, _ in ROLES]
    assert sorted(names) == sorted(expected)
    assert set(exported_abis()) == set(expected)

    manifest = json.loads((PACKAGE / "package.json").read_bytes())
    assert manifest["exports"] == {
        ".": "./src/index.ts",
        "./abis": "./src/abis.ts",
    }
    index = (PACKAGE / manifest["exports"]["."]).read_text()
    assert re.search(r"^export \* from './abis\.js';$", index, re.MULTILINE)
    assert (PACKAGE / manifest["exports"]["./abis"]).resolve() == (
        PACKAGE / "src/abis.ts"
    ).resolve()


@pytest.mark.parametrize("export,directory,role", ROLES)
def test_finding_27_authoritative_source_artifact_and_export(export, directory, role):
    entry = next(item for item in provenance()["abi_sources"] if item["export"] == export)
    assert set(entry) == {
        "export",
        "source",
        "source_sha256",
        "artifact",
        "artifact_sha256",
        "license",
    }
    assert entry["source"] == f"contracts/launchpad/contracts/{directory}/{role}.sol"
    assert entry["artifact"] == f"precompiles/launchpad/abi/{role}.json"
    for kind in ("source", "artifact"):
        data = contained_file(entry[kind]).read_bytes()
        digest = entry[f"{kind}_sha256"]
        assert re.fullmatch(r"[0-9a-f]{64}", digest), export
        assert hashlib.sha256(data).hexdigest() == digest, f"{export}: {kind} digest"

    source = contained_file(entry["source"]).read_text()
    assert re.search(rf"^contract\s+{role}\b", source, re.MULTILINE), export
    markers = [
        line.strip()
        for line in source.splitlines()
        if line.strip().startswith("// SPDX-License-Identifier:")
    ]
    assert entry["license"] == LICENSE
    assert markers == [entry["license"]], export

    artifact = json.loads(contained_file(entry["artifact"]).read_bytes())
    assert isinstance(artifact, list) and artifact, export
    assert exported_abis()[export] == artifact, f"{export}: complete authoritative ABI"


def test_finding_27_erc20_provenance_uses_current_role():
    entry = next(item for item in provenance()["abi_sources"] if item["export"] == "erc20Abi")
    assert entry["source"] == "contracts/launchpad/contracts/core/SidioraERC20.sol"
    assert entry["artifact"] == "precompiles/launchpad/abi/SidioraERC20.json"
    functions = {
        item["name"]: item
        for item in exported_abis()["erc20Abi"]
        if item["type"] == "function"
    }
    assert functions["bindPool"]["inputs"] == [
        {"internalType": "address", "name": "pool_", "type": "address"}
    ]
    for name in ("pool", "deploymentFactory"):
        assert functions[name]["inputs"] == []
        assert functions[name]["outputs"] == [
            {"internalType": "address", "name": "", "type": "address"}
        ]
        assert functions[name]["stateMutability"] == "view"
