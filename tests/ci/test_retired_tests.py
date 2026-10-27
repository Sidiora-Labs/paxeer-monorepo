from pathlib import Path

import pytest

import retired_tests
from retired_tests import (
    assert_paths_absent,
    assert_symbols_unreferenced,
    build_and_run_c_suite,
)


def test_assert_paths_absent_names_present_file(tmp_path):
    (tmp_path / "old.c").write_text("int old;\n", encoding="utf-8")
    (tmp_path / "old_suite").mkdir()
    (tmp_path / "dangling").symlink_to("missing-target")

    with pytest.raises(AssertionError) as failure:
        assert_paths_absent(
            ["gone.c", Path("old.c"), "old_suite", "dangling"], root=tmp_path
        )

    assert str(failure.value).splitlines() == [
        "Retired paths still exist:", "old.c", "old_suite", "dangling"
    ]
    assert_paths_absent(["gone.c", "old_suite/gone.c"], root=tmp_path)


def test_assert_symbols_unreferenced_names_live_caller(tmp_path):
    sources = tmp_path / "sources"
    sources.mkdir()
    suffixes = (".c", ".h", ".py", ".sh", ".sol", ".go")
    for suffix in suffixes:
        (sources / ("caller" + suffix)).write_text(
            "retired_call_suffix _retired_call xretired_call retired_call9\n"
            "retired_call(); retired_call();\n"
            "retired.value\n",
            encoding="utf-8",
        )
    (sources / "test_retired_example.py").write_text(
        "retired_call retired.value assert_paths_absent\n", encoding="utf-8"
    )
    (sources / "ignored.txt").write_text("retired_call\n", encoding="utf-8")
    clean = sources / "clean.py"
    clean.write_text("retired_call_suffix retiredXvalue\n", encoding="utf-8")

    with pytest.raises(AssertionError) as failure:
        assert_symbols_unreferenced(
            ["retired_call", "retired.value"], ["sources", "sources/caller.c"],
            root=tmp_path,
        )

    lines = str(failure.value).splitlines()
    assert lines[0] == "Retired symbols still referenced:"
    assert len(lines[1:]) == 18
    for suffix in suffixes:
        name = "sources/caller" + suffix
        assert f"{name}:2: retired_call (column 1)" in lines
        assert f"{name}:2: retired_call (column 17)" in lines
        assert f"{name}:3: retired.value (column 1)" in lines

    assert_symbols_unreferenced(
        ["retired_call", "retired.value", "assert_paths_absent"],
        [
            clean, sources / "test_retired_example.py", sources / "ignored.txt",
            Path(retired_tests.__file__),
        ],
        root=tmp_path,
    )


@pytest.mark.parametrize("excluded_root", ["sources/vendor", Path("sources/vendor")])
def test_assert_symbols_unreferenced_skips_excluded_roots(tmp_path, excluded_root):
    excluded = tmp_path / "sources/vendor/nested"
    excluded.mkdir(parents=True)
    excluded_file = excluded / "caller.c"
    excluded_file.write_text("retired_call();\n", encoding="utf-8")
    outside_file = tmp_path / "sources/vendor_live.c"
    outside_file.write_text("retired_call();\n", encoding="utf-8")

    with pytest.raises(AssertionError) as failure:
        assert_symbols_unreferenced(["retired_call"], ["sources"], root=tmp_path)

    assert str(failure.value).splitlines() == [
        "Retired symbols still referenced:",
        "sources/vendor/nested/caller.c:1: retired_call (column 1)",
        "sources/vendor_live.c:1: retired_call (column 1)",
    ]

    assert_symbols_unreferenced(
        ["retired_call"], ["sources/vendor", excluded_file], root=tmp_path,
        exclude=(excluded_root,),
    )
    with pytest.raises(AssertionError) as failure:
        assert_symbols_unreferenced(
            ["retired_call"], ["sources", excluded_file], root=tmp_path,
            exclude=(excluded_root,),
        )

    assert str(failure.value).splitlines() == [
        "Retired symbols still referenced:",
        "sources/vendor_live.c:1: retired_call (column 1)",
    ]


def test_build_and_run_c_suite_reports_exit_code(tmp_path):
    (tmp_path / "Makefile").write_text(
        "suite: suite.c\n\t$(CC) suite.c -o suite\n", encoding="utf-8"
    )
    (tmp_path / "suite.c").write_text(
        '#include <stdio.h>\n'
        'int main(void) {\n'
        '    for (int i = 0; i < 25; ++i) fprintf(stderr, "binary-%02d\\n", i);\n'
        '    return 3;\n'
        '}\n',
        encoding="utf-8",
    )

    with pytest.raises(AssertionError) as failure:
        build_and_run_c_suite("suite", root=tmp_path)

    lines = str(failure.value).splitlines()
    assert lines[0] == f"{tmp_path / 'suite'} exited with code 3:"
    assert lines[1:] == [f"binary-{i:02d}" for i in range(5, 25)]

    make_failure = tmp_path / "make_failure"
    make_failure.mkdir()
    (make_failure / "Makefile").write_text(
        'suite:\n\t@i=0; while [ $$i -lt 25 ]; do '
        'echo "make-$$i" >&2; i=$$((i + 1)); done; exit 7\n',
        encoding="utf-8",
    )
    with pytest.raises(AssertionError) as failure:
        build_and_run_c_suite("suite", root=make_failure)

    lines = str(failure.value).splitlines()
    assert lines[0] == f"make -C {make_failure} suite exited with code 2:"
    assert lines[1:-1] == [f"make-{i}" for i in range(6, 25)]
    assert "Error 7" in lines[-1]
    assert not (make_failure / "suite").exists()

    success = tmp_path / "success"
    success.mkdir()
    (success / "Makefile").write_text(
        "suite: suite.c\n\t$(CC) suite.c -o suite\n", encoding="utf-8"
    )
    (success / "suite.c").write_text(
        '#include <stdio.h>\n#include <string.h>\n'
        'int main(int argc, char **argv) {\n'
        '    if (argc != 2 || strcmp(argv[1], "--run")) return 4;\n'
        '    FILE *result = fopen("ran", "w");\n'
        '    if (!result) return 5;\n'
        '    return fclose(result);\n'
        '}\n',
        encoding="utf-8",
    )
    build_and_run_c_suite("suite", args=("--run",), root=str(success))
    assert (success / "ran").is_file()
