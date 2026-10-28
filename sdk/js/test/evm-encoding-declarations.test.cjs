const assert = require("node:assert/strict");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

const allowedFeeDenom = {
	denom: "usid",
	rate: "3.125000000000000001",
	rate_update_height: 2 ** 40 + 7,
};
const params = {
	priority_normalizer: "1.25",
	base_fee_per_gas: "1000000000",
	minimum_fee_per_gas: "500000000",
	whitelisted_cw_code_hashes_for_delegate_call: [Uint8Array.from([0, 1, 255])],
	deliver_tx_hook_wasm_gas_limit: 2 ** 40 + 100,
	max_dynamic_base_fee_upward_adjustment: "0.125",
	max_dynamic_base_fee_downward_adjustment: "0.1",
	target_gas_used_per_block: 2 ** 40 + 200,
	maximum_fee_per_gas: "10000000000",
	register_pointer_disabled: true,
	pax_sstore_set_gas_eip2200: 20000,
	allowed_fee_denoms: [allowedFeeDenom],
	max_fee_token_spread: "0.025",
	fee_token_enabled: true,
	fee_token_distribution: true,
	max_fee_token_rate_age: 3600,
};
const fixtures = {
	Params: params,
	AllowedFeeDenom: allowedFeeDenom,
	ChainConfig: {
		cancun_time: -1,
		prague_time: 1700000000,
		verkle_time: 1800000000,
		pax_sstore_set_gas_eip2200: 2 ** 40 + 20000,
	},
	GenesisState: {
		params,
		address_associations: [{ pax_address: "pax1sender", eth_address: "0x1234" }],
		codes: [{ address: "0x1234", code: Uint8Array.from([1, 2]) }],
		states: [{ address: "0x1234", key: Uint8Array.from([3]), value: Uint8Array.from([4]) }],
		nonces: [{ address: "0x1234", nonce: 7 }],
		serialized: [{ prefix: Uint8Array.from([5]), key: Uint8Array.from([6]), value: Uint8Array.from([7]) }],
	},
};
const aliases = {
	Params: {
		registerPointerDisabled: true,
		paxSstoreSetGasEip2200: 20000,
		allowedFeeDenoms: [{ denom: "usid", rate: "3", rateUpdateHeight: 7 }],
		maxFeeTokenSpread: "0.025",
		feeTokenEnabled: true,
		feeTokenDistribution: true,
		maxFeeTokenRateAge: 3600,
	},
	AllowedFeeDenom: { rateUpdateHeight: 7 },
	ChainConfig: { paxSstoreSetGasEip2200: 20000 },
};
const imports = `
import { Params, AllowedFeeDenom, aminoConverters as paramsAmino } from "../dist/types/encoding/evm/params";
import { ChainConfig, aminoConverters as configAmino } from "../dist/types/encoding/evm/config";
import { GenesisState, aminoConverters as genesisAmino } from "../dist/types/encoding/evm/genesis";
`;

function literal(value) {
	if (value instanceof Uint8Array) {
		return `Uint8Array.from(${JSON.stringify(Array.from(value))})`;
	}
	if (Array.isArray(value)) {
		return `[${value.map(literal).join(", ")}]`;
	}
	if (value !== null && typeof value === "object") {
		return `{ ${Object.entries(value).map(([key, entry]) => `${key}: ${literal(entry)}`).join(", ")} }`;
	}
	return JSON.stringify(value);
}

function compileProbe(source) {
	const fileName = path.join(__dirname, "evm-encoding-contract.probe.ts");
	const options = {
		strict: true,
		noEmit: true,
		skipLibCheck: false,
		esModuleInterop: true,
		target: ts.ScriptTarget.ES2022,
		module: ts.ModuleKind.CommonJS,
		moduleResolution: ts.ModuleResolutionKind.Node10,
		types: [],
	};
	const host = ts.createCompilerHost(options);
	const getSourceFile = host.getSourceFile.bind(host);
	host.getSourceFile = (file, languageVersion, onError, shouldCreateNewSourceFile) =>
		file === fileName
			? ts.createSourceFile(file, source, languageVersion, true)
			: getSourceFile(file, languageVersion, onError, shouldCreateNewSourceFile);
	const program = ts.createProgram([fileName], options, host);
	return ts.getPreEmitDiagnostics(program).map((diagnostic) => ({
		file: diagnostic.file?.fileName,
		line: diagnostic.file?.getLineAndCharacterOfPosition(diagnostic.start).line,
		code: diagnostic.code,
		message: ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n"),
	}));
}

