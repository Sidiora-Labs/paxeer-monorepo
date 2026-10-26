#ifndef LXP_KERNEL_CASE_H
#define LXP_KERNEL_CASE_H

#include <stddef.h>
#include <stdio.h>
#include <string.h>

typedef struct lxp_kernel_case {
    const char *name;
    int (*run)(void);
} lxp_kernel_case;

#define LXP_CASE_CHECK(condition) \
    do { \
        if (!(condition)) { \
            fprintf(stderr, "%s:%d: check failed: %s\n", \
                    __FILE__, __LINE__, #condition); \
            return 1; \
        } \
    } while (0)

static int lxp_kernel_case_main(const lxp_kernel_case *cases, size_t count,
                               int argc, char **argv)
{
    int arg;
    size_t i;

    if (argc <= 1) {
        fprintf(stderr, "No case named\n");
        return 2;
    }
    for (arg = 1; arg < argc; ++arg) {
        for (i = 0U; i < count; ++i)
            if (strcmp(argv[arg], cases[i].name) == 0) break;
        if (i == count) {
            fprintf(stderr, "Unknown case: %s\n", argv[arg]);
            return 2;
        }
    }
    for (arg = 1; arg < argc; ++arg) {
        for (i = 0U; i < count; ++i) {
            if (strcmp(argv[arg], cases[i].name) == 0) {
                if (cases[i].run() != 0) return 1;
                break;
            }
        }
    }
    return 0;
}

#endif
