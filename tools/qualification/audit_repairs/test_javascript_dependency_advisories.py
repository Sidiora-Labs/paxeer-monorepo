"""Check npm and pnpm resolutions, advisory identities and maintained forks."""

import json
from pathlib import Path
import re

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
EXPLORER = "services/explorer/backend/apps/block_scout_web/assets"

# Exact npm entries returned by GitHub, including the patched request fork.
ADVISORIES = {
    "GHSA-848j-6mx2-7j84": {
        "updated_at": "2026-01-09T20:19:43Z",
        "packages": {"elliptic": ("<= 6.6.1", None)},
    },
    "GHSA-p8p7-x288-28g6": {
        "updated_at": "2024-03-21T17:47:21Z",
        "packages": {
            "request": ("<= 2.88.2", None),
            "@cypress/request": ("<= 2.88.12", "3.0.0"),
        },
    },
    "GHSA-gcqx-w48m-w2h4": {
        "updated_at": "2026-10-27T21:34:58Z",
        "packages": {"deepmerge": (">= 4.2.0, <= 4.3.1", None)},
    },
}

# Replacement release pins/integrities are npm metadata, not invented advisory
# first_patched_version values for the original packages.
REPLACEMENTS = {
    "elliptic": {
        "name": "@mapped/elliptic",
        "version": "6.6.10",
        "integrity": (
            "sha512-ZUTqpoyUEwNWDu9/WgJa6O30Ji9M/4SPl86Dp75rmaybr2dyk43Dkgpza"
            "QAu35Le5GqlVIhAFShLo9TlpkkXIw=="
        ),
    },
    "request": {
        "name": "@cypress/request",
        "version": "4.0.1",
        "integrity": (
            "sha512-y20e+e6dFYkOUUJLVUZTsJRuTiXZaUQ32WD+R/ux/HBybbTx4ge7cNIN"
            "cua0pU8+SNkKuRbOF12mBmzuzM8n5w=="
        ),
    },
    "deepmerge": {
        "name": "@stackline/deepmerge",
        "version": "1.1.0",
        "integrity": (
            "sha512-bEQ1EpXrlu9PTTgKD9z85MWYGjDMv79fF2qn2LYdmukpuj6onkn5kZlR"
            "Au0PaZvUv9ZQerwtDVgIEn2LYtsE7w=="
        ),
    },
}

NPM_INSTALL = "npm ci --ignore-scripts --no-audit --no-fund --workspaces=false"
PNPM_INSTALL = "CI=true pnpm install --frozen-lockfile --ignore-scripts"
REPAIR_RECORD = {
    "directories": {
        "contracts": ("package-lock.json", NPM_INSTALL, ("elliptic", "request")),
        "contracts/launchpad": ("pnpm-lock.yaml", PNPM_INSTALL, ("elliptic",)),
        "sdk/js": ("package-lock.json", NPM_INSTALL, ()),
        EXPLORER: (
            "package-lock.json",
            NPM_INSTALL,
            ("elliptic", "request", "deepmerge"),
        ),
        "apps/launchpad": ("pnpm-lock.yaml", PNPM_INSTALL, ("deepmerge",)),
        "apps/perps": ("pnpm-lock.yaml", PNPM_INSTALL, ("elliptic", "deepmerge")),
    },
    "alerts": {
        29: ("contracts/package-lock.json", "remove upstream elliptic path via fork"),
        3778: (
            "contracts/launchpad/pnpm-lock.yaml",
            "remove upstream elliptic path via fork and upgrade gas reporter",
        ),
        4465: (
            "sdk/js/package-lock.json",
            "upgrade CosmJS to maintained 0.39 release; elliptic path removed",
        ),
        3548: (
            f"{EXPLORER}/package-lock.json",
            "remove upstream elliptic path via fork",
        ),
        3537: (
            f"{EXPLORER}/package-lock.json",
            "remove request path via patched maintained @cypress/request major 4",
        ),
        4530: (
            "apps/launchpad/pnpm-lock.yaml",
            "remove upstream deepmerge path via maintained compatible fork",
        ),
        4531: (
            "apps/perps/pnpm-lock.yaml",
            "remove upstream deepmerge path via maintained compatible fork",
        ),
        4532: ("apps/perps/package-lock.json", "remove stale npm lockfile"),
    },
    "dependency_inventory": {
        "npm": "npm ls --package-lock-only --all elliptic request deepmerge",
        "pnpm": "pnpm why --recursive elliptic request deepmerge",
        "pnpm9_alias": "pnpm why --recursive @stackline/deepmerge",
        "contracts": "Truffle/Web3 1, gas reporter/ethers 5, ethereumjs-util/secp256k1",
        "contracts/launchpad": "gas reporter/ethers 5, network-helpers/secp256k1",
        "sdk/js": "@cosmjs/proto-signing -> @cosmjs/crypto -> elliptic",
        EXPLORER: "WalletConnect/provider-engine, crypto-browserify, Jest/deepmerge",
        "apps/launchpad": "React Native community CLI -> deepmerge",
        "apps/perps": {
            "deepmerge": "svelte-i18n -> deepmerge@4.3.1",
            "elliptic": "@ethersproject/signing-key and secp256k1 -> elliptic@6.6.1",
        },
    },
}
DIRECTORIES = tuple(REPAIR_RECORD["directories"])


