import importlib.util
import json
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[5]
IMPORTER = ROOT / "services/wallet/ceremony/legacy-import"
SPEC = importlib.util.spec_from_file_location(
    "lockfloor", ROOT / "tools/deps/lockfloor.py"
)
LOCKFLOOR = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(LOCKFLOOR)


def test_legacy_import_lockfile():
    manifest_path = IMPORTER / "package.json"
    manifest = json.loads(manifest_path.read_text())
    assert manifest["devDependencies"]["vitest"] == "4.1.12"

    lock_path = IMPORTER / "pnpm-lock.yaml"
    assert lock_path.is_file(), "The standalone importer needs its own lockfile"
    assert LOCKFLOOR._lock_path(manifest_path) == lock_path
    original = lock_path.read_bytes()
    floors = json.loads(
        (ROOT / "tools/deps/floors/wallet-services.json").read_text()
    )["manifests"][lock_path.relative_to(ROOT).as_posix()]
    minimums = {
        "vitest": manifest["devDependencies"]["vitest"],
        "@vitest/mocker": "4.1.12",
    }
    versions = LOCKFLOOR._pnpm_lock(lock_path, set(minimums) | set(floors))
    for requirements in (minimums, floors):
        for package, floor in requirements.items():
            resolved = versions.get(package, [])
            assert resolved, f"{package} is unresolved in {lock_path}"
            for version in resolved:
                assert LOCKFLOOR.compare_versions(version, floor) >= 0, (
                    f"{package}@{version} is below {floor}"
                )

    result = subprocess.run(
        [
            "pnpm",
            "--dir",
            IMPORTER.relative_to(ROOT).as_posix(),
            "--ignore-workspace",
            "install",
            "--frozen-lockfile",
            "--ignore-scripts",
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=300,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert lock_path.read_bytes() == original, "Frozen install changed the lockfile"
