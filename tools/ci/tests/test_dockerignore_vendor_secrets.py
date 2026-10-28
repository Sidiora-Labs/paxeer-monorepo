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
