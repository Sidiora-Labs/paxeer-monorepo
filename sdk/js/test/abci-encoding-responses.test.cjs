const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const { Encoder } = require("@pax-js/cosmos/encoding");
const abci = require("../dist/cjs/encoding/tendermint/abci/types.js");
const { paxProtoRegistry } = require("../dist/cjs/encoding/registry.js");
const { aminoConverters } = require("../dist/cjs/encoding/amino.js");

function abci_encoding_keeps_only_head_messages() {
	const retired = [
		"Response",
		"ResponseException",
		"ResponseEcho",
		"ResponseFlush",
		"ResponseInitChain",
		"ResponseListSnapshots",
		"ResponseOfferSnapshot",
		"ResponseLoadSnapshotChunk",
		"ResponseApplySnapshotChunk",
		"ResponsePrepareProposal",
		"ResponseProcessProposal",
		"ResponseExtendVote",
		"ResponseVerifyVoteExtension",
		"CommitInfo",
		"ExtendedCommitInfo",
		"ExtendedVoteInfo",
		"Misbehavior",
		"Snapshot",
		"TxRecord",
		"Validator",
		"VoteInfo",
	];
	const retained = [
		"ResponseInfo",
		"ResponseQuery",
		"ResponseCheckTx",
		"ResponseDeliverTx",
		"ResponseCommit",
		"ResponseFinalizeBlock",
		"Event",
		"EventAttribute",
		"ExecTxResult",
		"TxResult",
		"ValidatorUpdate",
	];
	const proto = readFileSync(
		path.resolve(__dirname, "../../../consensus/proto/tendermint/abci/types.proto"),
		"utf8",
	);
	const headMessages = new Set(
		Array.from(proto.matchAll(/^message (\w+)\s*\{/gm), ([, name]) => name),
	);
	const registeredTypes = new Map(paxProtoRegistry);
	for (const name of retired) {
		const typeUrl = `/tendermint.abci.${name}`;
		assert.equal(headMessages.has(name), false, name);
		assert.equal(name in Encoder.tendermint.abci, false, name);
		assert.equal(name in abci, false, name);
		assert.equal(registeredTypes.has(typeUrl), false, typeUrl);
		assert.equal(Object.hasOwn(aminoConverters, typeUrl), false, typeUrl);
	}
	assert.deepEqual(
		Object.keys(Encoder.tendermint.abci)
			.filter((name) => typeof Encoder.tendermint.abci[name]?.encode === "function")
			.sort(),
		[...retained].sort(),
	);
	assert.deepEqual(
		abci.registry.map(([typeUrl]) => typeUrl).sort(),
		retained.map((name) => `/tendermint.abci.${name}`).sort(),
	);
	assert.deepEqual(
		Object.keys(abci.aminoConverters).sort(),
		retained.map((name) => `/tendermint.abci.${name}`).sort(),
	);
	for (const name of retained) {
		const typeUrl = `/tendermint.abci.${name}`;
		assert.ok(headMessages.has(name), name);
		assert.equal(typeof abci[name].encode, "function", name);
		assert.equal(typeof abci[name].decode, "function", name);
		assert.strictEqual(Encoder.tendermint.abci[name], abci[name], name);
		assert.strictEqual(registeredTypes.get(typeUrl), abci[name], typeUrl);
		assert.strictEqual(aminoConverters[typeUrl], abci.aminoConverters[typeUrl], typeUrl);
	}
	const retiredDeclaration = new RegExp(
		`\\b(?:createBase)?(?:${retired.join("|")})(?:_type)?\\b`,
	);
	for (const file of [
		"dist/cjs/encoding/tendermint/abci/types.js",
		"dist/esm/encoding/tendermint/abci/types.js",
		"dist/types/encoding/tendermint/abci/types.d.ts",
	]) {
		const source = readFileSync(path.join(__dirname, "..", file), "utf8");
		assert.doesNotMatch(source, retiredDeclaration, file);
		for (const name of retained) {
			assert.match(source, new RegExp(`\\btendermint\\.abci\\.${name}\\b`), file);
		}
	}
}

function abci_event_round_trips() {
	const event = {
		type: "transfer",
		attributes: [
			{ key: "sender", value: "pax1alice", index: true },
			{ key: "amount", value: "42upax", index: false },
		],
	};
	const expected = Uint8Array.from(
		Buffer.from(
			"0a087472616e7366657212150a0673656e646572120970617831616c696365180112100a06616d6f756e741206343275706178",
			"hex",
		),
	);
	const { Event, EventAttribute } = Encoder.tendermint.abci;
	const encoded = Event.encode(event).finish();
	assert.deepEqual(encoded, expected);
	const decoded = Event.decode(expected);
	assert.deepEqual(decoded, event);
	assert.deepEqual(Event.encode(decoded).finish(), expected);
	for (const attribute of event.attributes) {
		assert.deepEqual(
			EventAttribute.decode(EventAttribute.encode(attribute).finish()),
			attribute,
		);
	}
	const { TxResponse } = Encoder.cosmos.base.abci.v1beta1;
	const response = TxResponse.create({ events: [event] });
	assert.deepEqual(
		TxResponse.decode(TxResponse.encode(response).finish()).events,
		[event],
	);
}

test(abci_encoding_keeps_only_head_messages);
test(abci_event_round_trips);
