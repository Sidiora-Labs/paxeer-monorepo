"""Offline documentation/source contracts for audit 22.30.140.

This gate executes no JavaScript toolchain or network operations. Runtime and
TypeScript compilation claims are explicitly excluded in the repair record.
"""

import ast
import json
from pathlib import Path
import re
import tomllib
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[3]
SDK = ROOT / "sdk/js"
README = (SDK / "README.md").read_text(encoding="utf-8")
MANIFEST = json.loads((SDK / "package.json").read_text(encoding="utf-8"))
RECORD = tomllib.loads(Path(__file__).with_name("repair_22_33_372.kvx").read_text())


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def section(text, heading):
    marker = f"## {heading}\n"
    assert text.count(marker) == 1, heading
    return text.split(marker, 1)[1].split("\n## ", 1)[0]


def check_links(page, text):
    links = re.findall(r"\[[^\]]+\]\(([^)]+)\)", text)
    assert links, page
    resolved = set()
    for link in links:
        url = urlsplit(link)
        assert not url.scheme and not url.netloc, link
        target = (page.parent / unquote(url.path)).resolve() if url.path else page
        assert target.is_relative_to(ROOT), link
        assert target.is_file(), link
        if url.fragment:
            headings = re.findall(r"^#{1,6} (.+)$", target.read_text(), re.M)
            anchors = {
                re.sub(r"\s", "-", re.sub(r"[^\w\s-]", "", title.lower()))
                for title in headings
            }
            assert url.fragment in anchors, link
        resolved.add(target.relative_to(ROOT).as_posix())
    return resolved


def examples():
    blocks = re.findall(r"^```typescript\n(.*?)^```", README, re.M | re.S)
    assert len(blocks) == 6
    return blocks


def test_registry_contract_pages_missing_links_resolve():
    sdk_contracts = section(README, "Integration contracts")
    paths = check_links(SDK / "README.md", sdk_contracts)
    assert {
        "sdk/js/README.md",
        "contracts/bridge/README.md",
        "apps/explorer/public/openapi.yaml",
        "apps/explorer/lib/socket/types.ts",
        "apps/explorer/lib/socket/useSocketChannel.tsx",
        "services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex",
    } <= paths
    assert "Explorer HTTP and Phoenix WebSocket APIs are separate from Cosmos REST" in sdk_contracts
    package_links = check_links(SDK / "README.md", section(README, "Package"))
    assert {"sdk/js/package.json", "sdk/js/README.md"} <= package_links
    assert "contracts/sdk-js.md" not in README


def test_registry_contract_pages_missing_bridge_and_explorer_sources():
    bridge = ROOT / "contracts/bridge/README.md"
    contracts = section(bridge.read_text(), "Interface contracts")
    paths = check_links(bridge, contracts)
    assert {
        "modules/layerxbridge/ATTESTATION.md",
        "contracts/bridge/ATTESTATION-SOLANA.md",
        "contracts/bridge/vectors/solana_test.go",
        "services/bridge/deploy/checklist.sh",
        "sdk/js/README.md",
        "apps/explorer/public/openapi.yaml",
        "apps/explorer/lib/socket/types.ts",
    } <= paths
    assert "attestation, destination policy and nullifier checks" in contracts
    assert "durable obligations" in contracts
    http = source("apps/explorer/public/openapi.yaml")
    assert http.startswith("openapi: 3.0.3\n")
    assert "GET /api/v2/openapi" in http
    assert "paths:\n" in http and "responses:" in http
    endpoint = source(
        "services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex"
    )
    for route in ("/socket", "/socket/v2"):
        assert f'socket("{route}",' in endpoint
    payloads = source("apps/explorer/lib/socket/types.ts")
    assert "export namespace SocketMessage" in payloads
    assert "SocketMessageParamsGeneric<'new_block', NewBlockSocketResponse>" in payloads
    lifecycle = source("apps/explorer/lib/socket/useSocketChannel.tsx")
    for operation in ("ch.join()", ".receive('ok'", ".receive('error'", "ch.leave()"):
        assert operation in lifecycle


