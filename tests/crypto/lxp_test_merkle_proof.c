#include "layerx/lxp_merkle.h"

#include <string.h>

static int proof_transport_round_trip(const uint8_t leaf_hash[32],
                                     const lxp_merkle_proof *proof,
                                     const uint8_t root[32])
{
    uint8_t memory[2048];
    lxp_arena arena;
    lxp_codec_writer writer;
    lxp_codec_reader reader;
    lxp_byte_span siblings;
    lxp_merkle_proof decoded;
    if (lxp_arena_init(&arena, memory, sizeof(memory)) != LXP_OK ||
        lxp_codec_writer_init(&writer, &arena, sizeof(memory)) != LXP_OK ||
        lxp_merkle_proof_encode(&writer, proof) != LXP_OK ||
        writer.length != 17U + (size_t)proof->depth * 32U ||
        lxp_codec_reader_init(&reader, writer.bytes, writer.length) != LXP_OK ||
        lxp_codec_read_struct_header(&reader, 0x4d50U) != LXP_OK ||
        lxp_codec_read_u32(&reader, &decoded.leaf_index) != LXP_OK ||
        lxp_codec_read_u32(&reader, &decoded.leaf_count) != LXP_OK ||
        lxp_codec_read_u8(&reader, &decoded.depth) != LXP_OK ||
        lxp_codec_read_bytes(&reader, &siblings,
                             LXP_MERKLE_MAX_DEPTH * 32U) != LXP_OK ||
        lxp_codec_finish(&reader) != LXP_OK ||
        decoded.leaf_index != proof->leaf_index ||
        decoded.leaf_count != proof->leaf_count ||
        decoded.depth != proof->depth ||
        siblings.length != (size_t)proof->depth * 32U ||
        memcmp(siblings.bytes, proof->siblings, siblings.length) != 0)
        return 1;
    (void)memcpy(decoded.siblings, siblings.bytes, siblings.length);
    return lxp_merkle_proof_verify(leaf_hash, &decoded, root) == LXP_OK ? 0 : 1;
}

static int encode_rejected_without_write(lxp_codec_writer *writer,
                                         const lxp_merkle_proof *proof)
{
    uint8_t before[2048];
    uint8_t *bytes = writer->bytes;
    size_t capacity = writer->capacity;
    size_t length = writer->length;
    if (capacity > sizeof(before)) return 1;
    (void)memcpy(before, bytes, capacity);
    return lxp_merkle_proof_encode(writer, proof) != LXP_ERR_NON_CANONICAL ||
           writer->bytes != bytes || writer->capacity != capacity ||
           writer->length != length || memcmp(bytes, before, capacity) != 0;
}

