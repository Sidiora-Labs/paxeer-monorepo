import json
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
BINARY = ROOT / "build/bin/layerx-module-registry"
REFUSED = (
    "layerx-module-registry: refused: invalid input or unavailable canonical registry\n"
)
GENERATE = [
    "generate",
    "--network-id", "1",
    "--protocol-version", "3",
    "--asset", "b5a32b12029f8ddfb905f90f280f664b46390de0fc62770fc197dd87b18cd898",
    "--symbol", "LXT",
    "--currency", "LXT",
    "--decimals", "18",
]


def run_registry(args):
    return subprocess.run(
        [str(BINARY), *args], capture_output=True, text=True, timeout=15
    )


def test_read_node_is_refused():
    assert not (ROOT / "cmd/layerx-module-registry/node.c").exists()
    for args in (
        ["read-node"],
        [
            "read-node", "--network-id", "1", "--protocol-version", "3",
            "--socket", "/missing/registry.sock", "--actor", "did:layerx:test",
        ],
    ):
        result = run_registry(args)
        assert result.returncode == 1, result.stderr
        assert result.stdout == ""
        assert result.stderr == REFUSED


def test_socket_and_actor_flags_are_refused():
    generated = run_registry(GENERATE)
    assert generated.returncode == 0, generated.stderr
    assert generated.stderr == ""
    assert json.loads(generated.stdout)["schema_version"] == 2

    for flags in (
        ["--socket", "/missing/registry.sock"],
        ["--actor", "did:layerx:test"],
        ["--socket", "/missing/registry.sock", "--actor", "did:layerx:test"],
    ):
        result = run_registry([*GENERATE, *flags])
        assert result.returncode == 1, result.stderr
        assert result.stdout == ""
        assert result.stderr == REFUSED
