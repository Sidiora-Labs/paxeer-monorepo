/* Proves: lx_service_offer_publish_execute/lx_escrow_open_execute -> include/layerx/lx_service.h and lxp_kernel_dispatch -> lx_service_deliverable_check/lx_service_dispute_open_execute/lx_service_dispute_resolve_execute.
 * State crosses a destroyed/reopened store through lxp_snapshot_write/load.
 *
 * Repairs: lx_service_deliver.c accepted deliverables bound to a different
 * agreement/provider; lx_service_module.c lacked a reusable kernel-backed
 * escrow link reader; test_service_deliver.c used the retired availability
 * error contract and omitted the now-required escrow link/arbiter authority.
 * The host uses the production reader, not a replacement escrow transport.
 */
#include "../codec/lxp_kernel_case.h"
#include "../modules/test_service_helpers.h"

#include "layerx/lx_escrow.h"
#include "layerx/lxp_snapshot.h"

static lx_account_registry accounts;
static lx_asset_registry assets;
static lx_service_runtime runtime = {
    lx_service_kernel_escrow_link_read, &service_kernel
};
static lxp_authority_resolved provider;
static lxp_authority_resolved buyer;
static lxp_authority_resolved arbiter;
static lx_service_offer_request published;
static lx_service_commit_request committed;
static lx_service_agreement agreement;
static lx_escrow_record escrow;

static int create_kernel(void)
{
    LXP_CASE_CHECK(lxp_state_store_init(&store_state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&store_state, &accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_require_account_root(&store_state) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&service_kernel, &store_state,
        &state_journal, &parameter_set, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
        lx_service_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
        lx_escrow_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&service_kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&service_kernel,
        LXP_MODULE_SERVICE, &runtime) == LXP_OK);
    return 0;
}

/* The module harness commits KV; the host journal seals the global sequence. */
static lxp_result seal_sequence(uint64_t sequence)
{
    lxp_result status = lxp_state_journal_open(&store_state, sequence,
                                               &state_journal);
    if (status == LXP_OK) status = lxp_state_journal_commit(&state_journal);
    return status;
}

static int commit_producer(lxp_module_ctx *ctx)
{
    LXP_CASE_CHECK(lxp_module_ctx_commit(ctx) == LXP_OK);
    LXP_CASE_CHECK(seal_sequence(lxp_ctx_global_sequence(ctx)) == LXP_OK);
    return 0;
}

static lxp_result consume(uint32_t activity_type, const uint8_t *payload,
                          size_t length,
                          const lxp_authority_resolved *authority,
                          uint64_t timestamp, uint64_t sequence,
                          uint8_t marker, lxp_result *outcome)
{
    lxp_result status = dispatch(activity_type, payload, length, authority,
                                  timestamp, sequence, marker, outcome);
    if (status == LXP_OK && *outcome == LXP_OK)
        status = seal_sequence(sequence);
    return status;
}

