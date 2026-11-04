from pathlib import Path


ROOT = Path(__file__).resolve().parents[5]
ENGINE_PATH = ROOT / "services/platform/hosted/tests/topology-check.sh"
MANIFEST_PATH = ROOT / "services/platform/hosted/node/deployment.yaml"


def test_node_deployment_runs_no_guarantor():
    engine_source = ENGINE_PATH.read_text().split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0]
    engine = {"__name__": "layerx_topology"}
    exec(
        compile(engine_source.rsplit("sys.exit(main())", 1)[0], str(ENGINE_PATH), "exec"),
        engine,
    )
    _, load = engine["choose_parser"]()
    documents = load(MANIFEST_PATH.read_text())
    nodes = [
        document
        for document in documents
        if document.get("kind") == "StatefulSet"
        and document["metadata"]["name"] == "layerx-node"
    ]
    assert len(nodes) == 1
    pod = nodes[0]["spec"]["template"]["spec"]
    containers = pod["containers"] + pod.get("initContainers", [])
    for container in containers:
        assert not container["name"].startswith("guarantor-"), container["name"]
        for entry in container.get("env", []):
            name = entry["name"]
            assert not name.startswith("LAYERX_GUARANTOR_"), (container["name"], name)
            assert name not in {
                "LAYERX_AUTHORITY_LNI_SOCKET",
                "LAYERX_AGENT_BOUNDARY_LNI_SOCKET",
            }, (container["name"], name)
    core = next(container for container in pod["containers"] if container["name"] == "core-boundary")
    core_env = {entry["name"]: entry.get("value") for entry in core["env"]}
    assert "LAYERX_CORE_LNI_SOCKET" not in core_env
    assert core_env["LAYERX_CORE_CHAIN_RPC_URL"] == "http://127.0.0.1:8545"
    assert '${LAYERX_CORE_TREASURY_SIGNER_SOCKET:?treasury signer socket is required}' in core["args"][0]
    assert core_env["LAYERX_CORE_FUNDING_SENDER_KEY_FILE"] == "/run/funding-private/owner/sender.key"
    assert "LAYERX_CORE_FUNDING_SENDER_KMS_KEY_ID" not in core_env
    assert core_env["LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER"] == "/run/layerx/trust/ca.crt.der"
    config = next(document["data"] for document in documents
                  if document.get("kind") == "ConfigMap"
                  and document["metadata"]["name"] == "layerx-node-config")
    endpoint = next(entry for entry in core["env"] if entry["name"] == "LAYERX_CORE_SEQUENCER_ENDPOINT")
    reference = endpoint["valueFrom"]["configMapKeyRef"]
    assert reference == {"name": "layerx-node-config", "key": "gateway-endpoint"}
    assert config[reference["key"]] == "https://layerx-gateway.layerx-testnet.svc.cluster.local:9443"
    assert "generation_client.py core --socket /run/layerx/node/generation.sock" in core["args"][0]
    assert all("LNI" not in entry["name"] for container in containers
               for entry in container.get("env", []))
    native = next(container for container in pod["containers"] if container["name"] == "paxd")
    args = native["args"]
    assert args[args.index("--treasury-signer-socket") + 1] == "/run/layerx/node/treasury-signer.sock"
    signer = next(container for container in pod["containers"] if container["name"] == "treasury-signer")
    assert signer["args"][signer["args"].index("--socket") + 1] == "/run/layerx/node/treasury-signer.sock"
    retired_secrets = {
        "layerx-guarantor-1-tls",
        "layerx-guarantor-2-tls",
        "paxeer-checkpoint-submitter",
        "layerx-guarantor-checkpoint-authority-key",
    }
    for volume in pod["volumes"]:
        assert volume.get("secret", {}).get("secretName") not in retired_secrets, volume["name"]
