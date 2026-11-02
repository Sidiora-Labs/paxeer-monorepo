import ast
import tokenize
from importlib.util import resolve_name
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
PLATFORM_ROOT = REPO_ROOT / "services/platform"


def test_relay_runtime_and_forwarder_absent():
    remaining = [
        str(path.relative_to(REPO_ROOT))
        for name in ("runtime.py", "forward.py")
        if (path := PLATFORM_ROOT / "relay_archive" / name).exists()
        or path.is_symlink()
    ]
    assert not remaining, f"Retired relay serving files remain: {remaining}"


def test_no_platform_python_imports_relay_runtime():
    violations = []
    for path in sorted(PLATFORM_ROOT.rglob("*.py")):
        relative_path = path.relative_to(REPO_ROOT)
        package = ".".join(relative_path.parent.parts)
        with tokenize.open(path) as source:
            tree = ast.parse(source.read(), filename=str(relative_path))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                imported_names = [alias.name for alias in node.names]
            elif isinstance(node, ast.ImportFrom):
                module = node.module or ""
                if node.level:
                    module = resolve_name("." * node.level + module, package)
                imported_names = [module] + [
                    f"{module}.{alias.name}" for alias in node.names
                ]
            else:
                continue
            for name in imported_names:
                parts = name.split(".")
                if any(
                    parent == "relay_archive" and child in {"runtime", "forward"}
                    for parent, child in zip(parts, parts[1:])
                ):
                    violations.append(f"{relative_path}:{node.lineno}: {name}")
    assert not violations, "Retired relay imports remain:\n" + "\n".join(violations)
