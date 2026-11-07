import ast
from dataclasses import dataclass, field
from pathlib import Path
import re
import shlex
import subprocess

import yaml


ROOT = Path(__file__).resolve().parents[2]
MAKEFILES = (ROOT / "Makefile", ROOT / "services/platform/Makefile.inc")
WORKFLOWS = (
    ROOT / ".github/workflows/platform.yml",
    ROOT / ".github/workflows/human.yml",
)
BETA_SCRIPT = ROOT / "services/platform/hosted/tests/beta-cluster.sh"
VARIABLE = re.compile(r"\$\(([A-Za-z_][A-Za-z_0-9]*)\)")
ASSIGNMENT = re.compile(r"^([A-Za-z_][A-Za-z_0-9]*)\s*([:+?]?=)\s*(.*)$")


@dataclass
class Rule:
    prerequisites: list[str] = field(default_factory=list)
    recipe: list[str] = field(default_factory=list)


def expand_variables(value, variables):
    for _ in range(len(variables) + 1):
        expanded = VARIABLE.sub(
            lambda match: variables.get(match[1], match[0]), value
        )
        if expanded == value:
            return value
        value = expanded
    raise AssertionError(f"Recursive Make variable in {value!r}")


def make_rules():
    rules = {}
    variables = {"CURDIR": "."}
    for path in MAKEFILES:
        current = []
        for line in path.read_text(encoding="utf-8").replace("\\\n", " ").splitlines():
            if line.startswith("\t"):
                for target in current:
                    rules[target].recipe.append(line.strip())
                continue
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            current = []
            assignment = ASSIGNMENT.match(line)
            if assignment:
                name, operator, value = assignment.groups()
                if operator == "+=":
                    variables[name] = variables.get(name, "") + " " + value
                elif operator != "?=" or name not in variables:
                    variables[name] = value
                continue
            header = re.match(r"^([^\s:][^:]*):\s*(.*)$", line)
            if not header or re.match(r"(?:export\s+)?\w+\s*[:+?]?=", header[2]):
                continue
            targets = expand_variables(header[1], variables).split()
            for target in targets:
                rule = rules.setdefault(target, Rule())
                rule.prerequisites.extend(header[2].split())
            current = targets
    for rule in rules.values():
        rule.recipe = [expand_variables(line, variables) for line in rule.recipe]
        rule.prerequisites = expand_variables(
            " ".join(rule.prerequisites), variables
        ).split()
    return rules


def shell_make_targets(script):
    lexer = shlex.shlex(
        script.replace("\\\n", " "), posix=True, punctuation_chars=";&|()<>\n"
    )
    lexer.whitespace = " \t\r"
    lexer.whitespace_split = True
    words = list(lexer)
    for index, word in enumerate(words):
        # A quoted bash -c body is itself a shell program.
        if "\n" in word and re.search(r"\bmake[ \t]", word):
            yield from shell_make_targets(word)
        if word != "make":
            continue
        for position in range(index + 1, len(words)):
            argument = words[position]
            if argument.startswith("-") or "=" in argument:
                continue
            if all(character in ";&|()<>\n" for character in argument):
                break
            if argument.isdecimal() and position + 1 < len(words):
                if words[position + 1].startswith((">", "<")):
                    break
            assert re.fullmatch(r"[A-Za-z_0-9./+-]+", argument), argument
            yield argument


def workflow_targets():
    targets = set()
    for path in WORKFLOWS:
        workflow = yaml.safe_load(path.read_text(encoding="utf-8"))
        reached = {
            target
            for job in workflow["jobs"].values()
            for step in job.get("steps", [])
            for target in shell_make_targets(step.get("run", ""))
        }
        assert reached, f"No Make targets found in {path}"
        targets.update(reached)
    return targets


def repository_paths(recipe):
    # Build/install products (build/, node_modules/, etc.) are not repository
    # inputs. Derive the source roots from Git rather than allowlisting scripts.
    roots = subprocess.run(
        ["git", "ls-tree", "--name-only", "HEAD"],
        cwd=ROOT,
        check=True,
        text=True,
        capture_output=True,
    ).stdout.splitlines()
    pattern = re.compile(
        r"(?<![\w./-])(?:\./)?(?:"
        + "|".join(re.escape(name) for name in roots)
        + r")(?:/[A-Za-z_0-9.+-]+)*(?![\w./-])"
    )
    paths = {match[0].removeprefix("./") for match in pattern.finditer(recipe)}
    # Cargo output directories may also sit inside a source root. They are
    # produced by the owning manifest, and need not exist before a build.
    return {
        path
        for path in paths
        if not any(
            directory.name == "target"
            and (directory.parent / "Cargo.toml").is_file()
            for directory in (ROOT / path, *(ROOT / path).parents)
        )
    }