static int produce_escrow(void)
{
    static lxp_module_ctx ctx;
    lx_asset_record asset = {0};
    lx_account *owner;
    lx_account *hold;
    lx_escrow_open_request request = {0};
    lxp_receipt receipt;
    const char *owner_name = "agent:did:key:buyer:main";
    const char *hold_name = "agent:did:key:buyer:escrow:service-proof";

    id32(asset.asset_id, 11U);
    (void)memcpy(asset.symbol, "USD", 4U);
    asset.symbol_length = 3U;
    asset.name[0] = (uint8_t)'A';
    asset.name_length = 1U;
    asset.issuer_kind = 2U;
    id32(asset.issuer_did32, 1U);
    asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    asset.custody_reference[0] = 1U;
    asset.custody_reference_length = 1U;
    LXP_CASE_CHECK(lx_asset_registry_init(&assets, 0U) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_register(&assets, &asset, 0U,
        (lxp_u128){0U, 0U}) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_account_open(&assets, &accounts, asset.asset_id,
        (const uint8_t *)owner_name, strlen(owner_name), 1U,
        LX_ACCOUNT_OPEN_CREDIT, NULL, &owner) == LXP_OK);
    LXP_CASE_CHECK(lx_asset_account_open(&assets, &accounts, asset.asset_id,
        (const uint8_t *)hold_name, strlen(hold_name), 2U,
        LX_ACCOUNT_OPEN_CREDIT, NULL, &hold) == LXP_OK);
    LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(owner, asset.asset_id,
        (lxp_u128){0U, 100U}, 0U) == LXP_OK);
    (void)memset(&buyer, 0, sizeof(buyer));
    buyer.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(buyer.principal, owner->id, 32U);
    (void)memcpy(buyer.actor, owner->id, 32U);
    (void)memset(&escrow, 0, sizeof(escrow));
    id32(escrow.escrow_id, 41U);
    (void)memcpy(escrow.owner, owner->id, 32U);
    (void)memcpy(escrow.escrow_account, hold->id, 32U);
    (void)memcpy(escrow.beneficiary, provider.principal, 32U);
    (void)memcpy(escrow.arbiter, arbiter.principal, 32U);
    (void)memcpy(escrow.asset_id, asset.asset_id, 32U);
    escrow.locked_amount.lo = 25U;
    escrow.state = LX_ESCROW_STATE_OPEN;
    escrow.expiry = 2000U;
    escrow.dispute_window = 1000U;
    id32(escrow.terms_hash, 12U);
    id32(escrow.agreement_reference, 40U);
    request.owner = owner;
    request.escrow_account = hold;
    request.asset = &asset;
    request.amount = escrow.locked_amount;
    request.record = escrow;
    LXP_CASE_CHECK(lxp_arena_init(&work_arena, work_bytes,
        sizeof(work_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_effect_buffer_init(&event_buffer) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &service_kernel, LXP_MODULE_ESCROW,
        100U, 0U, 1U, GAS_LIMIT, &work_arena, true) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_bind_effects(&ctx, &event_buffer) == LXP_OK);
    LXP_CASE_CHECK(lx_escrow_open_execute(&ctx, &request, &receipt) == LXP_OK);
    LXP_CASE_CHECK(commit_producer(&ctx) == 0);
    LXP_CASE_CHECK(owner->balance.lo == 75U && hold->balance.lo == 25U);
    LXP_CASE_CHECK(lx_escrow_open_execute(&ctx, &request, &receipt) ==
        LXP_ERR_ESCROW_STATE);
    lxp_module_ctx_rollback(&ctx);
    return 0;
}

static int produce_service(void)
{
    static lxp_module_ctx ctx;
    lx_service_offer offer;
    lx_service_agreement_request propose = {0};
    lx_service_commitment commitment;

    (void)memset(&published, 0, sizeof(published));
    published.authority = &provider;
    id32(published.offer.offer_id, 10U);
    id32(published.offer.activity_id, 20U);
    (void)memcpy(published.offer.offering_agent, provider.principal, 32U);
    (void)memcpy(published.offer.asset_id, escrow.asset_id, 32U);
    published.offer.price.lo = 25U;
    (void)memcpy(published.offer.terms_hash, escrow.terms_hash, 32U);
    id32(published.offer.deliverable_specification_hash, 13U);
    published.offer.delivery_deadline = 1000U;
    published.offer.acceptance_window = 200U;
    published.offer.dispute_window = 300U;
    published.offer.default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    published.offer.offer_expiry = 900U;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 2U) == 0);
    LXP_CASE_CHECK(lx_service_offer_publish_execute(&ctx, &published,
        &offer) == LXP_OK);
    LXP_CASE_CHECK(commit_producer(&ctx) == 0);
    id32(propose.agreement_id, 40U);
    (void)memcpy(propose.offer_id, offer.offer_id, 32U);
    (void)memcpy(propose.buyer, buyer.principal, 32U);
    (void)memcpy(propose.terms_hash, offer.terms_hash, 32U);
    (void)memcpy(propose.escrow_id, escrow.escrow_id, 32U);
    propose.authority = &buyer;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 3U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_propose_execute(&ctx, &propose,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(commit_producer(&ctx) == 0);
    propose.authority = &provider;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 4U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_accept_execute(&ctx, &propose,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(commit_producer(&ctx) == 0);
    (void)memset(&committed, 0, sizeof(committed));
    committed.authority = &provider;
    id32(committed.commitment.commitment_id, 50U);
    id32(committed.commitment.activity_id, 21U);
    (void)memcpy(committed.commitment.provider, provider.principal, 32U);
    (void)memcpy(committed.commitment.agreement_id, agreement.agreement_id, 32U);
    id32(committed.commitment.task_hash, 51U);
    (void)memcpy(committed.commitment.escrow_id, escrow.escrow_id, 32U);
    committed.commitment.deadline = 900U;
    committed.commitment.resource_bound = 100U;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 5U) == 0);
    LXP_CASE_CHECK(lx_service_commit_task_execute(&ctx, &committed,
        &commitment) == LXP_OK);
    LXP_CASE_CHECK(commit_producer(&ctx) == 0);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx, agreement.agreement_id,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(agreement.state == LX_SERVICE_AGREEMENT_COMMITTED);
    return 0;
}

