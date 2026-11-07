import os
from pathlib import Path
import re
import shlex
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[2]
RUNTIME = ROOT / "wasm-runtime"


@pytest.fixture(scope="module")
def alpine_recipes():
    makefile = (RUNTIME / "Makefile").read_text()
    target = makefile.split("\nrelease-build-alpine:\n", 1)[1].split("\n\n", 1)[0]
    environment = os.environ.copy()
    for name in ("MAKEFLAGS", "MFLAGS", "MAKELEVEL"):
        environment.pop(name, None)
    result = subprocess.run(
        ["make", "-C", "wasm-runtime", "-n", "release-build-alpine"],
        cwd=ROOT,
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    return target.replace("\\\n", " "), result.stdout.replace("\\\n", " ")


def test_alpine_target_uses_tree_build_script_or_suffixed_name(alpine_recipes):
    for recipe in alpine_recipes:
        docker = next(line for line in recipe.splitlines() if "docker run " in line)
        arguments = shlex.split(docker.replace("$(shell pwd)", str(RUNTIME)))
        mounts = [arguments[i + 1] for i, value in enumerate(arguments) if value == "-v"]
        assert any(mount.endswith("/libwasmvm:/code") for mount in mounts)
        assert any(
            mount.endswith("/builders/guest/build_muslc.sh:/opt/build_muslc.sh:ro")
            for mount in mounts
        )
        assert arguments[-2:] == ["sh", "/opt/build_muslc.sh"]
        assert arguments[-3] in (
            "$(BUILDERS_PREFIX)-alpine",
            "cosmwasm/libwasmvm-builder:0103-alpine",
        )
        assert re.search(
            r"if \[ -f libwasmvm/artifacts/libwasmvm_muslc\.a \]; then\s+"
            r"cp libwasmvm/artifacts/libwasmvm_muslc\.a internal/api/libwasmvm_muslc\.a;\s+"
            r"else\s+"
            r"cp libwasmvm/artifacts/libwasmvm_muslc\.x86_64\.a "
            r"internal/api/libwasmvm_muslc\.a;\s+fi",
            recipe,
        ), recipe
    script = (RUNTIME / "builders/guest/build_muslc.sh").read_text()
    assert (
        "cp target/x86_64-unknown-linux-musl/release/examples/libwasmvmstatic.a "
        "artifacts/libwasmvm_muslc.x86_64.a"
    ) in script


def test_alpine_target_keeps_internal_api_archive_names(alpine_recipes):
    for recipe in alpine_recipes:
        copies = [
            shlex.split(command.strip())
            for command in re.findall(r"\bcp [^;\n]+", recipe)
        ]
        archive_copies = [args for args in copies if args[1].endswith(".a")]
        assert archive_copies == [
            [
                "cp",
                "libwasmvm/artifacts/libwasmvm_muslc.a",
                "internal/api/libwasmvm_muslc.a",
            ],
            [
                "cp",
                "libwasmvm/artifacts/libwasmvm_muslc.x86_64.a",
                "internal/api/libwasmvm_muslc.a",
            ],
            ["cp", "libwasmvm/artifacts/libwasmvm_muslc.aarch64.a", "internal/api"],
        ]
    script = (RUNTIME / "builders/guest/build_muslc.sh").read_text()
    assert (
        "cp target/aarch64-unknown-linux-musl/release/examples/libwasmvmstatic.a "
        "artifacts/libwasmvm_muslc.aarch64.a"
    ) in script
    checksums = (RUNTIME / "internal/api/static_archive_checksums_test.go").read_text()
    for name, digest in (
        (
            "libwasmvm_muslc.a",
            "2c2d90423e5bebba911b0be7963b09062adb555ecd6149f7cc759a494bb2eda6",
        ),
        (
            "libwasmvm_muslc.aarch64.a",
            "409cd9da695d4f1989271230094563c38aa2d867b26908b2ca2082a0d71e3b8e",
        ),
        (
            "libwasmvmstatic_darwin.a",
            "713d221d712bee043794fc600e093b62e9e878b312f044f548a8852b670451c8",
        ),
    ):
        assert f'{{"{name}", "{digest}"}}' in checksums
