"""Offline source proof for audit 22.30.63/005, the retired WASM Makefile.

Parse the production Makefile and follow its developer guidance through the
maintained root build, chain recipe, daemon and embedded module. Native builds
and GNU Make execution are outside this Python-only source gate.
"""

import ast
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_1272.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def logical_lines(text):
    return re.sub(r"\\\n[ \t]*", " ", text).splitlines()


def make_rule(path, target):
    """Read actual prerequisites and shell recipe, joining Make continuations."""
    lines = logical_lines(source(path))
    declarations = [
        i
        for i, line in enumerate(lines)
        if not line.startswith(("\t", "#"))
        and ":" in line
        and not line.partition(":")[2].startswith("=")
        and target in line.partition(":")[0].split()
    ]
    assert len(declarations) == 1, (path, target, declarations)
    index = declarations[0]
    prerequisites = lines[index].split(":", 1)[1].split()
    recipe = []
    for line in lines[index + 1 :]:
        if not line.startswith("\t"):
            break
        recipe.append(line.lstrip("\t").removeprefix("@"))
    return prerequisites, recipe


def help_messages():
    prerequisites, recipe = make_rule("wasm/Makefile", "help")
    assert prerequisites == []
    assert len(recipe) == 1
    assert "$" not in recipe[0] and "`" not in recipe[0]
    argv = shlex.split(recipe[0])
    assert argv[:2] == ["printf", "%s\\n"]
    return argv[2:]


def test_wasm_makefile_retires_operational_goals_before_recipes():
    lines = [
        line
        for line in logical_lines(source("wasm/Makefile"))
        if line.strip() and not line.startswith("#")
    ]
    assignments = {}
    rules = {}
    directives = []
    for index, line in enumerate(lines):
        if line.startswith("\t"):
            continue
        assignment = re.fullmatch(r"([.A-Za-z_][\w.]*)\s*:=\s*(.+)", line)
        rule = re.fullmatch(r"([.A-Za-z_][\w.-]*):\s*(.*)", line)
        if assignment:
            assignments[assignment[1]] = assignment[2]
        elif rule:
            rules[rule[1]] = rule[2].split()
        else:
            directives.append((index, line))
    assert assignments == {".DEFAULT_GOAL": "help"}
    assert rules == {".PHONY": ["help"], "help": []}
    assert len(directives) == 3
    assert directives[0][1] == "ifneq ($(filter-out help,$(MAKECMDGOALS)),)"
    assert directives[1][1].startswith("$(error wasm/Makefile is retired;")
    assert directives[1][1].endswith(")")
    assert "run make paxd-build from the repository root" in directives[1][1]
    assert "help for module developer commands" in directives[1][1]
    assert directives[2][1] == "endif"
    assert [item[0] for item in directives] == list(
        range(directives[0][0], directives[0][0] + 3)
    )
    assert directives[2][0] < lines.index("help:")
    text = source("wasm/Makefile")
    for obsolete in (
        "$(shell", "include ", "go.mod", "go.sum", "./cmd/wasmd",
        "CosmWasm/wasmd", "github.com/cosmos/cosmos-sdk", "-ldflags",
        "$(DOCKER)", "protocgen.sh", "protoc-swagger-gen.sh",
    ):
        assert obsolete not in text, obsolete
    assert help_messages()[0].startswith("wasm/Makefile is retired:")


