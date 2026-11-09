"""Exercise row generation and gate approval through real Git history."""

from pathlib import Path
import subprocess
import sys

from test_consensus_commit_gate import (
    HEADER,
    LIST_PATH,
    commit_file,
    git,
    init_repo,
    run_gate,
)


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "tools/ci/consensus_ledger_rows.py"
sys.path.insert(0, str(SCRIPT.parent))
from consensus_ledger_rows import escape_cell


def run_rows(repo, base, head, *options):
    return subprocess.run(
        [sys.executable, str(SCRIPT), base, head, *options],
        cwd=repo,
        capture_output=True,
        text=True,
        timeout=30,
    )


def test_rows_include_only_unlisted_non_test_consensus_changes(tmp_path):
    base = init_repo(tmp_path)
    path = "node/sequencer/consensus_source.go"
    listed = commit_file(tmp_path, path, "first consensus rule\n")
    ledger = HEADER + (
        f"| `{listed}` | Listed rule | `{path}` | `v7.0` (`MergeUpgrade`) |\n"
    )
    commit_file(tmp_path, LIST_PATH, ledger)

    paths = ("modules/perps/keeper/orders.go", "node/app.go")
    for changed in (*paths, "node/sequencer/source_test.go"):
        target = tmp_path / changed
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text("next rule\n", encoding="utf-8")
        git(tmp_path, "add", "--", changed)
    git(tmp_path, "commit", "-m", "Apply orders | enforce sequencer")
    unlisted = git(tmp_path, "rev-parse", "HEAD")
    head = commit_file(
        tmp_path, "modules/perps/keeper/orders_test.go", "test-only change\n"
    )

    refused = run_gate(tmp_path, base, head)
    assert refused.returncode == 1, refused.stdout + refused.stderr
    assert refused.stdout.splitlines() == [
        f"Unlisted consensus commit {unlisted}:",
        *(f"  {changed}" for changed in paths),
    ]
    result = run_rows(tmp_path, base, head)
    assert result.returncode == 0, result.stdout + result.stderr
    assert result.stderr == ""
    assert result.stdout.splitlines() == [
        f"| `{unlisted}` | Apply orders &#124; enforce sequencer | "
        "`modules/perps/keeper/orders.go`, `node/app.go` | `v7.0` (`MergeUpgrade`) |"
    ]
    assert result.stdout.count("|") == 5
    assert listed not in result.stdout
    assert head not in result.stdout

    recorded = commit_file(tmp_path, LIST_PATH, ledger + result.stdout)
    accepted = run_gate(tmp_path, base, recorded)
    assert accepted.returncode == 0, accepted.stdout + accepted.stderr
    assert accepted.stdout == ""
    empty = run_rows(tmp_path, base, recorded)
    assert empty.returncode == 0, empty.stdout + empty.stderr
    assert empty.stdout == ""

    # The requested historical head still uses its own ledger.
    pinned = run_rows(tmp_path, base, head)
    assert pinned.returncode == 0, pinned.stdout + pinned.stderr
    assert pinned.stdout == result.stdout
    (tmp_path / LIST_PATH).write_text(HEADER, encoding="utf-8")
    dirty = run_rows(tmp_path, base, recorded)
    assert dirty.returncode == 0, dirty.stdout + dirty.stderr
    assert dirty.stdout == ""


def test_rows_follow_history_order_and_include_merge_only_edits(tmp_path):
    base = init_repo(tmp_path)
    first = commit_file(tmp_path, "node/app.go", "first rule\n")
    git(tmp_path, "checkout", "-b", "side")
    commit_file(tmp_path, "docs/side.md", "side branch\n")
    git(tmp_path, "checkout", "main")
    second = commit_file(tmp_path, "node/sequencer/source.go", "second rule\n")
    git(tmp_path, "merge", "--no-ff", "--no-commit", "side")
    merged = commit_file(tmp_path, "node/app.go", "merge-only consensus rule\n")
    result = run_rows(tmp_path, base, merged)
    assert result.returncode == 0, result.stdout + result.stderr
    rows = result.stdout.splitlines()
    assert [row.split("|")[1].strip(" `") for row in rows] == [
        first,
        second,
        merged,
    ]
    assert "`node/app.go`" in rows[2]
    recorded = commit_file(tmp_path, LIST_PATH, HEADER + result.stdout)
    accepted = run_gate(tmp_path, base, recorded)
    assert accepted.returncode == 0, accepted.stdout + accepted.stderr
    assert accepted.stdout == ""


def test_custom_ledger_and_invalid_revisions(tmp_path):
    base = init_repo(tmp_path)
    sha = commit_file(tmp_path, "node/app.go", "consensus rule\n")
    custom = "docs/release/custom.md"
    ledger = HEADER + f"| `{sha}` | Rule | `node/app.go` | `v7.0` (`MergeUpgrade`) |\n"
    head = commit_file(tmp_path, custom, ledger)
    result = run_rows(tmp_path, base, head, "--list", custom)
    assert result.returncode == 0, result.stdout + result.stderr
    assert result.stdout == ""
    missing = run_rows(tmp_path, base, head, "--list", "missing.md")
    assert missing.returncode == 2
    assert missing.stdout == ""
    assert "Consensus ledger rows failed:" in missing.stderr
    for low, high in (("missing-base", head), (base, "missing-head")):
        invalid = run_rows(tmp_path, low, high)
        assert invalid.returncode == 2
        assert invalid.stdout == ""
        assert "Consensus ledger rows failed:" in invalid.stderr


def test_subject_cell_escapes_pipes_and_all_newline_forms():
    escaped = escape_cell("orders | receipts\nnext\r\nthen\rlast")
    assert escaped == "orders &#124; receipts<br>next<br>then<br>last"
    assert "|" not in escaped
    assert "\n" not in escaped
    assert "\r" not in escaped
