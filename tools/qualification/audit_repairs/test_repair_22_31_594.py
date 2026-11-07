"""Qualify the hosted gateway's public mainnet node configuration."""

import json
from pathlib import Path
import re

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
MANIFEST = ROOT / "services/platform/hosted/gateway/deployment.yaml"


@pytest.fixture
def documents():
    resources = list(yaml.safe_load_all(MANIFEST.read_text(encoding="utf-8")))
    assert resources
    assert all(isinstance(resource, dict) for resource in resources)
    assert all(resource["apiVersion"] and resource["kind"] for resource in resources)
    identities = [(r["kind"], r["metadata"]["name"]) for r in resources]
    assert len(identities) == len(set(identities))
    return resources


def gateway_environment(documents):
    deployments = [
        resource
        for resource in documents
        if resource["kind"] == "Deployment"
        and resource["metadata"]["name"] == "layerx-gateway"
    ]
    assert len(deployments) == 1
    containers = deployments[0]["spec"]["template"]["spec"]["containers"]
    gateways = [container for container in containers if container["name"] == "gateway"]
    assert len(gateways) == 1
    entries = gateways[0]["env"]
    assert len({entry["name"] for entry in entries}) == len(entries)
    return {entry["name"]: entry for entry in entries}


def scalar_values(value):
    if isinstance(value, dict):
        for child in value.values():
            yield from scalar_values(child)
    elif isinstance(value, list):
        for child in value:
            yield from scalar_values(child)
    else:
        yield value


def test_manifest_parses_as_yaml(documents):
    assert gateway_environment(documents)["LAYERX_GATEWAY_NETWORK_ID"]["value"] == (
        "paxeer-mainnet-beta"
    )
    assert all(
        resource["metadata"]["namespace"] == "layerx-mainnet-beta"
        for resource in documents
    )


def test_rpc_urls_are_the_first_eight_public_node_roots(documents):
    environment = gateway_environment(documents)
    urls = json.loads(environment["LAYERX_GATEWAY_PAXEER_RPC_URLS"]["value"])
    assert isinstance(urls, list)
    assert len(urls) == 8
    assert all(
        isinstance(url, str)
        and re.fullmatch(r"https://api[1-8]\.mainnet-beta\.paxeer\.network/", url)
        for url in urls
    )
    assert urls == [
        f"https://api{number}.mainnet-beta.paxeer.network/"
        for number in range(1, 9)
    ]


def test_faucet_and_cache_require_operator_configmap(documents):
    environment = gateway_environment(documents)
    assert "ConfigMap layerx-gateway-upstreams" in MANIFEST.read_text(encoding="utf-8")
    for name, key in (
        ("LAYERX_GATEWAY_FAUCET_URL", "faucet-url"),
        ("LAYERX_GATEWAY_REDIS_URL", "redis-url"),
    ):
        assert environment[name] == {
            "name": name,
            "valueFrom": {
                "configMapKeyRef": {
                    "name": "layerx-gateway-upstreams",
                    "key": key,
                    "optional": False,
                }
            },
        }
    assert not any(
        resource["kind"] == "ConfigMap"
        and resource["metadata"]["name"] == "layerx-gateway-upstreams"
        for resource in documents
    )


def test_manifest_has_no_internal_service_dns_or_test_namespace(documents):
    assert "layerx-testnet" not in MANIFEST.read_text(encoding="utf-8")
    strings = [value for value in scalar_values(documents) if isinstance(value, str)]
    assert strings
    for value in strings:
        assert ".svc.cluster.local" not in value.lower(), value


def test_gateway_egress_allows_public_https_nodes(documents):
    policies = [
        resource
        for resource in documents
        if resource["kind"] == "NetworkPolicy"
        and resource["spec"]["podSelector"] == {
            "matchLabels": {"app": "layerx-gateway"}
        }
    ]
    assert any(
        "Egress" in policy["spec"]["policyTypes"]
        and any(
            {"ipBlock": {"cidr": "0.0.0.0/0"}} in rule.get("to", [])
            and {"protocol": "TCP", "port": 443} in rule.get("ports", [])
            for rule in policy["spec"].get("egress", [])
        )
        for policy in policies
    )
