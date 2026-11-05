#include "layerx/lxp_transfer.h"

#include <stdio.h>
#include <string.h>

static const uint8_t asset_ids[2][32] = {
    { 0x41U },
    { 0x41U, [31] = 0x01U }
};

#define REQUIRE(condition) do { \
    if (!(condition)) { \
        (void)fprintf(stderr, "%s:%d: %s\n", __func__, __LINE__, #condition); \
        goto done; \
    } \
} while (0)

static lxp_result open_accounts(lx_account_registry *registry,
                                 lx_account *accounts[4])
{
    static const char *const names[4] = {
        "agent:did:key:alice:main", "agent:did:key:bob:main",
        "agent:did:key:carol:main", "agent:did:key:dave:main"
    };
    size_t i;
    lxp_result status = lx_account_registry_init(registry);
    if (status != LXP_OK) return status;
    for (i = 0U; i < 4U; ++i) {
        uint8_t id[32];
        status = lx_account_id_from_string((const uint8_t *)names[i],
                                            strlen(names[i]), id);
        if (status != LXP_OK) return status;
        status = lx_account_open(registry, (const uint8_t *)names[i],
                                  strlen(names[i]), id, 1U,
                                  LX_ACCOUNT_OPEN_CREDIT, NULL, &accounts[i]);
        if (status != LXP_OK) return status;
        status = lxp_ledger_bootstrap_balance(accounts[i], asset_ids[i / 2U],
                                               (lxp_u128){ 0U, 100U }, 0U);
        if (status != LXP_OK) return status;
    }
    return LXP_OK;
}

static lxp_transfer_leg transfer_leg(lx_account *from, lx_account *to,
                                      size_t asset, lxp_u128 amount,
                                      uint8_t supply_mode)
{
    lxp_transfer_leg leg = { 0 };
    leg.from = from;
    leg.to = to;
    (void)memcpy(leg.asset_id, asset_ids[asset], sizeof(leg.asset_id));
    leg.amount = amount;
    leg.reason = LXP_REASON_PAYMENT;
    leg.supply_mode = supply_mode;
    return leg;
}

static int conservation_supply_modes(void)
{
    lx_account_registry registry = { 0 };
    lx_account *accounts[4];
    lxp_transfer_leg legs[6];
    int failed = 1;

    REQUIRE(open_accounts(&registry, accounts) == LXP_OK);
    legs[0] = transfer_leg(accounts[0], accounts[1], 0U,
                            (lxp_u128){ 0U, 5U }, LXP_TRANSFER_DEBIT_ONLY);
    legs[1] = transfer_leg(accounts[2], accounts[3], 1U,
                            (lxp_u128){ 0U, 7U }, LXP_TRANSFER_CREDIT_ONLY);
    legs[2] = transfer_leg(accounts[0], accounts[1], 0U,
                            (lxp_u128){ 0U, 13U }, LXP_TRANSFER_CONSERVED);
    legs[3] = transfer_leg(accounts[3], accounts[2], 1U,
                            (lxp_u128){ 0U, 7U }, LXP_TRANSFER_DEBIT_ONLY);
    legs[4] = transfer_leg(accounts[1], accounts[0], 0U,
                            (lxp_u128){ 0U, 5U }, LXP_TRANSFER_CREDIT_ONLY);
    legs[5] = transfer_leg(accounts[2], accounts[3], 1U,
                            (lxp_u128){ 0U, 17U }, LXP_TRANSFER_CONSERVED);
    REQUIRE(lxp_conservation_check(legs, 6U) == LXP_OK);
    REQUIRE(lxp_conservation_check(&legs[0], 1U) == LXP_ERR_CONSERVATION);
    REQUIRE(lxp_conservation_check(&legs[1], 1U) == LXP_ERR_CONSERVATION);
    REQUIRE(lxp_conservation_check(&legs[2], 1U) == LXP_OK);

    legs[4].amount = (lxp_u128){ 0U, 6U };
    REQUIRE(lxp_conservation_check(legs, 6U) == LXP_ERR_CONSERVATION);
    legs[4].amount = (lxp_u128){ 0U, 5U };
    legs[3].amount = (lxp_u128){ 0U, 8U };
    REQUIRE(lxp_conservation_check(legs, 6U) == LXP_ERR_CONSERVATION);
    legs[4].amount = (lxp_u128){ 0U, 6U };
    REQUIRE(lxp_conservation_check(legs, 6U) == LXP_ERR_CONSERVATION);
    legs[3].amount = (lxp_u128){ 0U, 7U };
    legs[4].amount = (lxp_u128){ 0U, 5U };
    REQUIRE(lxp_conservation_check(legs, 6U) == LXP_OK);
    failed = 0;
done:
    lx_account_registry_release(&registry);
    return failed;
}

