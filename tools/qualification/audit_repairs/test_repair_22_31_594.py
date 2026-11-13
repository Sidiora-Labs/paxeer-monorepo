"""Qualify the hosted gateway's public mainnet node configuration."""

from copy import deepcopy
import json
from pathlib import Path
import re
from urllib.parse import urlsplit

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


def selector_matches(selector, labels):
    if any(
        labels.get(key) != value
        for key, value in selector.get("matchLabels", {}).items()
    ):
        return False
    for expression in selector.get("matchExpressions", []):
        key = expression["key"]
        operator = expression["operator"]
        values = expression.get("values", [])
        assert operator in {"In", "NotIn", "Exists", "DoesNotExist"}
        if operator == "In" and (key not in labels or labels[key] not in values):
            return False
        if operator == "NotIn" and key in labels and labels[key] in values:
            return False
        if operator == "Exists" and key not in labels:
            return False
        if operator == "DoesNotExist" and key in labels:
            return False
    return True


def network_peer_matches(peer, policy_namespace, remote):
    assert set(peer) <= {"namespaceSelector", "podSelector", "ipBlock"}
    if "ipBlock" in peer:
        assert set(peer) == {"ipBlock"}
        # These manifests only use universal IP blocks, which include every pod IP.
        assert peer["ipBlock"] in ({"cidr": "0.0.0.0/0"}, {"cidr": "::/0"})
        return True
    if "namespaceSelector" in peer:
        if not selector_matches(peer["namespaceSelector"], remote["namespace_labels"]):
            return False
    elif "podSelector" in peer and remote["namespace"] != policy_namespace:
        return False
    return "podSelector" not in peer or selector_matches(
        peer["podSelector"], remote["labels"]
    )


def network_ports_allow(rule, destination, protocol, port):
    if not rule.get("ports"):
        return True
    for entry in rule["ports"]:
        if entry.get("protocol", "TCP") != protocol:
            continue
        if "port" not in entry:
            return True
        declared = entry["port"]
        if isinstance(declared, str):
            if any(
                candidate.get("name") == declared
                and candidate.get("protocol", "TCP") == protocol
                and candidate["containerPort"] == port
                for candidate in destination["ports"]
            ):
                return True
        elif declared <= port <= entry.get("endPort", declared):
            return True
    return False


def network_rule_allows(policy, rule, direction, remote, destination, protocol, port):
    peers = rule.get("to" if direction == "Egress" else "from", [])
    return network_ports_allow(rule, destination, protocol, port) and (
        not peers
        or any(
            network_peer_matches(peer, policy["metadata"]["namespace"], remote)
            for peer in peers
        )
    )


def network_direction_allows(policies, source, destination, direction, port):
    local, remote = (
        (source, destination) if direction == "Egress" else (destination, source)
    )
    selected = []
    for policy in policies:
        spec = policy["spec"]
        types = spec.get(
            "policyTypes", ["Ingress"] + (["Egress"] if "egress" in spec else [])
        )
        if (
            policy["metadata"]["namespace"] == local["namespace"]
            and selector_matches(spec["podSelector"], local["labels"])
            and direction in types
        ):
            selected.append(policy)
    return not selected or any(
        network_rule_allows(policy, rule, direction, remote, destination, "TCP", port)
        for policy in selected
        for rule in policy["spec"].get(direction.lower(), [])
    )


