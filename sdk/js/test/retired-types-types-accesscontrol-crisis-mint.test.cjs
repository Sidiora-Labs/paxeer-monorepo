const assert = require("node:assert/strict");
const test = require("node:test");

function types_types_accesscontrol_crisis_mint_modules_are_gone() {
	for (const modulePath of [
		"../dist/types/types/cosmos/accesscontrol/accesscontrol.d.ts",
		"../dist/types/types/cosmos/accesscontrol/constants.d.ts",
		"../dist/types/types/cosmos/accesscontrol/index.d.ts",
		"../dist/types/types/cosmos/accesscontrol/legacy.d.ts",
		"../dist/types/types/cosmos/crisis/v1beta1/genesis.d.ts",
		"../dist/types/types/cosmos/crisis/v1beta1/index.d.ts",
		"../dist/types/types/cosmos/crisis/v1beta1/tx.d.ts",
		"../dist/types/types/cosmos/mint/v1beta1/genesis.d.ts",
		"../dist/types/types/cosmos/mint/v1beta1/index.d.ts",
		"../dist/types/types/cosmos/mint/v1beta1/mint.d.ts",
		"../dist/types/types/cosmos/mint/v1beta1/query.d.ts",
	]) {
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.strictEqual(
		Encoder.cosmos.bank.v1beta1.MsgSend,
		require("../dist/cjs/encoding/cosmos/bank/v1beta1/tx.js").MsgSend,
	);
	for (const moduleName of ["accesscontrol", "crisis", "mint"]) {
		assert.equal(Object.hasOwn(Encoder.cosmos, moduleName), false, moduleName);
	}
}

test(types_types_accesscontrol_crisis_mint_modules_are_gone);
