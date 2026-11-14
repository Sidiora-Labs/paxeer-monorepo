"""Keep repository text aligned with the LayerX kernel's network role."""

from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
MAX_TEXT_BYTES = 3_000_000
RECORD_FILES = {"GOTCHA.kvx", "CHANGELOG.md", "CODEMAP.md"}
EXCLUDED_ROOTS = (
    "spec/",
    "core/deps/",
    "tools/hyperfide/",
    "tools/qualification/audit_repairs/",
    "consensus/",
)
BUILD_DIRECTORIES = {
    "build",
    "dist",
    "out",
    "target",
    "node_modules",
    "__pycache__",
    ".next",
    ".generated",
    "artifacts",
    "cache",
}
MARKUP_SUFFIXES = {".tsx", ".jsx", ".mdx", ".html"}
COMMENT_PREFIX = re.compile(r"^[ \t]*(?://+|/\*+|\*+|#+|<!--|--|;|>)[ \t]*", re.MULTILINE)
RETIRED_WORDING = re.compile(
    r"\bBFT\s+(?:chain\s+state|network|chain|node)\b"
    r"|\bproposes\s+blocks\b"
    r"|\bkernel\s+is\s+part\s+of\s+(?:the\s+same|the|that)\s+network\b"
    r"|\bkernel\s+participates\s+in\s+the\s+network\b"
    r"|\bparticipates\s+in\s+block\s+(?:proposal|production|execution)\b"
    r"|\bblock\s+proposal\s+(?:path|and\s+(?:execution|finality|block\s+commit))\b"
    r"|\bin\s+block\s+proposal\b"
    r"|\b(?:runs|executes)\s+within\s+the\s+network\b"
    r"|\bwithin\s+the\s+Paxeer\s+consensus\s+network\b",
    re.IGNORECASE,
)


def _retired_wordings(content, suffix=""):
    prose = " ".join(COMMENT_PREFIX.sub("", content).split())
    if suffix.lower() in MARKUP_SUFFIXES:
        prose = re.sub(r"<[^>]*>", "", prose)
    prose = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", prose)
    prose = " ".join(prose.split())
    return sorted({match.group().lower() for match in RETIRED_WORDING.finditer(prose)})


def test_tracked_text_uses_kernel_sentence():
    tracked = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    assert "README.md" in tracked, "Tracked text inventory must include README.md"
    vendor_roots = tuple(
        str(Path(name).parent) + "/"
        for name in tracked
        if name.startswith("tools/") and Path(name).name == "VENDOR.kvx"
    )
    references = []
    scanned = set()
    for name in sorted(set(tracked) - {""} - RECORD_FILES):
        relative = Path(name)
        if (
            name == Path(__file__).resolve().relative_to(ROOT).as_posix()
            or name.startswith(EXCLUDED_ROOTS + vendor_roots)
            or BUILD_DIRECTORIES.intersection(relative.parts[:-1])
        ):
            continue
        path = ROOT / name
        assert path.exists(), f"Tracked path is missing: {name}"
        if not path.is_file() or path.stat().st_size >= MAX_TEXT_BYTES:
            continue
        data = path.read_bytes()
        if b"\0" in data:
            continue
        try:
            content = data.decode("utf-8")
        except UnicodeDecodeError:
            continue
        scanned.add(name)
        words = _retired_wordings(content, relative.suffix)
        if words:
            references.append(f"{name}: {', '.join(words)}")
    assert {"README.md", "docs/readme/NETWORK.md"} <= scanned, (
        "Kernel network reference pages must be scanned"
    )
    assert not references, "Retired kernel wording in tracked text:\n" + "\n".join(
        references
    )


def test_kernel_sentence_wording_samples():
    retired = (
        ("Paxeer is a BFT network.", "bft network"),
        ("Paxeer is a BFT chain.", "bft chain"),
        ("Paxeer runs as a BFT NODE.", "bft node"),
        ("The receipt describes BFT chain state.", "bft chain state"),
        ("The kernel proposes blocks.", "proposes blocks"),
        ("The kernel is part of the network.", "kernel is part of the network"),
        ("The kernel is part of that network.", "kernel is part of that network"),
        (
            "The kernel is part of the same network.",
            "kernel is part of the same network",
        ),
        ("The kernel participates in the network.", "kernel participates in the network"),
        (
            "// The kernel participates in block\n// proposal.",
            "participates in block proposal",
        ),
        (
            "The kernel participates in block production.",
            "participates in block production",
        ),
        (
            "The kernel participates in block execution.",
            "participates in block execution",
        ),
        ("The kernel supplies the block proposal path.", "block proposal path"),
        (
            "The kernel handles block proposal and execution.",
            "block proposal and execution",
        ),
        (
            "The kernel handles block proposal and finality.",
            "block proposal and finality",
        ),
        (
            " * The kernel handles block proposal and\n * block commit.",
            "block proposal and block commit",
        ),
        ("The kernel takes part in block proposal.", "in block proposal"),
        ("The kernel runs within the network.", "runs within the network"),
        ("The kernel executes within the network.", "executes within the network"),
        (
            "The kernel runs within the Paxeer consensus network.",
            "within the paxeer consensus network",
        ),
    )
    for sentence, expected in retired:
        assert _retired_wordings(sentence) == [expected], sentence

    for suffix in MARKUP_SUFFIXES:
        assert _retired_wordings(
            'The <a href="/kernel">kernel</a> is part of the same network.',
            suffix,
        ) == ["kernel is part of the same network"]
        assert _retired_wordings(
            "The kernel participates in <strong>block\nproposal</strong>.",
            suffix,
        ) == ["participates in block proposal"]
    assert _retired_wordings(
        "The [kernel](/kernel) is part of the same network."
    ) == ["kernel is part of the same network"]

    allowed = (
        "A BFT commit makes the receipt final.",
        "The node uses BFT consensus to commit the block.",
        "CometBFT is the consensus engine used by the node.",
    )
    for sentence in allowed:
        assert not _retired_wordings(sentence), sentence
