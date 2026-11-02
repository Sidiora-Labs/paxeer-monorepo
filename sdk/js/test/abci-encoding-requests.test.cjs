const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const { Encoder } = require("@pax-js/cosmos/encoding");
const abci = require("../dist/cjs/encoding/tendermint/abci/types.js");
const Request = Encoder.tendermint.abci.Request;

function abci_encoding_has_no_request_messages() {
	assert.equal(Request, undefined);
	assert.equal(typeof Encoder.tendermint.abci.ResponseInfo.encode, "function");
	assert.deepEqual(
		Object.getOwnPropertyNames(Encoder.tendermint.abci).filter((name) =>
			name.startsWith("Request"),
		),
		[],
	);
	assert.deepEqual(
		abci.registry.filter(([typeUrl]) =>
			typeUrl.startsWith("/tendermint.abci.Request"),
		),
		[],
	);
	assert.deepEqual(
		Object.keys(abci.aminoConverters).filter((typeUrl) =>
			typeUrl.startsWith("/tendermint.abci.Request"),
		),
		[],
	);

	for (const file of [
		"dist/cjs/encoding/tendermint/abci/types.js",
		"dist/esm/encoding/tendermint/abci/types.js",
		"dist/types/encoding/tendermint/abci/types.d.ts",
	]) {
		assert.doesNotMatch(
			readFileSync(path.join(__dirname, "..", file), "utf8"),
			/\b(?:createBase)?Request\w*\b/,
			file,
		);
	}
}

test(abci_encoding_has_no_request_messages);
