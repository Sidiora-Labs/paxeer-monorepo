/*
 * Proves: src/protocol/lxp_result.c lxp_result_registry_count(),
 * lxp_result_registry_at(), lxp_result_is_registered() ->
 * include/layerx/lxp_result.h C ABI -> proof_result_registry_happy_path(),
 * proof_result_registry_refusal(), the first result-registry consumers.
 *
 * The linked kernel owns the static registry; no persistent registry state
 * exists to reopen. Each observation calls the exported symbols afresh.
 * A real in-process kernel and state store also produce success and refusal
 * codes which the consumer resolves through that same registry.
 *
 * Out-of-scope finding: src/protocol/lxp_result.c lxp_result_registry_at()
 * returns LXP_ERR_UNKNOWN_FIELD for an index at or past count, although
 * contracts/kernel.md#result-registry expects LXP_ERR_NON_CANONICAL.
 * The refusal case preserves and asserts the current implementation.
 * tests/codec/lxp_kernel_case.h lxp_kernel_case_main() requires case names;
 * main supplies both names when invoked without arguments.
 */
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_result.h"
#include "../codec/lxp_kernel_case.h"

static lxp_kernel kernel;
static lxp_state_store store;
static lxp_state_journal journal;
static const uint64_t parameters = 1U;

/* Expectations come from the public protocol list, not a replacement registry. */
static const struct {
    lxp_result value;
    const char *name;
} expected_results[] = {
#define LXP_PROOF_RESULT_ROW(name, value) { value, #name },
    LXP_RESULT_CODE_LIST(LXP_PROOF_RESULT_ROW)
#undef LXP_PROOF_RESULT_ROW
};

static int proof_result_registry_happy_path(void)
{
    const size_t expected_count =
        sizeof(expected_results) / sizeof(expected_results[0]);
    size_t pass;
    size_t i;
    lxp_result produced;

    LXP_CASE_CHECK(lxp_state_store_init(&store, 0U) == LXP_OK);
    produced = lxp_kernel_create(&kernel, &store, &journal, &parameters, 1U);
    LXP_CASE_CHECK(produced == LXP_OK);
    LXP_CASE_CHECK(lxp_result_is_registered(produced));
    LXP_CASE_CHECK(expected_count > 0U);

    for (pass = 0U; pass < 2U; ++pass) {
        size_t observed = 0U;
        const size_t count = lxp_result_registry_count();

        LXP_CASE_CHECK(count == expected_count);
        for (i = 0U; i < count; ++i) {
            lxp_result value = INT32_MAX;
            const char *name = NULL;
            size_t previous;

            LXP_CASE_CHECK(lxp_result_registry_at(i, &value, &name) == LXP_OK);
            LXP_CASE_CHECK(value == expected_results[i].value);
            LXP_CASE_CHECK(name != NULL);
            LXP_CASE_CHECK(strcmp(name, expected_results[i].name) == 0);
            LXP_CASE_CHECK(lxp_result_is_registered(value));
            if (value == produced) {
                LXP_CASE_CHECK(strcmp(name, "LXP_OK") == 0);
                ++observed;
            }
            for (previous = 0U; previous < i; ++previous) {
                LXP_CASE_CHECK(value != expected_results[previous].value);
                LXP_CASE_CHECK(strcmp(name, expected_results[previous].name) != 0);
            }
        }
        LXP_CASE_CHECK(observed == 1U);
    }
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

static int proof_result_registry_refusal(void)
{
    static const lxp_result malformed[] = {
        INT32_MIN, INT32_MAX, 1, -9, -109, -223, -308, -419,
        -504, -603, -740, -806, -907, -1000, -1004
    };
    static const char untouched[] = "untouched";
    size_t pass;
    size_t i;
    lxp_result produced;

    LXP_CASE_CHECK(lxp_state_store_init(&store, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &store, &journal, &parameters,
                                     2U) == LXP_OK);
    for (pass = 0U; pass < 2U; ++pass) {
        const size_t count = lxp_result_registry_count();
        size_t observed = 0U;
        lxp_result value = INT32_MAX;
        const char *name = untouched;

        produced = lxp_kernel_set_epoch(&kernel, 1U);
        LXP_CASE_CHECK(produced == LXP_ERR_TIMESTAMP_REGRESSION);
        LXP_CASE_CHECK(kernel.epoch == 2U);
        LXP_CASE_CHECK(lxp_result_is_registered(produced));
        for (i = 0U; i < count; ++i) {
            value = INT32_MAX;
            name = NULL;
            LXP_CASE_CHECK(lxp_result_registry_at(i, &value, &name) == LXP_OK);
            LXP_CASE_CHECK(name != NULL);
            if (value == produced) {
                LXP_CASE_CHECK(strcmp(name, "LXP_ERR_TIMESTAMP_REGRESSION") == 0);
                ++observed;
            }
        }
        LXP_CASE_CHECK(observed == 1U);

        /* Corrupt a value actually emitted by the registry at the decode edge. */
        LXP_CASE_CHECK(lxp_result_registry_at(0U, &value, &name) == LXP_OK);
        LXP_CASE_CHECK(value == LXP_OK);
        value += 1;
        LXP_CASE_CHECK(!lxp_result_is_registered(value));
        for (i = 0U; i < sizeof(malformed) / sizeof(malformed[0]); ++i) {
            LXP_CASE_CHECK(!lxp_result_is_registered(malformed[i]));
        }

        value = INT32_MAX;
        name = untouched;
        LXP_CASE_CHECK(lxp_result_registry_at(count, &value, &name) ==
                         LXP_ERR_UNKNOWN_FIELD);
        LXP_CASE_CHECK(value == INT32_MAX && name == untouched);
        LXP_CASE_CHECK(lxp_result_registry_at(SIZE_MAX, &value, &name) ==
                         LXP_ERR_UNKNOWN_FIELD);
        LXP_CASE_CHECK(value == INT32_MAX && name == untouched);
        LXP_CASE_CHECK(lxp_result_registry_at(0U, NULL, &name) ==
                         LXP_ERR_NON_CANONICAL);
        LXP_CASE_CHECK(name == untouched);
        LXP_CASE_CHECK(lxp_result_registry_at(0U, &value, NULL) ==
                         LXP_ERR_NON_CANONICAL);
        LXP_CASE_CHECK(value == INT32_MAX);
        LXP_CASE_CHECK(lxp_result_registry_at(0U, NULL, NULL) ==
                         LXP_ERR_NON_CANONICAL);
        LXP_CASE_CHECK(lxp_result_registry_at(count, NULL, NULL) ==
                         LXP_ERR_UNKNOWN_FIELD);
        LXP_CASE_CHECK(lxp_result_registry_count() == count);
        LXP_CASE_CHECK(lxp_result_is_registered(LXP_ERR_UNKNOWN_FIELD));
        LXP_CASE_CHECK(lxp_result_is_registered(LXP_ERR_NON_CANONICAL));

        /* Refusals leave the static producer usable by the next consumer. */
        LXP_CASE_CHECK(lxp_result_registry_at(0U, &value, &name) == LXP_OK);
        LXP_CASE_CHECK(value == LXP_OK && strcmp(name, "LXP_OK") == 0);
    }
    LXP_CASE_CHECK(lxp_state_store_destroy(&store) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        { "proof_result_registry_happy_path", proof_result_registry_happy_path },
        { "proof_result_registry_refusal", proof_result_registry_refusal }
    };
    char *all_cases[] = {
        argv[0], "proof_result_registry_happy_path", "proof_result_registry_refusal"
    };

    if (argc == 1) {
        return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                    3, all_cases);
    }
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
