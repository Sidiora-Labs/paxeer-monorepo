#include "lxp_kernel_case.h"

#include "layerx/lx_service.h"
#include "layerx/lxp_kernel.h"

enum { WORK_ARENA_BYTES = 16384, GAS_LIMIT = 100000 };

static lxp_state_store store;
static lxp_state_journal journal;
static lxp_kernel kernel;
static lxp_module_ctx ctx;
static lxp_arena arena;
static uint8_t arena_bytes[WORK_ARENA_BYTES];
static const uint64_t parameter_set = 1U;

static int seed_delivery(lx_service_agreement *agreement,
                         lx_service_delivery *delivery)
{
    lx_service_offer offer = {0};
    lx_service_agreement seeded = {0};

    LXP_CASE_CHECK(lxp_state_store_init(&store, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &store, &journal,
                                     &parameter_set, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(
                       &kernel, lx_service_module_iface()) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes,
                                  sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(
                       &ctx, &kernel, LXP_MODULE_SERVICE, 100U, 0U, 1U,
                       (uint64_t)GAS_LIMIT, &arena, true) == LXP_OK);

    offer.offer_id[0] = 1U;
    offer.activity_id[0] = 2U;
    offer.offering_agent[0] = 3U;
    offer.asset_id[0] = 4U;
    offer.price = (lxp_u128){0U, 25U};
    offer.terms_hash[0] = 5U;
    offer.deliverable_specification_hash[0] = 6U;
    offer.delivery_deadline = 1000U;
    offer.acceptance_window = 200U;
    offer.dispute_window = 300U;
    offer.default_outcome = LX_SERVICE_DEFAULT_ACCEPT;
    offer.offer_expiry = 900U;
    LXP_CASE_CHECK(lx_service_offer_put(&ctx, &offer) == LXP_OK);

    seeded.agreement_id[0] = 7U;
    (void)memcpy(seeded.offer_id, offer.offer_id, 32U);
    (void)memcpy(seeded.provider, offer.offering_agent, 32U);
    seeded.buyer[0] = 8U;
    (void)memcpy(seeded.terms_hash, offer.terms_hash, 32U);
    seeded.escrow_id[0] = 9U;
    seeded.delivery_deadline = offer.delivery_deadline;
    seeded.acceptance_window_end = 1200U;
    seeded.dispute_window_end = 1500U;
    seeded.default_outcome = offer.default_outcome;
    seeded.state = LX_SERVICE_AGREEMENT_COMMITTED;
    LXP_CASE_CHECK(lx_service_agreement_put(&ctx, &seeded) == LXP_OK);
    LXP_CASE_CHECK(lx_service_agreement_lookup(
                       &ctx, seeded.agreement_id, agreement) == LXP_OK);

    (void)memset(delivery, 0, sizeof(*delivery));
    delivery->delivery_id[0] = 10U;
    delivery->activity_id[0] = 11U;
    (void)memcpy(delivery->agreement_id, agreement->agreement_id, 32U);
    (void)memcpy(delivery->provider, agreement->provider, 32U);
    delivery->deliverable_count = 1U;
    (void)memcpy(delivery->deliverables[0].hash,
                 offer.deliverable_specification_hash, 32U);
    delivery->deliverables[0].artifact_size = 10U;
    delivery->deliverables[0].availability_reference[31] = 12U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, agreement, delivery) == LXP_OK);
    return 0;
}

static int deliverable_empty_reference_non_canonical(void)
{
    lx_service_agreement agreement;
    lx_service_delivery delivery;

    LXP_CASE_CHECK(seed_delivery(&agreement, &delivery) == 0);
    delivery.deliverables[0].artifact_size = 0U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) == LXP_ERR_NON_CANONICAL);

    delivery.deliverables[0].artifact_size = 10U;
    (void)memset(delivery.deliverables[0].availability_reference, 0, 32U);
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) == LXP_ERR_NON_CANONICAL);

    delivery.deliverables[0].artifact_size = 0U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) == LXP_ERR_NON_CANONICAL);

    delivery.deliverables[0].artifact_size = 10U;
    delivery.deliverables[0].availability_reference[31] = 12U;
    delivery.deliverable_count = 0U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) == LXP_ERR_NON_CANONICAL);
    delivery.deliverable_count = LX_SERVICE_MAX_DELIVERABLES + 1U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) == LXP_ERR_NON_CANONICAL);
    delivery.deliverable_count = 1U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) == LXP_OK);

    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

static int deliverable_hash_mismatch_unchanged(void)
{
    lx_service_agreement agreement;
    lx_service_delivery delivery;

    LXP_CASE_CHECK(seed_delivery(&agreement, &delivery) == 0);
    delivery.deliverables[0].hash[31] = 1U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) ==
                   LXP_ERR_DELIVERABLE_MISMATCH);

    delivery.deliverables[0].artifact_size = 0U;
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) ==
                   LXP_ERR_DELIVERABLE_MISMATCH);
    delivery.deliverables[0].artifact_size = 10U;
    (void)memset(delivery.deliverables[0].availability_reference, 0, 32U);
    LXP_CASE_CHECK(lx_service_deliverable_check(
                       &ctx, &agreement, &delivery) ==
                   LXP_ERR_DELIVERABLE_MISMATCH);

    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "deliverable_empty_reference_non_canonical",
          deliverable_empty_reference_non_canonical },
        { "deliverable_hash_mismatch_unchanged",
          deliverable_hash_mismatch_unchanged }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
