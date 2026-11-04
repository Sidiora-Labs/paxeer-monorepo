"""Exercise production Bash selection and check its guarded dotenv consumer.

Only Bash builtins run: no Playwright, Yarn, dotenv, Node or browser is needed.
"""

from pathlib import Path
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "apps/explorer/tools/scripts/pw.sh"
BASE_COMMAND = ["playwright", "test", "-c", "playwright-ct.config.ts"]
SELECTION = [
    "ui/addresses/AddressesListItem.spec.tsx",
    "ui/selection with spaces/[literal]*.spec.tsx",
    r"ui/selection\with\backslashes.spec.tsx",
]
CALLER_ARGS = [
    "--project=chromium",
    "--grep",
    "address list | token transfer",
    "ui/requested test.spec.tsx",
    "",
    "*.spec.tsx",
    "--affectedness=false",
    "$(touch should-not-exist)",
]


def selected_invocation(tmp_path, flags, content, arguments=CALLER_ARGS):
    directory = tmp_path / "playwright"
    directory.mkdir()
    if content is not None:
        (directory / "affected-tests.txt").write_text(content, encoding="utf-8")
    # A matching pathname makes accidental unquoted glob expansion observable.
    (tmp_path / "expanded.spec.tsx").write_text("", encoding="utf-8")
    result = subprocess.run(
        [
            "bash",
            "--noprofile",
            "--norc",
            "-c",
            'source "$1"; shift; '
            'select_playwright_command "$@"; status=$?; '
            'if [ "$status" -ne 0 ]; then exit "$status"; fi; '
            'printf "%s\\0" "${playwright_command[@]}"',
            "selection-proof",
            str(SCRIPT),
            *flags,
            *arguments,
        ],
        cwd=tmp_path,
        capture_output=True,
        timeout=5,
        check=False,
    )
    assert result.stderr == b"", result.stderr.decode()
    assert not (tmp_path / "should-not-exist").exists()
    return result


@pytest.mark.parametrize(
    "flags,affected",
    [
        ([], False),
        (["--affected"], True),
        (["--affected=true"], True),
        (["--affected=false"], False),
    ],
    ids=["absent", "bare", "true", "false"],
)
@pytest.mark.parametrize("selection_state", ["populated", "empty", "missing"])
def test_playwright_affected_flags_select_exact_invocation(
    tmp_path, flags, affected, selection_state
):
    content = {
        "populated": "\n".join(SELECTION) + "\n",
        "empty": "",
        "missing": None,
    }[selection_state]
    result = selected_invocation(tmp_path, flags, content)
    if affected and selection_state == "empty":
        assert result.returncode == 1
        assert result.stdout == b""
    else:
        assert result.returncode == 0
        selected = SELECTION if affected and selection_state == "populated" else []
        assert result.stdout.split(b"\0") == [
            word.encode() for word in BASE_COMMAND + selected + CALLER_ARGS
        ] + [b""]


@pytest.mark.parametrize(
    "content",
    ["\n\n", "\n".join(SELECTION), "\n" + "\n\n".join(SELECTION) + "\n\n"],
)
def test_playwright_selection_reads_complete_lines(tmp_path, content):
    result = selected_invocation(tmp_path, ["--affected=true"], content, [])
    if content == "\n\n":
        assert result.returncode == 1
        assert result.stdout == b""
    else:
        assert result.returncode == 0
        assert result.stdout.split(b"\0") == [
            word.encode() for word in BASE_COMMAND + SELECTION
        ] + [b""]


@pytest.mark.parametrize(
    "flags,affected",
    [
        (["--affected=false", "--affected=true"], False),
        (["--affected=true", "--affected=false"], True),
    ],
)
def test_playwright_first_flag_retains_precedence(tmp_path, flags, affected):
    result = selected_invocation(tmp_path, flags, "\n".join(SELECTION), [])
    assert result.returncode == 0
    expected = BASE_COMMAND + (SELECTION if affected else [])
    assert result.stdout.split(b"\0") == [word.encode() for word in expected] + [b""]


def test_playwright_shell_syntax_and_guarded_consumer():
    syntax = subprocess.run(
        ["bash", "-n", str(SCRIPT)], capture_output=True, timeout=5, check=False
    )
    assert syntax.returncode == 0, syntax.stderr.decode()
    source = SCRIPT.read_text(encoding="utf-8")
    assert 'if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then\n    main "$@"\nfi' in source
    assert 'if ! select_playwright_command "$@"; then\n' in source
    guard = source.split('if ! select_playwright_command "$@"; then\n', 1)[1]
    skipped, invocation = guard.split("    fi\n", 1)
    assert 'echo "No affected tests found in the file. Exiting..."' in skipped
    assert "return 0" in skipped
    assert '"${playwright_command[@]}"' in invocation
    dotenv = invocation.split("    dotenv \\\n", 1)[1].split("\n}", 1)[0]
    assert shlex.split(dotenv.replace("\\\n", "")) == [
        "-v",
        'NODE_OPTIONS="--max-old-space-size=8192"',
        "-e",
        "$config_file",
        "--",
        "${playwright_command[@]}",
    ]
    assert '      -- "${playwright_command[@]}"' in dotenv


def test_playwright_repair_record_names_finding_and_proof():
    record = tomllib.loads(
        Path(__file__).with_name("repair_22_33_388.kvx").read_text(encoding="utf-8")
    )
    finding = record["finding"]["playwright-affected-false"]
    assert finding["file"] == SCRIPT.relative_to(ROOT).as_posix()
    assert finding["tests"] == [
        "test_playwright_affected_flags_select_exact_invocation",
        "test_playwright_selection_reads_complete_lines",
        "test_playwright_first_flag_retains_precedence",
        "test_playwright_shell_syntax_and_guarded_consumer",
    ]
    task = record["task"]["22"]["33"]["388"]
    assert task["wiring_requests"] == []
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
