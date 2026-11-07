const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_types_accesscontrol_crisis_mint_part_a_modules_are_gone() {
	for (const path of [
		"../dist/types/encoding/cosmos/accesscontrol/accesscontrol.d.ts",
		"../dist/types/encoding/cosmos/accesscontrol/constants.d.ts",
		"../dist/types/encoding/cosmos/accesscontrol/index.d.ts",
		"../dist/types/encoding/cosmos/accesscontrol/legacy.d.ts",
		"../dist/types/encoding/cosmos/crisis/v1beta1/genesis.d.ts",
	]) {
		assert.throws(
			() => require.resolve(path),
			{ code: "MODULE_NOT_FOUND" },
			path,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.ok(Encoder.cosmos.bank.v1beta1.MsgSend);
}

test(encoding_types_accesscontrol_crisis_mint_part_a_modules_are_gone);
