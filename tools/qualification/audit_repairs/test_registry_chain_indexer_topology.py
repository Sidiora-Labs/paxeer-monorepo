"""Source-contract proof for the registry's maintained chain-indexer topology."""

from pathlib import Path
import re
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
REGISTRY = ROOT / "services/platform/hosted/registry"
INDEXER = ROOT / "services/platform/hosted/indexer"
DEPLOYMENT = REGISTRY / "deployment.yaml"


def documents():
    docs = list(yaml.safe_load_all(DEPLOYMENT.read_text()))
    identities = [(doc["kind"], doc["metadata"]["name"]) for doc in docs]
    assert len(identities) == len(set(identities))
    assert all(doc["metadata"]["namespace"] == "layerx-testnet" for doc in docs)
    return dict(zip(identities, docs))


def pod():
    return documents()["StatefulSet", "layerx-program-registry"]["spec"]["template"]


def container(name):
    return next(item for item in pod()["spec"]["containers"] if item["name"] == name)


def environment(name):
    entries = container(name)["env"]
    assert len({item["name"] for item in entries}) == len(entries)
    return {item["name"]: item for item in entries}


def test_retired_explorer_topology_removed():
    text = DEPLOYMENT.read_text()
    for retired in (
        "layerx-explorer-index",
        "layerx-registry-lni-bridge",
        "explorer-boundary",
        "explorer-observation",
        "explorer-read",
        "explorer-clock",
        "LAYERX_EXPLORER_",
        "LAYERX_NETWORK_GATEWAY_ENDPOINT",
        "observed-sealed-batch",
        "finalised-checkpoint",
        ":443/rpc",
        "layerx-runtime-clock",
        "socat",
    ):
        assert retired not in text
    assert {item["name"] for item in pod()["spec"]["containers"]} == {
        "registry",
        "chain-indexer",
    }
    assert container("chain-indexer")["command"] == ["/usr/local/bin/layerx-indexer"]
    assert "args" not in container("chain-indexer")


def test_maintained_indexer_package_image_and_configuration():
    manifest = tomllib.loads((REGISTRY / "Cargo.toml").read_text())
    metadata = manifest["package"]["metadata"]["layerx"]["deployment"]
    assert (REGISTRY / metadata["manifest"]).resolve() == DEPLOYMENT.resolve()
    companion = (REGISTRY / metadata["chain-indexer-manifest"]).resolve()
    assert companion == INDEXER / "Cargo.toml"
    producer = tomllib.loads(companion.read_text())
    assert producer["package"]["name"] == "layerx-indexer"
    assert {"name": "layerx-indexer", "path": "src/main.rs"} in producer["bin"]
    assert manifest["package"]["name"] == "layerx-platform-registry"
    assert {"name": "layerx-program-registry", "path": "src/main.rs"} in manifest["bin"]

    indexer = container("chain-indexer")
    assert indexer["image"] == "ghcr.io/sidiora-labs/platform-indexer:0.1.0"
    image_build = (ROOT / "docker/platform-indexer/Dockerfile").read_text()
    assert "-p layerx-indexer" in image_build
    assert "COPY --from=build /src/services/platform/target/release/layerx-indexer /usr/local/bin/" in image_build
    assert "COPY --from=build /abi /usr/local/share/layerx/precompiles" in image_build
    assert "isrg-root-x1.der" in image_build

    env = environment("chain-indexer")
    config = (INDEXER / "src/config.rs").read_text()
    for suffix, key in (
        ("EVM_URL", "evm-url"),
        ("COMET_URL", "comet-url"),
        ("START_BLOCK", "start-block"),
    ):
        variable = f"LAYERX_INDEXER_{suffix}"
        assert env[variable]["valueFrom"] == {
            "configMapKeyRef": {"name": "layerx-chain-indexer-chain", "key": key}
        }
        assert f'"{variable}"' in config
    assert env["LAYERX_INDEXER_EVM_CHAIN_ID"]["value"] == "125"
    for suffix in ("EVM_CA_DER", "COMET_CA_DER"):
        assert env[f"LAYERX_INDEXER_{suffix}"]["value"] == "/usr/local/share/layerx/isrg-root-x1.der"
        assert f'"LAYERX_INDEXER_{suffix}"' in config
    assert env["LAYERX_INDEXER_ABI_DIR"]["value"] == "/usr/local/share/layerx/precompiles"
    assert "LAYERX_INDEXER_ALLOW_REMOTE_PLAINTEXT" not in env
    assert "LAYERX_INDEXER_FINALITY_DEPTH" not in env
    assert "Security::PinnedTls" in config
    main = (INDEXER / "src/main.rs").read_text()
    assert "PaxeerIngester::new(" in main
    assert "ingester.daemon_step(store)" in main
    assert 'readiness.sources.push("paxeer")' in main
    assert 'readiness.sources.push("comet")' in main


