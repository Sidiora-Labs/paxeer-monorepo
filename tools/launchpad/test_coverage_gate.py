import os
from pathlib import Path
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[2]
GATE = ROOT / "tools/launchpad/coverage-gate.sh"
MAKEFILE = ROOT / "services/launchpad/Makefile"
SHARED = "../../sdk/launchpad/shared"
PROTOCOL = "../../sdk/launchpad/protocol"


def write_module(directory, covered):
    directory.mkdir(parents=True)
    (directory / "go.mod").write_text(
        f"module example.com/coverage/{directory.name}\n\ngo 1.20\n"
    )
    (directory / "values.go").write_text(
        "package coverage\n\n"
        + "".join(f"func Value{i}() int {{ return {i} }}\n" for i in range(20))
    )
    functions = ", ".join(f"Value{i}" for i in range(covered))
    (directory / "values_test.go").write_text(
        'package coverage\n\nimport "testing"\n\n'
        "func TestValues(t *testing.T) {\n"
        f"\tfunctions := []func() int{{{functions}}}\n"
        "\tfor i, value := range functions {\n"
        '\t\tif got := value(); got != i { t.Fatalf("got %d, want %d", got, i) }\n'
        "\t}\n"
        "}\n"
    )


@pytest.mark.parametrize(
    ("api", "shared", "protocol", "failed"),
    [
        (17, 18, 18, None),
        (20, 17, 18, SHARED),
        (20, 18, 17, PROTOCOL),
        (16, 18, 18, "api"),
        (None, 18, 18, None),
    ],
    ids=["thresholds-pass", "shared-fails", "protocol-fails", "service-fails", "sdk-only"],
)
def test_make_cover_check_includes_moved_sdk_profiles(tmp_path, api, shared, protocol, failed):
    service = tmp_path / "services/launchpad"
    service.mkdir(parents=True)
    gate = tmp_path / "tools/launchpad/coverage-gate.sh"
    gate.parent.mkdir(parents=True)
    gate.symlink_to(GATE)
    modules = {SHARED: shared, PROTOCOL: protocol}
    if api is not None:
        modules["api"] = api
    for module, covered in modules.items():
        write_module((service / module).resolve(), covered)
    workspace = service / "go.work"
    workspace.write_text("go 1.20\n\nuse (\n" + "".join(f"\t./{m}\n" for m in modules) + ")\n")
    env = os.environ.copy()
    for variable in ("COVER_DIR", "DEFAULT_MIN", "MONEY_MIN", "MONEY", "MAKEFLAGS", "GOFLAGS"):
        env.pop(variable, None)
    env.update(GOWORK=str(workspace), GOTOOLCHAIN="local", GOPROXY="off", GOSUMDB="off")
    result = subprocess.run(
        ["make", "--no-print-directory", "-f", str(MAKEFILE), "cover-check", f"MODULES={' '.join(modules)}"],
        cwd=service,
        env=env,
        capture_output=True,
        text=True,
        timeout=180,
    )
    print(result.stdout + result.stderr)
    assert (result.returncode == 0) == (failed is None), result.stdout + result.stderr
    assert "SKIP" not in result.stdout
    for module, covered in modules.items():
        profile = service / ".cover" / (module.replace("/", "_") + ".out")
        assert profile.is_file()
        assert len(profile.read_text().splitlines()) > 1
        minimum = "85.0%" if module == "api" else "90.0%"
        status = "FAIL" if module == failed else "PASS"
        assert [module, f"{covered * 5:.1f}%", minimum, status] in [
            line.split() for line in result.stdout.splitlines()
        ]
    if failed is None:
        assert "coverage-gate: PASSED" in result.stdout
    else:
        assert "coverage-gate: FAILED" in result.stderr
        assert "coverage-gate: PASSED" not in result.stdout


@pytest.mark.parametrize("contents", ["missing", "empty", "unrelated"])
def test_coverage_gate_rejects_missing_profiles(tmp_path, contents):
    cover = tmp_path / ".cover"
    if contents != "missing":
        cover.mkdir()
    if contents == "unrelated":
        (cover / ".not-a-profile.txt").write_text("not coverage\n")
    result = subprocess.run(
        ["bash", str(GATE)],
        cwd=tmp_path,
        env={**os.environ, "COVER_DIR": str(cover)},
        capture_output=True,
        text=True,
        timeout=10,
    )
    print(result.stdout + result.stderr)
    assert result.returncode == 1
    expected = "directory (run 'make cover' first)" if contents == "missing" else "no coverage profiles found"
    assert expected in result.stderr
    assert "coverage-gate: PASSED" not in result.stdout
