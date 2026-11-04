"""Repair 22.31.453: inspect the seven real Go manifests and checksum files.

Advisory metadata was read during implementation with
gh api /advisories/<GHSA> (public Authorization: anonymous header).
The gate uses only committed files, without credentials, network or Go subprocesses.
Each alert is closed by upgrading the requirer, go-ethereum, to v1.17.4,
which migrates NAT discovery to STUN v3. Its original v3.1.2 requirement is
raised to the advisory's v3.1.5 minimum, bringing DTLS v3.1.4 with it.
go mod tidy ran in every module with GOWORK=off to regenerate its own go.sum.
hf-build and hf-done record the content-derived verified revision and private log.
"""

from pathlib import Path
import re
import shlex

import pytest


ROOT = Path(__file__).resolve().parents[3]
GETH = "github.com/ethereum/go-ethereum"
ADVISORIES = {
    "GHSA-9f3f-wv7r-qc8r": {
        "updated_at": "2026-02-13T14:18:29Z",
        "packages": {
            "github.com/pion/dtls": (("<= 1.5.4", None),),
            "github.com/pion/dtls/v2": (("<= 2.2.12", None),),
            "github.com/pion/dtls/v3": (
                ("< 3.0.11", "3.0.11"),
                ("= 3.1.0", "3.1.1"),
            ),
        },
    },
    "GHSA-34rh-wp3j-6cxc": {
        "updated_at": "2026-07-31T22:03:43Z",
        "packages": {
            "github.com/pion/stun": (("<= 1.23.1", None),),
            "github.com/pion/stun/v2": (("<= 2.0.0", None),),
            "github.com/pion/stun/v3": (("<= 3.1.4", "3.1.5"),),
        },
    },
}
MODULE_ALERTS = {
    "sdk/launchpad/shared": (2773, 2789),
    "services/launchpad/api": (2807, 2823),
    "services/launchpad/gateway": (2825, 2841),
    "services/launchpad/livestream": (4725, 4726),
    "services/launchpad/metadata": (2875, 2893),
    "services/launchpad/pnl-tracker": (2895, 2913),
    "services/launchpad/trading-charts": (2993, 3009),
}
REPAIR_RECORD = {
    "base_revision": "357234853ae90e2dbfd210d59cb4085eaf01ed95",
    "qualification": "hf-done JSON: revision, command, exit, private log",
    "requiring_path": (
        "github.com/ethereum/go-ethereum/p2p/nat",
        "github.com/pion/stun/v2",
        "github.com/pion/dtls/v2",
    ),
    "alerts": {
        alert: {
            "manifest": f"{directory}/go.mod",
            "advisory": advisory,
            "resolution": "upgrade requirer to maintained Pion major 3",
        }
        for directory, alerts in MODULE_ALERTS.items()
        for alert, advisory in zip(alerts, ADVISORIES, strict=True)
    },
    "gotchas": (
        "v2 has no patch: migrate the requirer instead of replacing v2 with v3.",
        "go-ethereum v1.17.4 alone still selects vulnerable STUN v3.1.2.",
        "Workspace go mod why shows workspace roots; tidy each module standalone.",
        "Public gh API reads require Authorization: anonymous without a credential.",
    ),
}


def manifest_directives(directory):
    """Parse require/replace/exclude blocks and single-line Go directives."""
    result = {name: [] for name in ("require", "replace", "exclude")}
    block = None
    for raw in (ROOT / directory / "go.mod").read_text().splitlines():
        line = raw.split("//", 1)[0].strip()
        if not line:
            continue
        if line == ")":
            assert block is not None, (directory, raw)
            block = None
            continue
        tokens = shlex.split(line)
        if block is None:
            directive, *tokens = tokens
            if tokens == ["("]:
                block = directive
                continue
        else:
            directive = block
        if directive in result:
            assert tokens, (directory, raw)
            result[directive].append(tokens)
    assert block is None, directory
    assert result["require"], directory
    return result


def checksums(directory):
    entries = {}
    path = ROOT / directory / "go.sum"
    for line in path.read_text().splitlines():
        name, version, checksum = line.split()
        assert re.fullmatch(r"h1:[A-Za-z0-9+/]{43}=", checksum), (path, line)
        assert (name, version) not in entries, (path, line)
        entries[name, version] = checksum
    assert entries, path
    return entries


def release(version):
    assert re.fullmatch(r"v?\d+\.\d+\.\d+", version), version
    return tuple(int(component) for component in version.removeprefix("v").split("."))


def assert_advisory_safe(name, version):
    for advisory, record in ADVISORIES.items():
        if name not in record["packages"]:
            continue
        patches = record["packages"][name]
        assert all(minimum is not None for _, minimum in patches), (advisory, name)
        selected = release(version)
        if name == "github.com/pion/dtls/v3":
            # Both maintained release lines have a distinct first fixed version.
            minimum = patches[0][1] if selected[:2] == (3, 0) else patches[1][1]
        else:
            minimum = patches[0][1]
        assert selected[0] == 3, (advisory, name, version)
        assert selected >= release(minimum), (advisory, name, version, minimum)


@pytest.mark.parametrize("directory", MODULE_ALERTS)
def test_manifests_and_sums_exclude_unpatched_pion_majors(directory):
    directives = manifest_directives(directory)
    sums = checksums(directory)
    for name, version in directives["require"]:
        assert_advisory_safe(name, version)
    for name, version in sums:
        assert_advisory_safe(name, version.removesuffix("/go.mod"))
    for tokens in directives["replace"] + directives["exclude"]:
        # A substitution/exclusion cannot stand in for migrating the real requirer.
        for token in tokens:
            assert token != GETH and not token.startswith("github.com/pion/"), (
                directory,
                tokens,
            )


@pytest.mark.parametrize("directory", MODULE_ALERTS)
def test_requirer_upgrade_and_pion_resolutions_are_checksum_locked(directory):
    requirements = dict(manifest_directives(directory)["require"])
    sums = checksums(directory)
    assert release(requirements[GETH]) >= (1, 17, 4), directory
    assert (GETH, requirements[GETH]) in sums, directory
    assert (GETH, requirements[GETH] + "/go.mod") in sums, directory
    for name in ("github.com/pion/stun/v3", "github.com/pion/dtls/v3"):
        # Tidy can remove Pion entirely when the module never imports geth/node.
        if name in requirements:
            version = requirements[name]
            assert_advisory_safe(name, version)
            assert (name, version) in sums, (directory, name, version)
            assert (name, version + "/go.mod") in sums, (directory, name, version)


def test_repair_record_covers_all_fourteen_alerts_and_advisory_minima():
    assert set(REPAIR_RECORD["alerts"]) == {
        2773, 2789, 2807, 2823, 2825, 2841, 4725,
        4726, 2875, 2893, 2895, 2913, 2993, 3009,
    }
    for directory, alerts in MODULE_ALERTS.items():
        for alert, advisory in zip(alerts, ADVISORIES, strict=True):
            entry = REPAIR_RECORD["alerts"][alert]
            assert entry["manifest"] == f"{directory}/go.mod"
            assert entry["advisory"] == advisory
            assert entry["resolution"] == "upgrade requirer to maintained Pion major 3"
    assert ADVISORIES["GHSA-9f3f-wv7r-qc8r"]["packages"]["github.com/pion/dtls/v3"] == (
        ("< 3.0.11", "3.0.11"), ("= 3.1.0", "3.1.1"),
    )
    assert ADVISORIES["GHSA-34rh-wp3j-6cxc"]["packages"]["github.com/pion/stun/v3"] == (
        ("<= 3.1.4", "3.1.5"),
    )
