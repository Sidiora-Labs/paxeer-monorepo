#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_state.h"
#include "lxp_kernel_case.h"

#include <stdlib.h>

#ifndef LXP_TEST_LIBRARY_PATH
#error "Makefile must supply LXP_TEST_LIBRARY_PATH from $(abspath $(LIBRARY))"
#endif

static FILE *retired_archive_symbols(void)
{
    static const char prefix[] = "nm -g --defined-only -- '";
    static const char archive[] = LXP_TEST_LIBRARY_PATH;
    char command[sizeof(prefix) + 4U * sizeof(archive) + 1U];
    size_t used = sizeof(prefix) - 1U;

    if (archive[0] == '\0') {
        (void)fputs("LXP_TEST_LIBRARY_PATH must name the linked archive\n", stderr);
        return NULL;
    }
    (void)memcpy(command, prefix, used);
    for (size_t i = 0U; archive[i] != '\0'; ++i) {
        if (archive[i] == '\'') {
            (void)memcpy(command + used, "'\\''", 4U);
            used += 4U;
        } else {
            command[used++] = archive[i];
        }
    }
    command[used++] = '\'';
    command[used] = '\0';
    return popen(command, "r");
}

extern void lxp_idempotency_lookup(void);
extern void lxp_idempotency_record(void);
extern void lxp_idempotency_canonical_lookup(void);
extern void lxp_idempotency_snapshot_encode(void);
extern void lxp_idempotency_snapshot_decode(void);
extern void lxp_idempotency_reserve(void);
extern void lxp_idempotency_prune(void);
extern void lxp_idempotency_append(void);
extern void lxp_idempotency_live_count(void);
extern void lxp_idempotency_can_commit(void);
extern void lxp_idempotency_commit_staged(void);

#pragma weak lxp_idempotency_lookup
#pragma weak lxp_idempotency_record
#pragma weak lxp_idempotency_canonical_lookup
#pragma weak lxp_idempotency_snapshot_encode
#pragma weak lxp_idempotency_snapshot_decode
#pragma weak lxp_idempotency_reserve
#pragma weak lxp_idempotency_prune
#pragma weak lxp_idempotency_append
#pragma weak lxp_idempotency_live_count
#pragma weak lxp_idempotency_can_commit
#pragma weak lxp_idempotency_commit_staged

static int idempotency_symbols_absent(void)
{
    FILE *symbols;
    char line[4096];
    int absent = 1;
    int journal_present = 0;
    int read_ok;

    LXP_CASE_CHECK(lxp_idempotency_lookup == NULL);
    LXP_CASE_CHECK(lxp_idempotency_record == NULL);
    LXP_CASE_CHECK(lxp_idempotency_canonical_lookup == NULL);
    LXP_CASE_CHECK(lxp_idempotency_snapshot_encode == NULL);
    LXP_CASE_CHECK(lxp_idempotency_snapshot_decode == NULL);
    LXP_CASE_CHECK(lxp_idempotency_reserve == NULL);
    LXP_CASE_CHECK(lxp_idempotency_prune == NULL);
    LXP_CASE_CHECK(lxp_idempotency_append == NULL);
    LXP_CASE_CHECK(lxp_idempotency_live_count == NULL);
    LXP_CASE_CHECK(lxp_idempotency_can_commit == NULL);
    LXP_CASE_CHECK(lxp_idempotency_commit_staged == NULL);

    symbols = retired_archive_symbols();
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
        if (strcmp(name, "lxp_state_journal_commit") == 0)
            journal_present = 1;
        if (strncmp(name, "lxp_idempotency_", 16U) == 0) {
            fprintf(stderr, "archive still defines %s\n", name);
            absent = 0;
        }
    }
    read_ok = !ferror(symbols);
    LXP_CASE_CHECK(pclose(symbols) == 0);
    LXP_CASE_CHECK(read_ok && absent && journal_present);
    return 0;
}

