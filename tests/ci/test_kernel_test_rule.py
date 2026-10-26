import pathlib
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[2]


def test_kernel_rule_builds_and_runs_codec_primitives():
    source = "tests/codec/lxp_test_codec_primitives.c"
    target = "build/tests/kernel/codec/lxp_test_codec_primitives"
    build = subprocess.run(
        ["make", "-W", source, target],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert build.returncode == 0, build.stdout + build.stderr
    recipe = build.stdout.replace("\\\n", " ")
    assert any(
        source in line.split() and f"-o {target}" in line
        for line in recipe.splitlines()
    ), build.stdout
    assert (ROOT / target).is_file()
    result = subprocess.run(
        [str(ROOT / target)],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
