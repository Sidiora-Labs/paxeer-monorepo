"""Offline source proofs for the root-discovered perf-db dependency policy."""

from fnmatch import fnmatchcase
from pathlib import Path
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
CRATE = "services/platform/perf-db"
CONFIG = ROOT / ".github/dependabot.yml"
RECORD = Path(__file__).with_name("repair_22_33_1379.kvx")
EXISTING_UPDATES = (
    ("github-actions", "/", 5),
    ("gomod", "/", 5),
    ("gomod", "/tools/hpx/registry", 3),
    ("gomod", "/sdk/cosmovisor", 3),
    ("gomod", "/sdk/ics23", 3),
    ("npm", "/contracts", 5),
    ("npm", "/tests/integration/dapp_tests", 3),
    ("npm", "/tests/integration/rpc_tests", 3),
    ("cargo", "/wasm-runtime/libwasmvm", 3),
)


def load_config():
    config = yaml.safe_load(CONFIG.read_text(encoding="utf-8"))
    assert isinstance(config, dict)
    assert config["version"] == 2
    assert isinstance(config["updates"], list)
    return config


def test_perf_dependabot_location():
    config = load_config()
    policies = [
        update
        for update in config["updates"]
        if update["package-ecosystem"] == "cargo"
        and update["directory"] == f"/{CRATE}"
    ]
    assert policies == [
        {
            "package-ecosystem": "cargo",
            "directory": f"/{CRATE}",
            "schedule": {"interval": "weekly"},
            "open-pull-requests-limit": 10,
            "groups": {"dependencies": {"patterns": ["*"]}},
        }
    ]
    assert not (ROOT / CRATE / ".github/dependabot.yml").exists()
    assert not (ROOT / CRATE / ".github/dependabot.yaml").exists()
    assert not any(
        update["package-ecosystem"] == "cargo" and update["directory"] == "/"
        for update in config["updates"]
    )


def test_existing_dependabot_updates_preserved():
    config = load_config()
    existing = [
        {
            "package-ecosystem": ecosystem,
            "directory": directory,
            "schedule": {"interval": "weekly"},
            "open-pull-requests-limit": limit,
        }
        for ecosystem, directory, limit in EXISTING_UPDATES
    ]
    assert set(config) == {"version", "updates"}
    assert len(config["updates"]) == len(existing) + 1
    assert config["updates"][:-1] == existing
    actions = [
        update
        for update in config["updates"]
        if update["package-ecosystem"] == "github-actions"
    ]
    assert actions == [existing[0]]


def test_perf_dependabot_policy_reaches_manifest_dependencies():
    config = load_config()
    policy = next(
        update
        for update in config["updates"]
        if update["package-ecosystem"] == "cargo"
        and update["directory"] == f"/{CRATE}"
    )
    manifest_path = ROOT / policy["directory"].lstrip("/") / "Cargo.toml"
    assert manifest_path == ROOT / CRATE / "Cargo.toml"
    manifest = tomllib.loads(manifest_path.read_text(encoding="utf-8"))
    assert manifest["package"]["name"] == "perfdb"
    assert manifest["workspace"] == {}
    lock = tomllib.loads(manifest_path.with_name("Cargo.lock").read_text())
    locked_names = {package["name"] for package in lock["package"]}
    dependencies = set(manifest["dependencies"]) | set(manifest["dev-dependencies"])
    assert dependencies
    assert dependencies <= locked_names
    patterns = policy["groups"]["dependencies"]["patterns"]
    for dependency in dependencies:
        assert any(fnmatchcase(dependency, pattern) for pattern in patterns), dependency


def test_perf_dependabot_repair_record():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.1379"
    assert record["repair"]["wiring_requests"] == []
    assert set(record["finding"]) == {"perf-dependabot-location"}
    finding = record["finding"]["perf-dependabot-location"]
    assert finding["audit"] == "22.30.75"
    assert finding["file"] == f"{CRATE}/.github/dependabot.yml"
    assert finding["tests"] == [
        "test_perf_dependabot_location",
        "test_existing_dependabot_updates_preserved",
        "test_perf_dependabot_policy_reaches_manifest_dependencies",
    ]
    assert record["evidence"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1379.py"
    )
