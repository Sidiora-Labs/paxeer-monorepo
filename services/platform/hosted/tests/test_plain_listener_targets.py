import subprocess
from pathlib import Path


def test_make_targets_name_no_relay_archive():
    root = Path(__file__).resolve().parents[4]
    makefile = root / "services/platform/Makefile.inc"
    assert "tests/relay-archive" not in makefile.read_text()

    result = subprocess.run(
        ["make", "-n", "-f", "Makefile", "platform-test-plain-listeners"],
        cwd=root,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "tests/relay-archive" not in result.stdout
    for package in ("layerx-reference-ramp", "layerx-interop-service"):
        assert any(
            f"-p {package} --test plain_listener" in line
            for line in result.stdout.splitlines()
        ), result.stdout
