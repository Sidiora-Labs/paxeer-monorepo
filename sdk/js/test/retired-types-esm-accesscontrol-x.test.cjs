const assert = require("node:assert/strict");
const test = require("node:test");

function types_esm_accesscontrol_x_modules_are_gone() {
	for (const name of ["genesis", "gov", "index", "query", "tx"]) {
		const modulePath = `../dist/esm/types/cosmos/accesscontrol_x/${name}.js`;
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.equal(typeof Encoder, "object");
	assert.notEqual(Encoder, null);
	assert.equal("accesscontrol_x" in Encoder.cosmos, false);
}

test(types_esm_accesscontrol_x_modules_are_gone);
