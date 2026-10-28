import json
from fnmatch import fnmatchcase
from pathlib import Path

import pytest


@pytest.mark.parametrize(
    "ignore_path",
    (
        "docker/layerx/Dockerfile.dockerignore",
        "docker/paxeer/Dockerfile.paxd-node.dockerignore",
    ),
)
def test_vendor_reinclude_keeps_secret_exclusions(ignore_path):
    root = Path(__file__).resolve().parents[3]
    patterns = (root / ignore_path).read_text(encoding="utf-8").splitlines()
    reinclude = "!programs/vendor/**"
    assert reinclude in patterns, f"{ignore_path}: missing vendor re-include"
    last_reinclude = max(
        index for index, pattern in enumerate(patterns) if pattern == reinclude
    )
    vendor_exclusions = set(patterns[last_reinclude + 1 :])

    for suffix in (
        ".env",
        ".env.*",
        "*.env",
        "credentials",
        "credentials/**",
        "secrets",
        "secrets/**",
        "*.key",
        "*.pem",
        "*.der",
        "*.crt",
        "*.seed",
        "*.token",
        "*.p12",
        "*.pfx",
        "*.jks",
        "*.keystore",
        "*credentials*.json",
        "credentials.*",
        "id_rsa*",
        "id_ed25519*",
    ):
        pattern = f"programs/vendor/**/{suffix}"
        assert pattern in vendor_exclusions, (
            f"{ignore_path}: {pattern} must follow the last {reinclude}"
        )

    for pattern in ("**/*.key", "**/*.pem"):
        assert pattern in patterns, f"{ignore_path}: missing global {pattern}"


@pytest.mark.parametrize(
    "ignore_path",
    (
        "docker/layerx/Dockerfile.dockerignore",
        "docker/paxeer/Dockerfile.paxd-node.dockerignore",
    ),
)
def test_vendor_fixture_reincludes_match_checksum_manifests(ignore_path):
    root = Path(__file__).resolve().parents[3]
    patterns = (root / ignore_path).read_text(encoding="utf-8").splitlines()
    last_reinclude = max(
        index
        for index, pattern in enumerate(patterns)
        if pattern == "!programs/vendor/**"
    )
    vendor_patterns = patterns[last_reinclude + 1 :]
    exclusion_prefix = "programs/vendor/**/"
    secret_patterns = [
        pattern.removeprefix(exclusion_prefix)
        for pattern in vendor_patterns
        if pattern.startswith(exclusion_prefix)
    ]
    fixture_reincludes = [
        pattern for pattern in vendor_patterns if pattern.startswith("!programs/vendor/")
    ]

    expected = set()
    manifests = sorted((root / "programs/vendor").glob("*/.cargo-checksum.json"))
    assert manifests, "missing vendored crate checksum manifests"
    for manifest in manifests:
        files = json.loads(manifest.read_text(encoding="utf-8"))["files"]
        for relative_path in files:
            if any(
                fnmatchcase(Path(relative_path).name, pattern)
                for pattern in secret_patterns
            ):
                fixture = manifest.parent / relative_path
                assert fixture.is_file(), f"missing checksum-listed fixture: {fixture}"
                expected.add(f"!{fixture.relative_to(root).as_posix()}")

    assert expected, "no checksum-listed fixtures match the secret patterns"
    assert fixture_reincludes == sorted(expected), (
        f"{ignore_path}: fixture re-includes must exactly match checksum manifests "
        "in sorted order"
    )
    last_exclusion = max(
        index
        for index, pattern in enumerate(vendor_patterns)
        if pattern.startswith(exclusion_prefix)
    )
    assert all(
        index > last_exclusion
        for index, pattern in enumerate(vendor_patterns)
        if pattern.startswith("!programs/vendor/")
    ), f"{ignore_path}: fixture re-includes must follow all vendor secret exclusions"