static int journal_commit_without_idempotency(void)
{
    lxp_state_store *store = malloc(sizeof(*store));
    lxp_state_journal journal;
    const uint8_t first[32] = { 0x31U };
    const uint8_t second[32] = { 0x32U };
    lxp_u128 value = { 0U, 0U };
    bool found = false;

    LXP_CASE_CHECK(store != NULL);
    (void)memset(store, 0xa5, sizeof(*store));
    LXP_CASE_CHECK(lxp_state_store_init(store, 7U) == LXP_OK);
    LXP_CASE_CHECK(store->count == 0U && store->next_sequence == 7U);
    LXP_CASE_CHECK(store->accounts == NULL && !store->account_root_required);
    LXP_CASE_CHECK(lxp_state_store_get(store, first, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(!found);
    LXP_CASE_CHECK(lxp_state_journal_open(store, 6U, &journal) ==
                   LXP_ERR_SEQUENCE_GAP);
    LXP_CASE_CHECK(lxp_state_journal_open(store, 7U, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, first,
        (lxp_u128){ 1U, 42U }) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, first,
        (lxp_u128){ 2U, 43U }) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, second,
        (lxp_u128){ 0U, 99U }) == LXP_OK);
    LXP_CASE_CHECK(journal.count == 2U && store->count == 0U);
    LXP_CASE_CHECK(lxp_state_store_get(store, first, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(!found);
    LXP_CASE_CHECK(lxp_state_journal_commit(&journal) == LXP_OK);
    LXP_CASE_CHECK(!journal.open);
    LXP_CASE_CHECK(store->count == 2U && store->next_sequence == 8U);
    LXP_CASE_CHECK(lxp_state_store_get(store, first, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(found && value.hi == 2U && value.lo == 43U);
    LXP_CASE_CHECK(lxp_state_store_get(store, second, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(found && value.hi == 0U && value.lo == 99U);
    LXP_CASE_CHECK(lxp_state_journal_commit(&journal) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(store->next_sequence == 8U);

    LXP_CASE_CHECK(lxp_state_journal_open(store, 8U, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, first,
        (lxp_u128){ 0U, 100U }) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&journal) == LXP_OK);
    LXP_CASE_CHECK(!journal.open && journal.count == 0U);
    LXP_CASE_CHECK(store->next_sequence == 8U && store->count == 2U);
    LXP_CASE_CHECK(lxp_state_store_get(store, first, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(found && value.hi == 2U && value.lo == 43U);
    LXP_CASE_CHECK(lxp_state_journal_open(store, 8U, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, first,
        (lxp_u128){ 0U, 100U }) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&journal) == LXP_OK);
    LXP_CASE_CHECK(store->count == 2U && store->next_sequence == 9U);
    LXP_CASE_CHECK(lxp_state_store_get(store, first, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(found && value.hi == 0U && value.lo == 100U);
    LXP_CASE_CHECK(lxp_state_journal_open(store, 9U, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&journal) == LXP_OK);
    LXP_CASE_CHECK(store->count == 2U && store->next_sequence == 10U);

    store->next_sequence = UINT64_MAX;
    LXP_CASE_CHECK(lxp_state_journal_open(store, UINT64_MAX, &journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_set(&journal, first,
        (lxp_u128){ 0U, 101U }) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&journal) ==
                   LXP_ERR_SEQUENCE_EXHAUSTED);
    LXP_CASE_CHECK(store->next_sequence == UINT64_MAX && journal.open);
    LXP_CASE_CHECK(lxp_state_store_get(store, first, &value, &found) == LXP_OK);
    LXP_CASE_CHECK(found && value.hi == 0U && value.lo == 100U);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_destroy(store) == LXP_OK);
    free(store);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "idempotency_symbols_absent", idempotency_symbols_absent },
        { "journal_commit_without_idempotency",
          journal_commit_without_idempotency }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
