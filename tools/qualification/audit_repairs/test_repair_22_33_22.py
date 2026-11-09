"""Offline source qualification for audit 22.30.101/test-discovery.

Parse the real package manifest, compiler manifest, maintained suites and runner.
This gate does not execute TypeScript suites or certify external CI provisioning.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SDK = ROOT / "services/agent/sdk/typescript"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_22.kvx"


def read(relative):
    return (SDK / relative).read_text(encoding="utf-8")


def manifest():
    return json.loads(read("package.json"))


def inventory():
    sources = {path.relative_to(SDK / "test").as_posix()
               for path in (SDK / "test").rglob("*.test.ts")}
    requirements = manifest()["sdkTestRequirements"]
    external = [name for names in requirements.values() for name in names]
    assert sources
    assert len(external) == len(set(external))
    assert set(external) <= sources
    return sources, sources - set(external), requirements


def test_test_discovery_manifest_covers_maintained_and_previous_suites():
    package = manifest()
    scripts = package["scripts"]
    assert scripts["build"] == "tsc -p tsconfig.json"
    assert scripts["test"] == "npm run test:ci"
    modes = {"self-contained", "anvil", "ci", "external-node",
             "case-file", "native-effect-case"}
    for mode in modes:
        assert shlex.split(scripts[f"test:{mode}"]) == [
            "npm", "run", "build", "&&", "node", "scripts/run-tests.mjs", mode,
        ]
    sources, local, requirements = inventory()
    assert requirements == {
        "anvil": ["exchange-v2.test.ts", "gas-station.test.ts"],
        "case-file": ["agent-operation-envelope.test.ts"],
    }
    assert local | set(requirements["anvil"]) | set(requirements["case-file"]) == sources
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    previous = set(record["baseline"]["previous_test_inventory"])
    assert previous <= sources
    assert "gas-station.test.ts" in previous
    assert previous - set(requirements["anvil"]) <= local
    compiler = json.loads(read("tsconfig.json"))
    assert "test/**/*.ts" in compiler["include"]
    assert compiler["compilerOptions"]["rootDir"] == "."
    assert compiler["compilerOptions"]["outDir"] == "dist"
    assert not compiler.get("exclude")


def test_test_discovery_recurses_sources_and_requires_compiled_consumers():
    runner = read("scripts/run-tests.mjs")
    for fact in (
        'const packageRoot = new URL("../", import.meta.url);',
        'const sourceRoot = new URL("test/", packageRoot);',
        'readFileSync(new URL("package.json", packageRoot), "utf8")',
        'readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {',
        'if (entry.isDirectory()) return discoverTests(new URL(`${entry.name}/`, directory), `${name}/`);',
        'return entry.isFile() && name.endsWith(".test.ts") ? [name] : [];',
        '}).sort();',
        'const discovered = discoverTests(sourceRoot);',
        'if (discovered.length === 0) throw new Error',
        'const requirements = manifest.sdkTestRequirements;',
        'const special = Object.values(requirements).flat();',
        'if (new Set(special).size !== special.length) throw new Error',
        'if (!discovered.includes(name)) throw new Error',
        'selected = discovered.filter((name) => !special.includes(name));',
        'selected = discovered.filter((name) => !requirements["case-file"].includes(name));',
        'const compiled = selected.map((name) => {',
        'new URL(`dist/test/${name.replace(/\\.ts$/u, ".js")}`, packageRoot)',
        'if (!statSync(target).isFile()) throw new Error',
    ):
        assert fact in runner, fact
    assert runner.index("const compiled = selected.map") < runner.index("for (const target of compiled)")
    assert 'readdirSync(new URL("dist' not in runner
    assert "process.cwd()" not in runner
    assert "existsSync" not in runner


def test_test_discovery_executes_each_file_and_propagates_all_failures():
    runner = read("scripts/run-tests.mjs")
    execution = runner.split("let failures = 0;", 1)[1]
    for fact in (
        "for (const target of compiled)",
        "spawnSync(process.execPath, [fileURLToPath(target), ...testArgs]",
        "cwd: fileURLToPath(packageRoot)",
        'stdio: "inherit"',
        "if (result.error || result.signal || result.status !== 0)",
        "failures += 1;",
        "console.error(`Failed ${fileURLToPath(target)}:",
        "process.exitCode = failures === 0 ? 0 : 1;",
    ):
        assert fact in execution, fact
    for masking in ("catch", "continue;", "break;", "process.exit(0)",
                    "shell:", "--test-skip-pattern", "--test-name-pattern"):
        assert masking not in execution
    assert "--test-skip-pattern" not in runner
    assert "--test-name-pattern" not in runner
    assert "if (selected.length === 0) throw new Error" in runner
    assert 'if (mode !== "external-node" && args.length !== 0) throw new Error' in runner


def test_test_discovery_keeps_real_interface_and_recovery_suites_in_default():
    _, local, _ = inventory()
    required = {
        "rpc-node.test.ts", "rpc-subscription-eth.test.ts",
        "sequencer-receipt-verify.test.ts", "gas-station-pending.test.ts",
        "history-cursor.test.ts", "native-effect.test.ts",
        "proof_exchange.test.ts", "proof_gas_station.test.ts",
        "proof_x402_receive.test.ts", "x402-draw-sequencer.test.ts",
        "repair-22-31-148.test.ts", "repair-22-31-149.test.ts",
        "repair-22-33-20.test.ts", "repair-22-33-21.test.ts",
        "repair-22-33-24.test.ts",
    }
    assert required <= local
    # Confirm the omitted names identify executable suites on production paths,
    # including the maintained process-restart tests, rather than helper modules.
    source_facts = {
        "rpc-node.test.ts": ('from "../src/rpc.js"',
                             "test(JsonRpcClient_reads_use_only_eth_and_px_methods)"),
        "rpc-subscription-eth.test.ts": ("test(subscribeRpc_sends_eth_subscribe_newHeads)",),
        "sequencer-receipt-verify.test.ts": ('from "../src/verifier.js"',
                                            "test(verify_committed_receipt_recorded_inclusion)"),
        "gas-station-pending.test.ts": ('from "../src/gas-station.js"',),
        "history-cursor.test.ts": ("test(history_pages_resume_from_the_gateway_last_row_without_gaps)",),
        "native-effect.test.ts": ("encodeNativeEffectPrepareRequest(request)",),
        "proof_exchange.test.ts": ('from "../src/index.js"',
                                   "test(proof_exchange_order_reaches_both_consumers_twice)"),
        "proof_gas_station.test.ts": ('from "../src/index.js"', "registerHooks"),
        "proof_x402_receive.test.ts": ('from "../src/index.js"',
                                       "assertPayerGrantRoundTrip(canonical)"),
        "repair-22-31-149.test.ts": ('from "../src/x402/draw.js"',
                                    "test(draw_restart_recovers_a_crash_before_send)",
                                    "test(draw_restart_redelivers_after_an_uncertain_send_acknowledgement)"),
        "x402-draw-sequencer.test.ts": ("test(prepared_draw_reopens_before_send)",
                                        "test(prepared_draw_settles_recorded_payment_after_restart)"),
    }
    for name, facts in source_facts.items():
        source = read(f"test/{name}")
        for fact in facts:
            assert fact in source, (name, fact)


def test_test_discovery_documents_and_guards_external_probe_commands():
    runner = read("scripts/run-tests.mjs")
    documentation = read("TESTING.md")
    for mode in ("self-contained", "anvil", "ci", "external-node", "case-file", "native-effect-case"):
        assert f"npm run test:{mode}" in documentation
        assert f'case "{mode}":' in runner
    assert 'if (mode === "case-file" && !process.env.PAXEER_X_AGENT_ENVELOPE_CASE)' in runner
    assert 'if (!process.env.NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST)' in runner
    assert 'if (args.length !== 1) throw new Error' in runner
    assert 'testArgs = ["--programs-proof", args[0]];' in runner
    assert 'selected = ["ai-market.test.ts"];' in runner
    assert 'selected = ["native-effect.test.ts"];' in runner
    assert 'throw new Error(`Unknown test mode: ${mode}`)' in runner
    for name in manifest()["sdkTestRequirements"]["anvil"]:
        assert 'process.env.ANVIL_BIN ?? "anvil"' in read(f"test/{name}")
    probe = read("test/agent-operation-envelope.test.ts")
    assert "process.env.PAXEER_X_AGENT_ENVELOPE_CASE" in probe
    assert "process.exitCode = error instanceof AgentOperationProbeInputError ? 2 : 1;" in probe
    programs = read("test/ai-market.test.ts")
    assert 'if (process.argv[2] === "--programs-proof")' in programs
    assert "await TestProofLayerxprograms0x101dConsumer();" in programs
    assert 'readFile(process.argv[3]!, "utf8")' in programs
    native = read("test/native-effect.test.ts")
    assert "process.env.NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST" in native
    assert "assert.deepEqual(encoded, actual)" in native
    for fact in ("Python 3", "cryptography", "ANVIL_BIN", "Node 24.11",
                 "retired daemon cases", "does not launch or connect to a node",
                 ".github/workflows/xweb-test.yml", "out-of-scope wiring"):
        assert fact in documentation


def test_test_discovery_record_maps_finding_and_states_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["repair"]
    assert task["task"] == "22.33.22"
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_22.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    assert record["baseline"]["verification_run"] is False
    assert "explicit task order" in record["baseline"]["reason"]
    assert record["baseline"]["observed_failing_tests"] == []
    finding = record["finding"]["test-discovery"]
    assert finding["audit"] == "22.30.101"
    assert finding["file"] == "services/agent/sdk/typescript/package.json"
    assert finding["severity"] == "medium"
    declared = {node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
                if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared)
    requests = record["wiring"]["requests"]
    assert any(".github/workflows/xweb-test.yml" in request and "Domain 9" in request
               and "npm run test:ci" in request for request in requests)
    assert any("precompiles/layerxprograms/proof_layerxprograms_0x101d_test.go" in request
               and "Domain 3" in request for request in requests)
