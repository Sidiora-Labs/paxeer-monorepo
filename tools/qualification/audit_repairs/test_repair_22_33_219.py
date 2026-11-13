"""Offline production-source proof for audit 22.30.123 findings 005/007/008.

No native build, shell command, RPC endpoint or artifact is needed by this gate.
Artifact size and runtime correctness remain owning-domain qualification work.
"""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_219.kvx")

GUEST_RECIPE_PATH = "programs/crates/layerx-programs-ai-market/tests/fixtures/guest-recipe.sh"


def guest_recipe():
    return (ROOT / GUEST_RECIPE_PATH).read_text(encoding="utf-8")


def guest_recipe_array(name):
    match = re.search(r"^" + re.escape(name) + r"=\(\n(.*?)\n\)", guest_recipe(), re.M | re.S)
    assert match, name
    return shlex.split(match.group(1))


def guest_recipe_compile_contract():
    recipe = guest_recipe()
    assert "PAXAI_GUEST_OPT_LEVEL=z" in recipe
    assert "PAXAI_GUEST_CRATE_TYPE=cdylib" in recipe
    assert "export CARGO_PROFILE_RELEASE_OPT_LEVEL=\"$PAXAI_GUEST_OPT_LEVEL\"" in recipe
    for value in ("LTO=true", "CODEGEN_UNITS=1", "PANIC=abort", "STRIP=true"):
        assert "export CARGO_PROFILE_RELEASE_" + value in recipe
    assert 'unset CARGO_ENCODED_RUSTFLAGS' in recipe
    flags = guest_recipe_array("PAXAI_GUEST_RUSTFLAGS")
    assert flags[:4] == ["-C", "target-feature=-reference-types", "-C", "link-arg=--compress-relocations"]
    assert flags[4:] == [
        "-C", "llvm-args=--enable-merge-functions", "-C", "llvm-args=--ir-outliner",
        "-C", "llvm-args=--inline-threshold=40",
    ]
    assert 'export RUSTFLAGS="${PAXAI_GUEST_RUSTFLAGS[*]}"' in recipe
    compile_line = re.search(
        r'^    ("\$\{PROGRAMS_CARGO:-cargo\}" rustc.*?)\n\)', recipe, re.M | re.S,
    )
    assert compile_line
    assert shlex.split(compile_line.group(1).replace("\\\n", " ")) == [
        "${PROGRAMS_CARGO:-cargo}", "rustc", "--locked", "--release", "--target",
        "wasm32-unknown-unknown", "-p", "layerx-programs-ai-market", "--lib",
        "--crate-type", "$PAXAI_GUEST_CRATE_TYPE",
    ]
    assert 'paxai_guest_compile "$source_dir"' in recipe
    assert 'paxai_guest_package "$target_dir/wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm" "$output"' in recipe


def guest_recipe_optimizer_contract():
    recipe = guest_recipe()
    guard = re.search(r"^    command -v wasm-opt[^\n]+\n(.*?)^    \}", recipe, re.M | re.S)
    assert guard and "wasm-opt is required" in guard.group(1)
    assert "return 1" in guard.group(1) and "|| true" not in guard.group(0)
    assert "PAXAI_GUEST_OPTIMIZER_VERSION='wasm-opt version 133 (version_133)'" in recipe
    assert 'if [[ "$(wasm-opt --version)" != "$PAXAI_GUEST_OPTIMIZER_VERSION" ]]; then' in recipe
    assert "AI market guest requires Binaryen 133" in recipe
    assert recipe.count("    paxai_guest_require_optimizer\n") == 2
    passes = guest_recipe_array("PAXAI_GUEST_PASSES")
    assert passes[0] == "-Oz"
    assert passes[-2:] == ["--strip-debug", "--strip-producers"]
    assert "--flatten" in passes and "--rereloop" in passes
    assert "--merge-similar-functions" in passes and passes.count("--converge") == 2
    assert 'wasm-opt "${PAXAI_GUEST_PASSES[@]}" "$1" -o "$2"' in recipe
    metadata = (ROOT / "programs/crates/layerx-programs-ai-market/tests/fixtures/ai-market-opt-z.kvx").read_text()
    assert f'postprocess = "wasm-opt {" ".join(passes)}"' in metadata
    return passes


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def readme_section(heading):
    text = source("programs/README.md")
    marker = heading + "\n"
    assert text.count(marker) == 1
    return text.split(marker, 1)[1].split("\n---\n", 1)[0]


def production_recipe():
    text = source("Makefile")
    match = re.search(
        r"^paxai-host-boundary-build: [^\n]+\n((?:\t[^\n]*\n)+)",
        text,
        re.M,
    )
    assert match, "production AI build recipe must exist"
    return match.group(1).splitlines()


