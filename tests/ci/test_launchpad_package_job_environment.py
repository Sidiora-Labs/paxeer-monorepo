"""Package producers receive the launchpad integration services and tools."""

from functools import lru_cache
import json
from pathlib import Path
import re

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/launchpad-test.yml"
AGENT_KIT = "sdk/launchpad/packages/agent-kit"
GO_TOKEN = re.compile(
    r'//[^\n]*|/\*.*?\*/|"(?:\\.|[^"\\])*"|`[^`]*`|'
    r"'(?:\\.|[^'\\])*'|[A-Za-z_]\w*|[^\s]",
    re.DOTALL,
)


def launchpad_workflow():
    workflow = yaml.safe_load(WORKFLOW.read_text())
    assert {"push", "pull_request"} <= workflow[True].keys()
    return workflow


@lru_cache(maxsize=None)
def module_directories():
    manifests = list((ROOT / "services/launchpad").glob("*/go.mod"))
    manifests.extend((ROOT / "sdk/launchpad").glob("*/go.mod"))
    return {
        re.search(r"^module\s+(\S+)", path.read_text(), re.MULTILINE)[1]: path.parent
        for path in manifests
    }


def local_imports(source):
    imports = {}
    for block in re.findall(r"(?m)^import\s*\((.*?)^\)", source, re.DOTALL):
        for alias, name in re.findall(r'(\w+)?\s*"([^"\n]+)"', block):
            for module, directory in module_directories().items():
                if name == module or name.startswith(module + "/"):
                    imports[alias or name.rsplit("/", 1)[-1]] = (
                        directory / name[len(module):].lstrip("/")
                    )
    return imports


def go_functions(source):
    tokens = [
        match for match in GO_TOKEN.finditer(source)
        if not match[0].startswith(("//", "/*"))
    ]
    for index, token in enumerate(tokens):
        if token[0] != "func" or index + 2 >= len(tokens):
            continue
        name = tokens[index + 1][0]
        if not re.fullmatch(r"\w+", name):
            continue
        opening = index + 2
        while opening < len(tokens) and tokens[opening][0] != "{":
            opening += 1
        depth = 0
        for closing in range(opening, len(tokens)):
            depth += (tokens[closing][0] == "{") - (tokens[closing][0] == "}")
            if depth == 0:
                yield name, source[token.start():tokens[closing].end()]
                break


@lru_cache(maxsize=None)
def package_functions(directory):
    declarations = {}
    for path in sorted(directory.glob("*.go")):
        if not (path.name.endswith("_test.go") or directory.name.endswith("test")):
            continue
        source = path.read_text()
        for name, body in go_functions(source):
            declarations.setdefault(name, []).append((source, body))
    return declarations


def environment_reads(source, context):
    constants = dict(re.findall(r'\b(\w+)\s*(?:string\s*)?=\s*"([A-Z][A-Z0-9_]+)"', context))
    names = set()
    for argument in re.findall(r"os\.(?:Getenv|LookupEnv)\(\s*([^()]+?)\s*\)", source):
        if argument.startswith('"'):
            names.add(json.loads(argument))
        elif argument in constants:
            names.add(constants[argument])
        elif "." in argument:
            alias, name = argument.split(".", 1)
            directory = local_imports(context)[alias]
            imported = "\n".join(path.read_text() for path in directory.glob("*.go"))
            names.add(re.search(rf'\b{re.escape(name)}\s*=\s*"([^"]+)"', imported)[1])
    names.update(re.findall(r'(?:requireEnv|fixtureEnv)\(\s*(?:t,\s*)?"([A-Z][A-Z0-9_]+)"', source))
    return names


