from pathlib import Path
import re
import subprocess


REPO_ROOT = Path(__file__).resolve().parents[2]
SOURCE_SUFFIXES = {".c", ".h", ".py", ".sh", ".sol", ".go"}


def assert_paths_absent(paths, root=REPO_ROOT):
    root = Path(root)
    present = [
        str(path) for path in paths
        if (root / path).exists() or (root / path).is_symlink()
    ]
    if present:
        raise AssertionError("Retired paths still exist:\n" + "\n".join(present))


def assert_symbols_unreferenced(symbols, roots, root=REPO_ROOT, exclude=()):
    root = Path(root).resolve()
    excluded_roots = [(root / path).resolve() for path in exclude]
    patterns = [
        (symbol, re.compile(
            r"(?<![A-Za-z0-9_])" + re.escape(symbol) + r"(?![A-Za-z0-9_])"
        ))
        for symbol in symbols
    ]
    files = set()
    for search_root in roots:
        path = root / search_root
        if path.is_file():
            files.add(path)
        elif path.is_dir():
            files.update(
                candidate for candidate in path.rglob("*") if candidate.is_file()
            )

    references = []
    for path in sorted(files):
        if any(
            path.resolve().is_relative_to(excluded) for excluded in excluded_roots
        ):
            continue
        if (
            path.suffix not in SOURCE_SUFFIXES
            or path.resolve() == Path(__file__).resolve()
            or path.match("test_retired_*.py")
        ):
            continue
        name = path.relative_to(root) if path.is_relative_to(root) else path
        for line_number, line in enumerate(
            path.read_text(encoding="utf-8").splitlines(), start=1
        ):
            for symbol, pattern in patterns:
                for match in pattern.finditer(line):
                    references.append(
                        f"{name}:{line_number}: {symbol} (column {match.start() + 1})"
                    )
    if references:
        raise AssertionError(
            "Retired symbols still referenced:\n" + "\n".join(references)
        )


def build_and_run_c_suite(target, args=(), root=REPO_ROOT):
    root = Path(root).resolve()
    commands = (["make", "-C", root, target], [root / target, *args])
    for command in commands:
        result = subprocess.run(
            command, cwd=root, capture_output=True, text=True, check=False
        )
        if result.returncode != 0:
            stderr_tail = "\n".join(result.stderr.splitlines()[-20:])
            raise AssertionError(
                f"{' '.join(str(arg) for arg in command)} exited with code "
                f"{result.returncode}:\n{stderr_tail}"
            )
