#include "layerx/lxp_merkle.h"
#include "lxp_kernel_case.h"

#include <string.h>

static int merkle_node_hash_domain_separated(void)
{
    uint8_t left[32], right[32], concatenated[64];
    uint8_t node[32], reversed[32], leaf[32];
    size_t i;

    for (i = 0U; i < sizeof(left); ++i) {
        left[i] = (uint8_t)i;
        right[i] = (uint8_t)(255U - i);
    }
    (void)memcpy(concatenated, left, sizeof(left));
    (void)memcpy(concatenated + sizeof(left), right, sizeof(right));

    LXP_CASE_CHECK(lxp_merkle_node_hash(left, right, node) == LXP_OK);
    LXP_CASE_CHECK(lxp_merkle_leaf_hash(concatenated, sizeof(concatenated),
                                       leaf) == LXP_OK);
    LXP_CASE_CHECK(memcmp(node, leaf, sizeof(node)) != 0);
    LXP_CASE_CHECK(lxp_merkle_node_hash(right, left, reversed) == LXP_OK);
    LXP_CASE_CHECK(memcmp(node, reversed, sizeof(node)) != 0);
    return 0;
}

static int merkle_proof_encode_canonical(void)
{
    enum { LEAF_COUNT = 9, ENCODED_HEADER_SIZE = 17 };
    static const uint8_t depths[LEAF_COUNT] = {
        0U, 1U, 2U, 2U, 3U, 3U, 3U, 3U, 4U
    };
    uint8_t memory[4096];
    uint8_t hashes[LEAF_COUNT][32];
    lxp_arena arena;
    size_t variant, count, index;

    for (variant = 0U; variant < 2U; ++variant) {
        for (index = 0U; index < LEAF_COUNT; ++index) {
            uint8_t value[3] = {
                (uint8_t)variant, (uint8_t)index, (uint8_t)(index * index)
            };
            LXP_CASE_CHECK(lxp_merkle_leaf_hash(value, sizeof(value),
                                               hashes[index]) == LXP_OK);
        }
        for (count = 1U; count <= LEAF_COUNT; ++count) {
            for (index = 0U; index < count; ++index) {
                lxp_merkle_proof proof;
                lxp_codec_writer first, second;
                lxp_codec_reader reader;
                lxp_byte_span siblings;
                uint8_t root[32], depth;
                uint32_t leaf_index, leaf_count;
                size_t sibling_bytes = (size_t)depths[count - 1U] * 32U;
                size_t encoded_length = ENCODED_HEADER_SIZE + sibling_bytes;

                LXP_CASE_CHECK(lxp_arena_init(&arena, memory,
                                              sizeof(memory)) == LXP_OK);
                LXP_CASE_CHECK(lxp_merkle_proof_generate(
                    (const uint8_t (*)[32])hashes, count, index, &arena,
                    &proof, root) == LXP_OK);
                LXP_CASE_CHECK(proof.leaf_index == index);
                LXP_CASE_CHECK(proof.leaf_count == count);
                LXP_CASE_CHECK(proof.depth == depths[count - 1U]);
                LXP_CASE_CHECK(lxp_merkle_proof_verify(hashes[index], &proof,
                                                       root) == LXP_OK);

                LXP_CASE_CHECK(lxp_codec_writer_init(&first, &arena,
                                                     encoded_length) == LXP_OK);
                LXP_CASE_CHECK(lxp_codec_writer_init(&second, &arena,
                                                     encoded_length) == LXP_OK);
                (void)memset(first.bytes, 0xa5, first.capacity);
                (void)memset(second.bytes, 0x5a, second.capacity);
                LXP_CASE_CHECK(lxp_merkle_proof_encode(&first, &proof) == LXP_OK);
                LXP_CASE_CHECK(lxp_merkle_proof_encode(&second, &proof) == LXP_OK);
                LXP_CASE_CHECK(first.length == encoded_length);
                LXP_CASE_CHECK(second.length == encoded_length);
                LXP_CASE_CHECK(memcmp(first.bytes, second.bytes,
                                      encoded_length) == 0);

                LXP_CASE_CHECK(lxp_codec_reader_init(&reader, first.bytes,
                                                     first.length) == LXP_OK);
                LXP_CASE_CHECK(lxp_codec_read_struct_header(&reader,
                                                            0x4d50U) == LXP_OK);
                LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &leaf_index) == LXP_OK);
                LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &leaf_count) == LXP_OK);
                LXP_CASE_CHECK(lxp_codec_read_u8(&reader, &depth) == LXP_OK);
                LXP_CASE_CHECK(leaf_index == index);
                LXP_CASE_CHECK(leaf_count == count);
                LXP_CASE_CHECK(depth == depths[count - 1U]);
                LXP_CASE_CHECK(lxp_codec_read_bytes(
                    &reader, &siblings, LXP_MERKLE_MAX_DEPTH * 32U) == LXP_OK);
                LXP_CASE_CHECK(siblings.length == sibling_bytes);
                LXP_CASE_CHECK(memcmp(siblings.bytes, proof.siblings,
                                      sibling_bytes) == 0);
                LXP_CASE_CHECK(lxp_codec_finish(&reader) == LXP_OK);
            }
        }
    }
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "merkle_node_hash_domain_separated",
          merkle_node_hash_domain_separated },
        { "merkle_proof_encode_canonical", merkle_proof_encode_canonical }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                               argc, argv);
}
