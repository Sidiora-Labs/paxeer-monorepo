const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_cjs_accesscontrol_x_modules_are_gone() {
	for (const file of ["genesis.js", "gov.js", "index.js", "query.js", "tx.js"]) {
		assert.throws(
			() => require.resolve(`../dist/cjs/encoding/cosmos/accesscontrol_x/${file}`),
			{ code: "MODULE_NOT_FOUND" },
			file,
		);
	}

	assert.doesNotThrow(() => {
		const { Encoder } = require("@pax-js/cosmos/encoding");
		assert.equal(typeof Encoder, "object");
		assert.notEqual(Encoder, null);
	});
}

test(encoding_cjs_accesscontrol_x_modules_are_gone);
