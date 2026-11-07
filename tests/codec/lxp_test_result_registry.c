#include "layerx/lxp_result.h"
#include "lxp_kernel_case.h"

#include <stdlib.h>
#include <string.h>

static const struct {
    lxp_result value;
    const char *name;
} expected_results[] = {
#define LXP_RESULT_TEST_ROW(name, value) { value, #name },
    LXP_RESULT_CODE_LIST(LXP_RESULT_TEST_ROW)
#undef LXP_RESULT_TEST_ROW
};

static int result_registry_enumerates_in_order(void)
{
    const size_t count = sizeof(expected_results) / sizeof(expected_results[0]);
    lxp_result value = LXP_FATAL_INVARIANT;
    const char *name = NULL;
    size_t i;

    LXP_CASE_CHECK(lxp_result_registry_count() == count);
    for (i = 0U; i < count; ++i) {
        LXP_CASE_CHECK(lxp_result_registry_at(i, &value, &name) == LXP_OK);
        LXP_CASE_CHECK(value == expected_results[i].value);
        LXP_CASE_CHECK(name != NULL);
        LXP_CASE_CHECK(strcmp(name, expected_results[i].name) == 0);
        LXP_CASE_CHECK(strcmp(name, lxp_result_name(value)) == 0);
    }
    LXP_CASE_CHECK(lxp_result_registry_at(count, &value, &name) ==
                   LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_result_registry_at(SIZE_MAX, &value, &name) ==
                   LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_result_registry_at(0U, NULL, &name) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_result_registry_at(0U, &value, NULL) ==
                   LXP_ERR_NON_CANONICAL);
    return 0;
}

static int compare_result_values(const void *left, const void *right)
{
    const lxp_result a = *(const lxp_result *)left;
    const lxp_result b = *(const lxp_result *)right;

    return (a > b) - (a < b);
}

static int compare_result_names(const void *left, const void *right)
{
    return strcmp(*(const char *const *)left, *(const char *const *)right);
}

static int result_registry_values_unique(void)
{
    lxp_result values[sizeof(expected_results) / sizeof(expected_results[0])];
    const char *names[sizeof(expected_results) / sizeof(expected_results[0])];
    const size_t count = sizeof(values) / sizeof(values[0]);
    size_t i;

    LXP_CASE_CHECK(lxp_result_registry_count() == count);
    LXP_CASE_CHECK(count > 0U);
    for (i = 0U; i < count; ++i) {
        LXP_CASE_CHECK(lxp_result_registry_at(i, &values[i], &names[i]) ==
                       LXP_OK);
        LXP_CASE_CHECK(names[i] != NULL);
    }
    qsort(values, count, sizeof(values[0]), compare_result_values);
    qsort(names, count, sizeof(names[0]), compare_result_names);
    for (i = 1U; i < count; ++i) {
        LXP_CASE_CHECK(values[i - 1U] != values[i]);
        LXP_CASE_CHECK(strcmp(names[i - 1U], names[i]) != 0);
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "result_registry_enumerates_in_order",
          result_registry_enumerates_in_order },
        { "result_registry_values_unique", result_registry_values_unique }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
