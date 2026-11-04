#ifndef REPAIR244_KERNEL_H
#define REPAIR244_KERNEL_H
#include <stddef.h>
#include <stdint.h>
int repair244_init(unsigned native);
void repair244_close(void);
int repair244_info(unsigned which, uint8_t id[32], uint8_t asset[32], uint8_t authority[32], uint64_t *balance);
int repair244_root(uint8_t root[32]);
int repair244_proof(unsigned module, const uint8_t *key, size_t keylen, uint8_t *out, size_t capacity, size_t *length);
int repair244_budget(unsigned native, const uint8_t *payload, size_t length);
int repair244_escrow(const uint8_t *payload, size_t length);
int repair244_transfer(const uint8_t from[32], const uint8_t asset[32], const uint8_t to[32], const uint8_t amount[16]);
#endif
