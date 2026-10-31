import re

from retired_tests import REPO_ROOT, assert_paths_absent


def _assert_no_test_references(pattern):
    sources = sorted(
        path
        for path in (REPO_ROOT / "tests").rglob("*")
        if path.is_file()
        and (REPO_ROOT / "tests" / "ci") not in path.parents
    )
    assert sources, "No test sources found"
    references = [
        f"{path.relative_to(REPO_ROOT)}:{number}: {line.strip()}"
        for path in sources
        for number, line in enumerate(
            path.read_text(encoding="utf-8", errors="surrogateescape").splitlines(),
            start=1,
        )
        if pattern.search(line)
    ]
    assert not references, "Retired test references remain:\n" + "\n".join(references)


def test_arbiter_native_authority_absent():
    assert_paths_absent(["tests/daemon/lxp_test_arbiter_native_authority.c"])
    _assert_no_test_references(re.compile(r"\blxp_verified_receipt_authority_facts\b"))


def test_web_intake_absent():
    assert_paths_absent(["tests/modules/test_web_intake.c"])
    _assert_no_test_references(re.compile(r"\bweb_intake\b"))


def test_layerxd_suite_absent():
    assert_paths_absent(["tests/test_layerxd.c"])
    _assert_no_test_references(re.compile(r"\btest_layerxd\b"))
