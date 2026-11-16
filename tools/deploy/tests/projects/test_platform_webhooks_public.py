"""Validate the public webhook project definition against its build inputs."""

from __future__ import annotations

import ast
import os
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DEFINITION = ROOT / "deploy/projects/platform/webhooks-public.kvx"
DOCKERFILE = ROOT / "docker/platform-webhooks/Dockerfile"
FILES_TABLE = ROOT / "docker/platform-webhooks/files.tsv"
ENV_INIT = ROOT / "services/platform/hosted/webhooks/env-init.sh"
SOURCE_ROOT = ROOT / "services/platform/hosted/webhooks/src"
COMMON_ENV_FILES = ROOT / "docker/common/env-files.sh"


def _parse_kvx(path: Path) -> tuple[dict[str, dict[str, object]], list[str]]:
    sections: dict[str, dict[str, object]] = {}
    comments: list[str] = []
    current: dict[str, object] | None = None
    for line in path.read_text().splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        if stripped.startswith("#"):
            comments.append(stripped[1:].strip())
            continue
        match = re.fullmatch(r"\[([^]]+)\]", stripped)
        if match:
            current = sections.setdefault(match.group(1), {})
            continue
        key, value = (part.strip() for part in stripped.split("=", 1))
        assert current is not None, f"assignment outside a section: {line}"
        current[key] = ast.literal_eval(value)
        comments.clear()
    return sections, comments


def _source_environment_names() -> set[str]:
    names: set[str] = set()
    literal = re.compile(
        r'(?:env::var(?:_os)?|read_(?:required|secret))\(\s*"([A-Z][A-Z0-9_]*)"'
    )
    for path in SOURCE_ROOT.rglob("*.rs"):
        names.update(literal.findall(path.read_text()))
    shell = ENV_INIT.read_text() + "\n" + COMMON_ENV_FILES.read_text()
    names.update(re.findall(r"\$\{([A-Z][A-Z0-9_]*)(?=[:}])", shell))
    names.update(re.findall(r"\$([A-Z][A-Z0-9_]*)", shell))
    names.update(re.findall(r"\b([A-Z][A-Z0-9_]*)\s*=", ENV_INIT.read_text()))
    return names


def _required_environment_names(names: set[str]) -> set[str]:
    required: set[str] = set()
    for path in SOURCE_ROOT.rglob("*.rs"):
        text = path.read_text()
        for name in names:
            if name not in text:
                continue
            if re.search(
                rf'(?:read_(?:required|secret)|env::var\("{re.escape(name)}"\)[^;]{{0,180}}required)',
                text,
                re.DOTALL,
            ):
                required.add(name)
    required.update({"LAYERX_ROLE", "PORT"})
    return required


def _public_names(names: set[str]) -> set[str]:
    excluded = {
        "LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER",
        "LAYERX_WEBHOOKS_COMPONENT_TOKEN_FILE",
        "LAYERX_WEBHOOKS_AUTHORITY_TOKEN_FILE",
        "LAYERX_WEBHOOKS_JOURNEY_SOURCE_TOKEN_FILE",
        "LAYERX_WEBHOOKS_PAYMENT_SOURCE_TOKEN_FILE",
        "LAYERX_WEBHOOKS_APPROVAL_SOURCE_TOKEN_FILE",
        "LAYERX_WEBHOOKS_PROGRAM_SOURCE_TOKEN_FILE",
        "LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE",
        "LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE",
        "LAYERX_WEBHOOKS_MARKET_POLICY_FILE",
        "LAYERX_WEBHOOKS_MARKET_OUTBOX_DIR",
        "LAYERX_HEALTH_ADDR",
        "LAYERX_WEBHOOKS_HEALTH_LISTEN",
        "LAYERX_WEBHOOKS_TLS_CERT_DER",
        "LAYERX_WEBHOOKS_TLS_KEY_DER",
    }
    excluded |= {
        name
        for name in names
        if any(
            marker in name
            for marker in (
                "_SOURCE_URL",
                "_SOURCE_TOKEN_FILE",
                "_NODE_URL",
                "_NODE_TOKEN_FILE",
                "_COMET_URL",
                "_CHAIN_POLICY_FILE",
                "_SEQUENCER_",
                "LAYERX_DASHBOARD_",
            )
        )
    }
    return names - excluded


