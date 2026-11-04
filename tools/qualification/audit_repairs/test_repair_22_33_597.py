"""Offline source qualification of the published verifier configuration examples."""

from pathlib import Path
import re
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
WORKSPACE = ROOT / "services/explorer/services/smart-contract-verifier"
SERVER = WORKSPACE / "smart-contract-verifier-server"
CONFIG = SERVER / "config"
RECORD = Path(__file__).with_name("repair_22_33_597.kvx")
PREFIX = "SMART_CONTRACT_VERIFIER__"


def rust_fields(source, name):
    """Extract public fields from the actual flat Settings declarations."""
    match = re.search(r"pub struct " + re.escape(name) + r" \{(.*?)^\}", source, re.M | re.S)
    assert match is not None, name
    fields = re.findall(r"^    pub (\w+): ([^\n]+),$", match[1], re.M)
    assert fields, name
    return dict(fields)


def leaves(mapping, prefix=()):
    result = {}
    for key, value in mapping.items():
        path = (*prefix, key)
        if isinstance(value, dict):
            result.update(leaves(value, path))
        else:
            result[path] = value
    return result


@pytest.mark.parametrize("filename", ["base.toml", "base.env"])
def test_obsolete_extension_examples_removed(filename):
    text = (CONFIG / filename).read_text(encoding="utf-8")
    assert "extensions" not in text.lower()
    assert "sig_provider" not in text.lower()


def test_published_toml_uses_supported_settings():
    config = tomllib.loads((CONFIG / "base.toml").read_text(encoding="utf-8"))
    source = (SERVER / "src/settings.rs").read_text(encoding="utf-8")
    settings = rust_fields(source, "Settings")
    assert set(config) == {
        "server", "solidity", "vyper", "sourcify", "metrics", "jaeger", "compilers"
    }
    assert set(config) <= settings.keys()
    assert '#[serde(default, deny_unknown_fields)]\npub struct Settings' in source
    assert "extensions" not in settings
    for section in ("solidity", "vyper", "sourcify", "compilers"):
        assert set(config[section]) == rust_fields(source, settings[section]).keys()
    for section in ("solidity", "vyper"):
        fetcher = config[section]["fetcher"]
        assert set(fetcher) == {"list"}
        assert set(fetcher["list"]) == rust_fields(source, "ListFetcherSettings").keys()
        assert re.search(r"pub enum FetcherSettings \{\s*List\(ListFetcherSettings\),", source)
    assert config["compilers"]["max_threads"] > 0
    assert config["sourcify"]["verification_attempts"] > 0
    assert config["server"] == {
        "http": {"enabled": True, "addr": "0.0.0.0:8050", "max_body_size": 2097152},
        "grpc": {"enabled": False, "addr": "0.0.0.0:8051"},
    }
    assert config["metrics"] == {"enabled": False, "addr": "0.0.0.0:6060", "route": "/metrics"}
    assert config["jaeger"] == {"enabled": False, "agent_endpoint": "localhost:6831"}


def test_commented_toml_examples_use_supported_fetcher_fields():
    text = (CONFIG / "base.toml").read_text(encoding="utf-8")
    tables = []
    for line in text.splitlines():
        match = re.fullmatch(r"\s*#\s*(\[[^\]]+\])\s*", line)
        if match:
            tables.append(match[1])
    assert tables == ["[solidity.fetcher.s3]"]
    s3_lines = []
    in_s3 = False
    for line in text.splitlines():
        if line == "#[solidity.fetcher.s3]":
            in_s3 = True
        elif in_s3 and not line.startswith("#"):
            break
        if in_s3 and (line.startswith("#[") or re.match(r"#\w+\s*=", line)):
            s3_lines.append(line[1:])
    example = tomllib.loads("\n".join(s3_lines))["solidity"]["fetcher"]["s3"]
    source = (SERVER / "src/settings.rs").read_text(encoding="utf-8")
    assert set(example) == rust_fields(source, "S3FetcherSettings").keys()
    assert example["bucket"] == "bucket"
    assert example["region"] == "region"
    assert example["endpoint"] == "endpoint"


