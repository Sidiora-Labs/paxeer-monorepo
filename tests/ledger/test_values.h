#ifndef LAYERX_TEST_LEDGER_VALUES_H
#define LAYERX_TEST_LEDGER_VALUES_H

#include <inttypes.h>
#include <stdio.h>

static inline int test_value_compare(uintmax_t observed, uintmax_t expected,
                                      int equal, const char *expression,
                                      const char *file, int line)
{
    int matches = equal != 0 ? observed == expected : observed != expected;
    if (!matches)
        fprintf(stderr, "%s:%d: %s: expected %s%ju (0x%jx), observed %ju (0x%jx)\n",
                file, line, expression, equal != 0 ? "" : "anything except ",
                expected, expected, observed, observed);
    return matches;
}

#define TEST_EQ(observed, expected) \
    test_value_compare((uintmax_t)(observed), (uintmax_t)(expected), 1, \
                       #observed, __FILE__, __LINE__)
#define TEST_NE(observed, expected) \
    test_value_compare((uintmax_t)(observed), (uintmax_t)(expected), 0, \
                       #observed, __FILE__, __LINE__)

#endif
