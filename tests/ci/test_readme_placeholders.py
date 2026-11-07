"""Reject unfinished wording in tracked public READMEs."""

from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
PLACEHOLDER_MARKERS = re.compile(
    r"\b(?:TODO|TBD)\b|\b(?i:work\s+in\s+progress|lorem\s+ipsum)\b"
)


def test_public_readmes_carry_no_placeholder_markers():
    vendored_trees = (
        Path("consensus/spec"),  # Upstream consensus specifications retain their wording.
        Path("core/deps"),  # Bundled dependencies retain upstream documentation.
        Path("tools/wasm-tools"),  # Vendored WebAssembly tools retain upstream READMEs.
        Path("programs/vendor"),  # Vendored Rust crates retain upstream status notices.
        Path("tools/hyperfide"),  # Upstream examples and plugins contain TODO guidance.
    )
    # This upstream README keeps its disclaimer as required by do_2.
    vendored_readme = Path(
        "sdk/crypto/keys/secp256k1/internal/secp256k1/libsecp256k1/README.md"
    )
    tracked = subprocess.run(
        ["git", "ls-files", "--full-name", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    readmes = [
        Path(name)
        for name in tracked
        if name
        and Path(name).name.lower().startswith("readme")
        and not any(Path(name).is_relative_to(tree) for tree in vendored_trees)
        and Path(name) != vendored_readme
    ]
    assert readmes, "No tracked public READMEs found"
    assert {Path("tools/hpx/README.md"), Path("tools/loadtest/README.md")} <= set(
        readmes
    ), "The repaired public READMEs must remain covered"

    loadtest = (ROOT / "tools/loadtest/README.md").read_text(encoding="utf-8")
    assert (
        "Replace every constructor and transaction placeholder that the scaffold "
        "emits before building a new scenario."
    ) in " ".join(loadtest.split())

    upstream_readme = (ROOT / vendored_readme).read_text(encoding="utf-8")
    introduction, disclaimer, _ = upstream_readme.partition(
        "This library is a work in progress and is being used to research best "
        "practices. Use at your own risk."
    )
    assert disclaimer, "Keep the upstream libsecp256k1 disclaimer"
    assert "`9d560f992db26612ce2630b194aef5f44d63a530`" in introduction
    assert "`sdk/crypto/keys/secp256k1/internal/secp256k1` Go package." in introduction
    assert not PLACEHOLDER_MARKERS.search(introduction), (
        "The local libsecp256k1 introduction must not contain placeholder markers"
    )

    violations = []
    for path in readmes:
        content = (ROOT / path).read_text(encoding="utf-8")
        for match in PLACEHOLDER_MARKERS.finditer(content):
            line = content.count("\n", 0, match.start()) + 1
            violations.append(f"{path}:{line}: {match.group()}")

    assert not violations, "Placeholder markers in public READMEs:\n" + "\n".join(
        violations
    )