def test_gateway_cross_namespace_peer_policies(documents):
    resources = list(documents)
    for relative in (
        "human/web-deployment.yaml",
        "identity/deployment.yaml",
        "paxeer/deployment.yaml",
        "registry/deployment.yaml",
        "testnet/deployment.yaml",
    ):
        manifest = ROOT / "services/platform/hosted" / relative
        parsed = list(yaml.safe_load_all(manifest.read_text(encoding="utf-8")))
        assert parsed and all(isinstance(resource, dict) for resource in parsed)
        resources.extend(parsed)

    namespace_label = "layerx.io/gateway-peer-namespace"
    namespaces = {
        resource["metadata"]["name"]: resource["metadata"].get("labels", {})
        for resource in resources
        if resource["kind"] == "Namespace"
    }
    assert namespaces["layerx-testnet"][namespace_label] == "true"
    assert [name for name, labels in namespaces.items() if namespace_label in labels] == [
        "layerx-testnet"
    ]
    assert all(
        namespace_label not in resource["metadata"].get("labels", {})
        for resource in resources
        if resource["kind"] != "Namespace"
    )

    workloads = {}
    for resource in resources:
        if resource["kind"] not in {"Deployment", "StatefulSet"}:
            continue
        metadata = resource["metadata"]
        namespace = metadata["namespace"]
        template = resource["spec"]["template"]
        labels = template["metadata"]["labels"]
        assert namespace_label not in labels
        assert selector_matches(resource["spec"]["selector"], labels)
        assert metadata["name"] not in workloads
        workloads[metadata["name"]] = {
            "namespace": namespace,
            "namespace_labels": {
                **namespaces.get(namespace, {}),
                "kubernetes.io/metadata.name": namespace,
            },
            "labels": labels,
            "ports": [
                port
                for container in template["spec"]["containers"]
                for port in container.get("ports", [])
            ],
            "containers": template["spec"]["containers"],
        }

    gateway = workloads["layerx-gateway"]
    registry = workloads["layerx-program-registry"]
    faucet = workloads["layerx-faucet"]
    identity = workloads["layerx-identity"]
    boundary = workloads["paxeer"]
    web = workloads["layerx-human-web"]
    assert gateway["namespace"] == "layerx-mainnet-beta"
    assert all(
        workload["namespace"] == "layerx-testnet"
        for workload in (registry, faucet, identity, boundary, web)
    )
    policies = [resource for resource in resources if resource["kind"] == "NetworkPolicy"]

    def policy_named(name, namespace):
        matches = [
            policy
            for policy in policies
            if policy["metadata"]["name"] == name
            and policy["metadata"]["namespace"] == namespace
        ]
        assert len(matches) == 1, (name, namespace)
        return matches[0]

    gateway_policy = policy_named("layerx-gateway", gateway["namespace"])
    web_policy = policy_named("layerx-human-web", web["namespace"])
    web_ingress = policy_named("layerx-human-web-gateway", gateway["namespace"])
    peer_namespace_selector = {"matchLabels": {namespace_label: "true"}}
    gateway_namespace_selector = {
        "matchLabels": {"kubernetes.io/metadata.name": gateway["namespace"]}
    }

    def assert_branch(
        policy, direction, local, remote, destination, label_key, ports, namespace_selector
    ):
        assert policy["metadata"]["namespace"] == local["namespace"]
        assert selector_matches(policy["spec"]["podSelector"], local["labels"])
        assert direction in policy["spec"]["policyTypes"]
        pod_selector = {"matchLabels": {label_key: remote["labels"][label_key]}}
        branches = [
            (rule, peer)
            for rule in policy["spec"][direction.lower()]
            for peer in rule.get("to" if direction == "Egress" else "from", [])
            if peer.get("podSelector") == pod_selector
        ]
        assert len(branches) == 1, (policy["metadata"]["name"], pod_selector)
        rule, peer = branches[0]
        assert peer == {
            "namespaceSelector": namespace_selector,
            "podSelector": pod_selector,
        }
        assert rule["ports"] == [{"protocol": "TCP", "port": port} for port in ports]
        policy_namespace = policy["metadata"]["namespace"]
        assert network_peer_matches(peer, policy_namespace, remote)
        unlabelled = {
            **remote,
            "namespace": "unlabelled",
            "namespace_labels": {"kubernetes.io/metadata.name": "unlabelled"},
        }
        unrelated = {
            **remote, "labels": {"app": "unrelated", "layerx-plane": "unrelated"}
        }
        assert not network_peer_matches(peer, policy_namespace, unlabelled)
        assert not network_peer_matches(peer, policy_namespace, unrelated)
        # A bare selector must remain local; this would reject the intended remote pod.
        assert remote["namespace"] != policy_namespace
        assert not network_peer_matches(
            {"podSelector": pod_selector}, policy_namespace, remote
        )
        for port in ports:
            assert network_ports_allow(rule, destination, "TCP", port)
            assert not network_ports_allow(rule, destination, "UDP", port)
        for port in (1, 9442, 65535):
            assert port not in ports
            assert not network_ports_allow(rule, destination, "TCP", port)

    for remote, key, ports in (
        (registry, "app", [9420]),
        (identity, "layerx-plane", [9443, 9445, 9446]),
        (boundary, "layerx-plane", [9443, 9445, 9446]),
        (faucet, "app", [9443]),
    ):
        assert_branch(
            gateway_policy, "Egress", gateway, remote, remote, key, ports,
            peer_namespace_selector,
        )
    assert_branch(
        gateway_policy, "Ingress", gateway, registry, gateway, "app", [9443],
        peer_namespace_selector,
    )
    for name, destination, ports in (
        ("layerx-program-registry", registry, [9420]),
        ("layerx-faucet-public-ingress", faucet, [9443]),
        ("layerx-identity-ingress", identity, [9443]),
        ("paxeer-boundary", boundary, [9443, 9444]),
    ):
        assert_branch(
            policy_named(name, destination["namespace"]),
            "Ingress", destination, gateway, destination, "app", ports,
            gateway_namespace_selector,
        )
    assert_branch(
        web_policy, "Egress", web, gateway, gateway, "app", [9443],
        gateway_namespace_selector,
    )
    assert_branch(
        web_ingress, "Ingress", gateway, web, gateway, "app", [9443],
        peer_namespace_selector,
    )

    for source, destination, port in (
        (gateway, registry, 9420),
        (gateway, faucet, 9443),
        (gateway, identity, 9443),
        (gateway, boundary, 9443),
        (web, gateway, 9443),
    ):
        assert any(
            entry["containerPort"] == port and entry.get("protocol", "TCP") == "TCP"
            for entry in destination["ports"]
        )
        for direction in ("Egress", "Ingress"):
            assert network_direction_allows(policies, source, destination, direction, port), (
                source["labels"], destination["labels"], direction, port
            )
            assert not network_direction_allows(
                policies, source, destination, direction, 65535
            )

    node_peer = {"podSelector": {"matchLabels": {"app": "layerx-node"}}}
    assert any(
        node_peer in rule.get("from", [])
        for rule in gateway_policy["spec"]["ingress"]
    )
    redis = workloads["layerx-gateway-redis"]
    assert network_direction_allows(policies, gateway, redis, "Egress", 6379)
    assert network_direction_allows(policies, gateway, redis, "Ingress", 6379)
    redis_policy = policy_named("layerx-gateway-redis", gateway["namespace"])
    assert redis_policy["spec"]["egress"] == []

    web_environments = [
        entry
        for container in web["containers"]
        for entry in container.get("env", [])
        if entry["name"] == "LAYERX_GATEWAY_URL"
    ]
    assert len(web_environments) == 1
    url = urlsplit(web_environments[0]["value"])
    assert url.scheme == "https" and url.path == "/"
    services = [
        resource
        for resource in documents
        if resource["kind"] == "Service"
        and selector_matches(
            {"matchLabels": resource["spec"]["selector"]}, gateway["labels"]
        )
    ]
    assert len(services) == 1
    service = services[0]
    metadata = service["metadata"]
    assert url.hostname == f'{metadata["name"]}.{metadata["namespace"]}.svc.cluster.local'
    assert metadata["namespace"] == gateway["namespace"]
    service_ports = [
        entry for entry in service["spec"]["ports"]
        if entry["port"] == (url.port or 443)
    ]
    assert len(service_ports) == 1
    target = service_ports[0]["targetPort"]
    assert any(
        (entry.get("name") == target or entry["containerPort"] == target)
        and entry["containerPort"] == 9443
        and entry.get("protocol", "TCP") == "TCP"
        for entry in gateway["ports"]
    )


