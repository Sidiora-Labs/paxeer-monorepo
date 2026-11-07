"""Resolve the vendored runtime docs against the source tree they describe."""

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]
VENDORED = ROOT / "tools/hyperfide"
DOCS = ROOT / "docs/hyperfide"
PAGES = (
    "hyperfide/operations.md",
    "hyperfide/source-measurement.md",
    "runtime/api_to_audit.md",
    "runtime/ci-performance.md",
    "runtime/cli-guide-and-skill.md",
    "runtime/debugging-and-qa.md",
    "runtime/forkable-plugins.md",
    "runtime/lifecycle-diagrams.md",
    "runtime/postgresql-storage.md",
    "runtime/provider-bridge-protocol.md",
    "runtime/provider-plugin-api.md",
    "runtime/testing.md",
    "runtime/timeline-pagination.md",
    "runtime/ui-code-splitting.md",
    "runtime/worktrees.md",
    "verification/manual-runbook.md",
)

# The single mapping table covers outputs and user-authored corpus inputs, not
# source-path migrations. These need not exist in a clean checkout; their real
# producers (or the corpus input schema) must exist instead. All other source
# references must resolve directly, including glob patterns.
PRODUCERS = {
    "apps/cli/dist/index.js": "apps/cli/src/index.ts",
    "apps/mobile/build-output/hyperfide-dev.apk": (
        "apps/mobile/scripts/build-android-local.sh"
    ),
    "core/control-plane/dist": "core/control-plane/tsconfig.json",
    "core/control-plane/dist/control.js": "core/control-plane/src/control.ts",
    "core/control-plane/dist/fleet-supervisor.js": (
        "core/control-plane/src/fleet-supervisor.ts"
    ),
    "lib/templates/src/generated/templates.generated.ts": (
        "lib/templates/scripts/generate-templates.mjs"
    ),
    "tools/bundled-plugins/dist": "tools/bundled-plugins/build.ts",
    "build/source-control/7.1": "tests/hyperfide/cases/source_control/7_1.py",
    "core/server/test/provider-corpus/allowlists/<ws>.json": (
        "core/server/test/provider-corpus/allowlists/README.md"
    ),
    "core/server/test/provider-corpus/allowlists/<ws>-row-classes.json": (
        "core/server/test/provider-corpus/row-diff-classes.ts"
    ),
}
FENCE = re.compile(
    r"^ {0,3}(?P<ticks>`{3,}|~{3,})(?P<language>[^\n]*)\n"
    r"(?P<body>.*?)^ {0,3}(?P=ticks)[ \t]*$",
    re.MULTILINE | re.DOTALL,
)
INLINE = re.compile(r"(?<!`)(`+)(?!`)(.*?)\1(?!`)", re.DOTALL)
REPO_PATH = re.compile(
    r"(?<![\w@/.$-])"
    r"(?:apps|contracts|core|deploy|docs|examples|lib|plugins|sdk|spec|tests|tools"
    r"|packages|scripts|src)/[^\s`\"'|,;()\[\]]+"
)
PRE_LAYOUT = re.compile(
    r"(?<![\w-])apps/(?:app|server)(?:/|(?=[`\s]|$))"
    r"|(?<![\w/-])(?:packages|scripts)/"
)


def _pages():
    for name in PAGES:
        page = DOCS / name
        yield name, page.read_text(encoding="utf-8")


def _code_fragments(text):
    """Read inline spans and shell fences without treating fenced JS as shell."""
    for match in FENCE.finditer(text):
        if match["language"].strip() in {"", "sh", "bash", "shell", "console", "zsh"}:
            yield text.count("\n", 0, match.start()) + 2, match["body"]
    # Preserve line numbers while keeping fence delimiters out of inline spans.
    prose = FENCE.sub(lambda match: "\n" * match[0].count("\n"), text)
    for match in INLINE.finditer(prose):
        yield prose.count("\n", 0, match.start()) + 1, match[2]


def _source_path(reference):
    if reference == "tools/hyperfide" or reference.startswith("tools/hyperfide/"):
        return ROOT / reference
    if reference.startswith("docs/"):
        return DOCS / reference.removeprefix("docs/")
    return VENDORED / reference


