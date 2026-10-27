const assert = require("node:assert/strict");
const test = require("node:test");
const Encoder = require("@pax-js/cosmos/encoding").Encoder;

function encoding_namespace_omits_modules_absent_at_head() {
	for (const name of ["accesscontrol", "accesscontrol_x", "crisis", "mint"]) {
		assert.equal(name in Encoder.cosmos, false, `Encoder.cosmos.${name}`);
	}
}

function encoding_namespace_keeps_paxchain_mint() {
	const mint = Encoder.mint.v1beta1;
	const messages = {
		Minter: {
			start_date: "2026-01-01",
			end_date: "2026-12-31",
			denom: "upax",
			total_mint_amount: 1000,
			remaining_mint_amount: 900,
			last_mint_amount: 100,
			last_mint_date: "2026-01-02",
			last_mint_height: 42,
		},
		ScheduledTokenRelease: {
			start_date: "2026-01-01",
			end_date: "2026-12-31",
			token_release_amount: 100,
		},
		Params: { mint_denom: "upax", token_release_schedule: [] },
		Version2Minter: {
			last_mint_amount: "100",
			last_mint_date: "2026-01-02",
			last_mint_height: 42,
			denom: "upax",
		},
		Version2ScheduledTokenRelease: {
			date: "2026-01-02",
			token_release_amount: 100,
		},
		Version2Params: { mint_denom: "upax", token_release_schedule: [] },
	};

	for (const [name, message] of Object.entries(messages)) {
		const codec = mint[name];
		assert.equal(codec.$type, `paxprotocol.paxchain.mint.${name}`);
		assert.deepEqual(codec.decode(codec.encode(message).finish()), message);
	}
}

test(encoding_namespace_omits_modules_absent_at_head);
test(encoding_namespace_keeps_paxchain_mint);
