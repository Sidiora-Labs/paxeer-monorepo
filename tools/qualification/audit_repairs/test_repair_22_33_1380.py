"""Offline source contracts for the SDK Makefile and network overview.

These checks parse production sources without executing Make, Go, Git, Bash or
Docker. Actual tool execution is a review-only limit recorded beside the tests.
"""

from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
SDK = ROOT / "sdk"
MAKEFILE = SDK / "Makefile"
RECORD = Path(__file__).with_name("repair_22_33_1380.kvx")
MODULE = re.search(r"^module (\S+)$", (ROOT / "go.mod").read_text(), re.M)[1]


def logical_lines():
    return MAKEFILE.read_text().replace("\\\n", " ").splitlines()


def rule(target):
    lines = logical_lines()
    headers = [i for i, line in enumerate(lines) if line.startswith(target + ":")]
    assert len(headers) == 1, target
    index = headers[0]
    prerequisites = lines[index].split(":", 1)[1].strip()
    commands = []
    for line in lines[index + 1 :]:
        if not line.startswith("\t"):
            break
        commands.append(line.strip().removeprefix("@"))
    return prerequisites, commands


def assignment(name, operator="="):
    values = [
        match[1]
        for line in logical_lines()
        if (match := re.fullmatch(re.escape(name) + r"\s*" + re.escape(operator) + r"\s*(.*)", line))
    ]
    assert len(values) == 1, name
    return values[0]


def shell_tokens(command):
    lexer = shlex.shlex(command, posix=True, punctuation_chars=";&<>")
    lexer.whitespace_split = True
    return list(lexer)


def test_sdk_make_module_root():
    assert assignment("BUILD_TARGETS", ":=").split() == ["build", "install"]
    prerequisites, commands = rule("$(BUILD_TARGETS)")
    assert prerequisites.split() == ["../go.sum", "$(BUILDDIR)/"]
    assert (SDK / prerequisites.split()[0]).resolve() == ROOT / "go.sum"
    assert (ROOT / "go.sum").is_file()
    assert not (SDK / "go.sum").exists()
    assert not (SDK / "go.mod").exists()
    assert commands == ["go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./..."]
    prerequisites, commands = rule("../go.sum")
    assert prerequisites == "../go.mod"
    assert (SDK / prerequisites).resolve() == ROOT / "go.mod"
    assert commands == [
        'echo "Ensure dependencies have not been modified ..." >&2',
        "cd .. && go mod verify",
        "cd .. && go mod tidy",
    ]
    assert "build: BUILD_ARGS=-o $(BUILDDIR)/" in logical_lines()
    assert "BUILDDIR ?= $(CURDIR)/build" in logical_lines()
    assert rule("$(BUILDDIR)/")[1] == ["mkdir -p $(BUILDDIR)/"]
    assert (SDK / "contrib/devtools/Makefile").is_file()
    assert "include contrib/devtools/Makefile" in logical_lines()
    readme = (SDK / "README.md").read_text()
    for target in ("build", "install"):
        assert f"make -C sdk {target}" in readme


def test_sdk_linker_flags_reach_declared_fork_variables():
    flags = shlex.split(assignment("ldflags"))
    expected = {
        f"{MODULE}/sdk/version.Name": "sim",
        f"{MODULE}/sdk/version.AppName": "simd",
        f"{MODULE}/sdk/version.Version": "$(VERSION)",
        f"{MODULE}/sdk/version.Commit": "$(COMMIT)",
        f"{MODULE}/sdk/version.BuildTags": "$(build_tags_comma_sep)",
        f"{MODULE}/consensus/version.TMVersion": "$(TMVERSION)",
    }
    assert flags[::2] == ["-X"] * len(expected)
    assert dict(flag.split("=", 1) for flag in flags[1::2]) == expected
    for symbol in expected:
        package, name = symbol.rsplit(".", 1)
        directory = ROOT / package.removeprefix(MODULE + "/")
        declarations = "\n".join(
            path.read_text() for path in directory.glob("*.go")
            if not path.name.endswith("_test.go")
        )
        assert re.search(r"^\s*(?:var\s+)?" + name + r"\s*=", declarations, re.M)
    backend_flags = re.findall(r"ldflags \+= -X (\S+)", MAKEFILE.read_text())
    assert backend_flags == [
        f"{MODULE}/sdk/types.DBBackend={backend}"
        for backend in ("cleveldb", "badgerdb", "rocksdb", "boltdb")
    ]
    assert re.search(r"^\s*DBBackend =", (SDK / "types/utils.go").read_text(), re.M)
    assert assignment("TMVERSION", ":=") == (
        "$(shell sed -n 's/^[[:space:]]*TMVersionDefault = \"\\(.*\\)\"/\\1/p' "
        "../consensus/version/version.go)"
    )
    consensus = (ROOT / "consensus/version/version.go").read_text()
    assert re.search(r'^\s*TMVersionDefault = "[^"\n]+"$', consensus, re.M)
    assert "var TMVersion = TMVersionDefault" in consensus