def test_registry_sdk_package_exports_and_typesversions():
    exports = MANIFEST["exports"]
    assert set(exports) == {"./types/*", "./rest", "./encoding"}
    rows = re.findall(
        r"^\| `([^`]+)` \| `([^`]+)` \| `([^`]+)` \| `([^`]+)` \|$",
        section(README, "Package"), re.M,
    )
    assert len(rows) == len(exports)
    documented = {
        key: dict(zip(("import", "require", "types"), targets))
        for key, *targets in rows
    }
    assert documented == exports
    assert MANIFEST["typesVersions"] == {
        "*": {
            "types/*": ["dist/types/types/*/index.d.ts"],
            "rest": ["dist/types/rest/index.d.ts"],
            "encoding": ["dist/types/encoding/index.d.ts"],
        }
    }
    for key, conditions in exports.items():
        assert set(conditions) == {"import", "require", "types"}
        for condition, tree, suffix in (
            ("import", "esm", ".js"),
            ("require", "cjs", ".js"),
            ("types", "types", ".d.ts"),
        ):
            expected = f"./dist/{tree}/{key[2:]}/index{suffix}"
            assert conditions[condition] == expected
        declarations = list(SDK.glob(conditions["types"]))
        assert declarations, key
        for declaration in declarations:
            relative = declaration.relative_to(SDK / "dist/types")
            for tree in ("esm", "cjs"):
                runtime = SDK / "dist" / tree / str(relative).replace(".d.ts", ".js")
                assert runtime.is_file(), runtime


def test_registry_sdk_package_retired_root_and_vendor_metadata():
    assert MANIFEST["name"] == "@pax-js/cosmos"
    assert "main" not in MANIFEST and "module" not in MANIFEST
    assert "." not in MANIFEST["exports"]
    assert not (SDK / "VENDOR.kvx").exists()
    contract = section(README, "Package")
    assert "retired `main` and `module` fields do not provide a root entry" in contract
    assert "`VENDOR.kvx` and its `ref`, `sha` and `url` fields are retired" in contract
    assert "upstream tag `v1.0.6-pax.1`" in section(README, "Provenance")
    for row in (
        "14.cfg-sdkjs-package-main",
        "14.cfg-sdkjs-package-module",
        "14.cfg-sdkjs-package-exports-types",
        "14.cfg-sdkjs-vendor-ref",
        "14.cfg-sdkjs-vendor-sha",
        "14.cfg-sdkjs-vendor-url",
    ):
        matches = [request for request in RECORD["wiring"].values() if row in request]
        assert len(matches) == 1, row
        assert "spec/layerx-merge/interfaces.kvx: domain 14" in matches[0]
        assert "sdk/js/README.md#" in matches[0]


def test_sdk_examples_codec_and_registry_match_production():
    block = next(block for block in examples() if "function encodeBankSend(" in block)
    for expression in (
        "import { Encoder } from '@pax-js/cosmos/encoding';",
        "const codec = Encoder.cosmos.bank.v1beta1.MsgSend;",
        "codec.fromPartial(message)",
        "codec.encode(msgSend).finish()",
        "codec.decode(encoded)",
        "typeUrl: `/${codec.$type}`, value: encoded",
    ):
        assert expression in block
    assert not re.search(r"(?<![\w.])MsgSend\.\$type", README)
    for tree in ("cjs", "esm"):
        namespace = source(f"sdk/js/dist/{tree}/encoding/index.js")
        assert "bank: { v1beta1: cosmos_bank_v1beta1 }" in namespace
        tx = source(f"sdk/js/dist/{tree}/encoding/cosmos/bank/v1beta1/tx.js")
        assert '$type: "cosmos.bank.v1beta1.MsgSend"' in tx
        assert '"/cosmos.bank.v1beta1.MsgSend"' in tx
        for field in ("from_address", "to_address", "amount"):
            assert f"message.{field}" in tx
        registry = source(f"sdk/js/dist/{tree}/encoding/registry.js")
        assert "cosmos/bank/v1beta1/tx" in registry
        assert "paxProtoRegistry" in registry
    declaration = source("sdk/js/dist/types/encoding/cosmos/bank/v1beta1/tx.d.ts")
    assert 'MsgSend: MessageFns<MsgSend, "cosmos.bank.v1beta1.MsgSend">' in declaration
    assert "unencoded message object as `value`" in README


