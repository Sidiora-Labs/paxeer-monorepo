#include "lxp_kernel_case.h"
#include "../modules/test_service_helpers.h"

#include "layerx/lx_escrow.h"

typedef struct escrow_reader_state {
    lxp_kernel *kernel;
    size_t calls;
    uint8_t escrow_id[32];
    lx_service_escrow_link link;
} escrow_reader_state;

static lxp_arena escrow_arena;
static uint8_t escrow_bytes[WORK_ARENA_BYTES];

static lxp_result open_escrow_ctx(lxp_module_ctx *ctx, lxp_kernel *kernel,
                                  bool mutable)
{
    lxp_result status = lxp_arena_init(&escrow_arena, escrow_bytes,
                                       sizeof(escrow_bytes));
    if (status != LXP_OK) return status;
    return lxp_module_ctx_init(ctx, kernel, LXP_MODULE_ESCROW, 100U, 0U,
                               1U, (uint64_t)GAS_LIMIT, &escrow_arena,
                               mutable);
}

static lxp_result host_read_escrow_link(void *user,
                                       const uint8_t escrow_id[32],
                                       lx_service_escrow_link *link)
{
    escrow_reader_state *reader = user;
    lx_escrow_record record;
    lxp_module_ctx ctx;
    lxp_result status;
    if (reader == NULL || link == NULL) return LXP_ERR_NON_CANONICAL;
    ++reader->calls;
    (void)memcpy(reader->escrow_id, escrow_id, 32U);
    status = open_escrow_ctx(&ctx, reader->kernel, false);
    if (status != LXP_OK) return status;
    status = lx_escrow_lookup(&ctx, escrow_id, &record);
    if (status != LXP_OK) return status;
    (void)memcpy(link->holder, record.owner, 32U);
    (void)memcpy(link->beneficiary, record.beneficiary, 32U);
    (void)memcpy(link->asset_id, record.asset_id, 32U);
    (void)memcpy(link->arbiter, record.arbiter, 32U);
    link->state = (uint8_t)record.state;
    reader->link = *link;
    return LXP_OK;
}

static int setup(lx_service_agreement *agreement,
                  lxp_authority_resolved *buyer)
{
    lxp_module_ctx ctx;
    lx_escrow_record escrow;
    LXP_CASE_CHECK(lxp_state_store_init(&store_state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&service_kernel, &store_state,
                                     &state_journal, &parameter_set,
                                     0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
                          lx_service_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&service_kernel,
                          lx_escrow_module_iface()) == LXP_OK);
    (void)memset(buyer, 0, sizeof(*buyer));
    id32(buyer->principal, 2U);
    (void)memset(agreement, 0, sizeof(*agreement));
    id32(agreement->agreement_id, 10U);
    id32(agreement->offer_id, 11U);
    id32(agreement->provider, 1U);
    (void)memcpy(agreement->buyer, buyer->principal, 32U);
    id32(agreement->terms_hash, 12U);
    id32(agreement->escrow_id, 41U);
    agreement->delivery_deadline = 1000U;
    agreement->acceptance_window_end = 1100U;
    agreement->dispute_window_end = 1200U;
    agreement->default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    agreement->state = LX_SERVICE_AGREEMENT_REJECTED;
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 1U) == 0);
    LXP_CASE_CHECK(lx_service_agreement_put(&ctx, agreement) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    (void)memset(&escrow, 0, sizeof(escrow));
    (void)memcpy(escrow.escrow_id, agreement->escrow_id, 32U);
    (void)memcpy(escrow.owner, agreement->buyer, 32U);
    (void)memcpy(escrow.beneficiary, agreement->provider, 32U);
    id32(escrow.escrow_account, 42U);
    id32(escrow.asset_id, 43U);
    id32(escrow.arbiter, 3U);
    escrow.state = LX_ESCROW_STATE_OPEN;
    escrow.locked_amount.lo = 100U;
    escrow.expiry = 2000U;
    escrow.dispute_window = 1000U;
    (void)memcpy(escrow.terms_hash, agreement->terms_hash, 32U);
    (void)memcpy(escrow.agreement_reference, agreement->agreement_id, 32U);
    LXP_CASE_CHECK(open_escrow_ctx(&ctx, &service_kernel, true) == LXP_OK);
    LXP_CASE_CHECK(lx_escrow_state_put(&ctx, &escrow) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    return 0;
}

static size_t open_payload(uint8_t *payload,
                            const lx_service_agreement *agreement)
{
    size_t offset = 0U;
    payload[offset++] = 0U;
    payload[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(payload + offset, 50U); offset += 32U;
    (void)memcpy(payload + offset, agreement->agreement_id, 32U);
    offset += 32U;
    payload[offset++] = 1U;
    id32(payload + offset, 51U); offset += 32U;
    return offset;
}

static size_t resolve_payload(uint8_t *payload)
{
    size_t offset = 0U;
    payload[offset++] = 0U;
    payload[offset++] = (uint8_t)LX_SERVICE_RECORD_VERSION;
    id32(payload + offset, 50U); offset += 32U;
    be16(payload + offset, 1U); offset += 2U;
    be32(payload + offset, 5000U); offset += 4U;
    id32(payload + offset, 52U); offset += 32U;
    return offset;
}

static int service_runtime_missing_disabled(void)
{
    lx_service_agreement agreement;
    lxp_authority_resolved buyer;
    lx_service_runtime runtime = {NULL, NULL};
    uint8_t payload[128];
    size_t length;
    lxp_result outcome;
    size_t attempt;
    LXP_CASE_CHECK(setup(&agreement, &buyer) == 0);
    for (attempt = 0U; attempt < 2U; ++attempt) {
        length = open_payload(payload, &agreement);
        LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                                  &buyer, 100U, 2U, 60U, &outcome) == LXP_OK);
        LXP_CASE_CHECK(outcome == LXP_ERR_MODULE_DISABLED);
        LXP_CASE_CHECK(event_buffer.count == 0U);
        length = resolve_payload(payload);
        LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload, length,
                                  &buyer, 100U, 3U, 61U, &outcome) == LXP_OK);
        LXP_CASE_CHECK(outcome == LXP_ERR_MODULE_DISABLED);
        LXP_CASE_CHECK(event_buffer.count == 0U);
        if (attempt == 0U)
            LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&service_kernel,
                              LXP_MODULE_SERVICE, &runtime) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    return 0;
}

