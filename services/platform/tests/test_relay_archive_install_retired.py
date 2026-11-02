from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
PLATFORM_ROOT = REPO_ROOT / "services/platform"


def test_relay_install_assets_absent():
    remaining = [
        str(path.relative_to(REPO_ROOT))
        for name in (
            "install.sh",
            "config.example.json",
            "deployment.yaml",
            "deployment.example.yaml",
            "layerx-relay-archive.service",
            "railway.env.example",
        )
        if (path := PLATFORM_ROOT / "relay_archive" / name).exists()
        or path.is_symlink()
    ]
    assert not remaining, f"Retired relay installation assets remain: {remaining}"


def test_no_platform_file_reads_relay_archive_variables():
    prefix = b"_".join((b"LAYERX", b"RELAY", b"ARCHIVE", b""))
    violations = []
    for path in sorted(PLATFORM_ROOT.rglob("*")):
        relative_path = path.relative_to(PLATFORM_ROOT)
        if relative_path.parts[0] == "hosted" or not path.is_file():
            continue
        for line_number, line in enumerate(path.read_bytes().splitlines(), start=1):
            if prefix in line:
                violations.append(f"{path.relative_to(REPO_ROOT)}:{line_number}")
    assert not violations, "Retired relay environment variables remain:\n" + "\n".join(
        violations
    )