def test_sdk_examples_signing_uses_explicit_cosmos_identity_and_denoms():
    block = next(block for block in examples() if "async function sendBankTokens(" in block)
    assert "import { AminoTypes, SigningStargateClient } from '@cosmjs/stargate';" in block
    assert "import { Registry } from '@cosmjs/proto-signing';" in block
    assert "import type { OfflineSigner } from '@cosmjs/proto-signing';" in block
    assert "new AminoTypes(aminoConverters)" in block
    assert "new Registry(paxProtoRegistry)" in block
    assert "SigningStargateClient.connectWithSigner(\n    cosmosRpcUrl," in block
    for setting in ("chainId", "recipient", "denom", "amount", "feeDenom", "feeAmount", "gas"):
        assert f"{setting}: string;" in block
        assert f"options.{setting}" in block
    assert block.index("await client.getChainId()") < block.index("client.signAndBroadcast(")
    assert "!== options.chainId" in block
    assert "if (!account)" in block and "if (result.code !== 0)" in block
    assert "finally {\n    client.disconnect();" in block
    assert "value: codec.fromPartial({" in block
    network = section(README, "Network and connection configuration")
    assert "EVM chain ID is `125`" in network
    assert "EVM numeric chain ID is not the Cosmos\nsigning chain ID" in network
    assert "Cosmos REST base URL" in network and "CometBFT-compatible RPC URL" in network
    assert "Choose transfer denoms, fee denoms, amounts and gas" in network
    assert "arctic" not in README.lower()


def test_sdk_examples_rest_transport_matches_production():
    block = next(block for block in examples() if "async function queryBalances(" in block)
    assert "import { Querier } from '@pax-js/cosmos/rest';" in block
    assert "Querier.cosmos.bank.v1beta1.AllBalances(" in block
    assert "{ address }," in block and "{ pathPrefix: cosmosRestUrl }," in block
    for tree in ("cjs", "esm"):
        query = source(f"sdk/js/dist/{tree}/rest/cosmos/bank/v1beta1/query.js")
        assert "static AllBalances(req, initReq)" in query
        assert '/cosmos/bank/v1beta1/balances/${req["address"]}?' in query
        assert 'method: "GET"' in query
        transport = source(f"sdk/js/dist/{tree}/rest/fetch.js")
        for expression in (
            "const { pathPrefix, ...req } = init || {};",
            "`${pathPrefix}${path}`",
            "fetch(url, req)",
            "r.json()",
            "if (!r.ok)",
            "throw body;",
            "return body;",
        ):
            assert expression in transport
    contract = section(README, "`@pax-js/cosmos/rest`")
    assert "including `balances` and optional\n`pagination`" in contract
    assert "throw the parsed response body" in contract
    assert "pass a `signal`" in contract


def test_sdk_examples_dependency_version_and_compiler_contract():
    versions = {
        MANIFEST["dependencies"][package]
        for package in ("@cosmjs/proto-signing", "@cosmjs/stargate")
    }
    assert len(versions) == 1
    assert f"CosmJS `{versions.pop()}`" in README
    assert "^0.32.4" not in README
    compiler = source("sdk/js/test/readme-examples.test.cjs")
    for expression in (
        'require("typescript")',
        "readme.matchAll(",
        "for (const [index, example] of examples.entries())",
        "strict: true",
        "noEmit: true",
        "ts.ModuleKind.ESNext",
        "ts.ModuleResolutionKind.Bundler",
        "ts.createCompilerHost(options)",
        "ts.createProgram([fileName], options, host)",
        "ts.getPreEmitDiagnostics(program)",
        "diagnostics.length,\n\t\t\t0,",
        "function readme_examples_compile_against_public_package()",
        "test(readme_examples_compile_against_public_package);",
    ):
        assert expression in compiler
    assert MANIFEST["scripts"]["test"] == "node --test test/"
    assert "TypeScript" in RECORD["repair"]["review_only"]
    assert "SDK CI" in RECORD["wiring"]["example_compilation"]


def test_repair_record_maps_every_finding_to_executed_tests():
    assert RECORD["repair"]["task"] == "22.33.372"
    assert RECORD["repair"]["audit"] == "22.30.140"
    assert RECORD["repair"]["requirement"] == "22998.1"
    assert set(RECORD["finding"]) == {
        "registry-contract-pages-missing", "sdk-examples", "registry-sdk-package"
    }
    functions = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    for finding in RECORD["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= functions
        assert (ROOT / finding["file"]).is_file()
        assert finding["resolution"]
    assert RECORD["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No unchanged-tree verification was run" in RECORD["repair"]["baseline"]
    assert "do not execute JavaScript" in RECORD["repair"]["review_only"]
    assert "domain 14" in RECORD["wiring"]["explorer_schemas"]
    assert "domain 17" in RECORD["wiring"]["bridge_contracts"]