def test_swagger_clean_check_source_contract():
    prerequisites, commands = rule("update-swagger-docs")
    assert prerequisites == "statik"
    assert len(commands) == 2
    assert shlex.split(commands[0]) == [
        "STATIK=$(STATIK)", "sh", "./scripts/update-swagger-ui-statik.sh",
    ]
    wrapper = (SDK / "scripts/update-swagger-ui-statik.sh").read_text()
    assert "set -eu" in wrapper
    assert 'exec sh "$SCRIPT_DIR/../../scripts/update-swagger-ui-statik.sh" --assets-only "$@"' in wrapper
    assert "statik -src" not in wrapper
    assert (ROOT / "docs/swagger-ui").is_dir()
    assert (SDK / "client/docs/statik/statik.go").is_file()
    # HEAD checks staged and unstaged tracked generation output. The literal
    # command is passed to the shell, never consumed as an undefined Make name.
    tokens = shell_tokens(commands[1])
    assert tokens[:11] == [
        "if", "git", "diff", "--quiet", "HEAD", "--",
        "../docs/swagger/statik.go", "client/docs/statik",
        ";", "then", "echo",
    ]
    assert tokens[11:] == [
        "\\033[92mSwagger docs are in sync\\033[0m", ";", "else", "echo",
        "\\033[91mSwagger docs are out of sync!!!\\033[0m", ">&", "2", ";",
        "exit", "1", ";", "fi",
    ]
    assert "$(git" not in commands[1]
    assert "|| true" not in commands[1]


@pytest.mark.parametrize("target,fix", [("lint", False), ("lint-fix", True)])
def test_markdown_container_image(target, fix):
    _, commands = rule(target)
    command = commands[-1]
    references = {
        name: assignment(name)
        for name in (
            "containerMarkdownLintImage", "containerMarkdownLint",
            "containerMarkdownLintFix",
        )
    }
    references["CURDIR"] = str(SDK)
    expanded = re.sub(
        r"\$[({]([A-Za-z][A-Za-z0-9_]*)[)}]",
        lambda match: references[match[1]], command,
    ).replace("$$", "$")
    assert assignment("containerMarkdownLintImage") == "tmknom/markdownlint"
    name = assignment("containerMarkdownLintFix" if fix else "containerMarkdownLint")
    tokens = shell_tokens(expanded)
    start = tokens.index("else") + 1
    finish = tokens.index(";", start)
    assert tokens[start:finish] == [
        "docker", "run", "--name", name, "-i", "-v", f"{SDK}:/work",
        "tmknom/markdownlint", *([".", "--fix"] if fix else []),
    ]
    assert ["docker", "start", "-a", name] == tokens[tokens.index("then") + 1:start - 2]
    assert f"^{name}$" in tokens
    assert "$(markdownLintImage)" not in command


def test_sdk_overview_one_network_native_path_and_endpoints():
    readme = (SDK / "README.md").read_text()
    for fact in (
        "one BFT network", "EVM chain ID `125` (`0x7d`)",
        "kernel runs inside `paxd`", "block covering a transaction is committed",
        "EIP-1559 (type 2)", "eth_sendRawTransaction", "pax_sequencerReceipt",
        "`submitted`, `receipted` and `committed`", "independently trusted receipt keys",
        "committed failure remains a failure", "root HTTPS", "root WebSocket",
    ):
        assert fact in readme
    for retired in ("two execution domains", "layerxd", "/evm", "/kernel", "lx_sendActivity"):
        assert retired not in readme
    endpoints = re.findall(r"`((?:https|wss)://[^`]+)`", readme)
    assert len(endpoints) == 2
    for endpoint in endpoints:
        url = urlsplit(endpoint)
        assert url.netloc == "api-mainnet-beta.paxeer.network"
        assert url.path in ("", "/")
        assert not url.query and not url.fragment
    programs = (SDK / "platform/go/programs.go").read_text()
    for method in ("eth_sendRawTransaction", "pax_sequencerReceipt"):
        assert f'programs.rpc(ctx, "{method}"' in programs
    for destination in re.findall(r"\]\(([^)]+)\)", readme):
        if not urlsplit(destination).scheme:
            assert (SDK / destination).exists(), destination


def test_repair_record_maps_all_findings_to_executable_tests():
    record = tomllib.loads(RECORD.read_text())
    assert record["repair"]["task"] == "22.33.1380"
    expected = {
        "sdk-make-module-root": ["test_sdk_make_module_root", "test_sdk_linker_flags_reach_declared_fork_variables"],
        "swagger-clean-check": ["test_swagger_clean_check_source_contract"],
        "markdown-container-image": ["test_markdown_container_image"],
        "sdk-overview-two-domains": ["test_sdk_overview_one_network_native_path_and_endpoints"],
    }
    assert set(record["finding"]) == set(expected)
    for finding, names in expected.items():
        entry = record["finding"][finding]
        assert entry["audit"] == "22.30.75"
        assert entry["tests"] == names
        assert entry["file"] == ("sdk/README.md" if finding == "sdk-overview-two-domains" else "sdk/Makefile")
        assert all(callable(globals()[name]) for name in names)
    assert record["repair"]["wiring_requests"] == []
    assert record["evidence"]["private_ledger"] == "/out/run/gates.jsonl"
    assert record["evidence"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1380.py"
    )
