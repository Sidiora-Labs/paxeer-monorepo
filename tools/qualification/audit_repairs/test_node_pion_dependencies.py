"""Offline dependency proof for the Pion advisory repair (22.31.454).

The GitHub advisory records were read with gh api during implementation:
GHSA-9f3f-wv7r-qc8r has no patched DTLS v2; its v3 fixes are 3.0.11
and 3.1.1 (3.1.0 is vulnerable). GHSA-34rh-wp3j-6cxc has no patched
STUN v2; its first patched v3 release is 3.1.5.

The required closure for alerts 827/859 (root) and 2042/2057 (embedded
geth) is migration of the requiring STUN package to its maintained v3
major at v3.1.5, which requires DTLS v3.1.4. These checks require that
migration in both manifests and checksum files; the declared build proves RPC
compilation with the native libraries.
"""

from pathlib import Path
import re
import shlex

import pytest


ROOT = Path(__file__).resolve().parents[3]
MODULES = (Path("."), Path("core/deps/geth"))
ADVISORIES = (
    ("GHSA-9f3f-wv7r-qc8r", "github.com/pion/dtls", "v3.1.1"),
    ("GHSA-34rh-wp3j-6cxc", "github.com/pion/stun", "v3.1.5"),
)


def parse_go_mod(path):
    requirements = {}
    replacements = []
    block = None
    for line_number, raw in enumerate(path.read_text().splitlines(), 1):
        line = raw.split("//", 1)[0].strip()
        if not line:
            continue
        if line == ")":
            assert block is not None, f"{path}:{line_number}: unexpected ')'"
            block = None
            continue
        fields = shlex.split(line)
        directive = block or fields.pop(0)
        if fields == ["("]:
            assert block is None, f"{path}:{line_number}: nested directive"
            block = directive
            continue
        if directive == "require":
            assert len(fields) == 2, f"{path}:{line_number}: invalid require"
            module, version = fields
            assert module not in requirements, f"{path}: duplicate {module}"
            requirements[module] = version
        elif directive == "replace":
            assert "=>" in fields, f"{path}:{line_number}: invalid replace"
            index = fields.index("=>")
            replacements.append((fields[:index], fields[index + 1 :]))
    assert block is None, f"{path}: unclosed directive"
    assert requirements, f"{path}: no module requirements parsed"
    return requirements, replacements


def parse_go_sum(path):
    checksums = {}
    for line_number, raw in enumerate(path.read_text().splitlines(), 1):
        if not raw.strip():
            continue
        fields = raw.split()
        assert len(fields) == 3, f"{path}:{line_number}: invalid checksum entry"
        module, version, checksum = fields
        assert re.fullmatch(r"h1:[A-Za-z0-9+/]{43}=", checksum), (
            f"{path}:{line_number}: invalid module checksum"
        )
        key = (module, version)
        assert key not in checksums, f"{path}: duplicate checksum for {key}"
        checksums[key] = checksum
    assert checksums, f"{path}: no module checksums parsed"
    return checksums


def release_version(version):
    match = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", version)
    assert match is not None, f"expected a stable release, got {version}"
    return tuple(map(int, match.groups()))


@pytest.mark.parametrize("relative", MODULES, ids=("root", "embedded-geth"))
def test_pion_advisories_are_closed(relative):
    manifest = ROOT / relative / "go.mod"
    sumfile = ROOT / relative / "go.sum"
    requirements, replacements = parse_go_mod(manifest)
    checksums = parse_go_sum(sumfile)

    for advisory, package, minimum in ADVISORIES:
        forbidden = {package, f"{package}/v2"}
        assert forbidden.isdisjoint(requirements), (
            f"{manifest}: {advisory} still requires an unpatched major"
        )
        assert not any(module in forbidden for module, _ in checksums), (
            f"{sumfile}: {advisory} still sums an unpatched major"
        )
        maintained = f"{package}/v3"
        assert maintained in requirements, (
            f"{manifest}: {advisory} requires migration to {maintained}"
        )
        resolved = requirements[maintained]
        assert release_version(resolved) >= release_version(minimum), (
            f"{manifest}: {advisory} requires {maintained} >= {minimum}, "
            f"got {resolved}"
        )
        assert not any(
            source[0].startswith(package) or target[0].startswith(package)
            for source, target in replacements
        ), f"{manifest}: replacement bypasses the audited {package} release"
        assert (maintained, resolved) in checksums, (
            f"{sumfile}: missing module checksum for {maintained} {resolved}"
        )
        assert (maintained, f"{resolved}/go.mod") in checksums, (
            f"{sumfile}: missing manifest checksum for {maintained} {resolved}"
        )
        for module, summed in checksums:
            if module == maintained:
                version = summed.removesuffix("/go.mod")
                assert release_version(version) >= release_version(minimum), (
                    f"{sumfile}: {advisory} retains vulnerable {module} {version}"
                )
