const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_types_accesscontrol_x_part_b_modules_are_gone() {
	for (const file of [
		"../dist/types/encoding/cosmos/accesscontrol_x/index.d.ts",
		"../dist/types/encoding/cosmos/accesscontrol_x/query.d.ts",
		"../dist/types/encoding/cosmos/accesscontrol_x/tx.d.ts",
	]) {
		assert.throws(() => require.resolve(file), { code: "MODULE_NOT_FOUND" }, file);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.ok(Encoder.cosmos.bank);
	assert.equal(Object.hasOwn(Encoder.cosmos, "accesscontrol_x"), false);
}

test(encoding_types_accesscontrol_x_part_b_modules_are_gone);
