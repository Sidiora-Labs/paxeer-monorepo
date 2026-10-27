const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_types_accesscontrol_crisis_mint_part_b_modules_are_gone() {
	for (const path of [
		"../dist/types/encoding/cosmos/crisis/v1beta1/index.d.ts",
		"../dist/types/encoding/cosmos/crisis/v1beta1/tx.d.ts",
		"../dist/types/encoding/cosmos/mint/v1beta1/genesis.d.ts",
		"../dist/types/encoding/cosmos/mint/v1beta1/index.d.ts",
		"../dist/types/encoding/cosmos/mint/v1beta1/mint.d.ts",
		"../dist/types/encoding/cosmos/mint/v1beta1/query.d.ts",
	]) {
		assert.throws(() => require.resolve(path), { code: "MODULE_NOT_FOUND" }, path);
	}
}

test(encoding_types_accesscontrol_crisis_mint_part_b_modules_are_gone);
