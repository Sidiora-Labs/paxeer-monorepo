const assert = require("node:assert/strict");
const test = require("node:test");

function encoding_esm_accesscontrol_x_modules_are_gone() {
	for (const name of ["genesis", "gov", "index", "query", "tx"]) {
		const modulePath = `../dist/esm/encoding/cosmos/accesscontrol_x/${name}.js`;
		assert.throws(
			() => require.resolve(modulePath),
			{ code: "MODULE_NOT_FOUND" },
			modulePath,
		);
	}
}

test(encoding_esm_accesscontrol_x_modules_are_gone);
