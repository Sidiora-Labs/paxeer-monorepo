const assert = require("node:assert/strict");
const test = require("node:test");

function types_types_accesscontrol_x_modules_are_gone() {
	for (const filename of [
		"genesis.d.ts",
		"gov.d.ts",
		"index.d.ts",
		"query.d.ts",
		"tx.d.ts",
	]) {
		const modulePath = `../dist/types/types/cosmos/accesscontrol_x/${filename}`;
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.ok(Encoder.cosmos.bank);
	assert.equal(Object.hasOwn(Encoder.cosmos, "accesscontrol_x"), false);
}

test(types_types_accesscontrol_x_modules_are_gone);