static int test_merkle_proof_encode_rejects_invalid_metadata(void)
{
    static const uint32_t invalid_counts[] = {0U, 0U, 1U, 1U, 3U, 3U};
    static const uint32_t invalid_indices[] = {
        0U, UINT32_MAX, 1U, UINT32_MAX, 3U, UINT32_MAX
    };
    uint8_t memory[2048];
    uint8_t leaf_hash[32];
    lxp_arena arena;
    lxp_codec_writer writer;
    lxp_merkle_proof proof = {0};
    size_t i;
    if (lxp_arena_init(&arena, memory, sizeof(memory)) != LXP_OK ||
        lxp_codec_writer_init(&writer, &arena, sizeof(memory)) != LXP_OK ||
        lxp_merkle_leaf_hash("single", 6U, leaf_hash) != LXP_OK)
        return 1;
    (void)memset(writer.bytes, 0xa5, writer.capacity);
    if (encode_rejected_without_write(&writer, NULL) != 0) return 1;
    for (i = 0U; i < sizeof(invalid_counts) / sizeof(invalid_counts[0]); ++i) {
        proof.leaf_count = invalid_counts[i];
        proof.leaf_index = invalid_indices[i];
        proof.depth = proof.leaf_count == 3U ? 2U : 0U;
        if (encode_rejected_without_write(&writer, &proof) != 0 ||
            lxp_merkle_proof_verify(leaf_hash, &proof, leaf_hash) !=
                LXP_ERR_NON_CANONICAL)
            return 1;
    }
    if (lxp_codec_write_u8(&writer, 0x5aU) != LXP_OK) return 1;
    for (i = 0U; i < sizeof(invalid_counts) / sizeof(invalid_counts[0]); ++i) {
        proof.leaf_count = invalid_counts[i];
        proof.leaf_index = invalid_indices[i];
        proof.depth = proof.leaf_count == 3U ? 2U : 0U;
        if (encode_rejected_without_write(&writer, &proof) != 0) return 1;
    }
    proof.leaf_count = 1U;
    proof.leaf_index = 0U;
    proof.depth = 0U;
    if (proof_transport_round_trip(leaf_hash, &proof, leaf_hash) != 0) return 1;
    for (i = 1U; i <= LXP_MERKLE_MAX_DEPTH + 1U; ++i) {
        proof.depth = (uint8_t)i;
        if (encode_rejected_without_write(&writer, &proof) != 0 ||
            lxp_merkle_proof_verify(leaf_hash, &proof, leaf_hash) !=
                LXP_ERR_NON_CANONICAL)
            return 1;
    }
    return 0;
}

static int test_merkle_proof_uint32_boundaries(void)
{
    static const struct {
        uint32_t count;
        uint8_t depth;
    } cases[] = {
        {UINT32_MAX, 32U},
        {UINT32_MAX - 1U, 32U},
        {UINT32_MAX - 2U, 32U},
        {UINT32_MAX / 2U, 31U},
        {UINT32_MAX / 2U + 1U, 31U},
        {UINT32_MAX / 2U + 2U, 32U}
    };
    uint8_t memory[2048];
    uint8_t leaf_hash[32];
    uint8_t repeated_leaf[32];
    lxp_arena arena;
    size_t c;
    if (lxp_arena_init(&arena, memory, sizeof(memory)) != LXP_OK ||
        lxp_merkle_leaf_hash("last", 4U, leaf_hash) != LXP_OK ||
        lxp_merkle_leaf_hash("repeated", 8U, repeated_leaf) != LXP_OK)
        return 1;
    for (c = 0U; c < sizeof(cases) / sizeof(cases[0]); ++c) {
        lxp_merkle_proof proof = {0};
        lxp_codec_writer writer;
        uint8_t current[32];
        uint8_t repeated[32];
        uint64_t count = cases[c].count;
        uint64_t index = count - 1U;
        size_t i;
        proof.leaf_count = cases[c].count;
        proof.leaf_index = cases[c].count - 1U;
        (void)memcpy(current, leaf_hash, sizeof(current));
        (void)memcpy(repeated, repeated_leaf, sizeof(repeated));
        /* All leaves but the last are identical: fold their complete subtrees
           with real two-leaf producers, using constant storage at each level. */
        while (count > 1U) {
            uint8_t pair[2][32];
            lxp_merkle_proof step;
            size_t position = (size_t)(index & 1U);
            const uint8_t *sibling = (index ^ 1U) >= count ? current : repeated;
            (void)memcpy(pair[position], current, 32U);
            (void)memcpy(pair[position ^ 1U], sibling, 32U);
            if (lxp_merkle_proof_generate((const uint8_t (*)[32])pair,
                                          2U, position, &arena, &step,
                                          current) != LXP_OK ||
                step.depth != 1U ||
                lxp_merkle_proof_verify(pair[position], &step, current) != LXP_OK)
                return 1;
            if (proof.depth >= LXP_MERKLE_MAX_DEPTH) return 1;
            (void)memcpy(proof.siblings[proof.depth], step.siblings[0], 32U);
            ++proof.depth;
            (void)memcpy(pair[0], repeated, 32U);
            (void)memcpy(pair[1], repeated, 32U);
            if (lxp_merkle_proof_generate((const uint8_t (*)[32])pair,
                                          2U, 0U, &arena, &step,
                                          repeated) != LXP_OK)
                return 1;
            count = (count + 1U) / 2U;
            index /= 2U;
        }
        if (proof.depth != cases[c].depth ||
            lxp_merkle_proof_verify(leaf_hash, &proof, current) != LXP_OK ||
            proof_transport_round_trip(leaf_hash, &proof, current) != 0)
            return 1;
        proof.siblings[proof.depth - 1U][0] ^= 1U;
        if (lxp_merkle_proof_verify(leaf_hash, &proof, current) !=
            LXP_ERR_ROOT_MISMATCH)
            return 1;
        proof.siblings[proof.depth - 1U][0] ^= 1U;
        if (lxp_codec_writer_init(&writer, &arena, sizeof(memory)) != LXP_OK)
            return 1;
        (void)memset(writer.bytes, 0xa5, writer.capacity);
        if (lxp_codec_write_u8(&writer, 0x5aU) != LXP_OK) return 1;
        for (i = 0U; i <= LXP_MERKLE_MAX_DEPTH + 1U; ++i) {
            if (i == cases[c].depth) continue;
            proof.depth = (uint8_t)i;
            if (encode_rejected_without_write(&writer, &proof) != 0 ||
                lxp_merkle_proof_verify(leaf_hash, &proof, current) !=
                    LXP_ERR_NON_CANONICAL)
                return 1;
        }
        if (lxp_arena_reset(&arena, 0U) != LXP_OK) return 1;
    }
    return 0;
}

