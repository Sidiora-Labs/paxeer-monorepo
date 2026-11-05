const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_esm_accesscontrol_crisis_mint_modules_are_gone() {
	for (const modulePath of [
		"../dist/esm/encoding/cosmos/accesscontrol/accesscontrol.js",
		"../dist/esm/encoding/cosmos/accesscontrol/constants.js",
		"../dist/esm/encoding/cosmos/accesscontrol/index.js",
		"../dist/esm/encoding/cosmos/accesscontrol/legacy.js",
		"../dist/esm/encoding/cosmos/crisis/v1beta1/genesis.js",
		"../dist/esm/encoding/cosmos/crisis/v1beta1/index.js",
		"../dist/esm/encoding/cosmos/crisis/v1beta1/tx.js",
		"../dist/esm/encoding/cosmos/mint/v1beta1/genesis.js",
		"../dist/esm/encoding/cosmos/mint/v1beta1/index.js",
		"../dist/esm/encoding/cosmos/mint/v1beta1/mint.js",
		"../dist/esm/encoding/cosmos/mint/v1beta1/query.js",
	]) {
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}
}

test(encoding_esm_accesscontrol_crisis_mint_modules_are_gone);
