"""Keep PerfDB documentation and metadata pointed at the monorepo."""

from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[2]
PERF_DB = ROOT / "services/platform/perf-db"
MONOREPO_URL = "https://github.com/Sidiora-Labs/paxeer-monorepo"


def test_perf_db_names_no_dead_origins():
    codeowners = PERF_DB / ".github/CODEOWNERS"
    assert not codeowners.exists(), "Nested PerfDB CODEOWNERS must be removed"
    assert not codeowners.is_symlink(), "Nested CODEOWNERS must not be a symlink"

    for relative in ("README.md", "Cargo.toml", ".github/ISSUE_TEMPLATE/config.yml"):
        contents = (PERF_DB / relative).read_text(encoding="utf-8").casefold()
        assert "cdn.redixusercontent.ocfstudio.com" not in contents, relative
        assert "github.com/sidiora-labs/perf-db" not in contents, relative

    readme = (PERF_DB / "README.md").read_text(encoding="utf-8")
    assert readme.count(f"git clone {MONOREPO_URL}\n") == 2
    assert readme.count("cd paxeer-monorepo/services/platform/perf-db\n") == 2
    assert "[`SECURITY.md`](../../../SECURITY.md)" in readme
    assert (PERF_DB / "../../../SECURITY.md").resolve() == ROOT / "SECURITY.md"
    assert (ROOT / "SECURITY.md").is_file()

    dependency_examples = [
        tomllib.loads(block)["dependencies"]["perfdb"]
        for block in re.findall(r"```toml\n(.*?)```", readme, flags=re.DOTALL)
    ]
    assert {"git": MONOREPO_URL} in dependency_examples
    assert {"path": "../paxeer-monorepo/services/platform/perf-db"} in dependency_examples

    template = (PERF_DB / ".github/ISSUE_TEMPLATE/config.yml").read_text(
        encoding="utf-8"
    )
    assert f"    url: {MONOREPO_URL}/blob/main/SECURITY.md\n" in template


def test_perf_db_repository_is_monorepo():
    manifest = tomllib.loads((PERF_DB / "Cargo.toml").read_text(encoding="utf-8"))
    assert manifest["package"]["repository"] == MONOREPO_URL