def assert_recipe_paths_exist(targets, rules):
    checked = set()
    missing = []
    for target in sorted(targets):
        assert target in rules, f"Workflow calls undefined Make target {target}"
        for path in repository_paths("\n".join(rules[target].recipe)):
            checked.add(path)
            if not (ROOT / path).exists():
                missing.append(f"{target}: {path}")
    assert checked, "No repository paths checked in Make recipes"
    assert not missing, "Make recipes name missing paths:\n" + "\n".join(missing)
    return checked


def test_workflow_reached_recipes_name_existing_paths():
    targets = workflow_targets()
    assert {
        "platform-test-tooling",
        "platform-verify-sdks",
        "platform-beta-cluster-up",
        "build/bin/layerx-genesis-build",
    } <= targets
    assert_recipe_paths_exist(targets, make_rules())


def test_platform_test_tooling_paths_exist():
    rules = make_rules()
    paths = assert_recipe_paths_exist({"platform-test-tooling"}, rules)
    assert "services/platform/hosted/tests/beta-cluster.sh" in paths
    assert "tools/platform/cli/tests/clean-bootstrap.sh" in paths
    retired = {
        "scripts/ci/beta-contract-check.sh",
        "services/platform/hosted/paxeer/settlement-domain.py",
        "services/platform/hosted/paxeer/prepare-beta.py",
    }
    assert not paths.intersection(retired)


def test_platform_verify_sdks_prerequisites_exist():
    rules = make_rules()
    prerequisites = set(rules["platform-verify-sdks"].prerequisites)
    assert {
        "programs-check-native-lifecycle-fixtures",
        "programs-check-capability-fixture",
        "platform-sdk-check",
    } <= prerequisites
    assert "programs-executed-fixture" not in rules
    assert "programs-check-executed-fixture" not in rules
    assert "programs-check-executed-fixture" not in prerequisites
    for prerequisite in prerequisites:
        assert prerequisite in rules or (ROOT / prerequisite).exists(), prerequisite
    assert_recipe_paths_exist(prerequisites, rules)


def test_beta_environment_script_loads_existing_modules():
    script = BETA_SCRIPT.read_text(encoding="utf-8")
    variables = {"REPO_ROOT": str(ROOT), "SCRIPT_DIR": str(BETA_SCRIPT.parent)}
    shell_variable = re.compile(r"\$([A-Za-z_][A-Za-z_0-9]*)\b")
    for name, value in re.findall(r'^(\w+)="([^"\n]+)"$', script, re.M):
        variables[name] = shell_variable.sub(
            lambda match: variables.get(match[1], match[0]), value
        )
    checked = set()
    for source in re.findall(r'^\s*(?:source|\.)\s+"([^"\n]+)"', script, re.M):
        path = shell_variable.sub(
            lambda match: variables.get(match[1], match[0]), source
        )
        assert "$" not in path, f"Unresolved shell module: {source}"
        if Path(path).is_relative_to(ROOT / "build"):
            # Runtime configuration is generated by the script, rather than
            # imported as a repository module. Require its real writer.
            assert re.search(r'>+\s*"' + re.escape(source) + '"', script), source
            continue
        assert Path(path).is_file(), f"Missing shell module: {source}"
        checked.add(path)
    heredocs = re.findall(
        r"<<'(?P<delimiter>PY\w*)'\n(?P<body>.*?)\n(?P=delimiter)(?=\n)",
        script,
        re.S,
    )
    assert heredocs, "No embedded Python programs inspected"
    python_modules = set()
    for delimiter, body in heredocs:
        tree = ast.parse(body, filename=f"{BETA_SCRIPT}:{delimiter}")
        for node in ast.walk(tree):
            if not isinstance(node, ast.Call) or not isinstance(node.func, ast.Name):
                continue
            if node.func.id not in {"module", "load"} or len(node.args) != 2:
                continue
            relative = node.args[1]
            assert isinstance(relative, ast.Constant) and isinstance(relative.value, str)
            assert (ROOT / relative.value).is_file(), relative.value
            python_modules.add(relative.value)
    assert "tests/support/lxgb_metadata.py" in python_modules
    assert "services/platform/hosted/node/genesis_fees.py" in python_modules
    assert str(BETA_SCRIPT.parent / "beta-native.sh") in checked
    assert "native_cluster_up \"$1\"" in script