def test_indexer_persistent_store_and_native_tls():
    env = environment("chain-indexer")
    indexer = container("chain-indexer")
    mounts = {item["name"]: item for item in indexer["volumeMounts"]}
    assert set(mounts) == {"indexer-state", "indexer-tls"}
    assert mounts["indexer-state"] == {
        "name": "indexer-state",
        "mountPath": "/var/lib/layerx-indexer",
    }
    assert env["LAYERX_INDEXER_DB"]["value"] == "/var/lib/layerx-indexer/indexer.sqlite"
    claims = documents()["StatefulSet", "layerx-program-registry"]["spec"]["volumeClaimTemplates"]
    claim = next(item for item in claims if item["metadata"]["name"] == "indexer-state")
    assert claim["spec"] == {
        "accessModes": ["ReadWriteOnce"],
        "resources": {"requests": {"storage": "100Gi"}},
    }
    volumes = {item["name"]: item for item in pod()["spec"]["volumes"]}
    secret = volumes["indexer-tls"]["secret"]
    assert secret["secretName"] == "layerx-chain-indexer-tls"
    assert secret["defaultMode"] == 0o440
    assert "optional" not in secret
    assert mounts["indexer-tls"]["readOnly"] is True
    assert {item["key"]: item["path"] for item in secret["items"]} == {
        "tls.crt.der": "tls.crt.der",
        "tls.key.der": "tls.key.der",
    }
    for suffix, filename in (("CERT", "tls.crt.der"), ("KEY", "tls.key.der")):
        assert env[f"LAYERX_INDEXER_TLS_{suffix}_DER"]["value"] == str(
            Path(mounts["indexer-tls"]["mountPath"]) / filename
        )
    security = indexer["securityContext"]
    assert security["runAsNonRoot"] is True
    assert security["allowPrivilegeEscalation"] is False
    assert security["readOnlyRootFilesystem"] is True
    assert security["capabilities"] == {"drop": ["ALL"]}
    assert security["runAsGroup"] == pod()["spec"]["securityContext"]["fsGroup"]
    main = (INDEXER / "src/main.rs").read_text()
    assert "Store::open(&config.database)?" in main
    assert 'tls::server_config("LAYERX_INDEXER")' in main
    assert "api::serve_with(&listener, &store, &Arc::new(readiness), tls.as_ref())" in main


def test_chain_indexer_service_and_network_transport():
    docs = documents()
    labels = pod()["metadata"]["labels"]
    services = [doc for (kind, _), doc in docs.items() if kind == "Service"]
    ports = {
        entry["name"]: entry["containerPort"]
        for item in pod()["spec"]["containers"]
        for entry in item["ports"]
    }
    assert len(ports) == 3
    for service in services:
        assert all(labels[key] == value for key, value in service["spec"]["selector"].items())
        for port in service["spec"]["ports"]:
            assert port["targetPort"] in ports
            assert ports[port["targetPort"]] == port["port"]
    service = docs["Service", "layerx-chain-indexer"]
    assert service["spec"]["ports"] == [
        {"name": "chain-indexer", "port": 8095, "targetPort": "chain-indexer"}
    ]
    listener = environment("chain-indexer")["LAYERX_INDEXER_LISTEN"]["value"]
    assert listener == "0.0.0.0:8095"
    policy = docs["NetworkPolicy", "layerx-chain-indexer"]["spec"]
    assert policy["podSelector"]["matchLabels"] == service["spec"]["selector"]
    assert policy["policyTypes"] == ["Ingress"]
    assert policy["ingress"] == [{
        "from": [
            {"podSelector": {"matchLabels": {"app": "layerx-human-web"}}},
            {"podSelector": {"matchLabels": {"app": "layerx-gateway"}}},
        ],
        "ports": [{"protocol": "TCP", "port": 8095}],
    }]
    egress = docs["NetworkPolicy", "layerx-program-registry"]["spec"]["egress"]
    assert {
        "to": [{"ipBlock": {"cidr": "0.0.0.0/0"}}, {"ipBlock": {"cidr": "::/0"}}],
        "ports": [{"protocol": "TCP", "port": 443}],
    } in egress
    assert {
        "to": [{"namespaceSelector": {}}],
        "ports": [{"protocol": "UDP", "port": 53}, {"protocol": "TCP", "port": 53}],
    } in egress
    assert not any(
        peer.get("podSelector", {}).get("matchLabels", {}).get("app") == "layerx-gateway"
        for rule in egress for peer in rule["to"]
    )


def test_native_readiness_and_registry_dependencies_preserved():
    indexer = container("chain-indexer")
    api = (INDEXER / "src/api.rs").read_text()
    for probe, route in (("readinessProbe", "/readyz"), ("livenessProbe", "/healthz")):
        assert indexer[probe]["httpGet"] == {
            "scheme": "HTTPS", "path": route, "port": "chain-indexer"
        }
        assert re.search(r'if path == "' + re.escape(route) + r'"', api)
    assert 'if document.status == "ready" { 200 } else { 503 }' in api
    assert "store.ping()" in api
    assert "plain HTTP listeners must be loopback" in api
    env = environment("registry")
    assert env["LAYERX_REGISTRY_CHAIN_RPC_URL"]["value"] == "https://api1.mainnet-beta.paxeer.network/"
    assert env["LAYERX_REGISTRY_NODE_ENDPOINT"]["value"] == "https://layerx-agent-boundary.layerx-testnet.svc.cluster.local:9443"
    assert env["LAYERX_REGISTRY_RECEIPT_AUTHORITY_ENDPOINT"]["value"] == "https://layerx-receipt-authority.layerx-testnet.svc.cluster.local:9443"
    mounts = {item["name"]: item for item in container("registry")["volumeMounts"]}
    assert mounts["journal"]["subPath"] == "journal"
    assert mounts["trust"]["readOnly"] is True
    assert mounts["state"]["mountPath"] == env["LAYERX_REGISTRY_STATE"]["value"]
