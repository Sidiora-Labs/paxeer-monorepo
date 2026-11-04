"""Check every locked copy against the GitHub advisory's patched release."""

import json
from pathlib import Path
import re
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
WORKSPACE = ROOT / "contracts/bridge/solana"
ADVISORY_MINIMUMS = (
    ("GHSA-82j2-j2ch-gfr8", "rustls-webpki", "0.103.13"),
    ("GHSA-xgp8-3hg3-c2mh", "rustls-webpki", "0.103.12"),
    ("GHSA-965h-392x-2mh5", "rustls-webpki", "0.103.12"),
    ("GHSA-vp88-798v-h67f", "rustls", "0.23.46"),
    ("GHSA-w44r-93j7-qv28", "rustls", "0.23.46"),
    ("GHSA-c8m5-c952-6cm4", "rustls", "0.23.46"),
    ("GHSA-h2v8-394m-p7vv", "rustls", "0.23.46"),
    ("GHSA-w5vr-6qhr-36cc", "ed25519-dalek", "2.0.0"),
    ("GHSA-x4gp-pqpj-f43q", "curve25519-dalek", "4.1.3"),
    ("GHSA-cq8v-f236-94qc", "rand", "0.8.6"),
    ("GHSA-g98v-hv3f-hcfr", "atty", None),
)


def stable_version(version):
    match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)(?:\+[0-9A-Za-z.-]+)?", version)
    assert match is not None, f"Expected a stable release, got {version!r}"
    return tuple(int(part) for part in match.groups())


@pytest.fixture(scope="module")
def packages():
    with (WORKSPACE / "Cargo.lock").open("rb") as source:
        locked = tomllib.load(source)["package"]
    assert locked
    assert any(p["name"] == "paxeer-x-bridge-solana-program" for p in locked)
    assert any(p["name"] == "paxeer-x-bridge-solana-admin" for p in locked)
    return locked


@pytest.fixture(scope="module")
def task_record():
    record = json.loads((WORKSPACE / "repair_22_31_449.json").read_text())
    assert record["task"] == "22.31.449"
    return record


@pytest.mark.parametrize("ghsa,crate,minimum", ADVISORY_MINIMUMS)
def test_repair_22_31_449_advisory_minimums(packages, task_record, ghsa, crate, minimum):
    advisory = next(a for a in task_record["advisories"] if a["ghsa_id"] == ghsa)
    assert advisory["html_url"] == f"https://github.com/advisories/{ghsa}"
    vulnerabilities = [
        v for v in advisory["vulnerabilities"]
        if v["package"] == {"ecosystem": "rust", "name": crate}
    ]
    assert vulnerabilities, f"No GitHub advisory evidence for {crate}"
    assert minimum in {v["first_patched_version"] for v in vulnerabilities}
    versions = [p["version"] for p in packages if p["name"] == crate]
    if minimum is None:
        assert versions == [], f"{ghsa}: unpatched {crate} remains: {versions}"
    else:
        vulnerable = [v for v in versions if stable_version(v) < stable_version(minimum)]
        if crate == "rand":
            branch_minimums = {(0, 9): (0, 9, 3), (0, 10): (0, 10, 1)}
            vulnerable.extend(
                v for v in versions
                if stable_version(v)[:2] in branch_minimums
                and stable_version(v) < branch_minimums[stable_version(v)[:2]]
            )
        requirers = [
            f"{p['name']} {p['version']} -> {dependency}"
            for p in packages
            for dependency in p.get("dependencies", [])
            if dependency.split()[0] == crate
        ]
        assert vulnerable == [], (
            f"{ghsa}: every {crate} must be >= {minimum}; "
            f"unresolved versions: {vulnerable}; requirers: {requirers}"
        )


def test_repair_22_31_449_record_covers_all_alerts(task_record):
    alerts = {
        alert for decision in task_record["decisions"] for alert in decision["alerts"]
    }
    assert alerts == {3768, 3769, 3770, 3771, 3772, 3773, 3774, 4483, 4484, 4485}
    assert task_record["sdk_release_with_patched_signing"] == "solana-sdk 3.0.0"
    assert task_record["ed25519_exception"] is None
    assert task_record["upstream_blocker"]["sources"]
    atty = next(d for d in task_record["decisions"] if d["crate"] == "atty")
    assert "Remove the dependency path" in atty["action"]


def test_repair_22_31_449_backports_are_the_compiled_requirers(packages, task_record):
    with (WORKSPACE / "Cargo.toml").open("rb") as source:
        workspace = tomllib.load(source)
    assert {p["name"] for p in task_record["vendored_packages"]} == {
        "agave-precompiles", "agave-syscalls",
    }
    for package in task_record["vendored_packages"]:
        name = package["name"]
        path = package["path"]
        assert workspace["patch"]["crates-io"][name] == {"path": path}
        assert path in workspace["workspace"]["members"]
        locked = [p for p in packages if p["name"] == name]
        assert len(locked) == 1
        assert locked[0]["version"] == package["version"]
        assert "source" not in locked[0]
        assert "checksum" not in locked[0]
        with (WORKSPACE / path / "Cargo.toml").open("rb") as source:
            manifest = tomllib.load(source)
        assert manifest["package"]["version"] == package["version"]
        assert manifest["dependencies"]["libsecp256k1"]["version"] == "0.7.2"
        assert (WORKSPACE / path / "LICENSE").is_file()
        assert re.fullmatch(r"[0-9a-f]{64}", package["upstream_crate_sha256"])
        test_sources = "\n".join(
            source.read_text() for source in (WORKSPACE / path / "src").glob("*.rs")
        )
        for name in package["retained_tests"]:
            assert re.search(r"#\[test\]\s*(?:pub\s+)?fn\s+" + re.escape(name) + r"\(", test_sources)
