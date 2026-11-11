"""Prevent stray tracked artifacts and encoding damage in first-party files."""

from functools import lru_cache
from pathlib import Path
import re
import subprocess

from test_tracked_file_size_budget import allowlist
from test_web_asset_weight import LARGE_ASSET_ALLOWLIST


ROOT = Path(__file__).resolve().parents[2]
VENDORED_TREES = (
    "core/deps/",
    "programs/vendor/",
    "tools/hyperfide/",
    "tools/wasm-tools/",
    "tools/uci/",
    "tools/paxctl/",
    "tools/loadtest/",
    "services/explorer/backend/",
    "services/explorer/services/",
    "services/interop/specs/vendor/",
    "apps/inference-peer/engine/",
    "apps/launchpad/web/public/charting_library/",
    "docs/swagger-ui/",
    "sdk/client/docs/swagger-ui/",
)
ENGINEERING_TREES = ("spec/", ".hyperfide/", ".codify/", ".git/")
DEPENDENCY_FILES = {
    "Cargo.lock", "go.sum", "go.work.sum", "package-lock.json",
    "pnpm-lock.yaml", "yarn.lock",
}
BUILD_DIRECTORIES = {
    "node_modules", "target", "__pycache__", ".venv", ".pytest_cache",
}
SOURCE_SUFFIXES = {
    ".bash", ".c", ".cc", ".cjs", ".cpp", ".css", ".cxx", ".ex", ".exs",
    ".go", ".h", ".hpp", ".html", ".java", ".js", ".json", ".jsx", ".kt",
    ".mjs", ".py", ".rs", ".scss", ".sh", ".sol", ".svelte", ".swift",
    ".ts", ".tsx", ".vue",
}
# Empty package markers are intentional; keep this list exact and reasoned.
EMPTY_SOURCE_ALLOWLIST = {
    "tools/jev/__init__.py": "Python package marker for the JEV tooling",
    "tools/jev/tests/__init__.py": "Python package marker for the JEV tests",
    "tools/launchpad/production/dispatcher/tests/__init__.py": (
        "Python package marker for the dispatcher tests"
    ),
    "tools/launchpad/production/feature-acceptance/__init__.py": (
        "Python package marker for the feature acceptance tooling"
    ),
}
DATED_PATCH_DIRECTORY = re.compile(r"^patches/\d{8}(?:_\d+)?/")
# Spell the signatures as escapes so the guard also scans its own source.
MOJIBAKE = re.compile(
    r"\u00e2[\u0080\u20ac\u2013\u2014\u2018\u2019\u2022\u2026]"
    r"|\u00c3[\u0080-\u00bf]"
    r"|\u00c2[\u0080-\u00bf]"
    r"|\u00ef\u00bb\u00bf"
    r"|\u00f0\u0178"
)


@lru_cache(maxsize=1)
def _owned_files():
    inventory = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    ).stdout
    names = sorted({
        name.decode("utf-8", errors="surrogateescape")
        for name in inventory.split(b"\0") if name
    })
    assert names, "No repository files inventoried"
    owned = []
    for name in names:
        relative = Path(name)
        if name.startswith(VENDORED_TREES + ENGINEERING_TREES):
            continue
        if {"vendor", "third_party"}.intersection(relative.parts):
            continue
        if BUILD_DIRECTORIES.intersection(relative.parts):
            continue
        if relative.name in DEPENDENCY_FILES or relative.suffix in {".pyc", ".pyo"}:
            continue
        path = ROOT / relative
        # Tracked deletions can remain in the index until the runner publishes.
        if path.is_file() and not path.is_symlink():
            owned.append(relative)
    assert owned, "No first-party files inventoried"
    return tuple(owned)


def test_no_empty_source_files_outside_allowlist():
    paths = _owned_files()
    empty = {
        path.as_posix() for path in paths
        if path.suffix in SOURCE_SUFFIXES and (ROOT / path).stat().st_size == 0
    }
    unexpected = empty - EMPTY_SOURCE_ALLOWLIST.keys()
    assert not unexpected, "Empty source files:\n" + "\n".join(sorted(unexpected))
    for name, reason in EMPTY_SOURCE_ALLOWLIST.items():
        assert reason and Path(name) in paths, f"Stale empty-source exception: {name}"
        assert (ROOT / name).stat().st_size == 0, f"Unneeded exception: {name}"


def test_no_tracked_git_bundles_or_patch_dirs():
    unexpected = []
    for path in _owned_files():
        name = path.as_posix()
        is_git_bundle = False
        if path.suffix == ".bundle":
            with (ROOT / path).open("rb") as source:
                is_git_bundle = source.readline() == b"# v2 git bundle\n"
        if is_git_bundle or DATED_PATCH_DIRECTORY.match(name):
            unexpected.append(name)
    assert not unexpected, (
        "Tracked Git bundles or dated patch directories:\n" + "\n".join(unexpected)
    )
    assert not (ROOT / "apps/perps/public/model.gltf").exists(), (
        "The unreferenced perps model must remain removed"
    )


def test_removed_model_has_no_size_allowances(allowlist: dict[str, int]):
    removed_model = "apps/perps/public/model.gltf"
    assert not (ROOT / removed_model).exists(), (
        "The unreferenced perps model must remain removed"
    )
    assert removed_model not in LARGE_ASSET_ALLOWLIST, (
        "The removed perps model must have no asset weight allowance"
    )
    assert removed_model not in allowlist, (
        "The removed perps model must have no tracked file size allowance"
    )


def test_no_gitkeep_beside_content():
    paths = _owned_files()
    populated_directories = {
        parent for path in paths if path.name != ".gitkeep"
        for parent in path.parents
    }
    redundant = [
        path.as_posix() for path in paths
        if path.name == ".gitkeep" and path.parent in populated_directories
    ]
    assert not redundant, (
        "Keep markers beside real tracked content:\n" + "\n".join(redundant)
    )


def test_no_mojibake_in_owned_text():
    findings = []
    for path in _owned_files():
        with (ROOT / path).open("rb") as source:
            prefix = source.read(8192)
            if b"\0" in prefix:
                continue
            data = prefix + source.read()
        if b"\0" in data:
            continue
        text = data.decode("utf-8", errors="surrogateescape")
        for number, line in enumerate(text.splitlines(), 1):
            match = MOJIBAKE.search(line)
            if match:
                findings.append(f"{path.as_posix()}:{number}: {match.group()!r}")
    assert not findings, "Mis-encoded first-party text:\n" + "\n".join(findings)
    installer = (ROOT / "scripts/codify.sh").read_text(encoding="utf-8")
    assert installer.splitlines()[1].startswith("# Codify installer/updater - ")
