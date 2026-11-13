"""Frontend assets and the launchpad builder's canonical chart declarations.

The image also needs webpack to resolve the source-exported launchpad SDK's
``.js`` imports to TypeScript. The unused Base Account and Coinbase CDP
connectors must be excluded from webpack resolution: their optional x402 and
Solana peers are absent from the canonical lockfile. Otherwise webpack fails
before Next's chart type validation; the build environment grants a 3 GiB heap.
The prior build also reported ``ESLint: The requested module 'minimatch' does
not provide an export named 'default'`` before missing canonical precompile
ABIs stopped type validation. Those repository ABIs must be staged as well.
"""

import fnmatch
import hashlib
import json
import re
import shlex
import shutil
import subprocess
import tomllib
from collections import defaultdict
from pathlib import Path
from urllib.parse import unquote, urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[2]
APPS = ("apps/launchpad/web", "apps/perps")
RENAME_COUNTS = {"apps/launchpad/web": 44, "apps/perps": 34}
TEXT_SUFFIXES = {".ts", ".tsx", ".js", ".jsx", ".css", ".scss", ".html", ".json"}
LITERALS = re.compile(r"[\"']([^\"'\r\n]+)[\"']|url\(\s*([^\s)\"']+)\s*\)")


def app_files(app):
    output = subprocess.check_output(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", app],
        cwd=ROOT,
    )
    return sorted(
        {
            ROOT / name.decode()
            for name in output.split(b"\0")
            if name and (ROOT / name.decode()).is_file()
        }
    )


def asset_references(app):
    references = defaultdict(set)
    app_root = ROOT / app
    for source in app_files(app):
        if source.suffix not in TEXT_SUFFIXES:
            continue
        if any(part in {"tests", "__tests__"} for part in source.parts):
            continue
        if re.search(r"\.(?:test|spec)\.[^.]+$", source.name):
            continue
        for match in LITERALS.finditer(source.read_text(encoding="utf-8")):
            value = match.group(1) or match.group(2)
            if not value.startswith(("/", "./", "../", "@/")) or value.startswith("//"):
                continue
            url = urlsplit(value)
            if url.scheme or url.netloc:
                continue
            path = unquote(url.path)
            if path.startswith("/"):
                target = app_root / "public" / path.lstrip("/")
            elif path.startswith("@/"):
                target = app_root / "src" / path[2:]
            elif path.startswith("."):
                target = source.parent / path
            else:
                continue
            references[target.resolve()].add(source.relative_to(ROOT).as_posix())
    return references


@pytest.mark.parametrize("app", APPS)
def test_no_spaces_in_app_asset_names(app):
    files = app_files(app)
    assert files, f"No files inventoried for {app}"
    invalid = [
        p.relative_to(ROOT).as_posix()
        for p in files
        if p.relative_to(ROOT / app).parts[0] in {"public", "src"}
        and any(character.isspace() for character in p.relative_to(ROOT).as_posix())
    ]
    assert not invalid, "Asset paths contain whitespace:\n" + "\n".join(invalid)


@pytest.mark.parametrize("app", APPS)
def test_no_duplicate_assets_within_an_app(app):
    groups = defaultdict(list)
    files = app_files(app)
    assert files, f"No files inventoried for {app}"
    for path in files:
        # Windows prescribes these tile names and sizes, including identical tiles.
        if path.is_relative_to(ROOT / "apps/perps/public/windows"):
            continue
        if path.stat().st_size > 20_000:
            digest = hashlib.sha256(path.read_bytes()).hexdigest()
            groups[digest].append(path.relative_to(ROOT).as_posix())
    duplicates = [paths for paths in groups.values() if len(paths) > 1]
    assert not duplicates, "Byte-identical files over 20 KB:\n" + "\n".join(
        " = ".join(paths) for paths in duplicates
    )

    manifest = tomllib.loads((ROOT / app / "asset-renames.kvx").read_text())
    for entry in manifest["deduplicate"].values():
        assert not (ROOT / app / entry["old"]).exists(), entry["old"]
        canonical = ROOT / app / entry["new"]
        assert canonical.is_file(), entry["new"]
        assert hashlib.sha256(canonical.read_bytes()).hexdigest() == entry["sha256"]