@lru_cache(maxsize=None)
def producer_inventory(package):
    if package == AGENT_KIT:
        # The local journey imports wallet gateway helpers outside this package;
        # those helpers start their own database and build Go attestors with TLS.
        return set(), {"go", "anvil", "initdb", "pg_ctl", "openssl", "node", "pnpm"}
    directory = ROOT / package
    sources = {
        path: path.read_text()
        for name in ("test", "tests")
        for path in sorted((directory / name).rglob("*"))
        if path.is_file() and path.suffix in {".go", ".txt", ".mjs", ".ts", ".tsx"}
    }
    overlays = {}
    for source in sources.values():
        for module, target, helper in re.findall(
            r"(?:startProducer|readProducerCases)\('([^']+)',\s*'([^']+)',\s*'([^']+)'",
            source,
        ):
            overlays[(directory / "test" / helper).resolve()] = (
                ROOT / "services/launchpad" / module / target
            ).resolve()
    pending = [(overlays.get(path.resolve(), path.parent), source, source)
               for path, source in sources.items()]
    visited = set()
    names = set()
    programs = set()
    while pending:
        cwd, context, body = pending.pop()
        key = cwd, body
        if key in visited:
            continue
        visited.add(key)
        names.update(environment_reads(body, context))
        programs.update(re.findall(r'exec\.Command(?:Context)?\(\s*(?:ctx,\s*)?"([^"\n]+)"', body))
        programs.update(re.findall(r"(?:spawn|execFile|run)\)?\(\s*['\"]([^'\"]+)['\"]", body))
        for name in re.findall(r"(?<![\w.])(\w+)\s*\(", body):
            for helper_context, helper in package_functions(cwd).get(name, []):
                pending.append((cwd, helper_context, helper))
        for alias, directory in local_imports(context).items():
            for name in re.findall(rf"\b{re.escape(alias)}\.(\w+)\s*\(", body):
                for helper_context, helper in package_functions(directory).get(name, []):
                    pending.append((directory, helper_context, helper))
    supplied = set()
    for source in sources.values():
        supplied.update(re.findall(r"\b([A-Z][A-Z0-9_]+)\s*:", source))
        supplied.update(re.findall(r'\.Setenv\("([A-Z][A-Z0-9_]+)"', source))
        supplied.update(re.findall(r'"([A-Z][A-Z0-9_]+)=', source))
    return names - supplied - {"HOME", "PATH"}, programs


def package_condition_matches(step, package):
    if "if" not in step:
        return True
    for term in step["if"].strip().split(" || "):
        equality = re.fullmatch(r"matrix.package == '([^']+)'", term.strip())
        if equality:
            if equality[1] == package:
                return True
            continue
        membership = re.fullmatch(
            r"contains\(fromJSON\('([^']+)'\), matrix.package\)", term.strip()
        )
        assert membership, step["if"]
        if package in json.loads(membership[1]):
            return True
    return False


def client_packages():
    matrix = launchpad_workflow()["jobs"]["launchpad-test"]["strategy"]["matrix"]
    return [package for package in matrix["package"] if package.startswith("sdk/")]


def test_package_job_shares_go_services_and_environment():
    jobs = launchpad_workflow()["jobs"]
    package_job, go_job = jobs["launchpad-test"], jobs["launchpad-go-test"]
    assert package_job["services"] is go_job["services"]
    assert package_job["env"] is go_job["env"]
    for package in client_packages():
        variables, _ = producer_inventory(package)
        for name in variables:
            assert name in go_job["env"], (package, name)
            assert package_job["env"][name] == go_job["env"][name], (package, name)


def test_producer_environment_inventory_includes_database_helpers():
    variables = set().union(*(producer_inventory(package)[0] for package in client_packages()))
    assert variables
    assert {"STATS_TEST_POSTGRES_DSN", "USER_TEST_POSTGRES_DSN"} <= variables


@pytest.mark.parametrize("package", client_packages())
def test_package_tools_and_database_precede_package_cases(package):
    jobs = launchpad_workflow()["jobs"]
    steps = jobs["launchpad-test"]["steps"]
    variables, programs = producer_inventory(package)
    test = next(step for step in steps if step["name"] == "Test package")
    setup = next(step for step in steps if step.get("uses", "").startswith("actions/setup-go@"))
    assert package_condition_matches(setup, package) == ("go" in programs)
    assert steps.index(setup) < steps.index(test)
    database = next(step for step in steps if step["name"] == "Create the pool test database")
    needs_database = any("POSTGRES" in name or "DATABASE_URL" in name for name in variables)
    assert package_condition_matches(database, package) == needs_database
    go_database = next(step for step in jobs["launchpad-go-test"]["steps"] if step["name"] == database["name"])
    assert database["run"] == go_database["run"]
    assert database["env"] is go_database["env"]
    assert steps.index(database) < steps.index(test)
    storage = next(step for step in steps if step["name"] == "Start object storage fixture")
    assert package_condition_matches(storage, package) == ("minio" in programs)
    install = 'GOBIN="$RUNNER_TEMP/launchpad-fixture-bin" GOFLAGS= go install github.com/minio/minio@'
    publish = 'echo "$RUNNER_TEMP/launchpad-fixture-bin" >> "$GITHUB_PATH"'
    assert install in storage["run"]
    assert publish in storage["run"]
    assert storage["run"].index(install) < storage["run"].index(publish)
    go_storage = next(step for step in jobs["launchpad-go-test"]["steps"] if step["name"] == storage["name"])
    assert storage["run"] == go_storage["run"]
    assert steps.index(setup) < steps.index(storage) < steps.index(test)
    scanner = next(step for step in steps if step["name"] == "Install the users scanner")
    assert package_condition_matches(scanner, package) == ("clamd" in programs)
    assert "clamav-daemon" in scanner["run"]
    assert steps.index(scanner) < steps.index(test)
    postgres = next(step for step in steps if step["name"] == "Install PostgreSQL for the candles producer")
    assert package_condition_matches(postgres, package) == ("pg_config" in programs)
    assert "postgresql" in postgres["run"]
    assert steps.index(postgres) < steps.index(test)


