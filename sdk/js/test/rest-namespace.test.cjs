const assert = require("node:assert/strict");
const { once } = require("node:events");
const { createServer } = require("node:http");
const test = require("node:test");

const Querier = require("@pax-js/cosmos/rest").Querier;

function rest_namespace_omits_routes_absent_at_head() {
	assert.equal(typeof Querier.cosmos, "object");
	assert.notEqual(Querier.cosmos, null);
	assert.equal("accesscontrol_x" in Querier.cosmos, false);
	assert.equal("mint" in Querier.cosmos, false);
	assert.strictEqual(
		Querier.cosmos.bank.v1beta1,
		require("../dist/cjs/rest/cosmos/bank/v1beta1/query.js").Query,
	);
}

async function rest_namespace_keeps_paxchain_mint_routes() {
	assert.strictEqual(
		Querier.mint.v1beta1,
		require("../dist/cjs/rest/mint/v1beta1/query.js").Query,
	);
	const server = createServer((request, response) => {
		response.writeHead(200, { "Content-Type": "application/json" });
		response.end(JSON.stringify({ method: request.method, url: request.url }));
	});
	server.listen(0, "127.0.0.1");
	await once(server, "listening");
	try {
		const pathPrefix = `http://127.0.0.1:${server.address().port}`;
		const response = await Querier.mint.v1beta1.Params(
			{},
			{
				pathPrefix,
				signal: AbortSignal.timeout(5000),
			},
		);
		const url = new URL(response.url, pathPrefix);
		assert.equal(response.method, "GET");
		assert.equal(url.pathname, "/paxchain/mint/v1beta1/params");
		assert.equal(url.search, "");
	} finally {
		await new Promise((resolve, reject) => {
			server.close((error) => (error ? reject(error) : resolve()));
		});
	}
}

test(rest_namespace_omits_routes_absent_at_head);
test(rest_namespace_keeps_paxchain_mint_routes);
