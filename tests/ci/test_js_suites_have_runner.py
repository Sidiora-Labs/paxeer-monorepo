"""Require tracked JavaScript test packages to have executable CI coverage."""

import itertools
import json
from pathlib import Path, PurePosixPath
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
EXCLUSIONS = ROOT / "tests/ci/data/js_suite_exclusions.txt"
# Imported upstream trees have their own suite inventories and release workflows.
VENDORED_TREES = (
    "tools/hyperfide/",
    "programs/vendor/",
    "vendor/",
    "third_party/",
    "services/explorer/backend/",
)
MATRIX_VALUE = re.compile(r"\$\{\{\s*matrix\.([\w-]+)\s*\}\}")


def tracked_manifests():
    paths = subprocess.check_output(
        ["git", "ls-files", "-z", "--", "*package.json"], cwd=ROOT
    ).decode().split("\0")
    return {
        name: json.loads((ROOT / name).read_text())
        for name in paths
        if name
        and PurePosixPath(name).name == "package.json"
        and "node_modules" not in PurePosixPath(name).parts
        and not name.startswith(VENDORED_TREES)
    }


def packages_with_tests(manifests):
    return {
        name
        for name, manifest in manifests.items()
        if any(
            key == "test" or key.startswith("test:")
            for key in manifest.get("scripts", {})
        )
    }


