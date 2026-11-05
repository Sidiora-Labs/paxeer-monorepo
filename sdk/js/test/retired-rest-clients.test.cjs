const assert = require("node:assert/strict");
const test = require("node:test");

function retired_rest_clients_do_not_resolve() {
	const retiredClients = [
		"../dist/cjs/rest/cosmos/accesscontrol_x/query.js",
		"../dist/cjs/rest/cosmos/mint/v1beta1/query.js",
		"../dist/esm/rest/cosmos/accesscontrol_x/query.js",
		"../dist/esm/rest/cosmos/mint/v1beta1/query.js",
		"../dist/types/rest/cosmos/accesscontrol_x/query.d.ts",
		"../dist/types/rest/cosmos/mint/v1beta1/query.d.ts",
	];

	for (const client of retiredClients) {
		assert.throws(
			() => require.resolve(client),
			{ code: "MODULE_NOT_FOUND" },
			`${client} must not resolve`,
		);
	}
}

test(retired_rest_clients_do_not_resolve);
