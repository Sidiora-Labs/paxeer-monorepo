"""Exercise the builder's test retry with Mix failure records."""

import json
import os
import subprocess
import sys

from test_mix_builder_redis import SCRIPT, SOURCE, shell_function


MIX_STAND_IN = r'''
import json
import os
from pathlib import Path
import subprocess
import sys

state = Path(os.environ["MIX_CALLS"])
calls = json.loads(state.read_text()) if state.exists() else []
calls.append({"args": sys.argv[1:], "database": os.environ["PGDATABASE"], "cwd": os.getcwd()})
state.write_text(json.dumps(calls))
if len(calls) == 1 and os.environ["RECORD"] != "none":
    manifest = Path(os.environ["MIX_BUILD_PATH"]) / "test/lib/block_scout_web/.mix/.mix_test_failures"
    manifest.parent.mkdir(parents=True, exist_ok=True)
    test_file = Path("retry_case.exs")
    test_file.write_text(r"""
      [path, record] = System.argv()
      if record == "all" do
        ExUnit.Filters.fail_all!(path)
      else
        ExUnit.start(autorun: false, formatters: [], failures_manifest_path: path)
        defmodule BuilderRetryCase do
          use ExUnit.Case
          test "records the result" do
            assert System.fetch_env!("RECORD") == "empty"
          end
        end
        ExUnit.run()
      end
    """)
    subprocess.run(["elixir", str(test_file), str(manifest), os.environ["RECORD"]], check=True)
sys.exit(int(os.environ["FIRST_EXIT" if len(calls) == 1 else "SECOND_EXIT"]))
'''


def run_with_mix(tmp_path, first_exit, second_exit, record, retry=True, stale_record=False):
    executable = tmp_path / "mix"
    executable.write_text(f"#!{sys.executable}\n" + MIX_STAND_IN)
    executable.chmod(0o755)
    calls = tmp_path / "calls.json"
    build_path = tmp_path / "build with spaces"
    build_path.mkdir()
    functions = "\n".join(
        f"{name}() {{\n{shell_function(name)}\n}}"
        for name in ("failed_tests_recorded", "run_mix_with_retry")
    )
    args = ["test", "apps/block_scout_web/test/path with spaces_test.exs", "--seed", "123"]
    env = dict(
        os.environ,
        PATH=f"{tmp_path}:{os.environ['PATH']}",
        MIX_BUILD_PATH=str(build_path),
        MIX_IN_BUILDER_RETRY_FAILED="1" if retry else "0",
        MIX_CALLS=str(calls),
        PGDATABASE="builder_tests",
        FIRST_EXIT=str(first_exit),
        SECOND_EXIT=str(second_exit),
        RECORD=record,
        ERL_FLAGS="+S 2:2",
    )
    if stale_record:
        subprocess.run(
            [str(executable), *args],
            env=dict(env, RECORD="failed", FIRST_EXIT="0"),
            cwd=tmp_path,
            capture_output=True,
            text=True,
            check=True,
        )
        calls.unlink()
        for manifest in build_path.rglob(".mix_test_failures"):
            os.utime(manifest, (1, 1))
    result = subprocess.run(
        ["sh", "-c", "set -e\n" + functions + '\nrun_mix_with_retry "$@"', "builder", *args],
        env=env,
        cwd=tmp_path,
        capture_output=True,
        text=True,
        check=False,
    )
    return result, json.loads(calls.read_text()), args


def test_passing_first_run_is_not_retried(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 0, 9, "none")
    assert result.returncode == 0, result.stderr
    assert [call["args"] for call in calls] == [args]
    assert "retrying" not in result.stderr


def test_recorded_failures_are_retried_once_with_original_arguments(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 2, 0, "failed")
    assert result.returncode == 0, result.stderr
    assert [call["args"] for call in calls] == [args, ["test", "--failed", *args[1:]]]
    assert calls[0]["database"] == calls[1]["database"] == "builder_tests"
    assert calls[0]["cwd"] == calls[1]["cwd"] == str(tmp_path)
    assert result.stderr.count("retrying failed tests once") == 1
    assert "retrying failed tests" not in result.stdout


def test_retry_failure_returns_its_exit_code_without_a_third_run(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 2, 7, "failed")
    assert result.returncode == 7, result.stderr
    assert [call["args"] for call in calls] == [args, ["test", "--failed", *args[1:]]]
    assert result.stderr.count("retrying failed tests once") == 1


def test_nonzero_exit_without_a_failure_record_keeps_first_exit_code(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 13, 0, "none")
    assert result.returncode == 13, result.stderr
    assert [call["args"] for call in calls] == [args]
    assert "retrying" not in result.stderr


def test_empty_failure_record_keeps_first_exit_code(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 3, 0, "empty")
    assert result.returncode == 3, result.stderr
    assert [call["args"] for call in calls] == [args]
    assert "retrying" not in result.stderr


def test_old_failure_record_does_not_retry_a_compile_error(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 1, 0, "none", stale_record=True)
    assert result.returncode == 1, result.stderr
    assert [call["args"] for call in calls] == [args]
    assert "retrying" not in result.stderr


def test_suite_load_error_without_named_tests_is_not_retried(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 1, 0, "all")
    assert result.returncode == 1, result.stderr
    assert [call["args"] for call in calls] == [args]
    assert "retrying" not in result.stderr


def test_without_option_failed_tests_are_not_retried(tmp_path):
    result, calls, args = run_with_mix(tmp_path, 2, 0, "failed", retry=False)
    assert result.returncode == 2, result.stderr
    assert [call["args"] for call in calls] == [args]
    assert "retrying" not in result.stderr


def test_retry_option_is_refused_before_running_other_mix_commands():
    for command in ("compile", "format", "run"):
        result = subprocess.run(
            ["sh", str(SCRIPT), "--retry-failed", "--check", command],
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 1
        assert "--retry-failed is only supported for the test command" in result.stderr


def test_retry_option_reaches_the_same_container_script_as_the_test_run():
    assert '--env MIX_IN_BUILDER_RETRY_FAILED="$retry_failed"' in shell_function("mix_in_container")
    script = SOURCE.split("cat <<'CONTAINER_SCRIPT_END'\n", 1)[1].split("\nCONTAINER_SCRIPT_END", 1)[0]
    assert 'run_mix_with_retry "$@"' in script
    assert "failed_tests_recorded()" in script
    assert "--retry-failed" in shell_function("usage")