def _pnpm_script(arguments):
    """Return the actual script and package selected by pnpm's global options."""
    words = arguments.split()
    directory = VENDORED
    package_filter = None
    while words:
        word = words.pop(0).strip("\"'();,")
        if word in {"-C", "--dir", "--filter", "-F"}:
            assert words, f"pnpm {word} has no value"
            value = words.pop(0)
            if word in {"-C", "--dir"}:
                directory = _source_path(value)
            else:
                package_filter = value
        elif word.startswith("--filter="):
            package_filter = word.partition("=")[2]
        elif word in {"--silent", "-s", "run"}:
            continue
        elif word in {"install", "exec"}:
            return None
        else:
            assert not word.startswith("-"), f"Unrecognised pnpm option: {word}"
            if package_filter is not None:
                manifests = [
                    path
                    for pattern in (
                        "apps/*",
                        "contracts/*",
                        "core/*",
                        "examples/*",
                        "examples/plugins/*",
                        "lib/*",
                        "plugins/*",
                        "sdk/*",
                        "tests/*",
                        "tools/*",
                    )
                    for path in VENDORED.glob(f"{pattern}/package.json")
                    if json.loads(path.read_text(encoding="utf-8")).get("name")
                    == package_filter
                ]
                assert len(manifests) == 1, f"Unknown pnpm filter: {package_filter}"
                directory = manifests[0].parent
            return directory / "package.json", word
    # A documented global-option prefix such as `pnpm -C tools/hyperfide`.
    return None


def test_vendored_doc_pnpm_scripts_exist():
    failures = []
    scripts = set()
    for page, text in _pages():
        for line, fragment in _code_fragments(text):
            for match in re.finditer(r"\bpnpm[ \t]+([^\n`]+)", fragment):
                invocation = _pnpm_script(match[1])
                if invocation is None:
                    continue
                manifest, script = invocation
                scripts.add((manifest, script))
                declared = json.loads(manifest.read_text(encoding="utf-8"))["scripts"]
                if script not in declared:
                    failures.append(f"{page}:{line}: {script!r} absent from {manifest}")
    assert (VENDORED / "package.json", "hyperfide-platform") in scripts
    assert (VENDORED / "package.json", "hyperfide-platform:dev") in scripts
    assert not failures, "Undefined pnpm scripts:\n" + "\n".join(failures)


def test_vendored_doc_repo_paths_exist():
    failures = []
    references = set()
    for page, text in _pages():
        for line, fragment in _code_fragments(text):
            for match in REPO_PATH.finditer(fragment):
                reference = match[0].rstrip(".:/")
                references.add(reference)
                relative = reference.removeprefix("tools/hyperfide/")
                if relative in PRODUCERS:
                    target = VENDORED / PRODUCERS[relative]
                    exists = target.is_file()
                    if exists and target.name == "tsconfig.json":
                        compiler = json.loads(target.read_text(encoding="utf-8"))
                        output = target.parent / compiler["compilerOptions"]["outDir"]
                        assert output == _source_path(reference), (
                            f"{page}:{line}: {reference} is not the declared output"
                        )
                else:
                    target = _source_path(reference)
                    if any(character in reference for character in "*?["):
                        # Resolve real matches, rather than accepting a glob's parent.
                        exists = any(ROOT.glob(target.relative_to(ROOT).as_posix()))
                    else:
                        exists = target.exists()
                if not exists:
                    failures.append(f"{page}:{line}: {reference} -> {target}")
    assert "tools/repository/hyperfide/measure-source-day.sh" in references
    assert "tools/workspace/provider-corpus/snapshot-rows.sh" in references
    assert "apps/web/scripts/check-bundle-budget.mjs" in references
    assert not failures, "Missing repository paths:\n" + "\n".join(failures)


def test_vendored_doc_no_pre_layout_prefixes():
    failures = []
    for page, text in _pages():
        for number, line in enumerate(text.splitlines(), 1):
            if PRE_LAYOUT.search(line):
                failures.append(f"{page}:{number}: {line.strip()}")
    assert len(PAGES) == 16
    assert not failures, "Retired layout prefixes:\n" + "\n".join(failures)
