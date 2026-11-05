"""Check the real wasmvm lockfile and the explicitly documented engine exception."""

import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
CRATE = ROOT / "wasm-runtime/libwasmvm"
COSMWASM_REVISION = "cd3c4399b8c0ea0d08b2068a893ed2973685fe30"
# Values read from GitHub's global advisory records during implementation.
ADVISORIES = {
    "curve25519-dalek": ("GHSA-x4gp-pqpj-f43q", "< 4.1.3", "4.1.3"),
    "atty": ("GHSA-g98v-hv3f-hcfr", "<= 0.2.14", None),
    "wasmer": ("GHSA-55f3-3qvg-8pv5", "<= 4.3.1", None),
}


def read_toml(path):
    return tomllib.loads(path.read_text())


def stable_version(value):
    match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)(?:\+[0-9A-Za-z.-]+)?", value)
    assert match is not None, f"Expected a stable release, got {value}"
    return tuple(int(part) for part in match.groups())


def packages_named(name):
    return [
        package
        for package in read_toml(CRATE / "Cargo.lock")["package"]
        if package["name"] == name
    ]


def test_advisory_records_match_the_implemented_bounds():
    for name, (ghsa, affected, patched) in ADVISORIES.items():
        record = json.loads((CRATE / "audit" / f"{ghsa}.json").read_text())
        assert record["ghsa_id"] == ghsa
        entries = [
            item
            for item in record["vulnerabilities"]
            if item["package"] == {"ecosystem": "rust", "name": name}
        ]
        assert len(entries) == 1
        assert entries[0]["vulnerable_version_range"] == affected
        assert entries[0]["first_patched_version"] == patched


def test_every_curve25519_dalek_resolution_is_patched():
    packages = packages_named("curve25519-dalek")
    assert packages, "The real crypto dependency must still be present"
    minimum = stable_version(ADVISORIES["curve25519-dalek"][2])
    for package in packages:
        assert stable_version(package["version"]) >= minimum
        assert package["source"] == "registry+https://github.com/rust-lang/crates.io-index"
        assert package["checksum"]


def test_crypto_upgrade_reaches_the_pinned_vm_through_the_real_patch():
    manifest = read_toml(CRATE / "Cargo.toml")
    patch = manifest["patch"]["https://github.com/Paxeer-Network/pax-cosmwasm.git"]
    assert patch["cosmwasm-crypto"]["path"] == "vendor/cosmwasm-crypto"
    crypto = read_toml(CRATE / patch["cosmwasm-crypto"]["path"] / "Cargo.toml")
    assert crypto["package"]["name"] == "cosmwasm-crypto"
    assert crypto["package"]["version"] == "1.5.12"
    assert stable_version(crypto["dependencies"]["ed25519-zebra"]) >= (4, 0, 3)
    resolved_crypto = packages_named("cosmwasm-crypto")
    assert len(resolved_crypto) == 1
    assert "source" not in resolved_crypto[0]
    assert "ed25519-zebra" in resolved_crypto[0]["dependencies"]
    zebras = packages_named("ed25519-zebra")
    assert zebras
    assert all(stable_version(package["version"]) >= (4, 0, 3) for package in zebras)
    assert all("curve25519-dalek" in package["dependencies"] for package in zebras)
    vms = packages_named("cosmwasm-vm")
    assert len(vms) == 1
    assert "cosmwasm-crypto" in vms[0]["dependencies"]


def test_atty_and_all_its_dependency_edges_are_absent():
    packages = read_toml(CRATE / "Cargo.lock")["package"]
    assert not packages_named("atty")
    for package in packages:
        assert all(edge.split()[0] != "atty" for edge in package.get("dependencies", []))
    generators = packages_named("cbindgen")
    assert generators
    assert all(stable_version(package["version"]) >= (0, 29, 2) for package in generators)
    claps = packages_named("clap")
    assert claps
    assert all(stable_version(package["version"]) >= (4, 0, 0) for package in claps)


def test_wasmer_is_outside_affected_range_or_has_the_exact_engine_exception():
    engines = packages_named("wasmer")
    assert engines
    affected_maximum = stable_version(ADVISORIES["wasmer"][1].removeprefix("<= "))
    affected = [p for p in engines if stable_version(p["version"]) <= affected_maximum]
    if not affected:
        return

    record = read_toml(CRATE / "repair_22_31_450.kvx")["alert"]["1116"]
    assert record["ghsa"] == ADVISORIES["wasmer"][0]
    assert record["affected"] == ADVISORIES["wasmer"][1]
    assert record["first_patched_version"] == "none"
    assert record["disposition"] == "retain pinned deterministic engine; owner dismissal required"
    assert record["engine_version"] == "4.2.2"
    assert record["cosmwasm_revision"] == COSMWASM_REVISION
    for term in ("=4.2.2", "Singlepass", "Gatekeeper", "gas", "canonicalize_nans", "chain upgrade"):
        assert term in record["engine_reason"]
    for term in ("SUPPORTED_IMPORTS", "env", "preopened directories", "path_open", "path_symlink"):
        assert term in record["mitigation"]
    assert "owner" in record["owner_action"]
    assert "does not dismiss" in record["owner_action"]

    manifest = read_toml(CRATE / "Cargo.toml")
    for name in ("cosmwasm-vm", "cosmwasm-std"):
        assert manifest["dependencies"][name]["rev"] == COSMWASM_REVISION
        assert manifest["dependencies"][name]["git"] == record["source_repository"] + ".git"
        for package in packages_named(name):
            assert package["source"].endswith("#" + COSMWASM_REVISION)
    for name in ("wasmer", "wasmer-types", "wasmer-middlewares"):
        packages = packages_named(name)
        assert len(packages) == 1
        assert packages[0]["version"] == record["engine_version"]
    for name in ("wasmer-wasi", "wasmer-wasix", "wasmer-wasix-types"):
        assert not packages_named(name), "A WASI host invalidates the recorded mitigation"


def test_task_record_accounts_for_each_alert_and_qualification():
    record = read_toml(CRATE / "repair_22_31_450.kvx")
    assert record["task"]["id"] == "22.31.450"
    assert "verified revision" in record["task"]["qualification"]
    for alert, name in (("1117", "curve25519-dalek"), ("1114", "atty"), ("1116", "wasmer")):
        entry = record["alert"][alert]
        ghsa, affected, patched = ADVISORIES[name]
        assert entry["ghsa"] == ghsa
        assert entry["package"] == name
        assert entry["affected"] == affected
        assert entry["first_patched_version"] == (patched or "none")
    assert record["alert"]["1117"]["disposition"] == "upgrade requiring package to maintained major"
    assert record["alert"]["1114"]["disposition"] == "remove dependency path"
