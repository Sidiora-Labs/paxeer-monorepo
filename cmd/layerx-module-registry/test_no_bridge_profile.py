import hashlib
import json
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
BINARY = ROOT / "build/bin/layerx-module-registry"


def test_registry_refuses_custody_profile(tmp_path):
    asset = "b5a32b12029f8ddfb905f90f280f664b46390de0fc62770fc197dd87b18cd898"
    generate = [
        "generate",
        "--network-id", "1",
        "--protocol-version", "3",
        "--asset", asset,
        "--symbol", "LXT",
        "--currency", "LXT",
        "--decimals", "18",
    ]
    generated = subprocess.run(
        [str(BINARY), *generate], capture_output=True, text=True, timeout=15
    )
    assert generated.returncode == 0, generated.stderr
    assert generated.stderr == ""
    plan = json.loads(generated.stdout)
    assert plan["schema_version"] == 2
    assert plan["assets"] == [
        {"asset": asset, "symbol": "LXT", "currency": "LXT", "decimals": 18}
    ]
    assert [module["module"] for module in plan["modules"]] == [1, 7, 9]

    reserve = b"system:paxeer-reserve"
    contract = bytes.fromhex("11" * 20)
    profile = tmp_path / "profile"
    profile.write_bytes(
        b"LXBC3"
        + (125).to_bytes(8, "big")
        + contract
        + hashlib.sha256(b"LX:CUSTODY:MODULE:v1layerxcustody" + contract).digest()
        + bytes.fromhex("22" * 32)
        + bytes.fromhex(asset)
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
        for args in ([*generate, *flags], ["generate", *flags, *generate[1:]]):
            result = subprocess.run(
                [str(BINARY), *args], capture_output=True, text=True, timeout=15
            )
            assert result.returncode == 1, result.stderr
            assert result.stdout == ""
            assert result.stderr == (
                "layerx-module-registry: refused: "
                "invalid input or unavailable canonical registry\n"
            )
