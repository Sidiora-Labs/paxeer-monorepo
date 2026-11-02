import hashlib
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


def test_custody_profile_flag_is_refused(tmp_path):
    reserve = b"system:paxeer-reserve"
    contract = bytes.fromhex("11" * 20)
    profile = tmp_path / "profile"
    profile.write_bytes(
        b"LXBC3"
        + (125).to_bytes(8, "big")
        + contract
        + hashlib.sha256(b"LX:CUSTODY:MODULE:v1layerxcustody" + contract).digest()
        + bytes.fromhex("22" * 32)
        + bytes.fromhex(GENERATE[GENERATE.index("--asset") + 1])
        + hashlib.sha256(
            b"LX:ACCOUNT:v1" + len(reserve).to_bytes(4, "big") + reserve
        ).digest()
        + (1).to_bytes(8, "big")
        + b"paxeer".ljust(32, b"\0")
        + (1).to_bytes(4, "big")
        + (3).to_bytes(2, "big")
        + (1).to_bytes(8, "big")
        + (1).to_bytes(8, "big")
    )
    for path in (profile, tmp_path / "missing"):
        flags = ["--custody-profile", str(path)]
        for args in ([*GENERATE, *flags], ["generate", *flags, *GENERATE[1:]]):
            result = run_registry(args)
            assert result.returncode == 1, result.stderr
            assert result.stdout == ""
            assert result.stderr == REFUSED


def test_generate_plan_has_no_bridge_module():
    optional_modules = ("budget", "escrow", "perps", "service", "spot", "stream")
    enabled_flags = [
        argument
        for module in optional_modules
        for argument in ("--enable-module", module)
    ]
    for flags, expected_ids in (
        ([], [1, 7, 9]),
        (enabled_flags, [1, 2, 3, 4, 5, 6, 7, 9, 10]),
    ):
        result = run_registry([*GENERATE, *flags])
        assert result.returncode == 0, result.stderr
        assert result.stderr == ""
        plan = json.loads(result.stdout)
        assert plan["schema_version"] == 2
        module_ids = [module["module"] for module in plan["modules"]]
        assert 8 not in module_ids
        assert module_ids == expected_ids
        assert all(module["ordinals"] for module in plan["modules"])
