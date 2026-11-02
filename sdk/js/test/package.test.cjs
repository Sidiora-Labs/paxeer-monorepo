const assert = require("node:assert/strict");
const { existsSync, readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const packageRoot = path.resolve(__dirname, "..");

function sdk_js_has_no_vendor_pin() {
	assert.equal(existsSync(path.join(packageRoot, "VENDOR.kvx")), false);
	const readme = readFileSync(path.join(packageRoot, "README.md"), "utf8");
	assert.match(readme, /## Provenance\b/);
	assert.match(readme, /upstream tag `v1\.0\.6-pax\.1`/);
	assert.match(
		readme,
		/`dist`\s+files are edited and maintained in this repository/,
	);
}

function package_entries_resolve_under_require() {
	const encoding = require("@pax-js/cosmos/encoding");
	const rest = require("@pax-js/cosmos/rest");
	const evm = require("@pax-js/cosmos/types/evm");

	assert.equal(typeof encoding.Encoder, "object");
	assert.notEqual(encoding.Encoder, null);
	assert.strictEqual(
		encoding.Encoder,
		require("../dist/cjs/encoding/index.js").Encoder,
	);
	assert.equal(typeof rest.Querier, "object");
	assert.notEqual(rest.Querier, null);
	assert.strictEqual(
		rest.Querier,
		require("../dist/cjs/rest/index.js").Querier,
	);
	assert.equal(typeof evm, "object");
	assert.notEqual(evm, null);
	assert.ok(Object.keys(evm).length > 0);
	assert.strictEqual(evm, require("../dist/cjs/types/evm/index.js"));

	for (const subpath of ["encoding", "rest", "types/evm"]) {
		assert.equal(
			require.resolve(`@pax-js/cosmos/${subpath}`),
			path.join(packageRoot, "dist", "cjs", subpath, "index.js"),
		);
	}
	const manifest = require("../package.json");
	assert.equal(Object.hasOwn(manifest, "main"), false);
	assert.equal(Object.hasOwn(manifest, "module"), false);
}

function package_types_subpath_resolves_to_runtime_module() {
	const exports = require("../package.json").exports;
	assert.equal(exports["./types/*"].import, "./dist/esm/types/*/index.js");
	assert.equal(exports["./types/*"].require, "./dist/cjs/types/*/index.js");
	assert.equal(exports["./types/*"].types, "./dist/types/types/*/index.d.ts");
	assert.ok(
		require
			.resolve("@pax-js/cosmos/types/evm")
			.endsWith(path.join("dist", "cjs", "types", "evm", "index.js")),
	);
}

test(sdk_js_has_no_vendor_pin);
test(package_entries_resolve_under_require);
test(package_types_subpath_resolves_to_runtime_module);
