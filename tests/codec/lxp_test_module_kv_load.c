#include "layerx/lxp_kernel_host.h"
#include "lxp_kernel_case.h"

static int next_permutation(size_t *indices, size_t count)
{
    size_t pivot = count - 1U;
    size_t last = count - 1U;
    size_t at;
    size_t temporary;

    while (pivot != 0U && indices[pivot - 1U] >= indices[pivot]) --pivot;
    if (pivot == 0U) return 0;
    while (indices[last] <= indices[pivot - 1U]) --last;
    temporary = indices[pivot - 1U];
    indices[pivot - 1U] = indices[last];
    indices[last] = temporary;
    for (at = count - 1U; pivot < at; ++pivot, --at) {
        temporary = indices[pivot];
        indices[pivot] = indices[at];
        indices[at] = temporary;
    }
    return 1;
}

static int module_kv_load_orders_permuted_input(void)
{
    static lxp_kernel kernel;
    const lxp_module_kv_entry expected[] = {
        { .module_id = 1U, .key_length = 1U, .key = { 0U },
          .value_length = 1U, .value = { 1U } },
        { .module_id = 1U, .key_length = 2U, .key = { 0U, 0U },
          .value_length = 1U, .value = { 2U } },
        { .module_id = 1U, .key_length = 2U, .key = { 0U, 255U },
          .value_length = 1U, .value = { 3U } },
        { .module_id = 1U, .key_length = 1U, .key = { 128U },
          .value_length = 1U, .value = { 4U } },
        { .module_id = 1U, .key_length = 1U, .key = { 255U },
          .value_length = 1U, .value = { 5U } },
        { .module_id = 256U, .key_length = 1U, .key = { 0U },
          .value_length = 1U, .value = { 6U } }
    };
    lxp_module_kv_entry rows[6];
    lxp_module_kv_entry original[6];
    size_t indices[] = { 0U, 1U, 2U, 3U, 4U, 5U };
    size_t permutations = 0U;
    size_t i;

    (void)memset(&kernel, 0, sizeof(kernel));
    do {
        for (i = 0U; i < 6U; ++i) rows[i] = expected[indices[i]];
        (void)memcpy(original, rows, sizeof(rows));
        LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 6U) == LXP_OK);
        LXP_CASE_CHECK(kernel.module_kv_count == 6U);
        LXP_CASE_CHECK(memcmp(rows, original, sizeof(rows)) == 0);
        for (i = 0U; i < 6U; ++i) {
            const lxp_module_kv_entry *entry = &kernel.module_kv[i];
            LXP_CASE_CHECK(entry->module_id == expected[i].module_id);
            LXP_CASE_CHECK(entry->key_length == expected[i].key_length);
            LXP_CASE_CHECK(entry->value_length == expected[i].value_length);
            LXP_CASE_CHECK(memcmp(entry->key, expected[i].key,
                                  entry->key_length) == 0);
            LXP_CASE_CHECK(memcmp(entry->value, expected[i].value,
                                  entry->value_length) == 0);
        }
        ++permutations;
    } while (next_permutation(indices, 6U));
    LXP_CASE_CHECK(permutations == 720U);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, kernel.module_kv,
                                            kernel.module_kv_count) == LXP_OK);
    LXP_CASE_CHECK(kernel.module_kv_count == 6U);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, expected, sizeof(expected)) == 0);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 1U) == LXP_OK);
    LXP_CASE_CHECK(kernel.module_kv_count == 1U);
    LXP_CASE_CHECK(memcmp(&kernel.module_kv[0], &rows[0], sizeof(rows[0])) == 0);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, NULL, 0U) == LXP_OK);
    LXP_CASE_CHECK(kernel.module_kv_count == 0U);
    return 0;
}

