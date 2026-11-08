"""Offline source-fact qualification of monitoring Compose security (f008)."""

import ast
from pathlib import Path
import re
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
COMPOSE = ROOT / "docker/docker-compose.monitoring.yml"
README = ROOT / "docker/README.md"
RECORD = Path(__file__).with_name("repair_22_33_875.kvx")


def monitoring_config():
    return yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))


def test_f008_ports_default_to_loopback_with_explicit_grafana_override():
    config = monitoring_config()
    services = config["services"]
    assert set(services) == {"grafana", "prometheus"}
    assert services["prometheus"]["ports"] == ["127.0.0.1:9099:9090"]
    assert services["grafana"]["ports"] == [
        "${GRAFANA_BIND_ADDRESS:-127.0.0.1}:3000:3000"
    ]
    for service in services.values():
        assert "network_mode" not in service
        assert service["networks"] == ["localnet"]
    assert config["networks"] == {"localnet": None}


def test_f008_secret_producer_mount_and_grafana_consumer_are_connected():
    config = monitoring_config()
    grafana = config["services"]["grafana"]
    environment = dict(entry.split("=", 1) for entry in grafana["environment"])
    assert environment == {
        "GF_SECURITY_ADMIN_USER": "admin",
        "GF_SECURITY_ADMIN_PASSWORD__FILE": "/run/secrets/grafana_admin_password",
        "GF_USERS_ALLOW_SIGN_UP": "false",
    }
    assert grafana["secrets"] == ["grafana_admin_password"]
    secret_name = grafana["secrets"][0]
    assert environment["GF_SECURITY_ADMIN_PASSWORD__FILE"] == (
        f"/run/secrets/{secret_name}"
    )
    assert set(config["secrets"]) == {secret_name}
    secret = config["secrets"][secret_name]
    assert set(secret) == {"file"}
    required_file = re.fullmatch(
        r"\$\{GRAFANA_ADMIN_PASSWORD_FILE:\?([^{}]+)\}", secret["file"]
    )
    assert required_file is not None
    assert "private file" in required_file[1]
    assert "unique Grafana admin password" in required_file[1]
    assert "secrets" not in config["services"]["prometheus"]
    assert "GF_SECURITY_ADMIN_PASSWORD=" not in COMPOSE.read_text(encoding="utf-8")


def test_f008_internal_prometheus_transport_and_provisioning_are_preserved():
    config = monitoring_config()
    grafana = config["services"]["grafana"]
    prometheus = config["services"]["prometheus"]
    assert grafana["image"] == "grafana/grafana:11.3.0"
    assert prometheus["image"] == "prom/prometheus:v2.55.1"
    assert grafana["depends_on"] == ["prometheus"]
    assert grafana["volumes"] == [
        "./docker_compose_monitoring/grafana-datasource.yaml:"
        "/etc/grafana/provisioning/datasources/grafana-datasource.yaml:ro"
    ]
    assert prometheus["volumes"] == [
        "./docker_compose_monitoring/prometheus.yaml:/etc/prometheus/prometheus.yml:ro"
    ]
    assert prometheus["command"] == [
        "--config.file=/etc/prometheus/prometheus.yml",
        "--storage.tsdb.path=/prometheus",
        "--web.enable-lifecycle",
    ]
    source = grafana["volumes"][0].split(":", 1)[0]
    provisioning = yaml.safe_load(
        (COMPOSE.parent / source).read_text(encoding="utf-8")
    )
    assert provisioning["apiVersion"] == 1
    assert len(provisioning["datasources"]) == 1
    datasource = provisioning["datasources"][0]
    assert datasource["type"] == "prometheus"
    assert datasource["access"] == "proxy"
    assert datasource["url"] == "http://prometheus:9090"


def test_f008_documentation_covers_secret_setup_remote_opt_in_and_existing_state():
    documentation = README.read_text(encoding="utf-8")
    monitoring = documentation.split("**Cluster and monitoring together:**", 1)[1]
    monitoring = monitoring.split("**Scripts only:**", 1)[0]
    assert 'export GRAFANA_ADMIN_PASSWORD_FILE="$HOME/' in monitoring
    assert (
        'umask 077; openssl rand -base64 32 > "$GRAFANA_ADMIN_PASSWORD_FILE"'
        in monitoring
    )
    assert "never `admin`" in monitoring
    assert "unset or empty `GRAFANA_ADMIN_PASSWORD_FILE`" in monitoring
    assert "/run/secrets/grafana_admin_password" in monitoring
    assert "GF_SECURITY_ADMIN_PASSWORD__FILE" in monitoring
    assert "127.0.0.1:3000" in monitoring
    assert "127.0.0.1:9099" in monitoring
    assert "export GRAFANA_BIND_ADDRESS=192.0.2.10" in monitoring
    assert "Replace with your host's private interface address" in monitoring
    assert "make docker-cluster-start-monitoring" in monitoring
    assert "authenticated TLS proxy" in monitoring
    assert "Prometheus stays\non loopback" in monitoring
    assert (
        "does not rotate a password already stored in Grafana's database"
        in monitoring
    )
    assert (
        "rotate their admin password through Grafana\nbefore enabling remote access"
        in monitoring
    )
    assert "Recreate existing containers" in monitoring
    assert "restarting an old container retains its settings" in monitoring


def test_f008_repair_record_maps_proofs_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.875"
    assert repair["requirement"] == "22998.1"
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_875.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["wiring_requests"] == []
    assert set(record["finding"]) == {"f008"}
    finding = record["finding"]["f008"]
    assert finding["audit"] == "22.30.183"
    assert finding["file"] == str(COMPOSE.relative_to(ROOT))
    assert finding["owner_domain"] == 16
    functions = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == functions - {
        "test_f008_repair_record_maps_proofs_and_qualification_limits"
    }
