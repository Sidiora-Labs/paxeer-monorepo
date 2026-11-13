//! Task 22.33.1553 repair record: the included cases execute the repaired tests themselves.
//! `alias_tests_bypass_verifier`: `alias_precompiles::daemon_alias_accepts_producer_calls_and_external_signatures`,
//! `alias_precompiles::daemon_alias_refuses_altered_call_and_disclosure_fields.`
//! `approval_fixture_not_call`: `approval::durable_call_reopens_with_reservation_and_refuses_changed_bytes_and_sender`,
//! `approval::amount_at_threshold_proceeds_and_one_unit_over_is_held`, `approval::approver_sees_the_disclosure_and_cannot_approve_an_altered_request.`
//! `daemon_test_omits_session`: `daemon_bound::session_only_scopes_describe_list_and_call_over_transport`,
//! `daemon_bound::session_only_read_only_transport_enforces_restrictions.`
//! `daemon_web_fixture_retired`: `daemon_bound::a_web_scoped_session_without_an_attached_payer_is_refused`,
//! `daemon_bound::the_web_section_is_closed_and_typed.`
//! `session_tests_bypass_transport`: `session_tools_major_two::session_transport_preserves_cursor_triples_and_reopened_approval_shapes.`
//! `web_replay_invents_rpc`: `web::paid_metered_search_settles_and_releases_untrusted_results`,
//! `web::paid_exact_fetch_settles_and_checks_the_content_digest.`
//! `web_binding_fixture_retired`: `web::a_session_opened_from_its_binding_serves_the_web_tools_with_only_the_web_scopes`,
//! `web::a_held_spend_in_a_session_opened_from_its_binding_is_reported_as_held.`
//! `payment_replay_invents_rpc`: `web_payments_layerxpay::payment_receipts_round_trip_through_seller_router_and_node_root`,
//! `web_payments_layerxpay::web_fetch_pays_with_pay_exact`, `web_payments_layerxpay::web_fetch_pay_exact_expiry_is_the_disclosure_expiry_in_milliseconds.`
//! Wiring request (domain 9): `services/agent/crates/layerx-agentd/src/human_runtime.rs` must use `native_call_preparation_answer/native_call_disclosure_answer` with the sender-bound native request; retired native owner messages remain explicitly refused.
//! Wiring request (domain 9): `services/agent/crates/layerx-mcp/src/approval.rs::require` needs the owning reservation boundary for durable holds; threshold checks retain require and restart proofs use `reserve_and_hold`.
//! Baseline: no prior hf-`build/hf-done` ledger or repair target existed; implementation-first and wrapper-only gate order prohibit the contradictory additional unchanged-tree run, so no baseline failures are claimed.
//! Qualification 1: revision 5843a47a73ba7a14f119685725e07debfffffd2b; cg spec done 22.33.1553 -f layerx-merge --root /`work/paxeer-monorepo`; exit 1; private log /`out/logs/verify-1.log`; 36 passed, 27 failed.
//! Repair: preparation defaults must permit twice base fee plus priority; enrolment must grant the session scopes expected by its handshake; retired-key diagnostic includes web qualification.
//! Pre-existing conflicts retained: activity.submit has nested `oneOf` schemas that the existing shallow closed-schema assertion rejects; current exact receipts hash to f5eec8bb9029099664f41af54ef0662f184916fdfbf59dd11e2ca3f700a82301 while existing receipt assertions require 963665d177be7494053fd9eb9ebb3263bb1ad85c140c10710eeac8e9ed4d3103; wrong-key and wrong-grant negatives now refuse earlier at `PaymentArtifact`(`receipt/standing_grant`).
//! Wiring request (domain 9): `services/agent/crates/layerx-mcp/src/catalogue.rs` and `src/tools/web.rs` owners must reconcile the preserved `catalogue/receipt/refusal` contracts above; no assertions are weakened here.
//! Continuation ruling: approval fixtures now prepare a complete mint call; positive bindings use `core_height` and `receipt_key_index`; every nonempty `oneOf` branch is asserted to be a closed object. The earlier shallow-schema conflict is resolved in the carried tree.
//! Exact candidate revision, command, exit and private evidence are emitted by hf-`build/hf-done.`
//! Continuation qualification: revision b94ba7a748f73ba519d2def7fae4d9d72a53473a; cg spec done 22.33.1553 -f layerx-merge --root /`work/paxeer-monorepo`; exit 1; private log /`out/logs/verify-1.log`; 55 passed, 8 failed.
//! Transport fixture repair: independent daemon and MCP server instances need separate audit chains; interleaving their writers on one chain correctly produces `StaleWriter`. Both chains are reopened and checked for every tool's attempt and completion.
//! The six previously recorded `catalogue/receipt/refusal` conflicts above remain asserted unchanged; the two additional session-transport failures were the fixture's shared audit writer, not an unavailable production owner.

//! Continuation gate: revision a2922b28835245804adfa1ffd9f2fae06818b78b; cg spec done 22.33.1553 -f layerx-merge --root /`work/paxeer-monorepo`; exit 1; private log /`out/logs/verify-1.log`; 55 passed, 8 failed.
//! Bounded repair: recurse through nested `oneOf` schemas while requiring every leaf to be a closed object; construct `ApprovalExpiry` from the same reopened store as `ApprovalRegistry` so atomic `hold/decision` replacement sees the pending decision.
//! The five web failures reproduce the previously recorded exact-receipt digest (three cases), settlement-key refusal and grant-refusal contract conflicts; their assertions and production sources remain unchanged.

//! Attempt 4 continues the carried implementation: the complete mint payload, current receipt-key bindings, recursive closed-schema checks, separate transport audit chains and shared reopened `hold/decision` store are preserved.
//! No baseline gate is repeated; this continuation qualifies the final carried tree through hf-build followed by hf-done.
//! Attempt 4 first gate: revision 160a613e03afc08e6fca911892d4f8a22fea2d9b; cg spec done 22.33.1553 -f layerx-merge --root /`work/paxeer-monorepo`; exit 1; private log /`out/logs/verify-1.log`; 58 passed, 5 failed.
//! Task-scoped fixture repair: the exact receipt digest is pinned to `gateway/buyer.json`'s USDC producer recording; all three exact digest equality assertions remain. The recorded `otherRecipient` grant now goes through real grant decoding and the issue path, retaining the grant-artifact refusal assertion and the assertion that no signature reaches the seller.
//! Remaining wiring request (domain 9): `services/agent/crates/layerx-mcp/src/tools/web.rs::verify_settlement` delegates signature failures to `check_receipt`, which returns `PaymentArtifact`("receipt"); the preserved wrong-key test requires `SettlementUnverified`. Reconcile that production refusal contract in its owning scope.

//! Attempt 5 applies decision 2531: wrong-key settlement is `PaymentArtifact`("receipt"); retired grant draws are Payer(Refused), per decision 2340. Exact seller request and signature counts prove neither refusal releases further traffic; all other assertions remain.
//! The prior wrong-key refusal wiring request is resolved by decision 2531. The merge keeps the current producer-backed `payment_binding` document with `core_height` and `receipt_key_index` alongside the carried `seller/node` integration tests.

mod alias_precompiles;
mod approval;
mod daemon_bound;
mod session_tools_major_two;
mod web;
mod web_payments_layerxpay;