@pytest.mark.parametrize("app", APPS)
def test_renamed_assets_are_imported(app):
    manifest = tomllib.loads((ROOT / app / "asset-renames.kvx").read_text())
    assert manifest["meta"]["app"] == app
    entries = list(manifest["rename"].values())
    assert len(entries) == RENAME_COUNTS[app]
    assert len({entry["old"] for entry in entries}) == len(entries)
    references = asset_references(app)
    missing = []
    for entry in entries:
        old = ROOT / app / entry["old"]
        new = ROOT / app / entry["new"]
        assert old.is_relative_to(ROOT / app) and new.is_relative_to(ROOT / app)
        assert any(character.isspace() for character in entry["old"])
        assert not old.exists(), f"Old asset still exists: {old.relative_to(ROOT)}"
        assert old.resolve() not in references, f"Stale asset reference: {entry['old']}"
        assert re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*\.[a-z0-9]+", new.name)
        assert new.is_file(), f"Renamed asset missing: {new.relative_to(ROOT)}"
        assert hashlib.sha256(new.read_bytes()).hexdigest() == entry["sha256"]
        if new.resolve() not in references:
            missing.append(new.relative_to(ROOT).as_posix())
    assert not missing, "Renamed assets have no import or URL reference:\n" + "\n".join(
        missing
    )


def test_shared_stylesheets_and_chart_declarations_reference_one_source():
    explorer = ROOT / "apps/explorer/.scss"
    perps = ROOT / "apps/perps/src/styles/.scss"
    stylesheets = sorted(explorer.glob("*.css"))
    assert len(stylesheets) == 25
    for canonical in stylesheets:
        reference = perps / canonical.name
        expected = f'@import "../../../../explorer/.scss/{canonical.name}";\n'
        assert reference.read_text() == expected
        assert (reference.parent / re.search(r'"([^"]+)"', expected)[1]).resolve() == canonical
    imports = re.findall(r'@import "\./([^"]+)";', (explorer / "index.css").read_text())
    assert set(imports) == {path.name for path in stylesheets if path.name != "index.css"}

    reference = ROOT / "apps/launchpad/web/public/charting_library/charting_library.d.ts"
    canonical = ROOT / "apps/perps/public/static/charting_library/charting_library.d.ts"
    expected = '../../../../perps/public/static/charting_library/charting_library'
    assert reference.read_text() == (
        f'export * from "{expected}";\nexport as namespace TradingView;\n'
    )
    assert (reference.parent / f"{expected}.d.ts").resolve() == canonical
    assert "export interface IChartingLibraryWidget" in canonical.read_text()


