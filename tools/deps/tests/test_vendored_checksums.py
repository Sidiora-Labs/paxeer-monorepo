import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[3]


def test_vendored_database_checksums():
    modules = {
        ROOT / "core/deps/db-backend": {
            "google.golang.org/grpc",
            "google.golang.org/grpc/credentials",
        },
        ROOT / "core/deps/tm-db": {
            "github.com/syndtr/goleveldb/leveldb",
            "github.com/tendermint/tm-db",
        },
    }
    before = {}
    for module in modules:
        checksum = module / "go.sum"
        assert checksum.is_file(), f"Missing checksum file: {checksum}"
        assert checksum.read_bytes().strip(), f"Empty checksum file: {checksum}"
        for name in ("go.mod", "go.sum"):
            path = module / name
            before[path] = path.read_bytes()

    env = {**os.environ, "GOWORK": "off", "GOFLAGS": "-mod=readonly"}
    try:
        for module, required_imports in modules.items():
            verified = subprocess.run(
                ["go", "mod", "verify"],
                cwd=module,
                env=env,
                capture_output=True,
                text=True,
                check=False,
                timeout=300,
            )
            assert verified.returncode == 0, (
                f"go mod verify failed in {module}:\n"
                f"{verified.stdout}{verified.stderr}"
            )
            dependencies = subprocess.run(
                ["go", "list", "-mod=readonly", "-deps", "./..."],
                cwd=module,
                env=env,
                capture_output=True,
                text=True,
                check=False,
                timeout=300,
            )
            assert dependencies.returncode == 0, (
                f"go list failed in {module}:\n"
                f"{dependencies.stdout}{dependencies.stderr}"
            )
            packages = set(dependencies.stdout.splitlines())
            assert packages, f"No dependencies resolved in {module}"
            assert required_imports <= packages, (
                f"Unresolved imports in {module}: {required_imports - packages}"
            )

            built = subprocess.run(
                ["go", "build", "-mod=readonly", "./..."],
                cwd=module,
                env=env,
                capture_output=True,
                text=True,
                check=False,
                timeout=300,
            )
            assert built.returncode == 0, (
                f"go build failed in {module}:\n"
                f"{built.stdout}{built.stderr}"
            )
    finally:
        for path, original in before.items():
            assert path.read_bytes() == original, f"Go modified {path}"
