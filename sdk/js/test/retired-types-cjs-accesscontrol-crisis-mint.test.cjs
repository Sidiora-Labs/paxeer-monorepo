const assert = require("node:assert/strict");
const test = require("node:test");

function types_cjs_accesscontrol_crisis_mint_modules_are_gone() {
	for (const modulePath of [
		"../dist/cjs/types/cosmos/accesscontrol/accesscontrol.js",
		"../dist/cjs/types/cosmos/accesscontrol/constants.js",
		"../dist/cjs/types/cosmos/accesscontrol/index.js",
		"../dist/cjs/types/cosmos/accesscontrol/legacy.js",
		"../dist/cjs/types/cosmos/crisis/v1beta1/genesis.js",
		"../dist/cjs/types/cosmos/crisis/v1beta1/index.js",
		"../dist/cjs/types/cosmos/crisis/v1beta1/tx.js",
		"../dist/cjs/types/cosmos/mint/v1beta1/genesis.js",
		"../dist/cjs/types/cosmos/mint/v1beta1/index.js",
		"../dist/cjs/types/cosmos/mint/v1beta1/mint.js",
		"../dist/cjs/types/cosmos/mint/v1beta1/query.js",
	]) {
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.equal(typeof Encoder, "object");
	assert.notEqual(Encoder, null);
	for (const name of ["accesscontrol", "crisis", "mint"]) {
		assert.equal(name in Encoder.cosmos, false, `Encoder.cosmos.${name}`);
	}
	assert.strictEqual(
		Encoder.cosmos.bank.v1beta1.MsgSend,
		require("../dist/cjs/encoding/cosmos/bank/v1beta1/tx.js").MsgSend,
	);
}

test(types_cjs_accesscontrol_crisis_mint_modules_are_gone);