static int restart(uint64_t sequence)
{
    static uint8_t snapshot_bytes[131072];
    lxp_arena arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest;
    uint8_t before[32];
    uint8_t after[32];
    uint8_t receipt_root[32] = {0};

    LXP_CASE_CHECK(lxp_state_root(&service_kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, snapshot_bytes,
        sizeof(snapshot_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&service_kernel, sequence, &arena,
        &snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_manifest(snapshot.bytes, snapshot.length,
        sequence, before, receipt_root, &manifest) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    lx_account_registry_release(&accounts);
    (void)memset(&service_kernel, 0, sizeof(service_kernel));
    (void)memset(&state_journal, 0, sizeof(state_journal));
    LXP_CASE_CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    LXP_CASE_CHECK(create_kernel() == 0);
    LXP_CASE_CHECK(service_kernel.module_kv_count == 0U);
    LXP_CASE_CHECK(accounts.count == 0U);
    LXP_CASE_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
        &manifest, &service_kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_verify_root(&service_kernel, &manifest) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(accounts.count == 2U);
    return 0;
}

static int setup(void)
{
    (void)memset(&provider, 0, sizeof(provider));
    (void)memset(&arbiter, 0, sizeof(arbiter));
    id32(provider.principal, 1U);
    id32(arbiter.principal, 3U);
    LXP_CASE_CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    LXP_CASE_CHECK(create_kernel() == 0);
    LXP_CASE_CHECK(produce_escrow() == 0);
    LXP_CASE_CHECK(produce_service() == 0);
    LXP_CASE_CHECK(restart(5U) == 0);
    return 0;
}

static size_t delivery_payload(uint8_t *out)
{
    out[0] = 0U;
    out[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(out + 2U, 90U);
    (void)memcpy(out + 34U, agreement.agreement_id, 32U);
    out[66] = 2U;
    id32(out + 67U, 13U);
    be64(out + 99U, 3000U);
    id32(out + 107U, 33U);
    id32(out + 139U, 13U);
    be64(out + 171U, 1000U);
    id32(out + 179U, 11U);
    return 211U;
}

static size_t dispute_payload(uint8_t *out)
{
    out[0] = 0U;
    out[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(out + 2U, 92U);
    (void)memcpy(out + 34U, agreement.agreement_id, 32U);
    out[66] = 1U;
    id32(out + 67U, 13U);
    return 99U;
}

static size_t resolution_payload(uint8_t *out)
{
    out[0] = 0U;
    out[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(out + 2U, 92U);
    be16(out + 34U, 1U);
    be32(out + 36U, 5000U);
    id32(out + 40U, 94U);
    return LX_SERVICE_DISPUTE_RESOLVE_PAYLOAD_BYTES;
}

static int deliver_and_reject(void)
{
    static lxp_module_ctx ctx;
    lx_service_delivery delivery;
    uint8_t payload[LX_SERVICE_DELIVER_PAYLOAD_MAX_BYTES];
    uint8_t delivery_id[32];
    uint8_t activity_id[32];
    size_t length = delivery_payload(payload);
    lxp_result outcome;

    LXP_CASE_CHECK(consume(LX_SERVICE_DELIVER, payload, length, &provider,
        100U, 6U, 60U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    id32(delivery_id, 90U);
    LXP_CASE_CHECK(event_is(LX_SERVICE_EVENT_DELIVERED, delivery_id,
        agreement.agreement_id, 2U, 6U) == 0);
    LXP_CASE_CHECK(expected_activity_id(LX_SERVICE_DELIVER, payload, length,
        60U, activity_id) == LXP_OK);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 7U) == 0);
    LXP_CASE_CHECK(lx_service_delivery_latest(&ctx, agreement.agreement_id,
        &delivery) == LXP_OK);
    LXP_CASE_CHECK(delivery.deliverable_count == 2U);
    LXP_CASE_CHECK(delivery.global_sequence == 6U);
    LXP_CASE_CHECK(memcmp(delivery.activity_id, activity_id, 32U) == 0);
    LXP_CASE_CHECK(delivery.deliverables[0].availability_reference[0] == 11U);
    LXP_CASE_CHECK(delivery.deliverables[0].artifact_size == 1000U);
    LXP_CASE_CHECK(delivery.deliverables[1].availability_reference[0] == 33U);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx, agreement.agreement_id,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(agreement.state == LX_SERVICE_AGREEMENT_DELIVERED);
    LXP_CASE_CHECK(lx_service_deliverable_check(&ctx, &agreement,
        &delivery) == LXP_OK);
    payload[0] = 0U;
    payload[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    (void)memcpy(payload + 2U, agreement.agreement_id, 32U);
    be16(payload + 34U, 7U);
    payload[36] = 1U;
    (void)memcpy(payload + 37U, delivery.deliverables[0].hash, 32U);
    LXP_CASE_CHECK(consume(LX_SERVICE_REJECT, payload, 69U, &buyer,
        100U, 7U, 61U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    LXP_CASE_CHECK(event_is(LX_SERVICE_EVENT_OUTCOME_REJECTED,
        agreement.agreement_id, provider.principal, 1U, 7U) == 0);
    return 0;
}

static int finish(void)
{
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    lx_account_registry_release(&accounts);
    return 0;
}

static int proof_lx_service_h_happy_path(void)
{
    static lxp_module_ctx ctx;
    lx_service_offer offer;
    lx_service_commitment commitment;
    lx_service_dispute_request request = {0};
    lx_service_dispute dispute;
    lx_service_escrow_link link;
    uint8_t payload[128];
    uint8_t before[32];
    uint8_t after[32];
    uint8_t escrow_before[32];
    uint8_t escrow_after[32];
    size_t length;
    lxp_result outcome;

    LXP_CASE_CHECK(setup() == 0);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, before) == LXP_OK);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 6U) == 0);
    LXP_CASE_CHECK(lx_service_offer_publish_execute(&ctx, &published,
        &offer) == LXP_ERR_SEQUENCE_REUSED);
    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 6U) == 0);
    LXP_CASE_CHECK(lx_service_commit_task_execute(&ctx, &committed,
        &commitment) == LXP_OK);
    LXP_CASE_CHECK(commitment.global_sequence == 5U);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(runtime.read_escrow_link(runtime.user, escrow.escrow_id,
        &link) == LXP_OK);
    LXP_CASE_CHECK(memcmp(link.holder, buyer.principal, 32U) == 0);
    LXP_CASE_CHECK(memcmp(link.beneficiary, provider.principal, 32U) == 0);
    LXP_CASE_CHECK(memcmp(link.asset_id, escrow.asset_id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(link.arbiter, arbiter.principal, 32U) == 0);
    LXP_CASE_CHECK(link.state == LX_ESCROW_STATE_OPEN);
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_ESCROW,
        escrow_before) == LXP_OK);
    LXP_CASE_CHECK(deliver_and_reject() == 0);
    LXP_CASE_CHECK(restart(7U) == 0);
    request.authority = &buyer;
    request.escrow_link = link;
    id32(request.dispute.dispute_id, 92U);
    id32(request.dispute.activity_id, 62U);
    (void)memcpy(request.dispute.agreement_id, agreement.agreement_id, 32U);
    (void)memcpy(request.dispute.raiser, buyer.principal, 32U);
    id32(request.dispute.evidence_hashes[0], 13U);
    request.dispute.evidence_hash_count = 1U;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 8U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_open_execute(&ctx, &request,
        &dispute) == LXP_OK);
    LXP_CASE_CHECK(!dispute.resolved && dispute.global_sequence == 8U);
    lxp_module_ctx_rollback(&ctx);
    length = dispute_payload(payload);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_OPEN, payload, length,
        &buyer, 100U, 8U, 62U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    LXP_CASE_CHECK(event_is(LX_SERVICE_EVENT_DISPUTE_OPENED,
        request.dispute.dispute_id, agreement.agreement_id, 1U, 8U) == 0);
    LXP_CASE_CHECK(restart(8U) == 0);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 9U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, request.dispute.dispute_id,
        &dispute) == LXP_OK);
    LXP_CASE_CHECK(!dispute.resolved && dispute.evidence_hash_count == 1U);
    LXP_CASE_CHECK(memcmp(dispute.raiser, buyer.principal, 32U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx, agreement.agreement_id,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(agreement.state == LX_SERVICE_AGREEMENT_DISPUTED);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_OPEN, payload, length,
        &buyer, 100U, 9U, 63U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_ERR_AGREEMENT_STATE);
    LXP_CASE_CHECK(event_buffer.count == 0U);
    request.authority = &arbiter;
    request.dispute.ruling = 1U;
    request.dispute.provider_basis_points = 5000U;
    id32(request.dispute.escrow_resolution_id, 94U);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 9U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_resolve_execute(&ctx, &request,
        &dispute) == LXP_OK);
    LXP_CASE_CHECK(dispute.resolved && dispute.provider_basis_points == 5000U);
    lxp_module_ctx_rollback(&ctx);
    length = resolution_payload(payload);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_RESOLVE, payload, length,
        &arbiter, 100U, 9U, 64U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    LXP_CASE_CHECK(event_is(LX_SERVICE_EVENT_DISPUTE_RESOLVED,
        request.dispute.dispute_id, agreement.agreement_id, 1U, 9U) == 0);
    LXP_CASE_CHECK(restart(9U) == 0);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 10U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, request.dispute.dispute_id,
        &dispute) == LXP_OK);
    LXP_CASE_CHECK(dispute.resolved && dispute.ruling == 1U);
    LXP_CASE_CHECK(dispute.provider_basis_points == 5000U);
    LXP_CASE_CHECK(dispute.resolution_sequence == 9U);
    LXP_CASE_CHECK(memcmp(dispute.escrow_resolution_id,
        request.dispute.escrow_resolution_id, 32U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx, agreement.agreement_id,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(agreement.state == LX_SERVICE_AGREEMENT_RESOLVED);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_RESOLVE, payload, length,
        &arbiter, 100U, 10U, 65U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_ERR_AGREEMENT_STATE);
    LXP_CASE_CHECK(event_buffer.count == 0U);
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_ESCROW,
        escrow_after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(escrow_before, escrow_after, 32U) == 0);
    return finish();
}

