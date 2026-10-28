const assert = require("node:assert/strict");
const test = require("node:test");
const { Querier } = require("@pax-js/cosmos/rest");

async function denom_authority_metadata_route_escapes_factory_denom(t) {
	const requests = [];
	const body = { authority_metadata: { admin: "pax1abc" } };
	t.mock.method(globalThis, "fetch", async (url, init) => {
		requests.push({ url, init });
		return new Response(JSON.stringify(body));
	});

	const result = await Querier.tokenfactory.DenomAuthorityMetadata(
		{ denom: "factory/pax1abc/coin" },
		{ pathPrefix: "https://rest.example" },
	);

	assert.deepEqual(result, body);
	assert.equal(requests.length, 1);
	const url = new URL(requests[0].url);
	assert.equal(url.origin, "https://rest.example");
	assert.equal(
		url.pathname,
		"/pax-protocol/paxchain/tokenfactory/denoms/factory%2Fpax1abc%2Fcoin/authority_metadata",
	);
	assert.equal(url.search, "");
	assert.equal(url.hash, "");
	assert.equal(requests[0].init.method, "GET");
}

async function denoms_from_creator_route_carries_pagination(t) {
	const requests = [];
	const body = { denoms: ["factory/pax1abc/coin"] };
	t.mock.method(globalThis, "fetch", async (url, init) => {
		requests.push({ url, init });
		return new Response(JSON.stringify(body));
	});

	const creator = "pax1abc/part?query#fragment% space";
	for (const pagination of [{ limit: 10 }, { limit: 10, key: "next+/page==" }]) {
		const result = await Querier.tokenfactory.DenomsFromCreator(
			{ creator, pagination },
			{ pathPrefix: "https://rest.example" },
		);
		assert.deepEqual(result, body);
	}

	assert.equal(requests.length, 2);
	for (const [index, request] of requests.entries()) {
		const url = new URL(request.url);
		assert.equal(url.origin, "https://rest.example");
		assert.equal(
			url.pathname,
			"/pax-protocol/paxchain/tokenfactory/denoms_from_creator/pax1abc%2Fpart%3Fquery%23fragment%25%20space",
		);
		const expectedParams = [["pagination.limit", "10"]];
		if (index === 1) {
			expectedParams.push(["pagination.key", "next+/page=="]);
		}
		assert.deepEqual([...url.searchParams], expectedParams);
		assert.equal(url.hash, "");
		assert.equal(request.init.method, "GET");
	}
}

test(denom_authority_metadata_route_escapes_factory_denom);
test(denoms_from_creator_route_carries_pagination);