def exclusions():
    result = {}
    for number, line in enumerate(EXCLUSIONS.read_text().splitlines(), 1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        path, separator, reason = line.partition("|")
        path, reason = path.strip(), reason.strip()
        assert separator and reason, f"{EXCLUSIONS.name}:{number}: missing reason"
        assert path not in result, f"Duplicate JS exclusion: {path}"
        assert not any(char in path for char in "*?[]"), path
        assert not PurePosixPath(path).is_absolute(), path
        assert ".." not in PurePosixPath(path).parts, path
        result[path] = reason
    return result


def matrix_rows(job):
    matrix = job.get("strategy", {}).get("matrix", {})
    # Runtime-generated matrices cannot establish static package coverage.
    # A suite reached only through one still needs a concrete runner or exclusion.
    if not isinstance(matrix, dict):
        return []
    if not isinstance(matrix.get("include", []), list):
        return []
    axes = {
        key: value if isinstance(value, list) else [value]
        for key, value in matrix.items()
        if key not in {"include", "exclude"}
    }
    if not axes:
        return matrix.get("include", [{}])
    rows = [dict(zip(axes, values)) for values in itertools.product(*axes.values())]
    rows = [
        row for row in rows if not any(
            all(row.get(key) == value for key, value in excluded.items())
            for excluded in matrix.get("exclude", [])
        )
    ]
    for included in matrix.get("include", []):
        compatible = [
            row for row in rows if all(
                key not in axes or row.get(key) == value
                for key, value in included.items()
            )
        ]
        if compatible:
            for row in compatible:
                row.update(included)
        else:
            rows.append(included.copy())
    return rows or [{}]


def expand_matrix(value, row):
    return MATRIX_VALUE.sub(lambda match: str(row.get(match[1], match[0])), value)


def manifest_at(directory):
    path = PurePosixPath(directory) / "package.json"
    return str(path)


def command_packages(command, directory, manifests, seen=None):
    """Follow test scripts, including npm workspace and prefix invocations."""
    seen = set() if seen is None else seen
    covered = set()
    # Shell comments and quoted messages are tokens, never runner declarations.
    segments = []
    for line in command.replace("\\\n", " ").splitlines():
        # Ignore embedded language/heredoc bodies; only executable shell lines
        # with a supported command can contribute coverage.
        if not re.match(
            r"\s*\(?\s*(?:[A-Za-z_]\w*=\S+\s+)*(?:npm|pnpm|yarn|cd)\s", line
        ):
            continue
        lexer = shlex.shlex(line, posix=True, punctuation_chars=";&|()")
        lexer.whitespace_split = True
        tokens = list(lexer)
        if "||" in tokens:
            continue
        segment = []
        for token in tokens + [";"]:
            if token and all(char in ";&|()" for char in token):
                if segment:
                    segments.append(segment)
                    segment = []
            else:
                segment.append(token)
    for words in segments:
        while words and re.match(r"^[A-Za-z_]\w*=", words[0]):
            words = words[1:]
        if not words:
            continue
        if words[0] == "cd" and len(words) == 2:
            directory = str(PurePosixPath(directory) / words[1])
            continue
        if words[0] not in {"npm", "pnpm", "yarn"}:
            continue
        words = words[1:]
        package_dir = directory
        workspace = None
        args = []
        index = 0
        while index < len(words):
            word = words[index]
            if word in {"--prefix", "--dir", "-C", "--workspace", "-w"}:
                if index + 1 >= len(words):
                    break
                value = words[index + 1]
                if word in {"--workspace", "-w"}:
                    workspace = value
                else:
                    package_dir = str(PurePosixPath(directory) / value)
                index += 2
                continue
            if word.startswith("--workspace="):
                workspace = word.split("=", 1)[1]
            else:
                args.append(word)
            index += 1
        if workspace:
            matches = [
                name for name, data in manifests.items()
                if data.get("name") == workspace
                or str(PurePosixPath(name).parent) == workspace
            ]
            if len(matches) != 1:
                continue
            package_dir = str(PurePosixPath(matches[0]).parent)
        name = manifest_at(package_dir)
        if name not in manifests or not args:
            continue
        scripts = manifests[name].get("scripts", {})
        if args[0] == "exec" and args[1:] == ["vitest", "run"]:
            if "vitest run" in scripts.get("test", ""):
                covered.add(name)
            continue
        script = (
            args[1] if args[0] in {"run", "run-script"} and len(args) > 1 else args[0]
        )
        if script not in scripts or (name, script) in seen:
            continue
        seen.add((name, script))
        if script == "test" or script.startswith("test:"):
            covered.add(name)
        covered.update(command_packages(scripts[script], package_dir, manifests, seen))
    return covered


def workflow_coverage(manifests):
    covered = set()
    make_commands = []
    for path in sorted((ROOT / ".github/workflows").glob("*.y*ml")):
        workflow = yaml.load(path.read_text(), Loader=yaml.BaseLoader)
        if not workflow or workflow.get("on") is None:
            continue
        workflow_directory = workflow.get("defaults", {}).get("run", {}).get(
            "working-directory", "."
        )
        for job in workflow.get("jobs", {}).values():
            if (
                job.get("if") in {"false", "${{ false }}"}
                or job.get("continue-on-error") == "true"
            ):
                continue
            job_directory = job.get("defaults", {}).get("run", {}).get(
                "working-directory", workflow_directory
            )
            for row in matrix_rows(job):
                for step in job.get("steps", []):
                    if (
                        "run" not in step or "if" in step
                        or step.get("continue-on-error") == "true"
                    ):
                        continue
                    command = expand_matrix(step["run"], row)
                    directory = expand_matrix(
                        step.get("working-directory", job_directory), row
                    )
                    covered.update(command_packages(command, directory, manifests))
                    if directory == ".":
                        make_commands.extend(re.findall(
                            r"^\s*make\s+([^\n;&|]+)", command, re.MULTILINE
                        ))
    # These workflow-reached targets invoke the package scripts through Make variables.
    makefile = (ROOT / "Makefile").read_text().replace("\\\n", " ")
    reached = {
        token for command in make_commands
        for token in re.findall(r"[A-Za-z0-9_][\w.-]*", command)
    }
    indirect = (
        (
            "developer-dashboard-static-test",
            "services/platform/hosted/dashboard/web/package.json",
            "developer-dashboard-static-test:",
            "npm --prefix services/platform/hosted/dashboard/web run test:static",
        ),
        (
            "programs-test", "programs/sdk/assemblyscript/package.json",
            "programs-sdk-assemblyscript:",
            "npm --prefix programs/sdk/assemblyscript run test:source",
        ),
    )
    for target, package, declaration, invocation in indirect:
        if target not in reached:
            continue
        if target == "programs-test":
            prerequisites = re.search(
                r"^programs-test:([^\n]+)", makefile, re.MULTILINE
            )
            assert prerequisites and "programs-sdk-assemblyscript" in prerequisites[1].split()
        recipe = re.search(
            r"^" + re.escape(declaration) + r"[^\n]*\n((?:\t[^\n]*\n)+)",
            makefile, re.MULTILINE,
        )
        assert recipe and invocation in recipe[1], f"Stale Make runner mapping: {target}"
        covered.update(command_packages(invocation, ".", manifests))
        assert package in covered
    return covered


def test_js_test_scripts_have_runner():
    manifests = tracked_manifests()
    packages = packages_with_tests(manifests)
    assert packages, "No tracked JavaScript test scripts found"
    covered = workflow_coverage(manifests)
    missing = packages - covered - exclusions().keys()
    assert not missing, (
        "JS test scripts without a workflow runner or reasoned exclusion:\n"
        + "\n".join(sorted(missing))
    )
    # Removing the real test steps must not leave installs/builds counting as coverage.
    assert not command_packages(
        "npm ci && npm run build", "apps/inference-peer", manifests
    )
    assert not command_packages(
        'echo "npm test" # npm test', "apps/inference-peer", manifests
    )
    assert not command_packages("npm test || true", "apps/inference-peer", manifests)
    assert "apps/inference-peer/package.json" in covered
    assert {
        "sdk/integrations/agents/package.json",
        "sdk/integrations/express/package.json",
        "sdk/integrations/express/example/package.json",
        "sdk/integrations/next/package.json",
        "sdk/integrations/next/example/package.json",
        "tools/codebase-map/package.json",
    } <= covered


def test_js_exclusion_list_entries_exist():
    manifests = tracked_manifests()
    packages = packages_with_tests(manifests)
    recorded = exclusions()
    assert recorded, "Live-service suites must have explicit exclusions"
    for name, reason in recorded.items():
        assert name in packages, f"Stale exclusion (not a tracked test package): {name}"
        assert (ROOT / name).is_file(), name
        assert len(reason.split()) >= 8, f"Explain the external dependency for {name}"
    perps = (ROOT / "apps/perps/tests/e2e-local-node.test.ts").read_text()
    assert all(key in perps for key in (
        "LOCAL_NODE_RPC", "LOCAL_NODE_KEY", "NEXT_PUBLIC_PERPS_GATEWAY_URL"
    ))
    assert "apps/perps/package.json" in recorded
    assert "tests/integration/rpc_tests/package.json" in recorded
    assert "test:portable" in manifests["tools/codebase-map/package.json"]["scripts"]
