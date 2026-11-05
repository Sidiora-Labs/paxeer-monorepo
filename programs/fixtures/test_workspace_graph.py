import json
from pathlib import Path
import subprocess


def test_workspace_graph_has_no_adapter_or_kernel_proof_edges():
    root = Path(__file__).resolve().parents[2]
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--offline",
            "--format-version",
            "1",
            "--manifest-path",
            "programs/Cargo.toml",
        ],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    )
    metadata = json.loads(result.stdout)
    packages = {package["id"]: package for package in metadata["packages"]}
    assert all(
        package["name"] != "layerx-programs-protocol-adapter"
        for package in packages.values()
    ), "the retired protocol adapter is still in the resolved graph"

    nodes = {node["id"]: node for node in metadata["resolve"]["nodes"]}
    members = {
        packages[package_id]["name"]: package_id
        for package_id in metadata["workspace_members"]
    }
    forbidden = {"layerx-proof", "layerx-wire"}
    for name in ("layerx-programs-registry", "layerx-programs-sandbox"):
        assert name in members, f"{name} is missing from the workspace"
        dependencies = {
            packages[dependency["pkg"]]["name"]
            for dependency in nodes[members[name]]["deps"]
        }
        assert not dependencies & forbidden, (
            f"{name} still depends on {sorted(dependencies & forbidden)}"
        )
        assert "layerx-types" in dependencies, f"{name} must retain layerx-types"
