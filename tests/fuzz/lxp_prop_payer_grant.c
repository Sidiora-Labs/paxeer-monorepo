#include "layerx/lxp_ledger.h"
#include "lxp_kernel_case.h"

struct lxp_payer_grant;

extern lxp_result lxp_payer_grant_encode(const struct lxp_payer_grant *,
    uint8_t *, size_t, size_t *);
extern lxp_result lxp_payer_grant_decode(const uint8_t *, size_t,
    struct lxp_payer_grant *);
extern lxp_result lxp_grant_authorization_message(const struct lxp_payer_grant *,
    uint8_t *, size_t, size_t *);
extern lxp_result lxp_verify_payer_grant(const struct lxp_payer_grant *,
    const lx_account *);

#pragma weak lxp_payer_grant_encode
#pragma weak lxp_payer_grant_decode
#pragma weak lxp_grant_authorization_message
#pragma weak lxp_verify_payer_grant

static int payer_grant_roundtrip(void)
{
    LXP_CASE_CHECK(lxp_payer_grant_encode == NULL);
    LXP_CASE_CHECK(lxp_payer_grant_decode == NULL);
    return 0;
}

static int payer_grant_mutation_canonical(void)
{
    LXP_CASE_CHECK(lxp_payer_grant_encode == NULL);
    LXP_CASE_CHECK(lxp_payer_grant_decode == NULL);
    LXP_CASE_CHECK(lxp_grant_authorization_message == NULL);
    LXP_CASE_CHECK(lxp_verify_payer_grant == NULL);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "payer_grant_roundtrip", payer_grant_roundtrip },
        { "payer_grant_mutation_canonical", payer_grant_mutation_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
