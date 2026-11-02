#include "lxp_kernel_case.h"
#include "../modules/test_service_helpers.h"

#include "layerx/lx_escrow.h"

typedef struct dispute_fixture {
    lx_service_agreement agreement;
    lx_escrow_record escrow;
    lxp_authority_resolved buyer;
    lxp_authority_resolved provider;
    lxp_authority_resolved arbiter;
    lxp_authority_resolved outsider;
} dispute_fixture;

static lxp_arena escrow_arena;
static uint8_t escrow_bytes[WORK_ARENA_BYTES];

static lxp_result escrow_ctx(lxp_module_ctx *ctx, lxp_kernel *kernel,
                              bool mutable)
{
    lxp_result status = lxp_arena_init(&escrow_arena, escrow_bytes,
                                       sizeof(escrow_bytes));
    if (status != LXP_OK) return status;
    return lxp_module_ctx_init(ctx, kernel, LXP_MODULE_ESCROW, 100U, 0U,
                               1U, (uint64_t)GAS_LIMIT, &escrow_arena,
                               mutable);
}

static lxp_result read_escrow_link(void *user, const uint8_t escrow_id[32],
                                   lx_service_escrow_link *link)
{
    lxp_module_ctx ctx;
    lx_escrow_record record;
    lxp_result status = escrow_ctx(&ctx, user, false);
    if (status != LXP_OK) return status;
    status = lx_escrow_lookup(&ctx, escrow_id, &record);
    if (status != LXP_OK) return status;
    (void)memcpy(link->holder, record.owner, 32U);
    (void)memcpy(link->beneficiary, record.beneficiary, 32U);
    (void)memcpy(link->asset_id, record.asset_id, 32U);
    (void)memcpy(link->arbiter, record.arbiter, 32U);
    link->state = (uint8_t)record.state;
    return LXP_OK;
}

static lx_service_runtime runtime = {read_escrow_link, &service_kernel};

