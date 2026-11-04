import os
from pathlib import Path
import subprocess


def test_go_root_nested_checksums():
    root = Path(__file__).resolve().parents[3]
    modules = (
        root / "tools/paxctl",
        root / "tools/loadtest",
        root / "services/wallet/ceremony",
        root / "sdk/cosmovisor",
    )
    snapshots = {}
    for module in modules:
        checksum = module / "go.sum"
        assert checksum.is_file(), f"{checksum} is missing"
        assert checksum.read_bytes().strip(), f"{checksum} is empty"
        for name in ("go.mod", "go.sum"):
            path = module / name
            snapshots[path] = path.read_bytes()

    env = dict(
        os.environ, GOWORK="off", GOFLAGS="-mod=readonly", GOTOOLCHAIN="auto"
    )
    try:
        for module in modules:
            packages = subprocess.run(
                ["go", "list", "-mod=readonly", "-deps", "-test", "./..."],
                cwd=module,
                env=env,
                capture_output=True,
                text=True,
                check=False,
            )
            assert packages.returncode == 0, (
                f"{module}: go list exited {packages.returncode}\n"
                f"{packages.stdout}\n{packages.stderr}"
            )
            assert packages.stdout.strip(), f"{module}: go list returned no packages"

            verified = subprocess.run(
                ["go", "mod", "verify"],
                cwd=module,
                env=env,
                capture_output=True,
                text=True,
                check=False,
            )
            assert verified.returncode == 0, (
                f"{module}: go mod verify exited {verified.returncode}\n"
                f"{verified.stdout}\n{verified.stderr}"
            )

            built = subprocess.run(
                ["go", "build", "-mod=readonly", "./..."],
                cwd=module,
                env=env,
                capture_output=True,
                text=True,
                check=False,
            )
            assert built.returncode == 0, (
                f"{module}: go build exited {built.returncode}\n"
                f"{built.stdout}\n{built.stderr}"
            )
    finally:
        for path, snapshot in snapshots.items():
            assert path.read_bytes() == snapshot, f"{path} changed during verification"