def _file_inputs() -> set[str]:
    names = set()
    for line in FILES_TABLE.read_text().splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        name, _path, _mode, roles = line.split("\t")
        if "*" in roles.split(",") or "public" in roles.split(","):
            names.add(name)
    return names


def _build_args() -> set[str]:
    return set(re.findall(r"^ARG\s+([A-Z][A-Z0-9_]*)", DOCKERFILE.read_text(), re.MULTILINE))


def test_webhooks_public_definition_tracks_real_service_inputs() -> None:
    sections, _ = _parse_kvx(DEFINITION)
    service = sections["service.webhooks-public"]
    vars_ = sections["vars.webhooks-public"]
    inputs = sections["inputs.webhooks-public"]

    assert service["kind"] == "repo"
    assert service["dockerfile"] == "docker/platform-webhooks/Dockerfile"
    assert service["root"] == "."
    assert service["health"] == "/healthz"
    assert service["restart"] == "ALWAYS"
    assert service["replicas"] == 3
    assert service["public"] == 1
    assert service["order"] == 8
    assert set(service["needs"]) == {"redis-internal", "internal-kms", "identity"}
    assert vars_["LAYERX_WEBHOOKS_LISTENER"] == "plain"
    assert vars_["PORT"] == "9444"
    assert vars_["LAYERX_WEBHOOKS_LISTEN"] == "[::]:9444"
    assert "LAYERX_HEALTH_ADDR" not in vars_
    assert "LAYERX_WEBHOOKS_HEALTH_LISTEN" not in vars_
    assert "LAYERX_WEBHOOKS_TLS_CERT_DER" not in vars_
    assert "LAYERX_WEBHOOKS_TLS_KEY_DER" not in vars_

    watch = set(service["watch"])
    assert {"docker/platform-webhooks/**", "docker/common/**", "services/platform/**"} <= watch
    assert all((ROOT / pattern.removesuffix("/**")).exists() for pattern in watch)

    source_names = _source_environment_names()
    expected = _required_environment_names(_public_names(source_names)) | _file_inputs() | _build_args()
    assert expected <= set(vars_), sorted(expected - set(vars_))
    assert set(vars_) <= (_source_environment_names() | _file_inputs() | _build_args())

    generated_prefix = "PLATFORM_WEBHOOKS_PUBLIC_"
    env_refs = {
        value.removeprefix("env:"): key
        for key, value in vars_.items()
        if isinstance(value, str) and value.startswith("env:")
    }
    assert env_refs
    assert set(env_refs) == set(inputs)
    for name in inputs:
        assert name.startswith(generated_prefix)
        assert re.fullmatch(r"[A-Z][A-Z0-9_]*", name)
    assert set(inputs.values()) <= {"generated", "provider", "chain", "operator"}

    raw_lines = DEFINITION.read_text().splitlines()
    for index, line in enumerate(raw_lines):
        if not line.startswith("PLATFORM_WEBHOOKS_PUBLIC_"):
            continue
        previous = raw_lines[index - 1].strip()
        assert previous.startswith("#"), line


def _run_init(tmp_path: Path, **overrides: str) -> subprocess.CompletedProcess[str]:
    capture = tmp_path / "webhooks-environment"
    arguments = tmp_path / "setpriv-arguments"
    binary = tmp_path / "layerx-webhooks"
    binary.write_text(
        "#!/bin/sh\n"
        "env | sort > \"$LAYERX_TEST_CAPTURE\"\n"
        "exit 0\n"
    )
    binary.chmod(0o755)
    setpriv = tmp_path / "setpriv"
    setpriv.write_text(
        "#!/bin/sh\n"
        "printf '%s\\n' \"$@\" > \"$LAYERX_TEST_ARGUMENTS\"\n"
        "exec \"$LAYERX_TEST_BINARY\"\n"
    )
    setpriv.chmod(0o755)
    environment = {
        "PATH": f"{tmp_path}:{os.environ['PATH']}",
        "LAYERX_ROLE": "public",
        "RAILWAY_REPLICA_ID": "webhooks-public-test",
        "LAYERX_TEST_CAPTURE": str(capture),
        "LAYERX_TEST_ARGUMENTS": str(arguments),
        "LAYERX_TEST_BINARY": str(binary),
    }
    environment.update(overrides)
    return subprocess.run(
        ["sh", str(ENV_INIT)],
        env=environment,
        capture_output=True,
        text=True,
        timeout=5,
    )