def test_wasm_help_points_to_real_root_paxd_build_and_native_prerequisites():
    messages = help_messages()
    assert "Run these commands from the repository root:" in messages
    commands = [
        shlex.split(message.strip())
        for message in messages
        if message.startswith("  ")
    ]
    assert commands == [
        ["make", "paxd-build"],
        ["make", "paxd-native-libs"],
        ["CGO_ENABLED=1", "go", "test", "-mod=readonly", "./wasm/x/wasm/..."],
    ]
    assert "Build native libraries before running the module tests." in messages
    prerequisites, recipe = make_rule("Makefile", commands[0][1])
    assert prerequisites == [commands[1][1]]
    assert len(recipe) == 1
    assert "version=$$(jq -er .version tools/release/version.json) &&" in recipe[0]
    assert 'GOPROXY=off $(PAXEER_MAKE) build LDFLAGS="-X ' in recipe[0]
    assert (
        "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/version.Version=$$version"
    ) in recipe[0]
    root_makefile = source("Makefile")
    assert "PAXEER_DIR := $(CURDIR)" in root_makefile
    assert "PAXEER_MAKE := $(MAKE) -f $(PAXEER_DIR)/chain.mk" in root_makefile
    prerequisites, recipe = make_rule("Makefile", commands[1][1])
    assert prerequisites == ["$(LIBRARY)", "programs-release-build"]
    assert recipe == []
    assert make_rule("Makefile", "programs-release-build")[1]
    prerequisites, recipe = make_rule("chain.mk", "build")
    assert prerequisites == ["paxd-native-libs"]
    assert len(recipe) == 1
    assert shlex.split(recipe[0]) == [
        "go", "build", "$(BUILD_FLAGS)", "-o", "./build/paxd", "./daemon/paxd",
    ]
    assert (ROOT / "daemon/paxd/main.go").is_file()
    assert "AppName=paxd" in source("chain.mk")


def test_wasm_module_test_guidance_uses_the_root_manifest_and_real_packages():
    command = shlex.split(
        next(message for message in help_messages() if "go test" in message)
    )
    assert command == [
        "CGO_ENABLED=1", "go", "test", "-mod=readonly", "./wasm/x/wasm/...",
    ]
    module = re.search(r"^module\s+(\S+)$", source("go.mod"), re.MULTILINE)
    assert module is not None
    assert module[1] == "github.com/Sidiora-Labs/Paxeer-X-Network"
    assert (ROOT / "go.sum").is_file()
    assert not (ROOT / "wasm/go.mod").exists()
    assert not (ROOT / "wasm/go.sum").exists()
    module_dir = ROOT / command[-1].removesuffix("/...").removeprefix("./")
    assert module_dir.is_dir()
    assert not list((ROOT / "wasm").rglob("go.mod"))
    assert list(module_dir.rglob("*_test.go"))
    module_source = (module_dir / "module.go").read_text(encoding="utf-8")
    assert '"' + module[1] + '/sdk/types/module"' in module_source
    assert '"' + module[1] + '/wasm/x/wasm/keeper"' in module_source
    assert "type AppModule struct" in module_source


def test_paxd_consumer_embeds_the_same_wasm_module():
    daemon = source("daemon/paxd/main.go")
    assert 'app "github.com/Sidiora-Labs/Paxeer-X-Network/node"' in daemon
    assert "rootCmd, _ := cmd.NewRootCmd()" in daemon
    assert "svrcmd.Execute(rootCmd, app.DefaultNodeHome)" in daemon
    app = source("node/app.go")
    assert '"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"' in app
    assert "app.WasmKeeper = wasm.NewKeeper(" in app
    assert (
        "wasm.NewAppModule(appCodec, &app.WasmKeeper, app.StakingKeeper, "
        "app.AccountKeeper, app.BankKeeper)"
    ) in app
    assert "WASM is embedded in the root paxd module." in help_messages()[0]


def test_repair_record_maps_finding_005_to_all_source_proofs():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.1272"
    finding = record["finding"]["005"]
    assert finding["audit"] == "spec/layerx-merge/programme/audit/records/22.30.63.kvx"
    assert finding["file"] == "wasm/Makefile"
    assert finding["class"] == "config-drift"
    assert finding["severity"] == "medium"
    assert finding["owner_domain"] == 6
    test_names = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == test_names
    assert len(finding["tests"]) == len(test_names)
    assert record["repair"]["wiring_requests"] == []
    qualification = record["qualification"]
    assert qualification["acceptance"] == "22998.1"
    assert qualification["build_cmd"] == "python3 --version"
    assert qualification["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1272.py"
    )
    assert qualification["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No pre-change verify was run" in record["repair"]["baseline"]
    assert "Native" in record["repair"]["review_only"]
