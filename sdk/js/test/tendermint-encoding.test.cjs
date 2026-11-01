const assert = require("node:assert/strict");
const test = require("node:test");
const { Encoder } = require("@pax-js/cosmos/encoding");
const types = require("../dist/cjs/encoding/tendermint/types/types.js");
const version = require("../dist/cjs/encoding/tendermint/version/types.js");
const Proposal = Encoder.tendermint.types.Proposal;
const Consensus = Encoder.tendermint.version.Consensus;

function proposal_encodes_head_fields() {
	assert.strictEqual(Proposal, types.Proposal);
	const proposal = Proposal.create({
		type: 32,
		height: 9,
		round: 2,
		pol_round: 1,
		block_id: { hash: Uint8Array.of(1) },
		timestamp: new Date(1000),
		signature: Uint8Array.of(0xaa),
		tx_keys: [
			{ tx_key: Uint8Array.of(1, 2) },
			{ tx_key: Uint8Array.of(3) },
			{},
		],
		evidence: {
			evidence: [{ duplicate_vote_evidence: { total_voting_power: 5 } }],
		},
		last_commit: { height: 8 },
		header: {
			chain_id: "pax",
			height: 9,
			proposer_address: Uint8Array.of(0xbb),
		},
		proposer_address: Uint8Array.of(0xcc, 0xdd),
	});
	const expected = Uint8Array.from([
		0x08, 0x20,
		0x10, 0x09,
		0x18, 0x02,
		0x20, 0x01,
		0x2a, 0x03, 0x0a, 0x01, 0x01,
		0x32, 0x02, 0x08, 0x01,
		0x3a, 0x01, 0xaa,
		0x42, 0x04, 0x0a, 0x02, 0x01, 0x02,
		0x42, 0x03, 0x0a, 0x01, 0x03,
		0x42, 0x00,
		0x4a, 0x06, 0x0a, 0x04, 0x0a, 0x02, 0x18, 0x05,
		0x52, 0x02, 0x08, 0x08,
		0x5a, 0x0a, 0x12, 0x03, 0x70, 0x61, 0x78, 0x18, 0x09, 0x72, 0x01, 0xbb,
		0x62, 0x02, 0xcc, 0xdd,
	]);
	assert.deepEqual(Proposal.encode(proposal).finish(), expected);
	assert.deepEqual(Proposal.decode(expected), proposal);
	assert.deepEqual(Proposal.fromPartial(proposal), proposal);
	const json = Proposal.toJSON(proposal);
	assert.deepEqual(json.tx_keys, [{ tx_key: "AQI=" }, { tx_key: "Aw==" }, {}]);
	assert.equal(json.proposer_address, "zN0=");
	assert.deepEqual(Proposal.fromJSON(json), proposal);
	const empty = Proposal.create();
	assert.deepEqual(Proposal.encode(empty).finish(), new Uint8Array(0));
	assert.deepEqual(Proposal.decode(new Uint8Array(0)), empty);
	assert.deepEqual(Proposal.fromJSON({}), empty);
	assert.deepEqual(empty.tx_keys, []);
	assert.equal(empty.evidence, undefined);
	assert.equal(empty.last_commit, undefined);
	assert.equal(empty.header, undefined);
	assert.deepEqual(empty.proposer_address, new Uint8Array(0));
	assert.deepEqual(Proposal.toJSON(empty), {});
	const addressOnly = Uint8Array.of(0x62, 0x02, 0xcc, 0xdd);
	assert.deepEqual(Proposal.decode(addressOnly).proposer_address, proposal.proposer_address);
	assert.deepEqual(
		Proposal.encode(Proposal.create({ proposer_address: proposal.proposer_address })).finish(),
		addressOnly,
	);
}

function version_namespace_has_only_consensus() {
	const namespace = Encoder.tendermint.version;
	assert.strictEqual(namespace.Consensus, version.Consensus);
	assert.deepEqual(
		Object.keys(namespace).filter((name) => namespace[name]?.$type),
		["Consensus"],
	);
	assert.equal("App" in namespace, false);
	assert.equal("App" in version, false);
	assert.deepEqual(version.registry, [["/tendermint.version.Consensus", version.Consensus]]);
	assert.deepEqual(Object.keys(version.aminoConverters), ["/tendermint.version.Consensus"]);
	const consensus = Consensus.create({ block: 11, app: 1 });
	const consensusBytes = Uint8Array.of(0x08, 0x0b, 0x10, 0x01);
	assert.deepEqual(Consensus.encode(consensus).finish(), consensusBytes);
	assert.deepEqual(Consensus.decode(consensusBytes), consensus);
	for (const name of ["ExtendedCommit", "ExtendedCommitSig"]) {
		assert.equal(name in Encoder.tendermint.types, false, name);
		assert.equal(name in types, false, name);
		assert.equal(types.registry.some(([url]) => url === `/tendermint.types.${name}`), false, name);
		assert.equal(`/tendermint.types.${name}` in types.aminoConverters, false, name);
	}
}

test(proposal_encodes_head_fields);
test(version_namespace_has_only_consensus);
