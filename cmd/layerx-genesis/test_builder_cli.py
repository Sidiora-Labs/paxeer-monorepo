import subprocess
from pathlib import Path


BUILDER = Path(__file__).resolve().parents[2] / "build/bin/layerx-genesis-build"


def test_custody_profile_argument_is_refused(tmp_path):
    result = subprocess.run(
        [BUILDER, "manifest", "key", "out", "--custody-profile", "profile"],
        cwd=tmp_path,
        capture_output=True,
        timeout=10,
        check=False,
    )

    assert result.returncode == 2
    assert result.stdout == b""
    assert result.stderr == b""
    assert list(tmp_path.iterdir()) == []


def test_custody_registry_argument_is_refused(tmp_path):
    output_directory = tmp_path / "out"
    output_directory.mkdir()
    result = subprocess.run(
        [BUILDER, "manifest", "key", "out", "--custody-registry", "registry"],
        cwd=tmp_path,
        capture_output=True,
        timeout=10,
        check=False,
    )

    assert result.returncode == 2
    assert result.stdout == b""
    assert result.stderr == b""
    assert list(output_directory.iterdir()) == []
    assert list(tmp_path.iterdir()) == [output_directory]