static int update_escrow(const dispute_fixture *fixture)
{
    lxp_module_ctx ctx;
    LXP_CASE_CHECK(escrow_ctx(&ctx, &service_kernel, true) == LXP_OK);
    LXP_CASE_CHECK(lx_escrow_state_update(&ctx, &fixture->escrow) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static int put_agreement(const dispute_fixture *fixture)
{
    lxp_module_ctx ctx;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 1U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_put(&ctx,
                                            &fixture->agreement) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static int setup(dispute_fixture *fixture)
{
    lxp_module_ctx ctx;
    lx_service_offer offer;
    (void)memset(fixture, 0, sizeof(*fixture));
    LXP_CASE_CHECK(lxp_state_store_init(&store_state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&service_kernel, &store_state,
                                     &state_journal, &parameter_set,
                                     0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
                          lx_service_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
                          lx_escrow_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&service_kernel,
                          LXP_MODULE_SERVICE, &runtime) == LXP_OK);
    id32(fixture->provider.principal, 1U);
    id32(fixture->buyer.principal, 2U);
    id32(fixture->arbiter.principal, 3U);
    id32(fixture->outsider.principal, 4U);
    id32(fixture->agreement.agreement_id, 10U);
    id32(fixture->agreement.offer_id, 11U);
    (void)memcpy(fixture->agreement.provider,
                 fixture->provider.principal, 32U);
    (void)memcpy(fixture->agreement.buyer, fixture->buyer.principal, 32U);
    id32(fixture->agreement.terms_hash, 12U);
    id32(fixture->agreement.escrow_id, 41U);
    fixture->agreement.delivery_deadline = 1000U;
    fixture->agreement.acceptance_window_end = 1100U;
    fixture->agreement.dispute_window_end = 1200U;
    fixture->agreement.default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    fixture->agreement.state = LX_SERVICE_AGREEMENT_REJECTED;
    (void)memset(&offer, 0, sizeof(offer));
    (void)memcpy(offer.offer_id, fixture->agreement.offer_id, 32U);
    id32(offer.activity_id, 13U);
    (void)memcpy(offer.offering_agent, fixture->agreement.provider, 32U);
    id32(offer.asset_id, 43U);
    offer.price.lo = 100U;
    (void)memcpy(offer.terms_hash, fixture->agreement.terms_hash, 32U);
    id32(offer.deliverable_specification_hash, 14U);
    offer.delivery_deadline = 1000U;
    offer.acceptance_window = 100U;
    offer.dispute_window = 100U;
    offer.default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    offer.offer_expiry = 900U;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 1U) == 0);
    LXP_CASE_CHECK(lx_service_offer_put(&ctx, &offer) == LXP_OK);
    LXP_CASE_CHECK(lx_service_agreement_put(&ctx,
                                            &fixture->agreement) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    (void)memcpy(fixture->escrow.escrow_id,
                 fixture->agreement.escrow_id, 32U);
    (void)memcpy(fixture->escrow.owner, fixture->agreement.buyer, 32U);
    (void)memcpy(fixture->escrow.beneficiary,
                 fixture->agreement.provider, 32U);
    (void)memcpy(fixture->escrow.arbiter, fixture->arbiter.principal, 32U);
    (void)memcpy(fixture->escrow.asset_id, offer.asset_id, 32U);
    id32(fixture->escrow.escrow_account, 42U);
    fixture->escrow.state = LX_ESCROW_STATE_OPEN;
    fixture->escrow.locked_amount.lo = 100U;
    fixture->escrow.expiry = 2000U;
    fixture->escrow.dispute_window = 1000U;
    (void)memcpy(fixture->escrow.terms_hash,
                 fixture->agreement.terms_hash, 32U);
    (void)memcpy(fixture->escrow.agreement_reference,
                 fixture->agreement.agreement_id, 32U);
    LXP_CASE_CHECK(escrow_ctx(&ctx, &service_kernel, true) == LXP_OK);
    LXP_CASE_CHECK(lx_escrow_state_put(&ctx, &fixture->escrow) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static size_t open_payload(uint8_t *payload, const dispute_fixture *fixture)
{
    payload[0] = 0U;
    payload[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(payload + 2U, 50U);
    (void)memcpy(payload + 34U, fixture->agreement.agreement_id, 32U);
    payload[66] = 2U;
    id32(payload + 67U, 52U);
    id32(payload + 99U, 51U);
    return 131U;
}

static size_t resolve_payload(uint8_t *payload)
{
    payload[0] = 0U;
    payload[1] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(payload + 2U, 50U);
    be16(payload + 34U, 1U);
    be32(payload + 36U, 5000U);
    (void)memset(payload + 40U, 53, 32U);
    return (size_t)LX_SERVICE_DISPUTE_RESOLVE_PAYLOAD_BYTES;
}

static int expect_dispatch(uint32_t activity_type, const uint8_t *payload,
                            size_t length,
                            const lxp_authority_resolved *authority,
                            uint64_t timestamp, uint64_t sequence,
                            lxp_result expected)
{
    uint8_t escrow_before[32];
    uint8_t escrow_after[32];
    uint8_t service_before[32];
    uint8_t service_after[32];
    lxp_result outcome;
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_ESCROW,
                                          escrow_before) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_SERVICE,
                                          service_before) == LXP_OK);
    LXP_CASE_CHECK(dispatch(activity_type, payload, length, authority,
                              timestamp, sequence, 60U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == expected);
    LXP_CASE_CHECK(event_buffer.count == (expected == LXP_OK ? 1U : 0U));
    LXP_CASE_CHECK(lx_service_effect_audit(activity_type,
                                           &event_buffer) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_ESCROW,
                                          escrow_after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(escrow_before, escrow_after, 32U) == 0);
    if (expected != LXP_OK) {
        LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel,
                          LXP_MODULE_SERVICE, service_after) == LXP_OK);
        LXP_CASE_CHECK(memcmp(service_before, service_after, 32U) == 0);
    }
    return 0;
}

static int resolve_requires_linked_arbiter(void)
{
    dispute_fixture fixture;
    lx_service_dispute dispute;
    lx_service_agreement agreement;
    lxp_module_ctx ctx;
    uint8_t payload[160];
    uint8_t dispute_id[32];
    size_t length;
    size_t field;
    LXP_CASE_CHECK(setup(&fixture) == 0);
    length = open_payload(payload, &fixture);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.buyer, 100U, 2U, LXP_OK) == 0);
    length = resolve_payload(payload);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                        length, &fixture.buyer, 100U, 3U,
                        LXP_ERR_UNAUTHORIZED_ARBITER) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                        length, &fixture.provider, 100U, 3U,
                        LXP_ERR_UNAUTHORIZED_ARBITER) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                        length, &fixture.outsider, 100U, 3U,
                        LXP_ERR_UNAUTHORIZED_ARBITER) == 0);
    for (field = 0U; field < 3U; ++field) {
        uint8_t *identity = field == 0U ? fixture.escrow.owner :
                            field == 1U ? fixture.escrow.beneficiary :
                                          fixture.escrow.asset_id;
        identity[31] ^= 1U;
        LXP_CASE_CHECK(update_escrow(&fixture) == 0);
        LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                            length, &fixture.arbiter, 100U, 3U,
                            LXP_ERR_ESCROW_LINK_MISMATCH) == 0);
        identity[31] ^= 1U;
    }
    LXP_CASE_CHECK(update_escrow(&fixture) == 0);
    LXP_CASE_CHECK(put_agreement(&fixture) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                        length, &fixture.arbiter, 100U, 3U,
                        LXP_ERR_UNAUTHORIZED_DISPUTANT) == 0);
    fixture.agreement.state = LX_SERVICE_AGREEMENT_DISPUTED;
    LXP_CASE_CHECK(put_agreement(&fixture) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                        length, &fixture.arbiter, 1300U, 7U, LXP_OK) == 0);
    id32(dispute_id, 50U);
    LXP_CASE_CHECK(open_ctx(&ctx, 1300U, 8U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, dispute_id,
                                              &dispute) == LXP_OK);
    LXP_CASE_CHECK(dispute.resolved);
    LXP_CASE_CHECK(dispute.ruling == 1U);
    LXP_CASE_CHECK(dispute.provider_basis_points == 5000U);
    LXP_CASE_CHECK(dispute.resolution_sequence == 7U);
    LXP_CASE_CHECK(memcmp(dispute.escrow_resolution_id,
                           payload + 40U, 32U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx,
                        fixture.agreement.agreement_id, &agreement) == LXP_OK);
    LXP_CASE_CHECK(agreement.state == LX_SERVICE_AGREEMENT_RESOLVED);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload,
                        length, &fixture.arbiter, 1300U, 8U,
                        LXP_ERR_AGREEMENT_STATE) == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    return 0;
}

