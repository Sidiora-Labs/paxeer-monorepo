const assert = require("node:assert/strict");
const test = require("node:test");

function types_cjs_accesscontrol_x_modules_are_gone() {
	for (const file of ["genesis.js", "gov.js", "index.js", "query.js", "tx.js"]) {
		const modulePath = `../dist/cjs/types/cosmos/accesscontrol_x/${file}`;
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}

	const { Encoder } = require("@pax-js/cosmos/encoding");
	assert.equal(typeof Encoder.cosmos, "object");
	assert.equal(Object.hasOwn(Encoder.cosmos, "accesscontrol_x"), false);
	assert.ok(Encoder.cosmos.bank);
}

test(types_cjs_accesscontrol_x_modules_are_gone);