def test_005_ai_release_override_matches_real_guest_package():
    workspace = tomllib.loads(source("programs/Cargo.toml"))
    member = "crates/layerx-programs-ai-market"
    assert member in workspace["workspace"]["members"]
    package = tomllib.loads(source(f"programs/{member}/Cargo.toml"))
    name = package["package"]["name"]
    assert name == "layerx-programs-ai-market"
    release = workspace["profile"]["release"]
    assert release["opt-level"] == 3
    assert release["package"][name]["opt-level"] == "z"
    assert release["lto"] is True
    assert release["codegen-units"] == 1
    assert release["panic"] == "abort"
    assert release["strip"] is True


def test_005_production_build_consumes_release_profile():
    commands = [line.strip() for line in production_recipe()]
    build = "PAXAI_CHAIN_DOMAIN=$(PAXAI_CHAIN_DOMAIN) PROGRAMS_CARGO=$(PROGRAMS_CARGO) bash " + GUEST_RECIPE_PATH + " build programs $(PAXAI_HOST_BOUNDARY_DIR)/ai_market.wasm"
    assert commands.count(build) == 1
    assert commands[2] == "@bash " + GUEST_RECIPE_PATH + " check"
    guest_recipe_compile_contract()
    cargo_config = tomllib.loads(source("programs/.cargo/config.toml"))
    assert cargo_config["net"]["offline"] is True
    assert cargo_config["source"]["vendored-sources"]["directory"] == "vendor"


def test_005_optimized_payload_guard_includes_real_envelope():
    commands = [line.strip() for line in production_recipe()]
    guest_recipe_optimizer_contract()
    optimizer = next(line for line in commands if "guest-recipe.sh build programs " in line)
    artifact = shlex.split(optimizer)[-1]
    assert artifact == "$(PAXAI_HOST_BOUNDARY_DIR)/ai_market.wasm"
    guard = next(line for line in commands if line.startswith("@test "))
    match = re.fullmatch(
        r'@test "\$\$\(wc -c < ' + re.escape(artifact)
        + r'\)" -le (\d+) \|\| \{ .+ >&2; exit 1; \}',
        guard,
    )
    assert match, "size overflow must fail before the harness consumes the artifact"
    guest_limit = int(match.group(1))
    protocol = source("include/layerx/lxp_protocol.h")
    payload_limit = int(re.search(r"LXP_MAX_PAYLOAD_BYTES\s*=\s*(\d+)", protocol).group(1))
    decoder = source("src/modules/programs/deploy.c")
    assert "if (length < 104U) return LXP_ERR_TRUNCATED;" in decoder
    assert "wasm_length = read_u32(payload + 100U);" in decoder
    assert "(size_t)wasm_length == length - 104U" in decoder
    assert "value->wasm = payload + 104U;" in decoder
    assert payload_limit == 524288
    assert guest_limit == payload_limit - 104 == 524184
    assert "length - 108U" in decoder
    assert "value->interface_length = read_u32(payload + 104U);" in decoder
    consumer = "$(RUN_PREFIX) $(BUILD_DIR)/tests/paxai_host_boundary " + artifact + " < /dev/null"
    assert consumer in commands
    assert commands.index(optimizer) < commands.index(guard) < commands.index(consumer)
    manifest = commands[-1]
    assert "$(abspath " + artifact + ")" in manifest
    assert manifest.endswith("> $(PAXAI_HOST_BOUNDARY_DIR)/manifest")


def test_007_committed_paxd_finality_and_rpc_ownership():
    overview = source("programs/README.md").split("\n---\n", 1)[0]
    prose = " ".join(overview.split())
    for fact in (
        "one BFT network: `paxd`, EVM chain ID `125` (`0x7d`)",
        "kernel runs inside `paxd`",
        "participates in block proposal and execution",
        "A receipt is final when its block is committed",
        "before commitment is execution evidence, not finality",
        "do not require periodic checkpoint settlement or a separate settlement deployment",
        "public node JSON-RPC at the root URL",
        "Hosted gateway `px_` methods have their own",
    ):
        assert fact in prose, fact
    urls = re.findall(r"`(https://[^`]+)`", overview)
    assert len(urls) == 2
    for url in urls:
        parsed = urlsplit(url)
        assert parsed.scheme == "https"
        assert parsed.path in {"", "/"}
        assert not parsed.query and not parsed.fragment
    contract = "../services/platform/hosted/gateway/openrpc.json"
    assert f"[OpenRPC contract]({contract})" in overview
    rpc = json.loads((ROOT / "programs" / contract).read_text(encoding="utf-8"))
    assert any(method["name"].startswith("px_") for method in rpc["methods"])
    assert "periodically settles" not in overview


