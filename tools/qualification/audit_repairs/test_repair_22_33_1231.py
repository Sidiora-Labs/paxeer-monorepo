"""Offline source proof of the production devtools installation configuration.

Parse Make assignments, dependencies and shell words without invoking Make, Go,
the shell or a network service. Compilation and installed-tool execution remain
review-only, as required by this task's qualification environment.
"""

import ast
from pathlib import Path
import re
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEVTOOLS = "wasm/contrib/devtools/"
RECORD = Path(__file__).with_name("repair_22_33_1231.kvx")
TOOLS = (
    ("statik", "STATIK", "github.com/rakyll/statik", "v0.1.8"),
    ("runsim", "RUNSIM", "github.com/cosmos/tools/cmd/runsim", "v1.0.0"),
    (
        "golangci-lint",
        "GOLANGCI_LINT",
        "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
        "v2.5.0",
    ),
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def make_assignments():
    """Read the single-line variable declarations used by the installer."""
    return {
        name: (operator, value)
        for name, operator, value in re.findall(
            r"^([A-Z][A-Z0-9_]*)\s*(\?=|:=|=)\s*(.*?)\s*$",
            source(DEVTOOLS + "Makefile"),
            re.M,
        )
    }


def make_rules(path=DEVTOOLS + "Makefile"):
    """Parse explicit one-line targets and their tab-indented recipes."""
    rules = {}
    current = None
    for line in source(path).splitlines():
        if line.startswith("\t"):
            if current is not None and not line.lstrip().startswith("#"):
                rules[current][1].append(line[1:])
            continue
        match = re.fullmatch(r"([^\s:#=]+):\s*(.*)", line)
        if match:
            current, dependencies = match.groups()
            assert current not in rules, current
            rules[current] = (dependencies.split(), [])
        elif line.strip() and not line.startswith("#"):
            current = None
    return rules


@pytest.mark.parametrize("name,variable,package,version", TOOLS)
def test_finding_17_pinned_install_recipes(name, variable, package, version):
    rules = make_rules()
    assert rules[name] == ([f"$({variable})"], [])
    dependencies, recipes = rules[f"$({variable})"]
    assert dependencies == []
    assert len(recipes) == 2
    assert shlex.split(recipes[0].removeprefix("@")) == [
        "echo", f"Installing {name}...",
    ]
    # Exact shell words also reject fallbacks, ignored errors, unpinned installs,
    # additional manifest-mutating commands, and an ambient GOBIN or Go binary.
    assert shlex.split(recipes[1].removeprefix("@")) == [
        "GOBIN=$(abspath $(TOOLS_DESTDIR))",
        "$(GO)",
        "install",
        f"{package}@{version}",
    ]
    assert recipes[1].startswith('@GOBIN="$(abspath $(TOOLS_DESTDIR))" "$(GO)" ')


def test_finding_17_destination_matches_binary_prerequisites():
    assignments = make_assignments()
    assert assignments["GOPATH"] == ("?=", "$(shell $(GO) env GOPATH)")
    assert assignments["TOOLS_DESTDIR"] == ("?=", "$(GOPATH)/bin")
    for name, variable, _, _ in TOOLS:
        assert assignments[variable] == ("=", f"$(TOOLS_DESTDIR)/{name}")
    # abspath in every recipe is the Make operation that accepts both relative
    # and absolute overrides while satisfying Go's absolute-GOBIN requirement.
    recipes = [make_rules()[f"$({variable})"][1][1] for _, variable, _, _ in TOOLS]
    assert all('GOBIN="$(abspath $(TOOLS_DESTDIR))"' in recipe for recipe in recipes)


def test_finding_17_tools_target_and_cleanup_share_destinations():
    rules = make_rules()
    assert rules["tools"] == (["tools-stamp"], [])
    assert rules["tools-stamp"] == (
        [name for name, _, _, _ in TOOLS], ["touch $@"],
    )
    dependencies, cleanup = rules["tools-clean"]
    assert dependencies == []
    assert [shlex.split(command) for command in cleanup] == [
        ["rm", "-f", "$(STATIK)", "$(GOLANGCI_LINT)", "$(RUNSIM)"],
        ["rm", "-f", "tools-stamp"],
    ]


def test_finding_17_obsolete_get_workaround_is_removed():
    makefile = source(DEVTOOLS + "Makefile")
    assert not re.search(r"\bgo\s+get\b|/tmp|30515|polute", makefile)
    install_recipes = [
        recipe
        for _, recipes in make_rules().values()
        for recipe in recipes
        if " install " in recipe
    ]
    assert len(install_recipes) == len(TOOLS)
    for recipe in install_recipes:
        assert "cd " not in recipe
        assert "go.mod" not in recipe and "go.sum" not in recipe


def test_finding_17_supported_go_and_linter_consumer():
    assert re.findall(r"^go\s+(\S+)$", source("go.mod"), re.M) == ["1.25.8"]
    assert re.findall(r'^version:\s*"(\d+)"$', source(".golangci.yml"), re.M) == ["2"]
    wasm_makefile = source("wasm/Makefile")
    assert ".DEFAULT_GOAL := help" in wasm_makefile
    assert "$(error wasm/Makefile is retired; run make paxd-build from the repository root; use help for module developer commands)" in wasm_makefile
    assert "CGO_ENABLED=1 go test -mod=readonly ./wasm/x/wasm/..." in wasm_makefile
    assert not re.findall(r"^include |^lint:", wasm_makefile, re.M)


def test_finding_17_readme_matches_real_installation_path():
    readme = source(DEVTOOLS + "README.md")
    rows = re.findall(r"^\| `([^`]+)` \| `([^`]+)` \| `([^`]+)` \|$", readme, re.M)
    assert rows == [(name, package, version) for name, _, package, version in TOOLS]
    for fact in (
        "Go 1.25.8", "go install package@version", "v2 module and configuration format",
        "Go 1.25 support starting with v2.4.0", "`TOOLS_DESTDIR` defaults to `$(GOPATH)/bin`",
        "sets `GOBIN` to the absolute", "path of `TOOLS_DESTDIR`",
        "project's `go.mod` and `go.sum` unchanged", "no temporary working directory",
        "Go module proxy or a populated module cache", "Existing binaries satisfy Make's prerequisites",
        "using the same destination", "through `PATH`",
    ):
        assert fact in readme, fact
    examples = re.findall(r"```sh\n(.*?)\n```", readme, re.S)
    assert len(examples) == 1
    assert [shlex.split(line) for line in examples[0].splitlines()] == [
        ["make", "-C", "wasm/contrib/devtools", "tools", "TOOLS_DESTDIR=$PWD/.devtools/bin"],
        ["export", "PATH=$PWD/.devtools/bin:$PATH"],
    ]
    assert 'make -C wasm/contrib/devtools tools-clean TOOLS_DESTDIR="$PWD/.devtools/bin"' in readme
    assert "not a working installer" not in readme


def test_repair_record_maps_finding_17_to_proof_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["meta"]["task"] == "22.33.1231"
    assert record["meta"]["acceptance"] == "22998.1"
    finding = record["finding"]["17"]
    assert finding["audit"] == "22.30.59"
    assert finding["file"] == DEVTOOLS + "Makefile"
    assert finding["class"] == "config-drift" and finding["severity"] == "medium"
    declared_tests = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared_tests
    assert record["qualification"]["build_cmd"] == "python3 --version"
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1231.py"
    )
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in record["qualification"]["evidence"]
    assert "No pre-change verification was run" in record["baseline"]["verification"]
    assert "not executed" in record["proof"]["review_only"]
    assert record["meta"]["wiring_requests"] == []