function evm_encoding_declarations_match_codec_fields() {
	const source = Object.entries(fixtures).map(([name, value]) => `
const expected${name} = ${literal(value)};
const full${name}: ${name} = expected${name};
const keys${name}: Record<keyof ${name}, true> = ${literal(Object.fromEntries(Object.keys(value).map((key) => [key, true])))};
const created${name}: ${name} = ${name}.create(expected${name});
const partial${name}: ${name} = ${name}.fromPartial(expected${name});
const decoded${name}: ${name} = ${name}.decode(${name}.encode(full${name}).finish());
const json${name}: ${name} = ${name}.fromJSON(${name}.toJSON(full${name}));
${name === "GenesisState" ? "" : `const exact${name}: ${name === "Params" ? 'Omit<typeof expectedParams, "whitelisted_cw_code_hashes_for_delegate_call"> & { whitelisted_cw_code_hashes_for_delegate_call: Uint8Array[] }' : `typeof expected${name}`} = decoded${name};`}
`).join("\n");
	assert.deepEqual(compileProbe(`${imports}${source}
const paramsConverted: Params = paramsAmino["/paxprotocol.paxchain.evm.Params"].fromAmino(fullParams);
const denomConverted: AllowedFeeDenom = paramsAmino["/paxprotocol.paxchain.evm.AllowedFeeDenom"].toAmino(fullAllowedFeeDenom);
const configConverted: ChainConfig = configAmino["/paxprotocol.paxchain.evm.ChainConfig"].toAmino(fullChainConfig);
const nested: Params | undefined = decodedGenesisState.params;
const genesisConverted: GenesisState = genesisAmino["/paxprotocol.paxchain.evm.GenesisState"].fromAmino(fullGenesisState);
const genesisAminoParams: Params | undefined = genesisAmino["/paxprotocol.paxchain.evm.GenesisState"].toAmino(fullGenesisState).params;
const noParams: GenesisState = { address_associations: [], codes: [], states: [], nonces: [], serialized: [] };
`), []);
}

function evm_encoding_declarations_reject_aliases_and_missing_fields() {
	const rejected = [];
	for (const [name, fields] of Object.entries(aliases)) {
		for (const [field, value] of Object.entries(fields)) {
			for (const method of ["create", "fromPartial"]) {
				rejected.push(`${name}.${method}({ ${field}: ${literal(value)} });`);
			}
			rejected.push(`${name}.decode(new Uint8Array()).${field};`);
			if (name === "Params") {
				rejected.push(`GenesisState.create({ params: { ${field}: ${literal(value)} } });`);
			}
		}
	}
	for (const [name, fields] of Object.entries(fixtures)) {
		for (const [field, value] of Object.entries(fields)) {
			if (name !== "GenesisState" || field !== "params") {
				const remaining = Object.fromEntries(Object.entries(fields).filter(([key]) => key !== field));
				rejected.push(`const missing_${name}_${field}: ${name} = ${literal(remaining)};`);
			}
			const wrongValue = typeof value === "boolean" ? "wrong" : false;
			rejected.push(`${name}.create({ ${field}: ${literal(wrongValue)} });`);
		}
	}
	rejected.push("Params.create({ allowed_fee_denoms: [{ rateUpdateHeight: 7 }] });");
	rejected.push("GenesisState.create({ params: { allowed_fee_denoms: [{ rateUpdateHeight: 7 }] } });");
	const prefix = imports.trim();
	const firstLine = prefix.split("\n").length;
	const diagnostics = compileProbe(`${prefix}\n${rejected.join("\n")}\n`);
	assert.equal(diagnostics.length, rejected.length, JSON.stringify(diagnostics, null, 2));
	for (const [index, source] of rejected.entries()) {
		const matching = diagnostics.filter((diagnostic) => diagnostic.line === firstLine + index);
		assert.equal(matching.length, 1, `${source}\n${JSON.stringify(diagnostics, null, 2)}`);
		assert.equal(matching[0].file, path.join(__dirname, "evm-encoding-contract.probe.ts"));
		assert.ok([2322, 2339, 2353, 2551, 2741].includes(matching[0].code), JSON.stringify(matching[0]));
	}
}


function checkRuntime(modules) {
	for (const [name, expected] of Object.entries(fixtures)) {
		const module = modules.find((candidate) => candidate[name]);
		const codec = module[name];
		const converter = module.aminoConverters[`/${codec.$type}`];
		const results = [
			codec.create(expected),
			codec.fromPartial(expected),
			codec.decode(codec.encode(expected).finish()),
			codec.fromJSON(codec.toJSON(expected)),
			converter.fromAmino(converter.toAmino(expected)),
		];
		for (const result of results) {
			assert.deepEqual(result, expected, name);
			for (const alias of Object.keys(aliases[name] ?? {})) {
				assert.equal(Object.hasOwn(result, alias), false, `${name}.${alias}`);
			}
		}
	}
}

function evm_encoding_runtime_matches_declared_fields_cjs() {
	checkRuntime([
		require("../dist/cjs/encoding/evm/params.js"),
		require("../dist/cjs/encoding/evm/config.js"),
		require("../dist/cjs/encoding/evm/genesis.js"),
	]);
}

async function evm_encoding_runtime_matches_declared_fields_esm() {
	checkRuntime(await Promise.all([
		import("../dist/esm/encoding/evm/params.js"),
		import("../dist/esm/encoding/evm/config.js"),
		import("../dist/esm/encoding/evm/genesis.js"),
	]));
}

test(evm_encoding_declarations_match_codec_fields);
test(evm_encoding_declarations_reject_aliases_and_missing_fields);
test(evm_encoding_runtime_matches_declared_fields_cjs);
test(evm_encoding_runtime_matches_declared_fields_esm);
