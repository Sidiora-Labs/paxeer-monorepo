const assert = require("node:assert/strict");
const test = require("node:test");
const { BinaryReader, BinaryWriter } = require("@bufbuild/protobuf/wire");
const cjs = require("../dist/cjs/encoding/evm/params.js");
const Params = cjs.Params;

async function codecs() {
	const esm = await import("../dist/esm/encoding/evm/params.js");
	const wire = await import("@bufbuild/protobuf/wire");
	return [
		{ ...cjs, Params, Reader: BinaryReader },
		{ ...esm, Reader: wire.BinaryReader },
	];
}

function feeTokenFields() {
	return {
		register_pointer_disabled: true,
		pax_sstore_set_gas_eip2200: 2 ** 40 + 20000,
		allowed_fee_denoms: [
			{ denom: "upax", rate: "1.000000000000000000", rate_update_height: 7 },
			{ denom: "usdl", rate: "3.125000000000000001", rate_update_height: 2 ** 40 + 9 },
		],
		max_fee_token_spread: "0.025000000000000000",
		fee_token_enabled: true,
		fee_token_distribution: true,
		max_fee_token_rate_age: 2 ** 40 + 3600,
	};
}

function writeFeeTokenFields(writer) {
	writer.uint32(112).bool(true);
	writer.uint32(120).uint64(2 ** 40 + 20000);
	writer.uint32(130).bytes(new BinaryWriter()
		.uint32(10).string("upax")
		.uint32(26).string("1.000000000000000000")
		.uint32(32).int64(7).finish());
	writer.uint32(130).bytes(new BinaryWriter()
		.uint32(10).string("usdl")
		.uint32(26).string("3.125000000000000001")
		.uint32(32).int64(2 ** 40 + 9).finish());
	writer.uint32(138).string("0.025000000000000000");
	writer.uint32(144).bool(true);
	writer.uint32(152).bool(true);
	writer.uint32(160).int64(2 ** 40 + 3600);
	return writer;
}

async function evm_params_round_trip_fee_token_fields() {
	const expectedWire = writeFeeTokenFields(new BinaryWriter()).finish();
	for (const {
		Params,
		AllowedFeeDenom,
		registry,
		aminoConverters,
	} of await codecs()) {
		const fields = feeTokenFields();
		const message = Params.create(fields);
		for (const [name, value] of Object.entries(fields)) {
			assert.deepEqual(message[name], value, name);
		}
		assert.deepEqual(Params.encode(message).finish(), expectedWire);
		assert.deepEqual(Params.decode(expectedWire), message);
		assert.deepEqual(Params.decode(Params.encode(message).finish()), message);
		assert.deepEqual(Params.fromJSON(Params.toJSON(message)), message);
		assert.deepEqual(Params.fromPartial(message), message);
		const converter = aminoConverters["/paxprotocol.paxchain.evm.Params"];
		assert.deepEqual(converter.fromAmino(converter.toAmino(message)), message);

		for (const age of [
			3600,
			-3600,
			Number.MAX_SAFE_INTEGER,
			Number.MIN_SAFE_INTEGER,
		]) {
			const value = Params.create({ ...fields, max_fee_token_rate_age: age });
			assert.deepEqual(Params.decode(Params.encode(value).finish()), value);
		}
		assert.throws(
			() =>
				Params.decode(
					new BinaryWriter().uint32(120).uint64(9007199254740992n).finish(),
				),
			/Value is larger than Number.MAX_SAFE_INTEGER/,
		);
		assert.throws(
			() =>
				Params.decode(
					new BinaryWriter().uint32(160).int64(-9007199254740992n).finish(),
				),
			/Value is smaller than Number.MIN_SAFE_INTEGER/,
		);
		const empty = Params.create();
		assert.deepEqual(Params.decode(new Uint8Array()), empty);
		assert.deepEqual(Params.fromJSON({}), empty);
		assert.deepEqual(Params.encode(empty).finish(), new Uint8Array());
		assert.deepEqual(Params.toJSON(empty), {});
		for (const name of [
			"register_pointer_disabled",
			"fee_token_enabled",
			"fee_token_distribution",
		]) {
			assert.equal(empty[name], false);
		}
		assert.equal(empty.pax_sstore_set_gas_eip2200, 0);
		assert.deepEqual(empty.allowed_fee_denoms, []);
		assert.equal(empty.max_fee_token_spread, "");
		assert.equal(empty.max_fee_token_rate_age, 0);
		assert.deepEqual(AllowedFeeDenom.create(), { denom: "", rate: "", rate_update_height: 0 });
		assert.deepEqual(AllowedFeeDenom.fromJSON({}), { denom: "", rate: "", rate_update_height: 0 });
		assert.deepEqual(AllowedFeeDenom.toJSON({ denom: "" }), {});
		assert.deepEqual(AllowedFeeDenom.decode(new Uint8Array()), { denom: "", rate: "", rate_update_height: 0 });
		assert.deepEqual(
			AllowedFeeDenom.encode({ denom: "" }).finish(),
			new Uint8Array(),
		);
		for (const value of fields.allowed_fee_denoms) {
			assert.deepEqual(
				AllowedFeeDenom.decode(AllowedFeeDenom.encode(value).finish()),
				value,
			);
			assert.deepEqual(AllowedFeeDenom.fromJSON(AllowedFeeDenom.toJSON(value)), value);
		}
		assert.equal(
			registry.find(
				([name]) => name === "/paxprotocol.paxchain.evm.AllowedFeeDenom",
			)[1],
			AllowedFeeDenom,
		);
		const emptyDenom = Params.create({ allowed_fee_denoms: [{}] });
		assert.deepEqual(Params.decode(Params.encode(emptyDenom).finish()), emptyDenom);
	}
}