static int module_kv_load_rejects_duplicate(void)
{
    static lxp_kernel kernel;
    static lxp_module_kv_entry before[LXP_KERNEL_MAX_MODULE_KV];
    const lxp_module_kv_entry rows[] = {
        { .module_id = 2U, .key_length = 1U, .key = { 7U },
          .value_length = 1U, .value = { 1U } },
        { .module_id = 1U, .key_length = 1U, .key = { 7U } },
        { .module_id = 2U, .key_length = 1U, .key = { 7U },
          .value_length = 1U, .value = { 2U } }
    };

    (void)memset(&kernel, 0, sizeof(kernel));
    (void)memcpy(before, kernel.module_kv, sizeof(before));
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 3U) ==
                   LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(kernel.module_kv_count == 0U);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 2U) == LXP_OK);
    LXP_CASE_CHECK(kernel.module_kv_count == 2U);
    (void)memcpy(before, kernel.module_kv, sizeof(before));
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 3U) ==
                   LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(kernel.module_kv_count == 2U);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);
    return 0;
}

static int module_kv_load_enforces_bounds(void)
{
    static lxp_kernel kernel;
    static lxp_module_kv_entry rows[LXP_KERNEL_MAX_MODULE_KV + 1U];
    static lxp_module_kv_entry before[LXP_KERNEL_MAX_MODULE_KV];
    size_t i;

    (void)memset(&kernel, 0, sizeof(kernel));
    (void)memset(rows, 0, sizeof(rows));
    for (i = 0U; i < LXP_KERNEL_MAX_MODULE_KV; ++i) {
        rows[i].module_id = (uint16_t)(LXP_KERNEL_MAX_MODULE_KV - i);
        rows[i].key_length = LXP_MODULE_MAX_KEY_BYTES;
        rows[i].value_length = LXP_MODULE_MAX_VALUE_BYTES;
        (void)memset(rows[i].key, 255, sizeof(rows[i].key));
        (void)memset(rows[i].value, 128, sizeof(rows[i].value));
    }
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows,
                                            LXP_KERNEL_MAX_MODULE_KV) == LXP_OK);
    LXP_CASE_CHECK(kernel.module_kv_count == LXP_KERNEL_MAX_MODULE_KV);
    for (i = 0U; i < LXP_KERNEL_MAX_MODULE_KV; ++i)
        LXP_CASE_CHECK(memcmp(&kernel.module_kv[i],
                              &rows[LXP_KERNEL_MAX_MODULE_KV - 1U - i],
                              sizeof(rows[0])) == 0);
    (void)memcpy(before, kernel.module_kv, sizeof(before));

    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows,
                                            LXP_KERNEL_MAX_MODULE_KV + 1U) ==
                   LXP_ERR_LENGTH_LIMIT);
    LXP_CASE_CHECK(kernel.module_kv_count == LXP_KERNEL_MAX_MODULE_KV);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);

    rows[1].key_length = LXP_MODULE_MAX_KEY_BYTES + 1U;
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 2U) ==
                   LXP_ERR_LENGTH_LIMIT);
    LXP_CASE_CHECK(kernel.module_kv_count == LXP_KERNEL_MAX_MODULE_KV);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);
    rows[1].key_length = LXP_MODULE_MAX_KEY_BYTES;
    rows[1].value_length = LXP_MODULE_MAX_VALUE_BYTES + 1U;
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 2U) ==
                   LXP_ERR_LENGTH_LIMIT);
    LXP_CASE_CHECK(kernel.module_kv_count == LXP_KERNEL_MAX_MODULE_KV);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);

    rows[1].value_length = 0U;
    rows[1].key_length = 0U;
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 2U) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(kernel.module_kv_count == LXP_KERNEL_MAX_MODULE_KV);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(NULL, rows, 1U) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, NULL, 1U) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(kernel.module_kv_count == LXP_KERNEL_MAX_MODULE_KV);
    LXP_CASE_CHECK(memcmp(kernel.module_kv, before, sizeof(before)) == 0);
    rows[1].key_length = LXP_MODULE_MAX_KEY_BYTES;
    LXP_CASE_CHECK(lxp_kernel_module_kv_load(&kernel, rows, 2U) == LXP_OK);
    LXP_CASE_CHECK(kernel.module_kv_count == 2U);
    LXP_CASE_CHECK(kernel.module_kv[0].value_length == 0U);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "module_kv_load_orders_permuted_input",
          module_kv_load_orders_permuted_input },
        { "module_kv_load_rejects_duplicate", module_kv_load_rejects_duplicate },
        { "module_kv_load_enforces_bounds", module_kv_load_enforces_bounds }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
