import hashlib
import re
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
