const assert = require("node:assert/strict");
const test = require("node:test");

function types_esm_accesscontrol_crisis_mint_modules_are_gone() {
	for (const modulePath of [
		"../dist/esm/types/cosmos/accesscontrol/accesscontrol.js",
		"../dist/esm/types/cosmos/accesscontrol/constants.js",
		"../dist/esm/types/cosmos/accesscontrol/index.js",
		"../dist/esm/types/cosmos/accesscontrol/legacy.js",
		"../dist/esm/types/cosmos/crisis/v1beta1/genesis.js",
		"../dist/esm/types/cosmos/crisis/v1beta1/index.js",
		"../dist/esm/types/cosmos/crisis/v1beta1/tx.js",
		"../dist/esm/types/cosmos/mint/v1beta1/genesis.js",
		"../dist/esm/types/cosmos/mint/v1beta1/index.js",
		"../dist/esm/types/cosmos/mint/v1beta1/mint.js",
		"../dist/esm/types/cosmos/mint/v1beta1/query.js",
	]) {
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}
}

test(types_esm_accesscontrol_crisis_mint_modules_are_gone);
