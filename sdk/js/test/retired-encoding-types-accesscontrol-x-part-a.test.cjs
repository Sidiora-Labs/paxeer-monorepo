const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_types_accesscontrol_x_part_a_modules_are_gone() {
	for (const filename of ["genesis.d.ts", "gov.d.ts"]) {
		const modulePath = `../dist/types/encoding/cosmos/accesscontrol_x/${filename}`;
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.equal(typeof Encoder.cosmos, "object");
	assert.equal("accesscontrol_x" in Encoder.cosmos, false);
}

test(encoding_types_accesscontrol_x_part_a_modules_are_gone);
