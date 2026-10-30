import importlib.util
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "tools/ci/consensus_commit_gate.py"
LIST_PATH = "docs/release/consensus-commits.md"
HEADER = (
    "| Full commit hash | State-machine change | Non-test consensus paths | Upgrade |\n"
    "| --- | --- | --- | --- |\n"
)
SEEDED_COMMITS = {
    "0b48316e2eea28256461f1a1a99c753b2666f98c",
    "12fbbeb80389f9021d791db00d3f32101ea51668",
    "cdd97253c4219a77363a62377c36d212cda5a7ba",
    "97d256444123c41a8db5c9b3d0c4ef2d05df4e0f",
    "68c8887a9520542d2878da3d6a44920ddd71b4b1",
}


def git(repo, *args):
    return subprocess.run(
        [
            "git",
            "-C",
            str(repo),
            "-c",
            "user.name=Consensus Gate Test",
            "-c",
            "user.email=consensus-gate@example.invalid",
            "-c",
            "commit.gpgsign=false",
            "-c",
            "core.hooksPath=/dev/null",
            *args,
        ],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()


def commit_file(repo, path, content):
    target = repo / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")
    git(repo, "add", "--", path)
    git(repo, "commit", "-m", f"Change {path}")
    return git(repo, "rev-parse", "HEAD")


def init_repo(repo):
    git(repo, "init", "--initial-branch=main")
    return commit_file(repo, LIST_PATH, HEADER)


def run_gate(repo, base, head, *options):
    return subprocess.run(
        [sys.executable, str(SCRIPT), base, head, *options],
        cwd=repo,
        capture_output=True,
        text=True,
        timeout=30,
    )


def test_unlisted_consensus_commit_is_refused(tmp_path):
    base = init_repo(tmp_path)
    paths = (
        "modules/layerxcustody/keeper/claims.go",
        "modules/layerxcustody/types/errors.go",
        "modules/perps/keeper/orders.go",
        "precompiles/perps/positions.go",
        "node/app.go",
        "node/sequencer/consensus_source.go",
        "programs/crates/layerx-programs-market/src/settle.rs",
    )
    changes = {}
    for path in paths:
        sha = commit_file(tmp_path, path, "consensus rule\n")
        changes[sha] = path
        result = run_gate(tmp_path, base, sha)
        assert result.returncode == 1, result.stdout + result.stderr
        for changed_sha, changed_path in changes.items():
            assert changed_sha in result.stdout
            assert changed_path in result.stdout

    git(tmp_path, "rm", "--", paths[0])
    git(tmp_path, "commit", "-m", "Remove a consensus rule")
    deleted = git(tmp_path, "rev-parse", "HEAD")
    result = run_gate(tmp_path, sha, deleted)
    assert result.returncode == 1, result.stdout + result.stderr
    assert deleted in result.stdout
    assert paths[0] in result.stdout


def test_listed_or_test_only_commit_passes(tmp_path):
    base = init_repo(tmp_path)
    path = "modules/layerxcustody/keeper/claims.go"
    sha = commit_file(tmp_path, path, "consensus rule\n")
    ledger = HEADER + f"| `{sha}` | Claim rule | `{path}` | V6120Upgrade |\n"
    listed_head = commit_file(tmp_path, LIST_PATH, ledger)
    result = run_gate(tmp_path, base, listed_head)
    assert result.returncode == 0, result.stdout + result.stderr
    assert result.stdout == ""

    # The checkout has the later list; the earlier head must still be refused.
    earlier = run_gate(tmp_path, base, sha)
    assert earlier.returncode == 1, earlier.stdout + earlier.stderr
    assert sha in earlier.stdout

    # A dirty list cannot remove approval stored in the requested head either.
    (tmp_path / LIST_PATH).write_text(HEADER, encoding="utf-8")
    result = run_gate(tmp_path, base, listed_head)
    assert result.returncode == 0, result.stdout + result.stderr
    (tmp_path / LIST_PATH).write_text(ledger, encoding="utf-8")

    for ignored_path in (
        "modules/layerxcustody/keeper/keeper_test.go",
        "modules/perps/keeper/orders_test.go",
        "precompiles/perps/positions_test.go",
        "node/sequencer/source_test.go",
        "docs/consensus-notes.md",
        "node/app.go.backup",
        "modules/layerxcustody/keepers/claims.go",
    ):
        head = commit_file(tmp_path, ignored_path, "test or documentation\n")
        result = run_gate(tmp_path, listed_head, head)
        assert result.returncode == 0, result.stdout + result.stderr
        assert result.stdout == ""

    custom = "docs/release/custom-ledger.md"
    custom_head = commit_file(tmp_path, custom, ledger)
    result = run_gate(tmp_path, base, custom_head, "--list", custom)
    assert result.returncode == 0, result.stdout + result.stderr

    # A merge-only edit is excluded by --no-merges; its parents are doc-only.
    git(tmp_path, "checkout", "-b", "side")
    commit_file(tmp_path, "docs/side.md", "side branch\n")
    git(tmp_path, "checkout", "main")
    commit_file(tmp_path, "docs/main.md", "main branch\n")
    git(tmp_path, "merge", "--no-ff", "--no-commit", "side")
    merge_head = commit_file(tmp_path, path, "merge-only consensus rule\n")
    result = run_gate(tmp_path, custom_head, merge_head)
    assert result.returncode == 0, result.stdout + result.stderr


def test_seeded_list_names_the_cited_commits():
    spec = importlib.util.spec_from_file_location("consensus_commit_gate", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    listed = module.listed_commits(ROOT / LIST_PATH)
    assert SEEDED_COMMITS <= listed
    for sha in SEEDED_COMMITS:
        git(ROOT, "cat-file", "-e", f"{sha}^{{commit}}")
        git(ROOT, "merge-base", "--is-ancestor", sha, "HEAD")
