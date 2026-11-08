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
    assert [container['name'] for container in pod['containers']] == ['paxd']
    assert [container['name'] for container in pod['initContainers']] == ['native-home']
    assert pod['shareProcessNamespace'] is False
    assert pod['automountServiceAccountToken'] is False
    assert all('LNI' not in entry['name'] and not entry['name'].startswith('LAYERX_CORE_')
               for container in containers for entry in container.get('env', []))
    native = pod['containers'][0]
    assert native['command'] == [
        'python3', '/opt/layerx/kernel_runtime.py', '--home', '/data/layerx/node']
    config = next(document['data'] for document in documents
                  if document.get('kind') == 'ConfigMap'
                  and document['metadata']['name'] == 'layerx-node-config')
    assert config['chain-id'] == 'hyperpax_125-1'
    assert config['paxeer-chain-id'] == '125'
    assert config['rpc-origin'] == 'http://layerx-native-chain.layerx-testnet.svc:8545'
    assert {mount['name'] for mount in native['volumeMounts']} == {'data', 'tmp'}
    init = pod['initContainers'][0]
    assert init['command'] == ['python3', '-c']
    assert "'receipt.key': 'config/receipt.key'" in init['args'][0]
    assert "'treasury-policy.json': 'treasury-policy.json'" in init['args'][0]
    retired_secrets = {
        "layerx-guarantor-1-tls",
        "layerx-guarantor-2-tls",
        "paxeer-checkpoint-submitter",
        "layerx-guarantor-checkpoint-authority-key",
    }
    for volume in pod["volumes"]:
        assert volume.get("secret", {}).get("secretName") not in retired_secrets, volume["name"]
