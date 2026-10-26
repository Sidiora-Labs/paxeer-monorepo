#include "layerx/lxp_kernel_host.h"

#include <stdlib.h>
#include <string.h>

lxp_result lxp_kernel_claim_writer(lxp_kernel *kernel)
{
    lxp_result status = LXP_OK;

    if (kernel == NULL || kernel->state == NULL || kernel->journal == NULL)
        return LXP_ERR_NON_CANONICAL;
    if (pthread_mutex_lock(&kernel->state->lock) != 0) return LXP_ERR_IO;
    if (kernel->journal->open)
        status = LXP_ERR_CONTEXT_MISMATCH;
    else
        kernel->state->writer = pthread_self();
    if (pthread_mutex_unlock(&kernel->state->lock) != 0)
        return LXP_FATAL_INVARIANT;
    return status;
}

static int kv_order(const void *left_row, const void *right_row)
{
    const lxp_module_kv_entry *left = left_row;
    const lxp_module_kv_entry *right = right_row;
    size_t common;
    int order;

    if (left->module_id != right->module_id)
        return left->module_id < right->module_id ? -1 : 1;
    common = left->key_length < right->key_length ?
        left->key_length : right->key_length;
    order = memcmp(left->key, right->key, common);
    if (order != 0) return order;
    return left->key_length < right->key_length ? -1 :
        left->key_length != right->key_length;
}

lxp_result lxp_kernel_module_kv_load(lxp_kernel *kernel,
                                     const lxp_module_kv_entry *rows,
                                     size_t count)
{
    lxp_module_kv_entry *scratch;
    size_t i;

    if (count > LXP_KERNEL_MAX_MODULE_KV) return LXP_ERR_LENGTH_LIMIT;
    if (kernel == NULL || (rows == NULL && count != 0U))
        return LXP_ERR_NON_CANONICAL;
    for (i = 0U; i < count; ++i) {
        if (rows[i].key_length > LXP_MODULE_MAX_KEY_BYTES ||
            rows[i].value_length > LXP_MODULE_MAX_VALUE_BYTES)
            return LXP_ERR_LENGTH_LIMIT;
        if (rows[i].key_length == 0U) return LXP_ERR_NON_CANONICAL;
    }
    if (count == 0U) {
        kernel->module_kv_count = 0U;
        return LXP_OK;
    }
    scratch = malloc(count * sizeof(*scratch));
    if (scratch == NULL) return LXP_ERR_IO;
    (void)memcpy(scratch, rows, count * sizeof(*scratch));
    qsort(scratch, count, sizeof(*scratch), kv_order);
    for (i = 1U; i < count; ++i) {
        if (kv_order(&scratch[i - 1U], &scratch[i]) == 0) {
            free(scratch);
            return LXP_ERR_SEQUENCE_REUSED;
        }
    }
    (void)memcpy(kernel->module_kv, scratch, count * sizeof(*scratch));
    kernel->module_kv_count = count;
    free(scratch);
    return LXP_OK;
}
