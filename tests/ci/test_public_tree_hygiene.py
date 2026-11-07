import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
WALLET = Path("apps/wallet")
PUBLIC = WALLET / "public"
MAX_PUBLIC_BYTES = 5_000_000
SOURCE_SUFFIXES = {
    ".cjs",
    ".css",
    ".html",
    ".js",
    ".json",
    ".jsx",
    ".less",
    ".mdx",
    ".mjs",
    ".sass",
    ".scss",
    ".svelte",
    ".ts",
    ".tsx",
    ".vue",
    ".webmanifest",
}


def _tracked_paths(*paths):
    result = subprocess.run(
        ["git", "ls-files", "-z", "--", *map(str, paths)],
        cwd=ROOT,
        check=True,
        capture_output=True,
    )
    return [
        Path(name.decode("utf-8", errors="surrogateescape"))
        for name in result.stdout.split(b"\0")
        if name
    ]


def test_wallet_public_has_no_unreferenced_large_image():
    tracked = _tracked_paths(WALLET)
    public_files = [path for path in tracked if PUBLIC in path.parents]
    sources = [path for path in tracked if path.suffix in SOURCE_SUFFIXES]
    assert public_files, "No tracked wallet public files found"
    assert sources, "No tracked wallet source files found"
    large_files = [
        path for path in public_files if (ROOT / path).stat().st_size > MAX_PUBLIC_BYTES
    ]
    source_texts = [
        (ROOT / path).read_text(encoding="utf-8", errors="surrogateescape")
        for path in sources
    ]
    unreferenced = [
        path.as_posix()
        for path in large_files
        if not any(path.relative_to(PUBLIC).as_posix() in text for text in source_texts)
    ]
    assert not unreferenced, (
        "Tracked wallet public files over 5 MB have no source references:\n"
        + "\n".join(unreferenced)
    )


def test_samples_track_no_env_files():
    tracked = _tracked_paths()
    assert tracked, "No tracked repository files found"
    env_files = [path.as_posix() for path in tracked if path.name == ".env"]
    assert not env_files, "Tracked .env files remain:\n" + "\n".join(env_files)
