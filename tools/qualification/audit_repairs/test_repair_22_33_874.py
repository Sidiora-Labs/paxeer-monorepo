"""Offline source qualification for 22.30.183/f007; no Docker daemon required."""

import ast
from pathlib import Path
import re
import shlex
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "docker/monitornode/scripts/start-grafana.sh"
SOURCE = SCRIPT.read_text(encoding="utf-8")
RECORD = Path(__file__).with_name("repair_22_33_874.kvx")


def docker_run_arguments():
    commands = re.findall(r"^docker run -d \\\n(.*?)(?=\n\n)", SOURCE, re.M | re.S)
    assert len(commands) == 1
    return shlex.split("docker run -d " + commands[0].replace("\\\n", " "))


def existing_container_guard():
    functions = re.findall(r"^check_existing_container\(\) \{\n(.*?)^\}", SOURCE, re.M | re.S)
    assert len(functions) == 1
    return functions[0]


def test_f007_shell_syntax():
    result = subprocess.run(
        ["bash", "-n", str(SCRIPT)], capture_output=True, text=True, check=False
    )
    assert result.returncode == 0, result.stderr


def test_f007_loopback_default_reaches_docker_publish():
    assert 'GRAFANA_BIND_ADDRESS="${GRAFANA_BIND_ADDRESS:-127.0.0.1}"' in SOURCE
    assert re.search(r"^GRAFANA_PORT=3000$", SOURCE, re.M)
    arguments = docker_run_arguments()
    publications = [arguments[i + 1] for i, arg in enumerate(arguments) if arg == "-p"]
    assert publications == ["${GRAFANA_BIND_ADDRESS}:${GRAFANA_PORT}:3000"]
    assert not any(arg in ("-P", "--publish-all", "--network=host") for arg in arguments)
    assert arguments[-1] == "grafana/grafana:latest"


def test_f007_remote_password_guard_precedes_all_container_operations():
    assert 'GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-}"' in SOURCE
    guard = re.search(
        r'^if \[\[ "\$GRAFANA_BIND_ADDRESS" != "127\.0\.0\.1" \]\]; then\n'
        r'(.*?)^fi$', SOURCE, re.M | re.S
    )
    assert guard is not None
    assert re.search(
        r'if \[\[ -z "\$GRAFANA_ADMIN_PASSWORD" \|\| '
        r'"\$GRAFANA_ADMIN_PASSWORD" == "admin" \|\| '
        r'"\$GRAFANA_ADMIN_PASSWORD" == \*\$\'\\n\'\* \]\]; then\n'
        r'\s*echo [^\n]+ >&2\n\s*exit 1\n\s*fi', guard[1]
    )
    assert guard.end() < SOURCE.index('export GF_SECURITY_ADMIN_PASSWORD=')
    assert guard.end() < SOURCE.index('if ! docker info')
    assert 'export GF_SECURITY_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"' in SOURCE
    arguments = docker_run_arguments()
    environment = [arguments[i + 1] for i, arg in enumerate(arguments) if arg == "-e"]
    assert environment == [
        "GF_SECURITY_ADMIN_USER=admin", "GF_SECURITY_ADMIN_PASSWORD",
        "GF_USERS_ALLOW_SIGN_UP=false",
    ]
    assert "GF_SECURITY_ADMIN_PASSWORD=admin" not in SOURCE
    for line in SOURCE.splitlines():
        if line.lstrip().startswith("echo "):
            assert not re.search(r"\$(?:\{)?(?:GF_SECURITY_ADMIN_PASSWORD|GRAFANA_ADMIN_PASSWORD)", line)
    assert "admin / admin" not in SOURCE


def test_f007_existing_bindings_fail_closed_before_start_or_success():
    guard = existing_container_guard()
    assert "{{.HostConfig.NetworkMode}}" in guard
    assert '{{range (index .HostConfig.PortBindings "3000/tcp")}}' in guard
    assert '{{printf "%s:%s\\n" .HostIp .HostPort}}{{end}}' in guard
    assert re.search(
        r'if \[\[ "\$network" == "host" \|\| "\$bindings" != '
        r'"\$\{GRAFANA_BIND_ADDRESS\}:\$\{GRAFANA_PORT\}" \]\]; then\n'
        r'\s*echo [^\n]+ >&2\n\s*exit 1\n\s*fi', guard
    )
    for query in ("-q", "-aq"):
        branch = re.search(
            r'^if docker ps ' + re.escape(query)
            + r' -f "name=\^\$\{CONTAINER_NAME\}\$" \| grep -q \.; then\n(.*?)^fi$',
            SOURCE, re.M | re.S
        )
        assert branch is not None
        statements = [line.strip() for line in branch[1].splitlines() if line.strip()]
        assert statements[0] == "check_existing_container"
        assert statements[-1] == "exit 0"
        if query == "-aq":
            assert 'docker start "$CONTAINER_NAME"' in statements


def test_f007_existing_remote_credential_is_checked_without_disclosure():
    guard = existing_container_guard()
    assert re.search(r'local .*password_matches=false$', guard, re.M)
    assert 'if [[ "$GRAFANA_BIND_ADDRESS" != "127.0.0.1" ]]; then' in guard
    assert "{{range .Config.Env}}{{println .}}{{end}}" in guard
    assert 'while IFS= read -r entry; do' in guard
    assert 'if [[ "$entry" == GF_SECURITY_ADMIN_PASSWORD=* ]]; then' in guard
    assert re.search(
        r'if \[\[ "\$entry" != "GF_SECURITY_ADMIN_PASSWORD=\$\{GF_SECURITY_ADMIN_PASSWORD\}" \]\]; then\n'
        r'\s*echo [^\n]+ >&2\n\s*exit 1\n\s*fi\n\s*password_matches=true', guard
    )
    assert 'done <<< "$environment"' in guard
    assert re.search(
        r'if \[\[ "\$password_matches" != true \]\]; then\n'
        r'\s*echo [^\n]+ >&2\n\s*exit 1\n\s*fi', guard
    )
    for line in guard.splitlines():
        if line.lstrip().startswith("echo "):
            assert "$entry" not in line and "$environment" not in line


def test_f007_repair_record_maps_every_proof_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["f007"]
    assert finding["audit"] == "22.30.183"
    assert finding["file"] == str(SCRIPT.relative_to(ROOT))
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests - {
        "test_f007_repair_record_maps_every_proof_and_evidence_limits"
    }
    repair = record["repair"]
    assert repair["task"] == "22.33.874"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_874.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not executed" in repair["review_only"]
    assert "not run" in repair["baseline"]
    assert any("docker/README.md" in request and "domain 16" in request for request in repair["wiring_requests"])