async function evm_params_decode_skips_nothing_from_head() {
	const expected = {
		priority_normalizer: "1.25",
		base_fee_per_gas: "1000000000",
		minimum_fee_per_gas: "500000000",
		whitelisted_cw_code_hashes_for_delegate_call: [
			Uint8Array.from([0, 1, 127, 255]),
			Uint8Array.from([42, 43]),
		],
		deliver_tx_hook_wasm_gas_limit: 2 ** 40 + 100,
		max_dynamic_base_fee_upward_adjustment: "0.125",
		max_dynamic_base_fee_downward_adjustment: "0.1",
		target_gas_used_per_block: 2 ** 40 + 200,
		maximum_fee_per_gas: "10000000000",
		...feeTokenFields(),
	};
	const writer = new BinaryWriter();
	writer.uint32(10).string("reserved-base-denom");
	writer.uint32(18).string("1.25");
	writer.uint32(26).string("1000000000");
	writer.uint32(34).string("500000000");
	writer.uint32(42).bytes(new Uint8Array([8, 1]));
	writer.uint32(50).string("reserved-chain-id");
	writer.uint32(58).string("reserved-codehash");
	writer.uint32(66).bytes(Uint8Array.from([0, 1, 127, 255]));
	writer.uint32(66).bytes(Uint8Array.from([42, 43]));
	writer.uint32(72).uint64(2 ** 40 + 100);
	writer.uint32(82).string("0.125");
	writer.uint32(90).string("0.1");
	writer.uint32(96).uint64(2 ** 40 + 200);
	writer.uint32(106).string("10000000000");
	const payload = writeFeeTokenFields(writer).finish();
	for (const { Params, Reader } of await codecs()) {
		const decoded = Params.decode(payload);
		assert.deepEqual(decoded, expected);
		const defaults = Params.create();
		assert.equal(Object.keys(decoded).length, 16);
		for (const [name, value] of Object.entries(expected)) {
			assert.notDeepEqual(decoded[name], defaults[name], name);
			assert.deepEqual(decoded[name], value, name);
		}
		assert.deepEqual(Params.decode(Params.encode(decoded).finish()), expected);
		const reader = new Reader(Uint8Array.from([...payload, 8, 1]));
		assert.deepEqual(Params.decode(reader, payload.length), expected);
		assert.equal(reader.pos, payload.length);
		assert.equal(reader.uint32(), 8);
		assert.equal(reader.bool(), true);
	}
}