def test_008_live_precompile_and_registry_chain_proof_guidance():
    layout = readme_section("## Workspace layout")
    rows = {}
    for line in layout.splitlines():
        if line.startswith("| `"):
            path, purpose = [cell.strip() for cell in line.split("|")[1:3]]
            rows[path.strip("`")] = purpose
    assert "../precompiles/layerxprograms" in rows
    assert "0x101D" in rows["../precompiles/layerxprograms"]
    assert "chain-proof" in rows["../precompiles/layerxprograms"]
    assert "crates/layerx-programs-protocol-adapter" not in rows
    prose = " ".join(layout.split())
    for fact in (
        "Registry reads use",
        "protocol_evidence.rs",
        "chain_proof/",
        "verify deployment, lifecycle and state evidence against chain commitments",
        "VerifiedProgramCatalog",
        "former protocol-adapter crate is retired",
        "retained adapter sources are historical, not a deployed read interface",
        "registry chain-proof verification supplies authenticated evidence",
    ):
        assert fact in prose, fact
    for path in (
        "crates/layerx-programs-registry/src/protocol_evidence.rs",
        "crates/layerx-programs-registry/src/chain_proof",
        "crates/layerx-programs-registry/src/resolver.rs",
    ):
        assert f"]({path})" in layout
        assert (ROOT / "programs" / path).exists()
    assert "pub struct VerifiedProgramCatalog" in source(
        "programs/crates/layerx-programs-registry/src/resolver.rs"
    )


def test_008_documented_balance_read_matches_published_abi_and_host():
    prose = " ".join(readme_section("## Workspace layout").split())
    for fact in (
        "node-root JSON-RPC `eth_call`",
        "0x000000000000000000000000000000000000101D",
        "getValueAccount(bytes32,bytes32)",
        "program id and registered account id",
        "explicit committed block height",
        "`uint128 balance` and `bool frozen`",
        "boolean is a freeze flag, not an existence flag",
        "absent account returns `(0, false)`",
    ):
        assert fact in prose, fact
    abi = json.loads(source("precompiles/layerxprograms/abi.json"))
    methods = [item for item in abi if item.get("name") == "getValueAccount"]
    assert len(methods) == 1
    method = methods[0]
    assert method["type"] == "function" and method["stateMutability"] == "view"
    assert [(arg["name"], arg["type"]) for arg in method["inputs"]] == [
        ("programId", "bytes32"), ("accountId", "bytes32")
    ]
    assert [(arg["name"], arg["type"]) for arg in method["outputs"]] == [
        ("balance", "uint128"), ("frozen", "bool")
    ]
    assert 'common.HexToAddress("0x000000000000000000000000000000000000101D")' in source(
        "precompiles/layerxprograms/events.go"
    )
    handler = source("precompiles/layerxprograms/view_accounts.go").split(
        "func getValueAccountHandler(", 1
    )[1]
    assert "p.host.ReadValueAccount(ctx, input.ProgramID, input.AccountID)" in handler
    assert "call.method.Outputs.Pack(account.Balance.BigInt(), account.Frozen)" in handler
    assert "errors.Is(err, lxphost.Code(-7))" in handler
    assert "call.method.Outputs.Pack(new(big.Int), false)" in handler


def test_005_packaging_docs_use_canonical_override_and_envelope():
    packaging = readme_section("### AI market guest packaging")
    prose = " ".join(packaging.split())
    assert "AI package's release override" in prose
    assert "automatically" in prose
    command = re.search(r"```sh\n(.*?)\n```", packaging, re.S).group(1)
    args = shlex.split(command.replace("\\\n", " "))
    assert args[:2] == ["cd", "programs"]
    domain = hashlib.sha256(b"PAXAI/chain-domain/v1" + (125).to_bytes(8, "big")).hexdigest()
    assert args[2] == "PAXAI_CHAIN_DOMAIN=" + domain
    assert "PAXAI_CHAIN_DOMAIN ?= " + domain in source("Makefile")
    assert args[3:] == [
        "cargo", "build", "--locked", "--release", "--target",
        "wasm32-unknown-unknown", "-p", "layerx-programs-ai-market",
    ]
    for fact in (
        "524288 bytes", "104-byte", "524184 bytes", "108 fixed bytes plus the interface bytes",
        "make paxai-host-boundary-build", "guest-recipe.sh", "Binaryen 133",
        "same file", "does not establish that the final payload fits",
    ):
        assert fact in prose, fact


def test_repair_record_maps_each_finding_to_executing_tests():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert set(record["finding"]) == {"005", "007", "008"}
    module = ast.parse(Path(__file__).read_text(encoding="utf-8"))
    names = {node.name for node in module.body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"]
        assert set(finding["tests"]) <= names
    task = record["task"]["22.33.219"]
    assert task["baseline_tests_run"] is False
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["review_only"]
    assert task["verified_revision_source"]
