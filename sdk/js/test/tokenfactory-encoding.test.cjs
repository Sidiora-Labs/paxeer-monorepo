const assert = require("node:assert/strict");
const test = require("node:test");
const {
	DenomAuthorityMetadata,
} = require("../dist/cjs/encoding/tokenfactory/authorityMetadata.js");
const {
	QueryDenomsFromCreatorRequest,
	QueryDenomsFromCreatorResponse,
} = require("../dist/cjs/encoding/tokenfactory/query.js");

function stringField(value) {
	const bytes = Buffer.from(value, "utf8");
	assert.ok(bytes.length < 128);
	return [10, bytes.length, ...bytes];
}

function denom_authority_metadata_round_trips_issuer_did() {
	const message = {
		admin: "pax1creator",
		issuerDid: `did:layerx:${"ab".repeat(32)}`,
	};
	const issuerBytes = Buffer.from(message.issuerDid, "utf8");
	const wire = Uint8Array.from([
		...stringField(message.admin),
		18,
		issuerBytes.length,
		...issuerBytes,
	]);
	assert.deepEqual(DenomAuthorityMetadata.encode(message).finish(), wire);
	assert.deepEqual(DenomAuthorityMetadata.decode(wire), message);
	assert.deepEqual(
		DenomAuthorityMetadata.decode(DenomAuthorityMetadata.encode(message).finish()),
		message,
	);
	assert.deepEqual(DenomAuthorityMetadata.fromJSON(message), message);
	assert.deepEqual(DenomAuthorityMetadata.toJSON(message), message);
	assert.deepEqual(DenomAuthorityMetadata.fromPartial(message), message);
	assert.deepEqual(DenomAuthorityMetadata.create(message), message);

	const defaults = { admin: "", issuerDid: "" };
	assert.deepEqual(DenomAuthorityMetadata.decode(new Uint8Array()), defaults);
	assert.deepEqual(DenomAuthorityMetadata.fromJSON({}), defaults);
	assert.deepEqual(DenomAuthorityMetadata.create(), defaults);
	assert.deepEqual(DenomAuthorityMetadata.encode(defaults).finish(), new Uint8Array());
	assert.deepEqual(DenomAuthorityMetadata.toJSON(defaults), {});
	assert.deepEqual(
		DenomAuthorityMetadata.decode(Uint8Array.from(stringField(message.admin))),
		{ admin: message.admin, issuerDid: "" },
	);
	assert.deepEqual(
		DenomAuthorityMetadata.encode(DenomAuthorityMetadata.create({ issuerDid: message.issuerDid })).finish(),
		Uint8Array.from([18, issuerBytes.length, ...issuerBytes]),
	);
}

function denoms_from_creator_round_trips_pagination() {
	const request = {
		creator: "pax1creator",
		pagination: {
			key: Uint8Array.from([0, 128, 255]),
			offset: 7,
			limit: 25,
			count_total: true,
			reverse: true,
		},
	};
	const response = {
		denoms: ["factory/pax1creator/one", "factory/pax1creator/two"],
		pagination: { next_key: Uint8Array.from([255, 0, 128]), total: 300 },
	};
	const cases = [
		{
			codec: QueryDenomsFromCreatorRequest,
			message: request,
			legacy: stringField(request.creator),
			page: [10, 3, 0, 128, 255, 16, 7, 24, 25, 32, 1, 40, 1],
			json: {
				creator: request.creator,
				pagination: { key: "AID/", offset: 7, limit: 25, count_total: true, reverse: true },
			},
			defaults: { creator: "", pagination: undefined },
		},
		{
			codec: QueryDenomsFromCreatorResponse,
			message: response,
			legacy: response.denoms.flatMap(stringField),
			page: [10, 3, 255, 0, 128, 16, 172, 2],
			json: {
				denoms: response.denoms,
				pagination: { next_key: "/wCA", total: 300 },
			},
			defaults: { denoms: [], pagination: undefined },
		},
	];

	for (const { codec, message, legacy, page, json, defaults } of cases) {
		const wire = Uint8Array.from([...legacy, 18, page.length, ...page]);
		assert.deepEqual(codec.encode(message).finish(), wire);
		assert.deepEqual(codec.decode(wire), message);
		assert.deepEqual(codec.decode(codec.encode(message).finish()), message);
		assert.deepEqual(codec.fromJSON(json), message);
		assert.deepEqual(codec.toJSON(message), json);
		assert.deepEqual(codec.fromPartial(message), message);
		assert.deepEqual(codec.create(message), message);
		assert.deepEqual(codec.decode(new Uint8Array()), defaults);
		assert.deepEqual(codec.fromJSON({}), defaults);
		assert.deepEqual(codec.fromJSON({ pagination: null }), defaults);
		assert.deepEqual(codec.create(), defaults);
		assert.deepEqual(codec.encode(defaults).finish(), new Uint8Array());
		assert.deepEqual(codec.toJSON(defaults), {});
		assert.deepEqual(codec.decode(Uint8Array.from(legacy)), {
			...message,
			pagination: undefined,
		});
		const emptyPage = codec.create({ pagination: {} });
		assert.deepEqual(codec.encode(emptyPage).finish(), Uint8Array.from([18, 0]));
		assert.deepEqual(codec.decode(Uint8Array.from([18, 0])), emptyPage);
		assert.deepEqual(codec.toJSON(emptyPage), { pagination: {} });
	}

	assert.deepEqual(
		QueryDenomsFromCreatorRequest.create({ pagination: { limit: 10 } }),
		{
			creator: "",
			pagination: { key: new Uint8Array(), offset: 0, limit: 10, count_total: false, reverse: false },
		},
	);
}

test(denom_authority_metadata_round_trips_issuer_did);
test(denoms_from_creator_round_trips_pagination);
