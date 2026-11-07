import ast
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
CI = ROOT / "tests/ci"


def test_tests_ci_python_modules_are_collectable_or_helpers():
    modules = set(CI.rglob("*.py"))
    tests = {path for path in modules if path.name.startswith("test_")}
    imported = set()
    for test in sorted(tests):
        tree = ast.parse(test.read_text(encoding="utf-8"), filename=str(test))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                names = [alias.name for alias in node.names]
                bases = (test.parent, CI, ROOT)
            elif isinstance(node, ast.ImportFrom):
                module = node.module or ""
                names = [module] if module else []
                names.extend(
                    f"{module}.{alias.name}" if module else alias.name
                    for alias in node.names
                    if alias.name != "*"
                )
                bases = (
                    (test.parents[node.level - 1],)
                    if node.level
                    else (test.parent, CI, ROOT)
                )
            else:
                continue
            for base in bases:
                for name in names:
                    path = base.joinpath(*name.split("."))
                    imported.add(path.with_suffix(".py"))
                    imported.add(path / "__init__.py")

    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", "tools", "Makefile", ".github/workflows"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    invoked = set()
    invocation = re.compile(
        r"(?:^|\s)(?:python(?:\d+(?:\.\d+)*)?|pytest)\s+([^\n;&|]+)"
    )
    module_path = re.compile(
        r"(?<![\w./-])(?:\./)?(tests/ci/[^\s\"';&|]+\.py)(?![\w./-])"
    )
    for name in tracked:
        path = ROOT / name
        if not path.is_file() or not (
            name == "Makefile"
            or (name.startswith("tools/") and path.suffix in {".sh", ".bash"})
            or (
                name.startswith(".github/workflows/")
                and path.suffix in {".yml", ".yaml"}
            )
        ):
            continue
        source = path.read_text(encoding="utf-8").replace("\\\n", " ")
        for line in source.splitlines():
            command = line.split("#", 1)[0]
            for match in invocation.finditer(command):
                invoked.update(
                    ROOT / item[1] for item in module_path.finditer(match[1])
                )
            direct = re.match(
                r"\s*(?:(?:run:|exec)\s+)?(?:\./)?(tests/ci/[^\s;&|]+\.py)(?=\s|$)",
                command,
            )
            if direct:
                invoked.add(ROOT / direct[1])

    uncollected = sorted(
        str(path.relative_to(ROOT)) for path in modules - tests - imported - invoked
    )
    assert not uncollected, (
        "Python modules must start with test_, be imported by a test module, "
        "or be run by path in a tracked tools shell script, Makefile, or public workflow:\n"
        + "\n".join(uncollected)
    )


def test_no_script_references_ci_recipe_contract():
    paths = subprocess.run(
        ["git", "ls-files", "-z", "--", "tools", "scripts", ".github"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    references = []
    for name in paths:
        path = ROOT / name
        if path.is_file() and path.suffix in {
            ".sh", ".bash", ".py", ".js", ".mjs", ".cjs", ".ts", ".yml", ".yaml"
        }:
            for number, line in enumerate(
                path.read_text(encoding="utf-8").splitlines(), 1
            ):
                if "ci-recipe-contract.py" in line:
                    references.append(f"{name}:{number}: {line}")
    assert not references, (
        "Scripts reference the retired module:\n" + "\n".join(references)
    )

    gate = subprocess.run(
        ["bash", str(ROOT / "tools/paxeer-x/gates/25.4.sh")],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    assert gate.returncode != 0
    assert "retired" in gate.stderr.lower()
