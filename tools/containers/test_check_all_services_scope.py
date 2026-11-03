from pathlib import Path
import re
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "tools/containers/check.sh"


def test_check_all_services_scope():
    result = subprocess.run(
        ["bash", str(SCRIPT)],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr


def test_every_tracked_dockerfile_has_scope():
    script = SCRIPT.read_text(encoding="utf-8")
    lists = dict(
        re.findall(
            r"^(SERVICES|OUT_OF_SCOPE)='\n(.*?)\n'$",
            script,
            re.MULTILINE | re.DOTALL,
        )
    )
    assert set(lists) == {"SERVICES", "OUT_OF_SCOPE"}
    placed = {row.split("|")[1] for row in lists["SERVICES"].splitlines()}
    out_of_scope = {
        row
        for row in lists["OUT_OF_SCOPE"].splitlines()
        if row and not row.lstrip().startswith("#")
    }
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    dockerfiles = {
        path
        for path in result.stdout.split("\0")
        if (
            re.match(r"^Dockerfile", Path(path).name)
            or Path(path).name == "dockerfile"
            or Path(path).name.endswith(".Dockerfile")
        )
        and not path.endswith(".dockerignore")
    }
    assert dockerfiles, "the tracked container inventory must not be empty"
    assert placed.isdisjoint(out_of_scope), "services must not also be out of scope"
    assert dockerfiles == placed | out_of_scope, (
        f"Tracked recipes without a scope: {sorted(dockerfiles - placed - out_of_scope)}; "
        f"Scope entries without a tracked recipe: {sorted((placed | out_of_scope) - dockerfiles)}"
    )


def check_copy_inputs(tmp_path, instructions):
    recipe = tmp_path / "inputs.Dockerfile"
    recipe.write_text(instructions, encoding="utf-8")
    function = re.search(
        r"^copy_inputs\(\) \{\n.*?^\}",
        SCRIPT.read_text(encoding="utf-8"),
        re.MULTILINE | re.DOTALL,
    )
    assert function is not None, "check.sh must declare copy_inputs"
    return subprocess.run(
        [
            "bash",
            "-c",
            function.group(0) + '\ncopy_inputs "$1" "$2"',
            "copy-inputs-test",
            str(recipe),
            str(tmp_path),
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )


@pytest.mark.parametrize(
    "copy",
    [
        "COPY artifacts/program /usr/local/bin/program",
        'COPY --chmod=0555 ["artifacts/program", "/usr/local/bin/program"]',
        "ADD artifacts/program /usr/local/bin/program",
        "COPY artifacts/first artifacts/second /usr/local/bin/",
        "COPY artifacts/first \\\n    artifacts/second /usr/local/bin/",
    ],
)
def test_pinned_prebuilt_source_is_accepted(tmp_path, copy):
    result = check_copy_inputs(
        tmp_path, f"FROM scratch AS program-prebuilt\nARG PROGRAM_SHA256\n{copy}\n"
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "COPY inputs present" in result.stdout
    assert not (tmp_path / "artifacts").exists()


@pytest.mark.parametrize("instruction", ["COPY", "ADD"])
@pytest.mark.parametrize(
    "prefix",
    [
        "FROM scratch AS program-prebuilt\n",
        "ARG PROGRAM_SHA256\nFROM scratch AS program-prebuilt\n",
        "FROM scratch AS earlier-prebuilt\nARG PROGRAM_SHA256\n"
        "FROM scratch AS program-prebuilt\n",
        "FROM scratch AS program-prebuilt\nARG PROGRAM_SHA256_SUFFIX\n",
    ],
)
def test_unpinned_prebuilt_source_is_refused(tmp_path, instruction, prefix):
    result = check_copy_inputs(
        tmp_path, prefix + f"{instruction} artifacts/program /usr/local/bin/program\n"
    )
    assert result.returncode != 0, result.stdout + result.stderr
    assert "stage-local _SHA256 ARG" in result.stderr


@pytest.mark.parametrize("instruction", ["COPY", "ADD"])
@pytest.mark.parametrize("stage", ["runtime", "prebuilt", "program-prebuilt-extra"])
def test_missing_source_outside_prebuilt_stage_is_refused(tmp_path, instruction, stage):
    result = check_copy_inputs(
        tmp_path,
        f"FROM scratch AS {stage}\nARG PROGRAM_SHA256\n"
        f"{instruction} artifacts/program /usr/local/bin/program\n",
    )
    assert result.returncode != 0, result.stdout + result.stderr
    assert "-prebuilt stage" in result.stderr


def test_prebuilt_pin_does_not_exempt_other_missing_sources(tmp_path):
    result = check_copy_inputs(
        tmp_path,
        "FROM scratch AS program-prebuilt\nARG PROGRAM_SHA256\n"
        "COPY missing-program /usr/local/bin/program\n",
    )
    assert result.returncode != 0, result.stdout + result.stderr
    assert "missing COPY/ADD input: missing-program" in result.stderr


@pytest.mark.parametrize("source", ["../artifacts/program", "/artifacts/program"])
def test_prebuilt_pin_does_not_exempt_invalid_sources(tmp_path, source):
    result = check_copy_inputs(
        tmp_path,
        "FROM scratch AS program-prebuilt\nARG PROGRAM_SHA256\n"
        f"COPY {source} /usr/local/bin/program\n",
    )
    assert result.returncode != 0, result.stdout + result.stderr
    assert "cannot be resolved in the declared context" in result.stderr


def test_webhooks_prebuilt_stage_keeps_the_published_target():
    recipe = (ROOT / "docker/platform-webhooks/Dockerfile").read_text(encoding="utf-8")
    stage = re.search(
        r"^FROM .* AS webhooks-prebuilt\n(.*?)(?=^FROM )", recipe, re.M | re.S
    )
    assert stage is not None
    assert re.search(r"^ARG WEBHOOKS_SHA256$", stage.group(1), re.M)
    assert (
        "COPY artifacts/layerx-webhooks /usr/local/bin/layerx-webhooks" in stage.group(1)
    )
    assert "FROM webhooks-prebuilt AS env-init-prebuilt" in recipe
