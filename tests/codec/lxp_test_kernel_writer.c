#include "layerx/lxp_kernel_host.h"
#include "lxp_kernel_case.h"

#include <stdlib.h>

typedef struct writer_claim {
    lxp_kernel *kernel;
    pthread_mutex_t lock;
    pthread_cond_t condition;
    bool ready;
    bool release;
    lxp_result owner_before;
    lxp_result claim;
    lxp_result owner_after;
    bool writer_is_self;
} writer_claim;

static void *claim_on_thread(void *argument)
{
    writer_claim *attempt = argument;

    if (pthread_mutex_lock(&attempt->lock) != 0) abort();
    attempt->owner_before = lxp_state_writer_assert_owner(attempt->kernel->state);
    attempt->claim = lxp_kernel_claim_writer(attempt->kernel);
    attempt->owner_after = lxp_state_writer_assert_owner(attempt->kernel->state);
    attempt->writer_is_self =
        pthread_equal(attempt->kernel->state->writer, pthread_self()) != 0;
    attempt->ready = true;
    if (pthread_cond_signal(&attempt->condition) != 0) abort();
    while (!attempt->release) {
        if (pthread_cond_wait(&attempt->condition, &attempt->lock) != 0)
            abort();
    }
    if (pthread_mutex_unlock(&attempt->lock) != 0) abort();
    return NULL;
}

static int check_thread_claim(lxp_kernel *kernel, lxp_result expected_claim,
                               lxp_result expected_worker_owner,
                               lxp_result expected_original_owner)
{
    writer_claim attempt = { .kernel = kernel };
    pthread_t thread;
    lxp_result original_owner;
    bool writer_is_original;

    LXP_CASE_CHECK(pthread_mutex_init(&attempt.lock, NULL) == 0);
    LXP_CASE_CHECK(pthread_cond_init(&attempt.condition, NULL) == 0);
    LXP_CASE_CHECK(pthread_create(&thread, NULL, claim_on_thread, &attempt) == 0);
    LXP_CASE_CHECK(pthread_mutex_lock(&attempt.lock) == 0);
    while (!attempt.ready) {
        LXP_CASE_CHECK(pthread_cond_wait(&attempt.condition, &attempt.lock) == 0);
    }
    original_owner = lxp_state_writer_assert_owner(kernel->state);
    writer_is_original = pthread_equal(kernel->state->writer, pthread_self()) != 0;
    attempt.release = true;
    LXP_CASE_CHECK(pthread_cond_signal(&attempt.condition) == 0);
    LXP_CASE_CHECK(pthread_mutex_unlock(&attempt.lock) == 0);
    LXP_CASE_CHECK(pthread_join(thread, NULL) == 0);
    LXP_CASE_CHECK(pthread_cond_destroy(&attempt.condition) == 0);
    LXP_CASE_CHECK(pthread_mutex_destroy(&attempt.lock) == 0);

    LXP_CASE_CHECK(attempt.owner_before == LXP_FATAL_INVARIANT);
    LXP_CASE_CHECK(attempt.claim == expected_claim);
    LXP_CASE_CHECK(attempt.owner_after == expected_worker_owner);
    LXP_CASE_CHECK(original_owner == expected_original_owner);
    LXP_CASE_CHECK(attempt.writer_is_self == (expected_worker_owner == LXP_OK));
    LXP_CASE_CHECK(writer_is_original == (expected_original_owner == LXP_OK));
    return 0;
}

static int claim_writer_moves_owner(void)
{
    static lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal = { 0 };
    const uint8_t parameters = 0U;

    LXP_CASE_CHECK(lxp_state_store_init(&state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal,
                                     &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_writer_assert_owner(&state) == LXP_OK);
    LXP_CASE_CHECK(check_thread_claim(&kernel, LXP_OK, LXP_OK,
                                      LXP_FATAL_INVARIANT) == 0);
    LXP_CASE_CHECK(lxp_kernel_claim_writer(&kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_writer_assert_owner(&state) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

static int claim_writer_refuses_open_journal(void)
{
    static lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal = { 0 };
    const uint8_t parameters = 0U;
    const uint8_t key[32] = { 1U };
    const lxp_u128 value = { .lo = 7U, .hi = 0U };

    LXP_CASE_CHECK(lxp_state_store_init(&state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal,
                                     &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_open(&state, 1U, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, key, value) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_claim_writer(&kernel) == LXP_ERR_CONTEXT_MISMATCH);
    LXP_CASE_CHECK(check_thread_claim(&kernel, LXP_ERR_CONTEXT_MISMATCH,
                                      LXP_FATAL_INVARIANT, LXP_OK) == 0);
    LXP_CASE_CHECK(journal.open);
    LXP_CASE_CHECK(journal.store == &state);
    LXP_CASE_CHECK(journal.global_sequence == 1U);
    LXP_CASE_CHECK(journal.count == 1U);
    LXP_CASE_CHECK(memcmp(journal.staged[0].key, key, sizeof(key)) == 0);
    LXP_CASE_CHECK(journal.staged[0].value.lo == value.lo);
    LXP_CASE_CHECK(journal.staged[0].value.hi == value.hi);
    LXP_CASE_CHECK(state.next_sequence == 1U);
    LXP_CASE_CHECK(state.count == 0U);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&journal) == LXP_OK);
    LXP_CASE_CHECK(check_thread_claim(&kernel, LXP_OK, LXP_OK,
                                      LXP_FATAL_INVARIANT) == 0);
    LXP_CASE_CHECK(lxp_kernel_claim_writer(&kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_writer_assert_owner(&state) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "claim_writer_moves_owner", claim_writer_moves_owner },
        { "claim_writer_refuses_open_journal", claim_writer_refuses_open_journal }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