static int proof_lx_service_h_refusal(void)
{
    static lxp_module_ctx ctx;
    lxp_authority_resolved outsider = {0};
    lx_service_delivery delivery;
    lx_service_dispute dispute;
    lx_service_dispute_request request = {0};
    uint8_t payload[LX_SERVICE_DELIVER_PAYLOAD_MAX_BYTES];
    uint8_t before[32];
    uint8_t after[32];
    size_t length;
    lxp_result outcome;

    LXP_CASE_CHECK(setup() == 0);
    id32(outsider.principal, 99U);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, before) == LXP_OK);
    length = delivery_payload(payload);
    LXP_CASE_CHECK(consume(LX_SERVICE_DELIVER, payload, length, &outsider,
        100U, 6U, 70U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_ERR_AGREEMENT_STATE);
    LXP_CASE_CHECK(event_buffer.count == 0U);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(deliver_and_reject() == 0);
    LXP_CASE_CHECK(restart(7U) == 0);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 8U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx, agreement.agreement_id,
        &agreement) == LXP_OK);
    LXP_CASE_CHECK(lx_service_delivery_latest(&ctx, agreement.agreement_id,
        &delivery) == LXP_OK);
    delivery.agreement_id[0] ^= 1U;
    LXP_CASE_CHECK(lx_service_deliverable_check(&ctx, &agreement,
        &delivery) == LXP_ERR_AGREEMENT_STATE);
    delivery.agreement_id[0] ^= 1U;
    delivery.provider[0] ^= 1U;
    LXP_CASE_CHECK(lx_service_deliverable_check(&ctx, &agreement,
        &delivery) == LXP_ERR_AGREEMENT_STATE);
    delivery.provider[0] ^= 1U;
    delivery.deliverables[0].artifact_size = 0U;
    LXP_CASE_CHECK(lx_service_deliverable_check(&ctx, &agreement,
        &delivery) == LXP_ERR_NON_CANONICAL);
    delivery.deliverables[0].artifact_size = 1000U;
    (void)memset(delivery.deliverables[0].availability_reference, 0, 32U);
    LXP_CASE_CHECK(lx_service_deliverable_check(&ctx, &agreement,
        &delivery) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(runtime.read_escrow_link(runtime.user, escrow.escrow_id,
        &request.escrow_link) == LXP_OK);
    {
        uint8_t unknown[32];
        lx_service_escrow_link missing;
        id32(unknown, 98U);
        LXP_CASE_CHECK(runtime.read_escrow_link(runtime.user, unknown,
            &missing) == LXP_ERR_ESCROW_STATE);
    }
    request.authority = &outsider;
    id32(request.dispute.dispute_id, 92U);
    id32(request.dispute.activity_id, 71U);
    (void)memcpy(request.dispute.agreement_id, agreement.agreement_id, 32U);
    (void)memcpy(request.dispute.raiser, outsider.principal, 32U);
    id32(request.dispute.evidence_hashes[0], 13U);
    request.dispute.evidence_hash_count = 1U;
    LXP_CASE_CHECK(lx_service_dispute_open_execute(&ctx, &request,
        &dispute) == LXP_ERR_UNAUTHORIZED_DISPUTANT);
    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, before) == LXP_OK);
    length = dispute_payload(payload);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_OPEN, payload, length,
        &outsider, 100U, 8U, 71U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_ERR_UNAUTHORIZED_DISPUTANT);
    LXP_CASE_CHECK(event_buffer.count == 0U);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_OPEN, payload, length,
        &buyer, 100U, 8U, 72U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    LXP_CASE_CHECK(restart(8U) == 0);
    length = resolution_payload(payload);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, before) == LXP_OK);
    LXP_CASE_CHECK(consume(LX_SERVICE_DISPUTE_RESOLVE, payload, length,
        &buyer, 100U, 9U, 73U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_ERR_UNAUTHORIZED_ARBITER);
    LXP_CASE_CHECK(event_buffer.count == 0U);
    LXP_CASE_CHECK(lxp_state_root(&service_kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 9U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, request.dispute.dispute_id,
        &dispute) == LXP_OK);
    LXP_CASE_CHECK(!dispute.resolved);
    return finish();
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"proof_lx_service_h_happy_path", proof_lx_service_h_happy_path},
        {"proof_lx_service_h_refusal", proof_lx_service_h_refusal}
    };
    char *all[] = {argv[0], (char *)cases[0].name, (char *)cases[1].name};
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
        argc == 1 ? 3 : argc, argc == 1 ? all : argv);
}
