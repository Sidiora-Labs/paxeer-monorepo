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
    assert core_env["LAYERX_CORE_LNI_SOCKET"] == "/run/layerx/node/layerxd.lni.sock"
    retired_secrets = {
        "layerx-guarantor-1-tls",
        "layerx-guarantor-2-tls",
        "paxeer-checkpoint-submitter",
        "layerx-guarantor-checkpoint-authority-key",
    }
    for volume in pod["volumes"]:
        assert volume.get("secret", {}).get("secretName") not in retired_secrets, volume["name"]