def test_environment_examples_match_supported_toml_paths_and_values():
    config = tomllib.loads((CONFIG / "base.toml").read_text(encoding="utf-8"))
    expected = leaves(config)
    source = (SERVER / "src/settings.rs").read_text(encoding="utf-8")
    s3_keys = rust_fields(source, "S3FetcherSettings").keys()
    allowed_s3 = {("solidity", "fetcher", "s3", key) for key in s3_keys}
    examples = {}
    primary = {}
    for line in (CONFIG / "base.env").read_text(encoding="utf-8").splitlines():
        if not line.strip() or line.startswith("## "):
            continue
        match = re.fullmatch(r"(#{1,2})" + PREFIX + r"([A-Z][A-Z0-9_]*)=(.+)", line)
        assert match is not None, line
        path = tuple(match[2].lower().split("__"))
        assert path in expected or path in allowed_s3, path
        examples.setdefault(path, []).append(match[3])
        if match[1] == "#":
            assert path not in primary, path
            primary[path] = match[3]
    assert set(primary) == set(expected)
    assert set(examples) == set(expected) | allowed_s3
    for path, value in expected.items():
        serialized = str(value).lower() if isinstance(value, bool) else str(value)
        assert primary[path] == serialized, path
    for key in s3_keys:
        assert examples[("solidity", "fetcher", "s3", key)] == [key]
    assert examples[("solidity", "fetcher", "list", "list_url")] == [
        "https://binaries.soliditylang.org/linux-amd64/list.json",
        "https://binaries.soliditylang.org/macosx-amd64/list.json",
        "https://binaries.soliditylang.org/windows-amd64/list.json",
    ]
    assert examples[("vyper", "fetcher", "list", "list_url")] == [
        "https://raw.githubusercontent.com/blockscout/solc-bin/main/vyper.list.json",
        "https://raw.githubusercontent.com/blockscout/solc-bin/main/vyper.macos.list.json",
    ]


def test_settings_build_and_constructor_source_path():
    settings = (SERVER / "src/settings.rs").read_text(encoding="utf-8")
    main = (SERVER / "src/main.rs").read_text(encoding="utf-8")
    run = (SERVER / "src/run.rs").read_text(encoding="utf-8")
    assert "impl ConfigSettings for Settings {" in settings
    assert 'const SERVICE_NAME: &\'static str = "SMART_CONTRACT_VERIFIER";' in settings
    assert "use blockscout_service_launcher::launcher::ConfigSettings;" in main
    assert 'let settings = Settings::build().expect("failed to read config");' in main
    assert "smart_contract_verifier_server::run(settings).await" in main
    for language, service in (
        ("solidity", "SolidityVerifierService"),
        ("vyper", "VyperVerifierService"),
        ("sourcify", "SourcifyVerifierService"),
    ):
        assert f"{service}::new(settings.{language}" in run
        constructor = (SERVER / f"src/services/{language}_verifier.rs").read_text(encoding="utf-8")
        assert "sig_provider" not in constructor
        assert "extensions" not in constructor
    for field in ("server", "metrics", "tracing", "jaeger", "compilers"):
        assert f"settings.{field}" in run
    manifest = tomllib.loads((SERVER / "Cargo.toml").read_text(encoding="utf-8"))
    workspace = tomllib.loads((WORKSPACE / "Cargo.toml").read_text(encoding="utf-8"))
    lock = tomllib.loads((WORKSPACE / "Cargo.lock").read_text(encoding="utf-8"))
    assert manifest["dependencies"]["blockscout-service-launcher"] == {"workspace": True}
    assert workspace["workspace"]["dependencies"]["blockscout-service-launcher"] == "0.17.0"
    launcher = [p for p in lock["package"] if p["name"] == "blockscout-service-launcher"]
    assert len(launcher) == 1
    assert launcher[0]["version"] == "0.17.0"
    assert launcher[0]["source"] == "registry+https://github.com/rust-lang/crates.io-index"


def test_repair_record_maps_finding_to_tests_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.597"
    finding = record["finding"]["obsolete-extension-config"]
    assert finding["audit"] == "22.30.161"
    assert finding["tests"] == [
        "test_obsolete_extension_examples_removed",
        "test_published_toml_uses_supported_settings",
        "test_commented_toml_examples_use_supported_fetcher_fields",
        "test_environment_examples_match_supported_toml_paths_and_values",
        "test_settings_build_and_constructor_source_path",
    ]
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "Settings::build" in record["repair"]["review_only"]
    assert record["repair"]["wiring_requests"]
