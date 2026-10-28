import re
import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
ARCHIVES = (
    "build/liblayerx.a",
    "programs/target/release/liblayerx_programs_sandbox.a",
)


@pytest.mark.parametrize("present", [(), (ARCHIVES[0],), (ARCHIVES[1],)])
def test_check_names_missing_archive(tmp_path, present):
    script = tmp_path / "scripts/check-paxd-native-digest.sh"
    script.parent.mkdir()
    shutil.copy2(ROOT / "scripts/check-paxd-native-digest.sh", script)
    for archive in present:
        path = tmp_path / archive
        path.parent.mkdir(parents=True)
        path.write_bytes(b"!<arch>\n")

    result = subprocess.run(
        ["sh", str(script)],
        cwd=tmp_path.parent,
        capture_output=True,
        text=True,
        check=False,
    )

    assert result.returncode == 1, result.stdout + result.stderr
    assert result.stdout == ""
    assert result.stderr.splitlines() == [
        f"paxd-native-digest-check: missing native archive: {archive}"
        for archive in ARCHIVES
        if archive not in present
    ]


def test_ci_and_image_stamp_native_digest():
    workflow = (ROOT / ".github/workflows/node-build.yml").read_text()
    paxd_job = re.search(r"^  paxd:\n.*?(?=^  [\w-]+:|\Z)", workflow, re.M | re.S)
    assert paxd_job is not None
    assert re.search(
        r"      - name: Check paxd version\n"
        r"        run: make paxd-version-check\n"
        r"      - name: [^\n]+\n"
        r"        run: make -f chain\.mk paxd-native-digest-check\n",
        paxd_job.group(),
    )

    dockerfile = (ROOT / "docker/layerx/Dockerfile").read_text()
    builder = dockerfile.split(" AS builder\n", 1)[1].split("\nFROM ", 1)[0]
    build_run = builder.rsplit("\nRUN ", 1)[1].replace("\\\n", "")
    assert (
        'VERSION_PKG="github.com/Sidiora-Labs/Paxeer-X-Network/sdk/version"'
        in build_run
    )
    ldflags = re.search(r'LDFLAGS="(.*?)"\s*&&', build_run, re.S)
    assert ldflags is not None
    assert (
        "-X ${VERSION_PKG}.NativeArchiveDigest=$(cat "
        "build/liblayerx.a programs/target/release/liblayerx_programs_sandbox.a "
        "| sha256sum | cut -d' ' -f1)"
    ) in ldflags.group(1)
    assert re.search(
        r'go build .*?-ldflags "\$\{LDFLAGS\}" .*?-o /go/bin/paxd ./daemon/paxd',
        build_run,
    )
    assert "COPY --from=builder /go/bin/paxd /usr/bin/" in dockerfile
