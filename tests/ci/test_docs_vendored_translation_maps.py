"""Keep the translated vendored README maps aligned with the relocated tree."""

from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[2]
VENDOR = ROOT / "tools/hyperfide"
DOCS = ROOT / "docs/hyperfide"
ENGLISH = VENDOR / "README.md"
TRANSLATIONS = tuple(
    DOCS / "i18n" / f"README.{language}.md"
    for language in ("ar", "es", "fr", "hi", "pt-BR", "zh-CN")
)


def _map_text(page):
    blocks = re.findall(
        r"^```\n(.*?)^```[ \t]*$",
        page.read_text(encoding="utf-8"),
        flags=re.MULTILINE | re.DOTALL,
    )
    maps = [
        block
        for block in blocks
        if "\ncore/control-plane/" in block and "\nartifacts/hyperfide/" in block
    ]
    assert len(maps) == 1, f"Expected one repository map in {page}"
    return maps[0]


def _source_entries(page):
    entries = re.findall(r"(?<![\w/])(?:core/native/)?src/[\w./-]*", _map_text(page))
    assert len(entries) == len(set(entries)) == 17, (page, entries)
    return entries


def _doc_entries(page):
    entries = re.findall(
        r"^(?:docs/|\.\./)[^\s]+\.md(?=\s)", _map_text(page), flags=re.MULTILINE
    )
    assert len(entries) == len(set(entries)) == 11, (page, entries)
    return entries


@pytest.mark.parametrize("page", TRANSLATIONS, ids=lambda page: page.name)
def test_translation_source_map_paths_exist(page):
    for entry in _source_entries(page):
        destination = VENDOR / entry
        assert (
            destination.is_dir() if entry.endswith("/") else destination.is_file()
        ), (page, entry, destination)


@pytest.mark.parametrize("page", TRANSLATIONS, ids=lambda page: page.name)
def test_translation_doc_map_paths_exist(page):
    for entry in _doc_entries(page):
        destination = (page.parent / entry).resolve()
        assert destination.is_relative_to(DOCS), (page, entry, destination)
        assert destination.is_file(), (page, entry, destination)


@pytest.mark.parametrize("page", TRANSLATIONS, ids=lambda page: page.name)
def test_translations_match_english_map_entries(page):
    assert sorted(_source_entries(page)) == sorted(_source_entries(ENGLISH)), page
    english_docs = sorted(
        (DOCS / entry.removeprefix("docs/")).resolve()
        for entry in _doc_entries(ENGLISH)
    )
    translated_docs = sorted(
        (page.parent / entry).resolve() for entry in _doc_entries(page)
    )
    assert translated_docs == english_docs, page
