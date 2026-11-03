from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]


def test_relay_archive_directory_absent():
    archive_directory = REPO_ROOT / "services/platform/relay_archive"
    assert not archive_directory.exists() and not archive_directory.is_symlink(), (
        "Retired relay archive directory remains: services/platform/relay_archive"
    )