def test_gateway_ingress_admits_existing_control_namespace_and_pods(documents):
    checker = ROOT / "services/platform/hosted/tests/topology-check.sh"
    code = checker.read_text(encoding="utf-8").split("<<'PY'\n", 1)[1]
    code = code.split("\nPY\n", 1)[0].rsplit("sys.exit(main())", 1)[0]
    module = {}
    exec(compile(code, str(checker), "exec"), module)
    topology = module["Topology"]()
    for document in documents:
        topology.add(document, str(MANIFEST))
    control_manifest = ROOT / "services/platform/hosted/testnet/deployment.yaml"
    for document in yaml.safe_load_all(control_manifest.read_text(encoding="utf-8")):
        topology.add(document, str(control_manifest))

    (gateway,) = [
        workload
        for workload in topology.workloads
        if workload["kind"] == "Deployment" and workload["name"] == "layerx-gateway"
    ]
    (control,) = [
        workload
        for workload in topology.workloads
        if workload["kind"] == "Deployment"
        and workload["name"] == "layerx-testnet-control"
    ]
    assert gateway["ns"] != control["ns"]
    assert control["labels"]["layerx-plane"] == "control"
    assert topology.namespaces[control["ns"]]["layerx-control-plane"] == "enabled"
    (policy,) = [
        policy
        for policy in topology.policies
        if policy["name"] == "layerx-gateway" and policy["ns"] == gateway["ns"]
    ]
    control_peer = {
        "namespaceSelector": {"matchLabels": {"layerx-control-plane": "enabled"}},
        "podSelector": {"matchLabels": {"layerx-plane": "control"}},
    }
    control_rules = [
        rule for rule in policy["ingress"] if control_peer in rule.get("from", [])
    ]
    assert len(control_rules) == 1
    assert control_rules[0]["from"].count(control_peer) == 1
    assert control_rules[0]["ports"] == [{"protocol": "TCP", "port": 9443}]

    admitted, reason = topology.ingress_admits(gateway, "9443", "TCP", control)
    assert admitted, reason

    without_namespace_label = deepcopy(topology)
    del without_namespace_label.namespaces[control["ns"]]["layerx-control-plane"]
    admitted, reason = without_namespace_label.ingress_admits(
        gateway, "9443", "TCP", control
    )
    assert not admitted, reason

    for labels in (
        {key: value for key, value in control["labels"].items() if key != "layerx-plane"},
        {**control["labels"], "layerx-plane": "public"},
    ):
        caller = {**control, "labels": labels}
        admitted, reason = topology.ingress_admits(gateway, "9443", "TCP", caller)
        assert not admitted, reason

    unlabelled_namespace = deepcopy(topology)
    namespace = deepcopy(topology.namespaces[control["ns"]])
    del namespace["layerx-control-plane"]
    namespace["kubernetes.io/metadata.name"] = "unlabelled-control"
    unlabelled_namespace.namespaces["unlabelled-control"] = namespace
    caller = {**control, "ns": "unlabelled-control"}
    admitted, reason = unlabelled_namespace.ingress_admits(
        gateway, "9443", "TCP", caller
    )
    assert not admitted, reason

    for port, protocol in (("9443", "UDP"), ("9444", "TCP")):
        admitted, reason = topology.ingress_admits(gateway, port, protocol, control)
        assert not admitted, reason

    admitted, reason = topology.ingress_admits(
        gateway, "9443", "TCP", module["INGRESS_CONTROLLER"]
    )
    assert admitted, reason