static int conservation_wide_accumulator(void)
{
    lx_account_registry registry = { 0 };
    lx_account *accounts[4];
    lxp_transfer_leg legs[LXP_MAX_TRANSFER_SET_LEGS + 1U];
    size_t i;
    int failed = 1;

    REQUIRE(open_accounts(&registry, accounts) == LXP_OK);
    for (i = 0U; i < LXP_MAX_TRANSFER_SET_LEGS + 1U; ++i)
        legs[i] = transfer_leg(accounts[0], accounts[1], 0U,
                                (lxp_u128){ UINT64_MAX, UINT64_MAX },
                                LXP_TRANSFER_CONSERVED);
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS) == LXP_OK);
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS + 1U) ==
            LXP_ERR_TOO_MANY_LEGS);
    REQUIRE(lxp_conservation_check(NULL, 1U) == LXP_ERR_TOO_MANY_LEGS);
    REQUIRE(lxp_conservation_check(legs, 0U) == LXP_ERR_TOO_MANY_LEGS);

    legs[LXP_MAX_TRANSFER_SET_LEGS - 1U].from = NULL;
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS) ==
            LXP_ERR_NON_CANONICAL);
    legs[LXP_MAX_TRANSFER_SET_LEGS - 1U].from = accounts[0];
    legs[LXP_MAX_TRANSFER_SET_LEGS - 1U].to = NULL;
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS) ==
            LXP_ERR_NON_CANONICAL);
    legs[LXP_MAX_TRANSFER_SET_LEGS - 1U].to = accounts[1];
    legs[LXP_MAX_TRANSFER_SET_LEGS - 1U].supply_mode =
        LXP_TRANSFER_DEBIT_ONLY + 1U;
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS) ==
            LXP_ERR_NON_CANONICAL);

    for (i = 0U; i < LXP_MAX_TRANSFER_SET_LEGS; ++i)
        legs[i].supply_mode = i < LXP_MAX_TRANSFER_SET_LEGS / 2U ?
                                 LXP_TRANSFER_CREDIT_ONLY :
                                 LXP_TRANSFER_DEBIT_ONLY;
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS) == LXP_OK);
    legs[LXP_MAX_TRANSFER_SET_LEGS - 1U].amount =
        (lxp_u128){ UINT64_MAX, UINT64_MAX - 1U };
    REQUIRE(lxp_conservation_check(legs, LXP_MAX_TRANSFER_SET_LEGS) ==
            LXP_ERR_CONSERVATION);

    legs[0].supply_mode = LXP_TRANSFER_CREDIT_ONLY;
    legs[1].supply_mode = LXP_TRANSFER_CREDIT_ONLY;
    legs[1].amount = (lxp_u128){ 0U, 2U };
    legs[2].supply_mode = LXP_TRANSFER_DEBIT_ONLY;
    legs[2].amount = (lxp_u128){ 0U, 1U };
    REQUIRE(lxp_conservation_check(legs, 3U) == LXP_ERR_CONSERVATION);
    legs[0].supply_mode = LXP_TRANSFER_DEBIT_ONLY;
    legs[1].supply_mode = LXP_TRANSFER_DEBIT_ONLY;
    legs[2].supply_mode = LXP_TRANSFER_CREDIT_ONLY;
    REQUIRE(lxp_conservation_check(legs, 3U) == LXP_ERR_CONSERVATION);
    failed = 0;
done:
    lx_account_registry_release(&registry);
    return failed;
}

int main(int argc, char **argv)
{
    static const struct {
        const char *name;
        int (*run)(void);
    } tests[] = {
        { "conservation_supply_modes", conservation_supply_modes },
        { "conservation_wide_accumulator", conservation_wide_accumulator }
    };
    size_t count = sizeof(tests) / sizeof(tests[0]);
    int argument;
    size_t i;

    if (argc == 1) {
        for (i = 0U; i < count; ++i) {
            if (tests[i].run() != 0) return 1;
            (void)printf("PASS %s\n", tests[i].name);
        }
        return 0;
    }
    for (argument = 1; argument < argc; ++argument) {
        for (i = 0U; i < count; ++i)
            if (strcmp(argv[argument], tests[i].name) == 0) break;
        if (i == count) {
            (void)fprintf(stderr, "Unknown test: %s\n", argv[argument]);
            return 1;
        }
        if (tests[i].run() != 0) return 1;
        (void)printf("PASS %s\n", tests[i].name);
    }
    return 0;
}