test(evm_params_round_trip_fee_token_fields);
test(evm_params_decode_skips_nothing_from_head);

async function evm_allowed_fee_denom_preserves_current_proto() {
	const canonical = Uint8Array.from([10, 4, 117, 115, 105, 100, 26, 1, 51, 32, 7]);
	const expected = { denom: "usid", rate: "3", rate_update_height: 7 };
	for (const { AllowedFeeDenom, Params, Reader, aminoConverters } of await codecs()) {
		assert.deepEqual(AllowedFeeDenom.decode(canonical), expected);
		assert.deepEqual(AllowedFeeDenom.encode(expected).finish(), canonical);
		assert.deepEqual(AllowedFeeDenom.create(expected), expected);
		assert.deepEqual(AllowedFeeDenom.fromPartial(expected), expected);
		assert.deepEqual(AllowedFeeDenom.toJSON(expected), expected);
		assert.deepEqual(AllowedFeeDenom.fromJSON({
			denom: "usid", rate: 3, rate_update_height: "7",
		}), expected);
		const converter = aminoConverters["/paxprotocol.paxchain.evm.AllowedFeeDenom"];
		assert.deepEqual(converter.fromAmino(converter.toAmino(expected)), expected);

		const nested = Uint8Array.from([130, 1, canonical.length, ...canonical]);
		const params = Params.create({ allowed_fee_denoms: [expected] });
		assert.deepEqual(Params.decode(nested), params);
		assert.deepEqual(Params.encode(params).finish(), nested);
		assert.deepEqual(Params.fromJSON(Params.toJSON(params)), params);

		const defaults = { denom: "", rate: "", rate_update_height: 0 };
		assert.deepEqual(AllowedFeeDenom.toJSON(defaults), {});
		assert.deepEqual(AllowedFeeDenom.encode(defaults).finish(), new Uint8Array());
		assert.deepEqual(AllowedFeeDenom.fromPartial({}), defaults);
		const legacy = { denom: "usid" };
		const legacyWire = canonical.slice(0, 6);
		assert.deepEqual(AllowedFeeDenom.encode(legacy).finish(), legacyWire);
		assert.deepEqual(AllowedFeeDenom.toJSON(legacy), legacy);
		assert.deepEqual(AllowedFeeDenom.decode(legacyWire), { ...defaults, ...legacy });

		for (const height of [0, -7, 2 ** 40 + 7, Number.MAX_SAFE_INTEGER, Number.MIN_SAFE_INTEGER]) {
			const message = AllowedFeeDenom.create({ ...expected, rate_update_height: height });
			assert.deepEqual(AllowedFeeDenom.decode(AllowedFeeDenom.encode(message).finish()), message);
			assert.deepEqual(AllowedFeeDenom.fromJSON(AllowedFeeDenom.toJSON(message)), message);
		}
		for (const [height, error] of [
			[9007199254740992n, /Value is larger than Number.MAX_SAFE_INTEGER/],
			[-9007199254740992n, /Value is smaller than Number.MIN_SAFE_INTEGER/],
		]) {
			const wire = new BinaryWriter().uint32(32).int64(height).finish();
			assert.throws(() => AllowedFeeDenom.decode(wire), error);
		}
		const reserved = new BinaryWriter().uint32(18).string("retired-pair").finish();
		assert.deepEqual(AllowedFeeDenom.decode(Uint8Array.from([...reserved, ...canonical])), expected);
		const reader = new Reader(Uint8Array.from([...canonical, 8, 1]));
		assert.deepEqual(AllowedFeeDenom.decode(reader, canonical.length), expected);
		assert.equal(reader.pos, canonical.length);
		assert.equal(reader.uint32(), 8);
		assert.equal(reader.bool(), true);
	}
}

test(evm_allowed_fee_denom_preserves_current_proto);