def test_agent_kit_fixture_steps_are_isolated_and_ordered():
    workflow = launchpad_workflow()
    job = workflow["jobs"]["launchpad-test"]
    steps = job["steps"]
    packages = job["strategy"]["matrix"]["package"]
    assert AGENT_KIT in packages
    assert producer_inventory(AGENT_KIT) == (
        set(), {"go", "anvil", "initdb", "pg_ctl", "openssl", "node", "pnpm"},
    )
    (go,) = [step for step in steps if step.get("uses", "").startswith("actions/setup-go@")]
    assert package_condition_matches(go, AGENT_KIT)
    (foundry,) = [
        step for step in steps
        if step.get("uses", "").startswith("foundry-rs/foundry-toolchain@")
    ]
    assert foundry["with"] == {"version": "v1.8.1"}
    for package in packages:
        assert package_condition_matches(foundry, package) == (
            package in {AGENT_KIT, "apps/launchpad/web"}
        )

    names = (
        "Install PostgreSQL fixture binaries for the agent kit",
        "Install agent SDK dependencies", "Build agent SDK",
        "Install wallet SDK dependencies", "Install wallet workspace dependencies",
    )
    selected = {}
    for name in names:
        (step,) = [step for step in steps if step.get("name") == name]
        selected[name] = step
        for package in packages:
            assert package_condition_matches(step, package) == (package == AGENT_KIT)
        assert "continue-on-error" not in step

    postgres = selected[names[0]]
    assert postgres["run"].splitlines() == [
        'bash "$GITHUB_WORKSPACE/scripts/ci/install-system-packages.sh" '
        '--yes --no-install-recommends postgresql openssl',
        'echo "PG_BIN_DIR=$(ls -d /usr/lib/postgresql/*/bin | sort -V | tail -1)" '
        '>> "$GITHUB_ENV"',
    ]
    gateway = yaml.safe_load(
        (ROOT / ".github/workflows/wallet-test.yml").read_text()
    )["jobs"]["gateway"]
    for name in (names[1], names[2]):
        (reference,) = [step for step in gateway["steps"] if step.get("name") == name]
        assert selected[name]["run"] == reference["run"]
        assert selected[name]["working-directory"] == reference["working-directory"]
    for name, directory, options in (
        (names[3], "sdk/wallet", "--ignore-workspace --frozen-lockfile"),
        (names[4], "services/wallet", "--frozen-lockfile"),
    ):
        step = selected[name]
        assert step["working-directory"] == directory
        manifest = json.loads((ROOT / directory / "package.json").read_text())
        manager = manifest["packageManager"].split("+", 1)[0]
        assert manager == "pnpm@10.34.9"
        assert step["run"] == f"npx --yes {manager} install {options}"

    def named(name):
        (step,) = [step for step in steps if step.get("name") == name]
        return step

    ordered = [
        named("Set up Node.js"), named("Install the workspace package manager"),
        named("Install workspace dependencies"), foundry, go,
        *(selected[name] for name in names),
        named("Build SDK packages"), named("Test package"),
    ]
    positions = [steps.index(step) for step in ordered]
    assert positions == sorted(set(positions))
    assert job["defaults"]["run"]["working-directory"] == "apps/launchpad"
    assert named("Build SDK packages")["run"] == "pnpm run build:packages"