static int open_requires_arbiter_and_link(void)
{
    dispute_fixture fixture;
    uint8_t payload[160];
    size_t length;
    size_t field;
    LXP_CASE_CHECK(setup(&fixture) == 0);
    length = open_payload(payload, &fixture);
    (void)memset(fixture.escrow.arbiter, 0, 32U);
    LXP_CASE_CHECK(update_escrow(&fixture) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.buyer, 100U, 2U,
                        LXP_ERR_ARBITER_REQUIRED) == 0);
    (void)memcpy(fixture.escrow.arbiter, fixture.arbiter.principal, 32U);
    for (field = 0U; field < 3U; ++field) {
        uint8_t *identity = field == 0U ? fixture.escrow.owner :
                            field == 1U ? fixture.escrow.beneficiary :
                                          fixture.escrow.asset_id;
        identity[31] ^= 1U;
        LXP_CASE_CHECK(update_escrow(&fixture) == 0);
        LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload,
                            length, &fixture.buyer, 100U, 2U,
                            LXP_ERR_ESCROW_LINK_MISMATCH) == 0);
        identity[31] ^= 1U;
    }
    LXP_CASE_CHECK(update_escrow(&fixture) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.arbiter, 100U, 2U,
                        LXP_ERR_UNAUTHORIZED_DISPUTANT) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.provider, 100U, 2U, LXP_OK) == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    return 0;
}

static int dispute_window_rules_kept(void)
{
    dispute_fixture fixture;
    lx_service_dispute dispute;
    lx_service_agreement agreement;
    lxp_module_ctx ctx;
    uint8_t payload[160];
    uint8_t dispute_id[32];
    uint8_t zero[32] = {0};
    size_t length;
    unsigned state;
    LXP_CASE_CHECK(setup(&fixture) == 0);
    length = open_payload(payload, &fixture);
    for (state = (unsigned)LX_SERVICE_AGREEMENT_FORMED;
         state <= (unsigned)LX_SERVICE_AGREEMENT_PROPOSED; ++state) {
        if (state == (unsigned)LX_SERVICE_AGREEMENT_REJECTED) continue;
        fixture.agreement.state = (lx_service_agreement_state)state;
        LXP_CASE_CHECK(put_agreement(&fixture) == 0);
        LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload,
                            length, &fixture.buyer, 100U, 2U,
                            LXP_ERR_AGREEMENT_STATE) == 0);
    }
    fixture.agreement.state = LX_SERVICE_AGREEMENT_REJECTED;
    LXP_CASE_CHECK(put_agreement(&fixture) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.buyer, 1201U, 2U,
                        LXP_ERR_DISPUTE_WINDOW_CLOSED) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.buyer, 1200U, 9U, LXP_OK) == 0);
    id32(dispute_id, 50U);
    LXP_CASE_CHECK(open_ctx(&ctx, 1200U, 10U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, dispute_id,
                                              &dispute) == LXP_OK);
    LXP_CASE_CHECK(dispute.global_sequence == 9U);
    LXP_CASE_CHECK(!dispute.resolved);
    LXP_CASE_CHECK(dispute.ruling == 0U);
    LXP_CASE_CHECK(dispute.provider_basis_points == 0U);
    LXP_CASE_CHECK(dispute.resolution_sequence == 0U);
    LXP_CASE_CHECK(memcmp(dispute.escrow_resolution_id, zero, 32U) == 0);
    LXP_CASE_CHECK(dispute.evidence_hash_count == 2U);
    LXP_CASE_CHECK(dispute.evidence_hashes[0][0] == 51U);
    LXP_CASE_CHECK(dispute.evidence_hashes[1][0] == 52U);
    LXP_CASE_CHECK(lx_service_agreement_lookup(&ctx,
                        fixture.agreement.agreement_id, &agreement) == LXP_OK);
    LXP_CASE_CHECK(agreement.state == LX_SERVICE_AGREEMENT_DISPUTED);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.buyer, 1200U, 10U,
                        LXP_ERR_AGREEMENT_STATE) == 0);
    LXP_CASE_CHECK(put_agreement(&fixture) == 0);
    LXP_CASE_CHECK(expect_dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                        &fixture.buyer, 1200U, 10U,
                        LXP_ERR_SEQUENCE_REUSED) == 0);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"resolve_requires_linked_arbiter", resolve_requires_linked_arbiter},
        {"open_requires_arbiter_and_link", open_requires_arbiter_and_link},
        {"dispute_window_rules_kept", dispute_window_rules_kept}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
