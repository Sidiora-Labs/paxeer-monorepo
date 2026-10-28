const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");

function retired_check_tx_type_is_absent_from_every_runtime_build() {
	const encoding = require("../dist/cjs/encoding/tendermint/abci/types.js");
	const types = require("../dist/cjs/types/tendermint/abci/types.js");
	for (const module of [encoding, types]) {
		for (const name of [
			"checkTxTypeFromJSON",
			"checkTxTypeToJSON",
			"CheckTxType",
		]) {
			assert.equal(Object.hasOwn(module, name), false, name);
		}
	}
	for (const name of ["misbehaviorTypeFromJSON", "misbehaviorTypeToJSON"]) {
		assert.equal(Object.hasOwn(encoding, name), true, name);
		assert.equal(typeof encoding[name], "function", name);
	}
	assert.equal(Object.hasOwn(types, "MisbehaviorType"), true);
	assert.equal(typeof types.MisbehaviorType, "object");
	for (const name of [
		"UNKNOWN",
		"DUPLICATE_VOTE",
		"LIGHT_CLIENT_ATTACK",
		"UNRECOGNIZED",
	]) {
		assert.equal(
			encoding.misbehaviorTypeFromJSON(name),
			types.MisbehaviorType[name],
		);
		assert.equal(
			encoding.misbehaviorTypeFromJSON(types.MisbehaviorType[name]),
			types.MisbehaviorType[name],
		);
		assert.equal(
			encoding.misbehaviorTypeToJSON(types.MisbehaviorType[name]),
			name,
		);
	}

	const esmEncoding = readFileSync(
		path.resolve(__dirname, "../dist/esm/encoding/tendermint/abci/types.js"),
		"utf8",
	);
	const esmTypes = readFileSync(
		path.resolve(__dirname, "../dist/esm/types/tendermint/abci/types.js"),
		"utf8",
	);
	for (const source of [esmEncoding, esmTypes]) {
		assert.doesNotMatch(source, /checkTxType|CheckTxType/);
	}
	assert.match(esmEncoding, /export function misbehaviorTypeFromJSON\(/);
	assert.match(esmEncoding, /export function misbehaviorTypeToJSON\(/);
	assert.match(esmTypes, /export var MisbehaviorType;/);
}

test(retired_check_tx_type_is_absent_from_every_runtime_build);