int main(void)
{
    enum { LEAVES = 1000 };
    uint8_t arena_memory[131072];
    uint8_t hashes[LEAVES][32];
    uint8_t root[32];
    lxp_arena arena;
    lxp_merkle_proof proof;
    size_t i;
    if (test_merkle_proof_uint32_boundaries() != 0 ||
        test_merkle_proof_encode_rejects_invalid_metadata() != 0)
        return 1;
    for (i = 0U; i < LEAVES; ++i) {
        uint8_t value[8];
        size_t j;
        for (j = 0U; j < sizeof(value); ++j)
            value[sizeof(value)-1U-j]=(uint8_t)((uint64_t)i>>(j*8U));
        if (lxp_merkle_leaf_hash(value,sizeof(value),hashes[i])!=LXP_OK) return 1;
    }
    if (lxp_arena_init(&arena,arena_memory,sizeof(arena_memory))!=LXP_OK) return 1;
    for (i = 0U; i < LEAVES; ++i) {
        if (lxp_merkle_proof_generate((const uint8_t (*)[32])hashes,LEAVES,i,&arena,&proof,root)!=LXP_OK ||
            lxp_merkle_proof_verify(hashes[i],&proof,root)!=LXP_OK) return 1;
    }
    if (lxp_merkle_proof_generate((const uint8_t (*)[32])hashes,LEAVES,517U,&arena,&proof,root)!=LXP_OK)
        return 1;
    if (proof_transport_round_trip(hashes[517], &proof, root) != 0) return 1;
    for (i = 0U; i < (size_t)proof.depth*32U*8U; ++i) {
        uint8_t *bytes=(uint8_t *)proof.siblings;
        bytes[i/8U]^=(uint8_t)(1U<<(i&7U));
        if (lxp_merkle_proof_verify(hashes[517],&proof,root)==LXP_OK) return 1;
        bytes[i/8U]^=(uint8_t)(1U<<(i&7U));
    }
    proof.depth=(uint8_t)(proof.depth+1U);
    return lxp_merkle_proof_verify(hashes[517],&proof,root)==LXP_ERR_NON_CANONICAL ? 0 : 1;
}
