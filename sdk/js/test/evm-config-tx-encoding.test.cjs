const assert = require("node:assert/strict");
const test = require("node:test");
const { BinaryWriter } = require("@bufbuild/protobuf/wire");
const ChainConfig = require("../dist/cjs/encoding/evm/config.js").ChainConfig;
const Asset = require("../dist/cjs/encoding/evm/tx.js").Asset;
const { MsgClaimSpecific } = require("../dist/cjs/encoding/evm/tx.js");

function evm_asset_round_trips_denom() {
	const defaults = { asset_type: 0, contract_address: "", denom: "" };
	assert.deepEqual(Asset.create(), defaults);
	assert.deepEqual(Asset.fromJSON({}), defaults);
	assert.deepEqual(Asset.decode(new Uint8Array()), defaults);
	assert.deepEqual(Asset.toJSON(defaults), {});
	assert.deepEqual(Asset.encode(defaults).finish(), new Uint8Array());
	const denomWire = Uint8Array.from([26, 4, 117, 115, 100, 108]);
	assert.deepEqual(
		Asset.encode(Asset.create({ denom: "usdl" })).finish(),
		denomWire,
	);
	assert.deepEqual(Asset.decode(denomWire), { ...defaults, denom: "usdl" });

	for (const denom of ["usdl", "", "factory/pax1issuer/資産", "u".repeat(160)]) {
		const message = { asset_type: 1, contract_address: "0x1234", denom };
		assert.deepEqual(Asset.create(message), message);
		assert.deepEqual(Asset.fromPartial(message), message);
		assert.deepEqual(Asset.decode(Asset.encode(message).finish()), message);
		assert.deepEqual(Asset.fromJSON(Asset.toJSON(message)), message);
		if (denom !== "") {
			assert.equal(Asset.toJSON(message).denom, denom);
		}
		const claim = MsgClaimSpecific.create({
			sender: "pax1sender",
			claimer: "pax1claimer",
			assets: [message],
		});
		assert.deepEqual(
			MsgClaimSpecific.decode(MsgClaimSpecific.encode(claim).finish()),
			claim,
		);
		assert.equal(claim.assets[0].denom, denom);
	}
}

function evm_chain_config_round_trips_sstore_gas() {
	const defaults = {
		cancun_time: 0,
		prague_time: 0,
		verkle_time: 0,
		pax_sstore_set_gas_eip2200: 0,
	};
	assert.deepEqual(ChainConfig.create(), defaults);
	assert.deepEqual(ChainConfig.fromJSON({}), defaults);
	assert.deepEqual(ChainConfig.decode(new Uint8Array()), defaults);
	assert.deepEqual(ChainConfig.toJSON(defaults), {});
	assert.deepEqual(ChainConfig.encode(defaults).finish(), new Uint8Array());
	const gasWire = Uint8Array.from([32, 160, 156, 1]);
	const gasOnly = { ...defaults, pax_sstore_set_gas_eip2200: 20000 };
	assert.deepEqual(ChainConfig.encode(gasOnly).finish(), gasWire);
	assert.deepEqual(ChainConfig.decode(gasWire), gasOnly);

	for (const gas of [0, 20000, 2 ** 32 + 7, Number.MAX_SAFE_INTEGER]) {
		const message = {
			cancun_time: -1,
			prague_time: 1700000000,
			verkle_time: 1800000000,
			pax_sstore_set_gas_eip2200: gas,
		};
		assert.deepEqual(ChainConfig.create(message), message);
		assert.deepEqual(ChainConfig.fromPartial(message), message);
		assert.deepEqual(
			ChainConfig.decode(ChainConfig.encode(message).finish()),
			message,
		);
		assert.deepEqual(ChainConfig.fromJSON(ChainConfig.toJSON(message)), message);
		assert.equal(
			ChainConfig.fromJSON({ pax_sstore_set_gas_eip2200: String(gas) })
				.pax_sstore_set_gas_eip2200,
			gas,
		);
		if (gas !== 0) {
			assert.equal(ChainConfig.toJSON(message).pax_sstore_set_gas_eip2200, gas);
		}
	}
	const unsafeGas = new BinaryWriter().uint32(32).uint64(2n ** 64n - 1n).finish();
	assert.throws(
		() => ChainConfig.decode(unsafeGas),
		/Value is larger than Number.MAX_SAFE_INTEGER/,
	);
}

test(evm_asset_round_trips_denom);
test(evm_chain_config_round_trips_sstore_gas);
