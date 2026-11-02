const assert = require("node:assert/strict");
const test = require("node:test");
const paxProtoRegistry = require("../dist/cjs/encoding/registry.js").paxProtoRegistry;
const aminoConverters = require("../dist/cjs/encoding/amino.js").aminoConverters;

const retiredPrefixes = [
	"/cosmos.accesscontrol.",
	"/cosmos.accesscontrol_x.",
	"/cosmos.crisis.",
	"/cosmos.mint.",
];

function registry_has_no_type_url_absent_at_head() {
	const {
		registry: mintRegistry,
	} = require("../dist/cjs/encoding/mint/v1beta1/mint.js");
	assert.ok(Array.isArray(paxProtoRegistry));
	const registeredTypes = new Map(paxProtoRegistry);
	assert.ok(registeredTypes.has("/cosmos.bank.v1beta1.MsgSend"));
	for (const [typeUrl] of paxProtoRegistry) {
		for (const prefix of retiredPrefixes) {
			assert.equal(typeUrl.startsWith(prefix), false, typeUrl);
		}
	}
	assert.ok(mintRegistry.length > 0);
	for (const [typeUrl, messageType] of mintRegistry) {
		assert.strictEqual(registeredTypes.get(typeUrl), messageType, typeUrl);
	}
}

function amino_converters_have_no_type_url_absent_at_head() {
	assert.ok(Object.hasOwn(aminoConverters, "/cosmos.bank.v1beta1.MsgSend"));
	for (const typeUrl of Object.keys(aminoConverters)) {
		for (const prefix of retiredPrefixes) {
			assert.equal(typeUrl.startsWith(prefix), false, typeUrl);
		}
	}
}

test(registry_has_no_type_url_absent_at_head);
test(amino_converters_have_no_type_url_absent_at_head);