def release(version):
    assert re.fullmatch(r"\d+\.\d+\.\d+", version), version
    return tuple(int(component) for component in version.split("."))


def manifest(directory):
    return json.loads((ROOT / directory / "package.json").read_text())


def lockfile(directory):
    filename = REPAIR_RECORD["directories"][directory][0]
    path = ROOT / directory / filename
    if filename.endswith(".json"):
        document = json.loads(path.read_text())
        assert document["lockfileVersion"] == 3
    else:
        document = yaml.safe_load(path.read_text())
        assert str(document["lockfileVersion"]) == "9.0"
        assert document["snapshots"]
        assert document["importers"]
    assert document["packages"], path
    return document


def resolutions(document):
    if "lockfileVersion" in document and document["lockfileVersion"] == 3:
        for location, package in document["packages"].items():
            if "node_modules/" in location:
                name = package.get("name", location.rsplit("node_modules/", 1)[1])
                yield name, package, location
    else:
        for location, package in document["packages"].items():
            name, version = location.split("(", 1)[0].rsplit("@", 1)
            yield name, {**package, "version": version}, location


@pytest.mark.parametrize("directory", DIRECTORIES)
def test_every_lockfile_excludes_unpatched_advisory_packages(directory):
    document = lockfile(directory)
    for name, package, location in resolutions(document):
        for advisory, record in ADVISORIES.items():
            if name in record["packages"]:
                _, minimum = record["packages"][name]
                assert minimum is not None, (directory, location, advisory)
                assert release(package["version"]) >= release(minimum), (
                    directory,
                    location,
                    advisory,
                )
    if "snapshots" in document:
        for location in document["snapshots"]:
            name = location.split("(", 1)[0].rsplit("@", 1)[0]
            assert name not in REPLACEMENTS, (directory, location)


@pytest.mark.parametrize("directory", DIRECTORIES)
def test_replacements_resolve_to_real_integrity_locked_forks(directory):
    document = lockfile(directory)
    required = REPAIR_RECORD["directories"][directory][2]
    packages = list(resolutions(document))
    for original in required:
        replacement = REPLACEMENTS[original]
        matches = [p for name, p, _ in packages if name == replacement["name"]]
        assert matches, (directory, original)
        for package in matches:
            assert package["version"] == replacement["version"]
            if "resolution" in package:
                assert package["resolution"]["integrity"] == replacement["integrity"]
            else:
                assert package["integrity"] == replacement["integrity"]
                basename = replacement["name"].rsplit("/", 1)[1]
                assert package["resolved"] == (
                    f"https://registry.npmjs.org/{replacement['name']}/-/"
                    f"{basename}-{replacement['version']}.tgz"
                )
        alias = f"npm:{replacement['name']}@{replacement['version']}"
        package = manifest(directory)
        if document["lockfileVersion"] == 3:
            assert package["overrides"][original] == alias
        else:
            assert package["pnpm"]["overrides"][original] == alias
            assert document["overrides"][original] == alias
        if directory == "apps/perps":
            assert package["overrides"][original] == alias


