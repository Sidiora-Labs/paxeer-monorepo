const assert = require("node:assert/strict");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

const imports = `
import type { ChainConfig } from "../dist/types/types/evm/config";
import type { Params, ParamsPreV580 } from "../dist/types/types/evm/params";
import type { Asset } from "../dist/types/types/evm/tx";
import { AssetType } from "../dist/types/types/evm/enums";
import type { DenomAuthorityMetadata } from "../dist/types/types/tokenfactory/authorityMetadata";
import type { QueryDenomsFromCreatorRequest, QueryDenomsFromCreatorResponse } from "../dist/types/types/tokenfactory/query";
`;

const existingParams = `{
    priority_normalizer: "1",
    base_fee_per_gas: "0.1",
    minimum_fee_per_gas: "0.01",
    whitelisted_cw_code_hashes_for_delegate_call: [new Uint8Array([1, 2])],
    deliver_tx_hook_wasm_gas_limit: 100000,
    max_dynamic_base_fee_upward_adjustment: "0.1",
    max_dynamic_base_fee_downward_adjustment: "0.1",
    target_gas_used_per_block: 1000000,
    maximum_fee_per_gas: "10"
}`;

const newParams = {
	registerPointerDisabled: "true",
	paxSstoreSetGasEip2200: "20000",
	allowedFeeDenoms:
		'[{ denom: "usdl", rate: "1000000", rateUpdateHeight: 42 }]',
	maxFeeTokenSpread: '"0.025"',
	feeTokenEnabled: "true",
	feeTokenDistribution: "false",
	maxFeeTokenRateAge: "3600",
};

function compileProbe(name, source) {
	const fileName = path.join(__dirname, `${name}.ts`);
	const options = {
		strict: true,
		noEmit: true,
		target: ts.ScriptTarget.ES2022,
		module: ts.ModuleKind.CommonJS,
		moduleResolution: ts.ModuleResolutionKind.Node10,
		types: [],
	};
	const host = ts.createCompilerHost(options);
	const getSourceFile = host.getSourceFile.bind(host);
	host.getSourceFile = (
		file,
		languageVersion,
		onError,
		shouldCreateNewSourceFile,
	) =>
		file === fileName
			? ts.createSourceFile(file, source, languageVersion, true)
			: getSourceFile(file, languageVersion, onError, shouldCreateNewSourceFile);
	const program = ts.createProgram([fileName], options, host);
	return ts.getPreEmitDiagnostics(program);
}

function diagnosticText(diagnostics) {
	return diagnostics
		.map((diagnostic) =>
			`TS${diagnostic.code}: ${ts.flattenDiagnosticMessageText(
				diagnostic.messageText,
				"\n",
			)}`,
		)
		.join("\n");
}

function app_declarations_accept_head_fields() {
	const diagnostics = compileProbe(
		"head-fields",
		`${imports}
const config: ChainConfig = {
    cancun_time: 0, prague_time: -1, verkle_time: -1,
    paxSstoreSetGasEip2200: 20000
};
const params: Params = {
    ...${existingParams},
    ${Object.entries(newParams)
			.map(([field, value]) => `${field}: ${value}`)
			.join(",\n    ")}
};
const asset: Asset = {
    asset_type: AssetType.UNRECOGNIZED, contract_address: "0x1234", denom: "usdl"
};
const authority: DenomAuthorityMetadata = {
    admin: "pax1admin", issuerDid: "did:layerx:" + "a".repeat(64)
};
const request: QueryDenomsFromCreatorRequest = {
    creator: "pax1creator",
    pagination: { key: new Uint8Array([1]), offset: 0, limit: 10, count_total: true, reverse: false }
};
const response: QueryDenomsFromCreatorResponse = {
    denoms: [asset.denom], pagination: { next_key: new Uint8Array([2]), total: 1 }
};
const unpagedRequest: QueryDenomsFromCreatorRequest = { creator: request.creator };
const unpagedResponse: QueryDenomsFromCreatorResponse = { denoms: response.denoms };
`,
	);
	assert.equal(diagnostics.length, 0, diagnosticText(diagnostics));
}

function app_declarations_keep_existing_fields() {
	const fields = Object.keys(newParams);
	const omissions = fields.map((missing) => {
		const remaining = Object.entries(newParams)
			.filter(([field]) => field !== missing)
			.map(([field, value]) => `${field}: ${value}`)
			.join(", ");
		return `const missing_${missing}: Params = { ...existing, ${remaining} };`;
	});
	const diagnostics = compileProbe(
		"existing-fields",
		`${imports}
const existing = ${existingParams};
const legacyParams: Omit<Params, ${fields
			.map((field) => JSON.stringify(field))
			.join(" | ")}> = existing;
const legacyAuthority: Omit<DenomAuthorityMetadata, "issuerDid"> = { admin: "pax1admin" };
const historicalParams: ParamsPreV580 = {
    priority_normalizer: existing.priority_normalizer,
    base_fee_per_gas: existing.base_fee_per_gas,
    minimum_fee_per_gas: existing.minimum_fee_per_gas,
    whitelisted_cw_code_hashes_for_delegate_call: existing.whitelisted_cw_code_hashes_for_delegate_call
};
${omissions.join("\n")}
const missing_issuerDid: DenomAuthorityMetadata = { ...legacyAuthority };
`,
	);
	const expectedFields = [...fields, "issuerDid"];
	assert.equal(
		diagnostics.length,
		expectedFields.length,
		diagnosticText(diagnostics),
	);
	for (const field of expectedFields) {
		const matching = diagnostics.filter((diagnostic) =>
			ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n")
				.startsWith(`Property '${field}' is missing in type`),
		);
		assert.equal(matching.length, 1, `${field}: ${diagnosticText(diagnostics)}`);
		assert.equal(matching[0].code, 2741, diagnosticText(matching));
		const declaration = matching[0].file.text.slice(
			matching[0].start,
			matching[0].start + matching[0].length,
		);
		assert.equal(declaration, `missing_${field}`);
	}
}

test(app_declarations_accept_head_fields);
test(app_declarations_keep_existing_fields);