def test_launchpad_docker_chart_declarations(tmp_path):
    dockerfile = ROOT / "deploy/launchpad/Dockerfile.web"
    instructions = [
        line.strip()
        for line in dockerfile.read_text().splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    stages = [i for i, line in enumerate(instructions) if line.startswith("FROM ")]
    assert instructions[stages[0]].endswith(" AS build")
    build = instructions[stages[0] + 1 : stages[1]]
    copies = []
    for i, line in enumerate(build):
        if line.startswith("COPY "):
            arguments = shlex.split(line)
            assert len(arguments) == 3 and not arguments[1].startswith("--"), line
            source, destination = arguments[1:]
            assert not Path(source).is_absolute() and ".." not in Path(source).parts
            assert destination.startswith("/src/")
            assert ".." not in Path(destination).parts
            copies.append((i, source, destination))

    canonical = "apps/perps/public/static/charting_library/charting_library.d.ts"
    canonical_copies = [entry for entry in copies if entry[1] == canonical]
    assert len(canonical_copies) == 1
    canonical_copy = canonical_copies[0]
    assert canonical_copy[2] == f"/src/{canonical}"
    assert canonical_copy[0] < build.index("RUN pnpm install --frozen-lockfile")
    assert build[-1] == "RUN . /src/web-build-env.sh && pnpm --filter @sidiora/web build"
    assert [source for _, source, _ in copies if source.startswith("apps/perps")] == [
        canonical
    ]

    dockerignore = dockerfile.with_name(dockerfile.name + ".dockerignore")
    patterns = [
        line.strip().strip("/")
        for line in dockerignore.read_text().splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    assert all(not pattern.startswith("!") for pattern in patterns)

    def ignored(path):
        relative = path.relative_to(ROOT)
        ancestors = [relative, *relative.parents[:-1]]
        return any(
            fnmatch.fnmatchcase(ancestor.as_posix(), candidate)
            for ancestor in ancestors
            for pattern in patterns
            for candidate in (
                (pattern, pattern[3:]) if pattern.startswith("**/") else (pattern,)
            )
        )

    def ignore_children(directory, names):
        return [name for name in names if ignored(Path(directory) / name)]

    def stage(directory, omit_canonical=False):
        directory.mkdir()
        for entry in copies:
            if omit_canonical and entry == canonical_copy:
                continue
            _, source, destination = entry
            source = ROOT / source
            target = directory / destination.lstrip("/")
            assert source.exists() and not ignored(source), source
            target.parent.mkdir(parents=True, exist_ok=True)
            if source.is_dir():
                shutil.copytree(
                    source, target, ignore=ignore_children, dirs_exist_ok=True
                )
            else:
                shutil.copy2(source, target)
        return directory / "src/apps/launchpad/web"

    web = stage(tmp_path / "builder")
    manifest = json.loads((web / "package.json").read_text())
    assert manifest["scripts"]["build"] == "next build"
    config = (web / "next.config.ts").read_text()
    assert not re.search(r"ignore(?:BuildErrors|DuringBuilds)\s*:\s*true", config)
    abi_module = web / "src/core/network/abis/canonical-abi.ts"
    abi_imports = re.findall(
        r"import type \w+ from ['\"]([^'\"]+\.json)['\"];", abi_module.read_text()
    )
    assert {Path(module).name for module in abi_imports} == {
        "FeesRouter.json",
        "Quoter.json",
        "Router.json",
    }
    for module in abi_imports:
        staged_abi = (abi_module.parent / module).resolve()
        relative = staged_abi.relative_to(tmp_path / "builder/src")
        assert relative.parent == Path("precompiles/launchpad/abi")
        assert staged_abi.read_bytes() == (ROOT / relative).read_bytes()
    forwarder = web / "public/charting_library/charting_library.d.ts"
    forwarding = forwarder.read_text()
    imports = re.findall(r'^export \* from "([^"]+)";', forwarding, re.M)
    assert len(imports) == 1
    assert re.search(r"^export as namespace TradingView;$", forwarding, re.M)
    dependency = (forwarder.parent / f"{imports[0]}.d.ts").resolve()
    assert dependency == tmp_path / "builder/src" / canonical
    assert dependency.read_bytes() == (ROOT / canonical).read_bytes()
    declarations = dependency.read_text()
    assert re.search(r"^export as namespace TradingView;$", declarations, re.M)
    required = set()
    for consumer in ("tv-chart.tsx", "udf-datafeed.ts"):
        source = (web / "src/widgets/trading" / consumer).read_text()
        chart_imports = re.findall(
            r"import type \{([^}]+)\} from ['\"]([^'\"]*charting_library)['\"];",
            source,
        )
        assert len(chart_imports) == 1
        names, module = chart_imports[0]
        imported = (web / "src/widgets/trading" / f"{module}.d.ts").resolve()
        assert imported == forwarder
        required.update(name.strip() for name in names.split(","))
    assert required == {
        "ChartingLibraryWidgetConstructor",
        "ChartingLibraryWidgetOptions",
        "IBasicDataFeed",
        "IChartingLibraryWidget",
        "LibrarySymbolInfo",
        "ResolutionString",
        "TimeFrameItem",
    }
    for name in required:
        assert re.search(
            rf"^export (?:declare )?(?:interface|type|class|enum|const) {name}\b",
            declarations,
            re.M,
        ), name

    missing_web = stage(tmp_path / "without-canonical", omit_canonical=True)
    missing_forwarder = missing_web / forwarder.relative_to(web)
    assert missing_forwarder.read_bytes() == forwarder.read_bytes()
    missing_dependency = (missing_forwarder.parent / f"{imports[0]}.d.ts").resolve()
    assert missing_dependency.is_relative_to(tmp_path / "without-canonical")
    assert not missing_dependency.exists()
    with pytest.raises(FileNotFoundError):
        missing_dependency.read_bytes()
