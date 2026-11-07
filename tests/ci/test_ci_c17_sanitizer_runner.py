import pathlib
import re

import yaml


ROOT = pathlib.Path(__file__).resolve().parents[2]


def test_c17_job_runs_on_hosted_ubuntu():
    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")
    )
    assert workflow["jobs"]["c17"]["runs-on"] == "ubuntu-24.04"


def test_asan_rules_unset_ld_preload():
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    recipes = [
        (number, line)
        for number, line in enumerate(makefile.splitlines(), start=1)
        if not line.lstrip().startswith("#")
        and re.search(r"\bASAN_OPTIONS=", line)
    ]
    assert len(recipes) >= 5, "Expected all five existing ASan test invocations"
    for number, line in recipes:
        assert re.match(r"^\t[@+-]*env -u LD_PRELOAD\s+ASAN_OPTIONS=", line), (
            f"Makefile:{number}: ASan invocation must unset LD_PRELOAD: {line}"
        )
