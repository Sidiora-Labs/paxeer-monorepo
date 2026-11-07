"""Check local links in the light-client verification specifications."""

from pathlib import Path
import re
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]
VERIFICATION = ROOT / "consensus/spec/light-client/verification"
INLINE_LINK = re.compile(r"\]\(\s*(?:<([^>\n]*)>|([^\s)]+))")
REFERENCE_LINK = re.compile(
    r"^[ \t]{0,3}\[[^\]\n]+\]:[ \t]*(?:<([^>\n]*)>|(\S+))", re.MULTILINE
)


def test_verification_spec_relative_links_resolve():
    documents = sorted(VERIFICATION.glob("*.md"))
    assert documents, "No verification specifications found"
    broken = []
    checked = 0
    for document in documents:
        text = document.read_text(encoding="utf-8")
        for pattern in (INLINE_LINK, REFERENCE_LINK):
            for match in pattern.finditer(text):
                target = next(value for value in match.groups() if value is not None)
                url = urlsplit(target)
                if url.scheme or url.netloc or url.path.startswith("/"):
                    continue
                destination = (
                    document.parent / unquote(url.path) if url.path else document
                )
                checked += 1
                if not destination.exists():
                    line = text.count("\n", 0, match.start()) + 1
                    broken.append(f"{document.relative_to(ROOT)}:{line}: {target}")
    assert checked, "No relative verification links checked"
    assert not broken, "Unresolved relative links:\n" + "\n".join(broken)


def test_fork_detector_reference_is_local():
    document = VERIFICATION / "verification_001_published.md"
    references = re.findall(
        r"^\[fork-detector\]:[ \t]*(\S+)[ \t]*$",
        document.read_text(encoding="utf-8"),
        re.MULTILINE,
    )
    assert references == ["../detection/README.md"]
    destination = (document.parent / references[0]).resolve()
    assert destination == ROOT / "consensus/spec/light-client/detection/README.md"
    assert destination.is_file()