static int service_runtime_reader_called_once(void)
{
    lx_service_agreement agreement;
    lx_service_dispute dispute;
    lxp_authority_resolved buyer;
    escrow_reader_state reader = {0};
    lx_service_runtime runtime = {host_read_escrow_link, &reader};
    uint8_t payload[128];
    uint8_t dispute_id[32];
    uint8_t escrow_root_before[32];
    uint8_t escrow_root_after[32];
    size_t length;
    lxp_result outcome;
    lxp_result missing;
    lxp_module_ctx ctx;
    lx_escrow_record record;
    LXP_CASE_CHECK(setup(&agreement, &buyer) == 0);
    reader.kernel = &service_kernel;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&service_kernel,
                          LXP_MODULE_SERVICE, &runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_ESCROW,
                                          escrow_root_before) == LXP_OK);
    length = open_payload(payload, &agreement);
    LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                              &buyer, 100U, 2U, 60U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    LXP_CASE_CHECK(reader.calls == 1U);
    LXP_CASE_CHECK(memcmp(reader.escrow_id, agreement.escrow_id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(reader.link.holder, agreement.buyer, 32U) == 0);
    LXP_CASE_CHECK(memcmp(reader.link.beneficiary, agreement.provider,
                           32U) == 0);
    LXP_CASE_CHECK(reader.link.asset_id[0] == 43U);
    LXP_CASE_CHECK(reader.link.arbiter[0] == 3U);
    LXP_CASE_CHECK(reader.link.state == (uint8_t)LX_ESCROW_STATE_OPEN);
    LXP_CASE_CHECK(event_buffer.count == 1U);
    id32(dispute_id, 50U);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 3U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, dispute_id,
                                              &dispute) == LXP_OK);
    LXP_CASE_CHECK(!dispute.resolved);
    reader.calls = 0U;
    length = resolve_payload(payload);
    LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_RESOLVE, payload, length,
                              &buyer, 100U, 3U, 61U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == LXP_OK);
    LXP_CASE_CHECK(reader.calls == 1U);
    LXP_CASE_CHECK(memcmp(reader.escrow_id, agreement.escrow_id, 32U) == 0);
    LXP_CASE_CHECK(open_ctx(&ctx, 100U, 4U) == 0);
    LXP_CASE_CHECK(lx_service_dispute_lookup(&ctx, dispute_id,
                                              &dispute) == LXP_OK);
    LXP_CASE_CHECK(dispute.resolved);
    LXP_CASE_CHECK(dispute.provider_basis_points == 5000U);
    LXP_CASE_CHECK(dispute.escrow_resolution_id[0] == 52U);
    LXP_CASE_CHECK(lxp_state_subtree_root(&service_kernel, LXP_MODULE_ESCROW,
                                          escrow_root_after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(escrow_root_before, escrow_root_after, 32U) == 0);
    id32(agreement.escrow_id, 99U);
    agreement.state = LX_SERVICE_AGREEMENT_REJECTED;
    LXP_CASE_CHECK(lx_service_agreement_put(&ctx, &agreement) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(open_escrow_ctx(&ctx, &service_kernel, false) == LXP_OK);
    missing = lx_escrow_lookup(&ctx, agreement.escrow_id, &record);
    LXP_CASE_CHECK(missing != LXP_OK);
    reader.calls = 0U;
    length = open_payload(payload, &agreement);
    LXP_CASE_CHECK(dispatch(LX_SERVICE_DISPUTE_OPEN, payload, length,
                              &buyer, 100U, 4U, 62U, &outcome) == LXP_OK);
    LXP_CASE_CHECK(outcome == missing);
    LXP_CASE_CHECK(reader.calls == 1U);
    LXP_CASE_CHECK(memcmp(reader.escrow_id, agreement.escrow_id, 32U) == 0);
    LXP_CASE_CHECK(event_buffer.count == 0U);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store_state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"service_runtime_missing_disabled", service_runtime_missing_disabled},
        {"service_runtime_reader_called_once", service_runtime_reader_called_once}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
