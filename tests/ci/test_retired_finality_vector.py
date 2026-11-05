from pathlib import PurePosixPath
import subprocess

from retired_tests import REPO_ROOT, assert_paths_absent


def test_finality_vector_retired():
    assert_paths_absent(["tests/vectors/finality_evidence_v1.vec"])
    result = subprocess.run(
        [
            "git", "grep", "--null", "--files-with-matches", "--fixed-strings",
            "-e", "finality_evidence_v1.vec", "--", ".",
            ":(exclude,glob)**/__pycache__/**",
        ],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode in (0, 1), result.stderr

    references = []
    for name in result.stdout.split("\0"):
        if not name:
            continue
        path = PurePosixPath(name)
        if (
            path.parts[0] == "spec"
            or any(part in {"docs", "documentation"} for part in path.parts[:-1])
            or path.suffix.lower() in {".md", ".markdown", ".mdx", ".rst", ".adoc"}
            or name == "tests/ci/test_retired_finality_vector.py"
        ):
            continue
        references.append(name)
    assert not references, "Retired vector still referenced:\n" + "\n".join(references)
