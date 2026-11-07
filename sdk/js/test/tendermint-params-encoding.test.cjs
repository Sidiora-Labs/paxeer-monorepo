const assert = require("node:assert/strict");
const test = require("node:test");
const {
	BlockParams,
	ConsensusParams,
} = require("../dist/cjs/encoding/tendermint/types/params.js");
const {
	PublicKey,
	aminoConverters,
} = require("../dist/cjs/encoding/tendermint/crypto/keys.js");

function block_params_encode_min_txs_and_max_gas_wanted() {
	const message = {
		max_bytes: 10,
		max_gas: 20,
		min_txs_in_block: 150,
		max_gas_wanted: 300,
	};
	const wire = Uint8Array.from([8, 10, 16, 20, 24, 150, 1, 32, 172, 2]);
	assert.deepEqual(BlockParams.encode(message).finish(), wire);
	assert.deepEqual(BlockParams.decode(wire), message);
	assert.deepEqual(BlockParams.create(message), message);
	assert.deepEqual(BlockParams.fromPartial(message), message);
	assert.deepEqual(BlockParams.toJSON(message), message);
	assert.deepEqual(
		BlockParams.fromJSON({
			max_bytes: "10",
			max_gas: "20",
			min_txs_in_block: "150",
			max_gas_wanted: "300",
		}),
		message,
	);

	const defaults = {
		max_bytes: 0,
		max_gas: 0,
		min_txs_in_block: 0,
		max_gas_wanted: 0,
	};
	assert.deepEqual(BlockParams.create(), defaults);
	assert.deepEqual(BlockParams.fromJSON({}), defaults);
	assert.deepEqual(BlockParams.decode(new Uint8Array()), defaults);
	assert.deepEqual(BlockParams.encode(defaults).finish(), new Uint8Array());
	assert.deepEqual(BlockParams.toJSON(defaults), {});

	const unlimited = { ...defaults, max_gas_wanted: -1 };
	const unlimitedWire = Uint8Array.from([
		32, 255, 255, 255, 255, 255, 255, 255, 255, 255, 1,
	]);
	assert.deepEqual(BlockParams.encode(unlimited).finish(), unlimitedWire);
	assert.deepEqual(BlockParams.decode(unlimitedWire), unlimited);
	const large = {
		...defaults,
		min_txs_in_block: 2 ** 32,
		max_gas_wanted: Number.MAX_SAFE_INTEGER,
	};
	assert.deepEqual(
		BlockParams.decode(BlockParams.encode(large).finish()),
		large,
	);
	const consensus = ConsensusParams.create({ block: message });
	assert.deepEqual(
		ConsensusParams.decode(ConsensusParams.encode(consensus).finish()).block,
		message,
	);
}

function public_key_decodes_only_ed25519() {
	const ed25519 = Uint8Array.from({ length: 32 }, (_, i) => i);
	const supportedWire = Uint8Array.from([10, ed25519.length, ...ed25519]);
	assert.deepEqual(PublicKey.decode(supportedWire), { ed25519 });
	assert.deepEqual(PublicKey.encode({ ed25519 }).finish(), supportedWire);
	assert.deepEqual(PublicKey.decode(Uint8Array.from([10, 0])), {
		ed25519: new Uint8Array(),
	});
	assert.deepEqual(
		PublicKey.encode({ ed25519: new Uint8Array() }).finish(),
		Uint8Array.from([10, 0]),
	);

	for (const [field, tag, length] of [
		["secp256k1", 18, 33],
		["sr25519", 26, 32],
	]) {
		const payload = new Uint8Array(length).fill(7);
		const unknownWire = Uint8Array.from([tag, length, ...payload]);
		const decoded = PublicKey.decode(unknownWire);
		assert.equal(decoded.ed25519, undefined);
		assert.equal(Object.hasOwn(decoded, "secp256k1"), false);
		assert.equal(Object.hasOwn(decoded, "sr25519"), false);
		assert.deepEqual(
			PublicKey.decode(Uint8Array.from([...unknownWire, ...supportedWire])),
			{ ed25519 },
		);
		assert.deepEqual(
			PublicKey.decode(Uint8Array.from([...supportedWire, ...unknownWire])),
			{ ed25519 },
		);
		assert.deepEqual(
			PublicKey.encode({ [field]: payload }).finish(),
			new Uint8Array(),
		);
		assert.deepEqual(PublicKey.fromPartial({ [field]: payload }), {
			ed25519: undefined,
		});
		assert.deepEqual(PublicKey.fromJSON({ [field]: "Bw==" }), {
			ed25519: undefined,
		});
		assert.deepEqual(PublicKey.toJSON({ [field]: payload }), {});
		const amino = aminoConverters["/tendermint.crypto.PublicKey"];
		assert.deepEqual(amino.toAmino({ ed25519, [field]: payload }), {
			ed25519,
		});
		assert.deepEqual(amino.fromAmino({ ed25519, [field]: payload }), {
			ed25519,
		});
	}
	const json = { ed25519: Buffer.from(ed25519).toString("base64") };
	assert.deepEqual(PublicKey.fromJSON(json), { ed25519 });
	assert.deepEqual(PublicKey.toJSON({ ed25519 }), json);
}

test(block_params_encode_min_txs_and_max_gas_wanted);
test(public_key_decodes_only_ed25519);
