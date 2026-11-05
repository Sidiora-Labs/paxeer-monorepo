#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_bridge.h"

#include "lxp_kernel_case.h"

#include <stdlib.h>

/* Makefile (kernel domain): pass $(abspath $(LIBRARY)) before the case names. */
static const char *bridge_library_path;

typedef struct lxp_bridge_credit lxp_bridge_credit;
typedef struct lxp_bridge_profile lxp_bridge_profile;
typedef struct lxp_bridge_light_trust lxp_bridge_light_trust;

extern lxp_result lxp_bridge_credit_parse(const uint8_t *, size_t,
                                           lxp_bridge_credit *);
extern lxp_result lxp_bridge_credit_verify_asset(
    const lxp_bridge_profile *, const lxp_bridge_credit *, uint32_t,
    uint16_t, const lxp_bridge_light_trust *, uint64_t, uint8_t[32],
    lxp_bridge_light_trust *);
extern lxp_result lxp_bridge_light_trust_encode(
    const lxp_bridge_light_trust *, uint8_t[89]);
extern const lxp_module_iface *lxp_bridge_module_iface(void);

#pragma weak lxp_bridge_credit_parse
#pragma weak lxp_bridge_credit_verify_asset
#pragma weak lxp_bridge_light_trust_encode
#pragma weak lxp_bridge_module_iface

static int bridge_credit_symbols_absent(void)
{
    static const char *const retired[] = {
        "lxp_bridge_credit_parse",
        "lxp_bridge_credit_verify_asset",
        "lxp_bridge_light_trust_encode",
        "lxp_bridge_module_iface"
    };
    FILE *symbols;
    char line[4096];
    int absent = 1;
    int read_ok;
    int close_status;
    size_t i;
    size_t path_length;
    size_t offset;
    char *command;

    LXP_CASE_CHECK(lxp_bridge_credit_parse == NULL);
    LXP_CASE_CHECK(lxp_bridge_credit_verify_asset == NULL);
    LXP_CASE_CHECK(lxp_bridge_light_trust_encode == NULL);
    LXP_CASE_CHECK(lxp_bridge_module_iface == NULL);

    LXP_CASE_CHECK(bridge_library_path != NULL && bridge_library_path[0] != '\0');
    path_length = strlen(bridge_library_path);
    LXP_CASE_CHECK(path_length <= (SIZE_MAX - 64U) / 4U);
    command = malloc(64U + path_length * 4U);
    LXP_CASE_CHECK(command != NULL);
    offset = (size_t)sprintf(command, "nm -g --defined-only -- '");
    for (i = 0U; i < path_length; ++i) {
        if (bridge_library_path[i] == '\'') {
            (void)memcpy(command + offset, "'\\''", 4U);
            offset += 4U;
        } else {
            command[offset++] = bridge_library_path[i];
        }
    }
    command[offset++] = '\'';
    command[offset] = '\0';
    symbols = popen(command, "r");
    free(command);
    LXP_CASE_CHECK(symbols != NULL);
    while (fgets(line, (int)sizeof(line), symbols) != NULL) {
        size_t length = strlen(line);
        char *name;
        if (length == 0U || line[length - 1U] != '\n') {
            absent = 0;
            continue;
        }
        line[length - 1U] = '\0';
        name = strrchr(line, ' ');
        if (name == NULL) continue;
        ++name;
        for (i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
            if (strcmp(name, retired[i]) == 0) {
                fprintf(stderr, "archive still defines %s\n", name);
                absent = 0;
            }
        }
    }
    read_ok = !ferror(symbols);
    close_status = pclose(symbols);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_status == 0);
    LXP_CASE_CHECK(absent);
    return 0;
}

static int deposit_nullifier_kept(void)
{
    static const uint8_t expected_deposit[32] = {
        0xb9, 0x19, 0x76, 0x31, 0x1b, 0x1d, 0xb2, 0xe4,
        0x5c, 0x2a, 0x1a, 0x33, 0xa7, 0x93, 0x6f, 0x0e,
        0xf7, 0x1f, 0xb5, 0x6d, 0xcc, 0xa1, 0x06, 0x21,
        0xc2, 0x1c, 0xdf, 0xed, 0x0c, 0xdf, 0xa8, 0xb0
    };
    static const uint8_t expected_withdrawal[32] = {
        0xd0, 0xad, 0xb1, 0x7b, 0xcb, 0xf6, 0xa2, 0x6d,
        0xe3, 0x2d, 0x94, 0x77, 0x5a, 0x14, 0xb8, 0x0e,
        0x7f, 0xae, 0xb3, 0x7b, 0x84, 0xe4, 0xc2, 0x3e,
        0x5a, 0x72, 0x97, 0x63, 0xda, 0xba, 0x98, 0x98
    };
    lx_deposit_proof deposit = {0};
    lx_withdrawal_request withdrawal = {0};
    lx_withdrawal_record decoded = {0};
    uint8_t nullifier[32];
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES];
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES];
    size_t i;

    for (i = 0U; i < 32U; ++i) {
        deposit.deposit_id[i] = (uint8_t)i;
        withdrawal.withdrawal_id[i] = (uint8_t)i;
        withdrawal.account_id[i] = (uint8_t)(i + 32U);
        withdrawal.asset_id[i] = (uint8_t)(i + 64U);
        withdrawal.checkpoint_id[i] = (uint8_t)(i + 96U);
        withdrawal.payout_recipient[i] = (uint8_t)(i + 128U);
    }
    withdrawal.network_id = UINT32_C(0x01020304);
    withdrawal.amount.hi = UINT64_C(0x0102030405060708);
    withdrawal.amount.lo = UINT64_C(0x1112131415161718);

    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_deposit, sizeof(nullifier)) == 0);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&withdrawal, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_withdrawal,
                          sizeof(nullifier)) == 0);
    LXP_CASE_CHECK(lx_withdrawal_nullifier(&withdrawal, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_withdrawal,
                          sizeof(nullifier)) == 0);
    LXP_CASE_CHECK(lx_withdrawal_state_encode(&withdrawal, key, value) == LXP_OK);
    LXP_CASE_CHECK(memcmp(key, "withdrawal:", 11U) == 0);
    LXP_CASE_CHECK(memcmp(key + 11U, expected_withdrawal, 32U) == 0);
    LXP_CASE_CHECK(lx_withdrawal_state_decode(key, sizeof(key), value,
                                             sizeof(value), &decoded) == LXP_OK);
    LXP_CASE_CHECK(memcmp(decoded.nullifier, expected_withdrawal, 32U) == 0);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&decoded.request, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_withdrawal,
                          sizeof(nullifier)) == 0);

    LXP_CASE_CHECK(lxp_deposit_nullifier(NULL, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, NULL) ==
                   LXP_ERR_NON_CANONICAL);
    (void)memset(deposit.deposit_id, 0, sizeof(deposit.deposit_id));
    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(NULL, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&withdrawal, NULL) ==
                   LXP_ERR_NON_CANONICAL);
    withdrawal.network_id = 0U;
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&withdrawal, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "bridge_credit_symbols_absent", bridge_credit_symbols_absent },
        { "deposit_nullifier_kept", deposit_nullifier_kept }
    };

    if (argc < 3) {
        fprintf(stderr, "usage: %s <linked-library-path> <case>...\n", argv[0]);
        return 2;
    }
    bridge_library_path = argv[1];
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc - 1, argv + 1);
}