def _captured_environment(tmp_path: Path) -> dict[str, str]:
    return {
        name: value
        for line in (tmp_path / "webhooks-environment").read_text().splitlines()
        for name, _, value in [line.partition("=")]
    }


def test_public_init_defaults_to_tls_when_listener_is_absent(tmp_path: Path) -> None:
    result = _run_init(tmp_path)
    assert result.returncode == 0, result.stderr
    captured = _captured_environment(tmp_path)
    assert captured["LAYERX_WEBHOOKS_LISTENER"] == "tls"
    assert captured["LAYERX_WEBHOOKS_LISTEN"] == "[::]:9444"
    assert captured["LAYERX_WEBHOOKS_HEALTH_LISTEN"] == "[::]:9442"
    assert captured["LAYERX_WEBHOOKS_TLS_CERT_DER"] == "/run/layerx/tls/server.der"
    assert captured["LAYERX_WEBHOOKS_TLS_KEY_DER"] == "/run/layerx/tls/server-key.der"
    assert captured["LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE"] == "/run/layerx/tokens/identity"
    assert (
        (tmp_path / "setpriv-arguments").read_text().splitlines()[-1]
        == "/usr/local/bin/layerx-webhooks"
    )


def test_public_init_plain_listener_uses_port(tmp_path: Path) -> None:
    result = _run_init(tmp_path, LAYERX_WEBHOOKS_LISTENER="plain", PORT="9444")
    assert result.returncode == 0, result.stderr
    captured = _captured_environment(tmp_path)
    assert captured["LAYERX_WEBHOOKS_LISTENER"] == "plain"
    assert captured["LAYERX_WEBHOOKS_LISTEN"] == "[::]:9444"
    assert captured["LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE"] == "/run/layerx/tokens/identity"


def test_public_init_plain_listener_prefers_explicit_listen(tmp_path: Path) -> None:
    result = _run_init(
        tmp_path,
        LAYERX_WEBHOOKS_LISTENER="plain",
        PORT="9555",
        LAYERX_WEBHOOKS_LISTEN="[::]:9666",
    )
    assert result.returncode == 0, result.stderr
    assert _captured_environment(tmp_path)["LAYERX_WEBHOOKS_LISTEN"] == "[::]:9666"


def test_public_init_plain_listener_has_no_tls_or_health_listener(tmp_path: Path) -> None:
    result = _run_init(
        tmp_path,
        LAYERX_WEBHOOKS_LISTENER="plain",
        PORT="9444",
        LAYERX_HEALTH_ADDR="[::]:9442",
        LAYERX_WEBHOOKS_HEALTH_LISTEN="[::]:9442",
        LAYERX_WEBHOOKS_TLS_CERT_DER="old-certificate",
        LAYERX_WEBHOOKS_TLS_KEY_DER="old-key",
    )
    assert result.returncode == 0, result.stderr
    captured = _captured_environment(tmp_path)
    assert "LAYERX_HEALTH_ADDR" not in captured
    assert "LAYERX_WEBHOOKS_HEALTH_LISTEN" not in captured
    assert "LAYERX_WEBHOOKS_TLS_CERT_DER" not in captured
    assert "LAYERX_WEBHOOKS_TLS_KEY_DER" not in captured


def test_public_init_refuses_another_listener_value(tmp_path: Path) -> None:
    result = _run_init(tmp_path, LAYERX_WEBHOOKS_LISTENER="other")
    assert result.returncode == 2
    assert "tls" in result.stderr and "plain" in result.stderr
    assert not (tmp_path / "webhooks-environment").exists()