@pytest.mark.parametrize("directory", DIRECTORIES)
def test_lockfile_importers_match_manifests_and_frozen_install_record(directory):
    document = lockfile(directory)
    package = manifest(directory)
    filename, command, _ = REPAIR_RECORD["directories"][directory]
    if filename == "package-lock.json":
        assert command == NPM_INSTALL
        importer = document["packages"][""]
    else:
        assert command == PNPM_INSTALL
        manager = "pnpm@9.15.0" if directory == "apps/launchpad" else "pnpm@10.34.9" if directory == "contracts/launchpad" else "pnpm@10.34.9+sha512.f49b5a8ea0fdedb44bb3c0c1d6575092a53c2950d95b7ad15df11f17974877a2de3a01c07e934e9b28ce716e51c09a94962c7306d09be2e1e4b18a8c8762fd32"
        assert package["packageManager"] == manager
        assert document["overrides"] == package["pnpm"]["overrides"]
        for relative, importer in document["importers"].items():
            owner = json.loads(
                (ROOT / directory / relative / "package.json").read_text()
            )
            groups = ("devDependencies", "dependencies", "optionalDependencies")
            if document["settings"]["autoInstallPeers"]:
                declared = {name for group in groups for name in owner.get(group, {})}
                owner["dependencies"] = {
                    **{
                        name: specifier
                        for name, specifier in owner.get("peerDependencies", {}).items()
                        if name not in declared
                    },
                    **owner.get("dependencies", {}),
                }
            owners = {
                name: group for group in groups for name in owner.get(group, {})
            }
            for group in groups:
                expected = {
                    name: document["overrides"].get(name, specifier)
                    for name, specifier in owner.get(group, {}).items()
                    if owners[name] == group
                }
                actual = {
                    name: entry["specifier"]
                    for name, entry in importer.get(group, {}).items()
                }
                assert actual == expected, (
                    directory, relative, group, actual, expected
                )
        importer = document["importers"]["."]
    for group in ("dependencies", "devDependencies", "optionalDependencies"):
        assert set(importer.get(group, {})) == set(package.get(group, {}))
        if filename == "package-lock.json":
            assert importer.get(group, {}) == package.get(group, {})


def test_cosmjs_migration_removes_elliptic_dependency_path():
    package = manifest("sdk/js")
    for name in ("@cosmjs/proto-signing", "@cosmjs/stargate"):
        assert package["dependencies"][name] == "^0.39.0"
    cosmos = [
        (name, package)
        for name, package, _ in resolutions(lockfile("sdk/js"))
        if name.startswith("@cosmjs/")
    ]
    assert {"@cosmjs/crypto", "@cosmjs/proto-signing", "@cosmjs/stargate"} <= {
        name for name, _ in cosmos
    }
    for _, package in cosmos:
        assert release(package["version"]) >= (0, 39, 0)
        assert "elliptic" not in package.get("dependencies", {})


@pytest.mark.parametrize("directory", ("contracts", "contracts/launchpad"))
def test_gas_reporter_migration_removes_legacy_ethers_chain(directory):
    package = manifest(directory)
    assert package["devDependencies"]["@nomicfoundation/hardhat-toolbox"] == "^6.1.2"
    assert package["devDependencies"]["hardhat-gas-reporter"] == "^2.3.0"
    matches = []
    for name, package, _ in resolutions(lockfile(directory)):
        assert name != "eth-gas-reporter"
        if name == "hardhat-gas-reporter":
            matches.append(package)
            assert release(package["version"]) >= (2, 3, 0)
    assert matches


def test_stale_perps_npm_lockfile_is_removed():
    assert manifest("apps/perps")["packageManager"] == "pnpm@10.34.9+sha512.f49b5a8ea0fdedb44bb3c0c1d6575092a53c2950d95b7ad15df11f17974877a2de3a01c07e934e9b28ce716e51c09a94962c7306d09be2e1e4b18a8c8762fd32"
    assert (ROOT / "apps/perps/pnpm-lock.yaml").is_file()
    assert not (ROOT / "apps/perps/package-lock.json").exists()
    assert not (ROOT / "apps/perps/yarn.lock").exists()
    for package in (ROOT / "apps/perps").glob("*/*/package.json"):
        for filename in ("pnpm-lock.yaml", "package-lock.json", "yarn.lock"):
            assert not (package.parent / filename).exists(), package.parent / filename


def test_repair_record_covers_all_alerts_and_original_advisory_minima():
    assert set(REPAIR_RECORD["alerts"]) == {29, 3778, 4465, 3548, 3537, 4530, 4531, 4532}
    paths = {
        f"{directory}/{entry[0]}"
        for directory, entry in REPAIR_RECORD["directories"].items()
    }
    assert {path for path, _ in REPAIR_RECORD["alerts"].values()} == paths | {
        "apps/perps/package-lock.json"
    }
    for name in REPLACEMENTS:
        entries = [
            record["packages"][name]
            for record in ADVISORIES.values()
            if name in record["packages"]
        ]
        assert len(entries) == 1
        assert entries[0][1] is None
    assert (
        ADVISORIES["GHSA-p8p7-x288-28g6"]["packages"]["@cypress/request"][1]
        == "3.0.0"
    )
