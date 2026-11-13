/* A committed codec fixture, not an additional host module registration.
 * Each of twelve module subtrees has key 01 and value <module>01a5.
 * The production C encoder, decoder and verifier qualify every emitted wire.
 */
static void fixture_leaf(const uint8_t *key, uint32_t key_length,
                         const uint8_t *value, uint32_t value_length,
                         uint8_t out[32])
{
    uint8_t material[8U + 32U + 32U];
    assert(key_length <= 32U && value_length <= 32U);
    for (size_t i = 0U; i < 4U; ++i) {
        material[i] = (uint8_t)(key_length >> (8U * (3U - i)));
        material[i + 4U] = (uint8_t)(value_length >> (8U * (3U - i)));
    }
    memcpy(material + 8U, key, key_length);
    memcpy(material + 8U + key_length, value, value_length);
    assert(lxp_hash_domain(LXP_DOMAIN_STATE_LEAF, material,
        8U + key_length + value_length, out) == LXP_OK);
}

static void compatibility_fixture(void)
{
    bool first = true;
    printf("{\"max_module\":%u,\"vectors\":[", LXP_MODULE_RESERVED_COUNT);
    for (uint16_t module = 10U; module <= 11U; ++module) {
        uint8_t nodes[12][32], root[32];
        lxp_state_witness *proof = calloc(1U, sizeof(*proof));
        assert(proof != NULL);
        proof->version = LXP_STATE_WITNESS_VERSION;
        proof->module_id = module;
        proof->key_length = 1U;
        proof->key[0] = 1U;
        proof->value_length = 3U;
        proof->value[0] = (uint8_t)module;
        proof->value[1] = 1U;
        proof->value[2] = 0xa5U;
        proof->layer_a.leaf_count = 1U;
        proof->layer_b.leaf_count = 12U;
        proof->layer_b.leaf_index = module;
        for (uint16_t i = 0U; i < 12U; ++i) {
            uint8_t key = 1U, value[3] = {(uint8_t)i, 1U, 0xa5U};
            uint8_t id[2] = {0U, (uint8_t)i}, subtree[32];
            fixture_leaf(&key, 1U, value, sizeof(value), subtree);
            fixture_leaf(id, sizeof(id), subtree, sizeof(subtree), nodes[i]);
        }
        size_t count = 12U, index = module;
        while (count > 1U) {
            size_t sibling = index ^ 1U;
            if (sibling >= count) sibling = index;
            memcpy(proof->layer_b.siblings[proof->layer_b.depth++], nodes[sibling], 32U);
            for (size_t i = 0U; i < count; i += 2U) {
                uint8_t pair[64];
                memcpy(pair, nodes[i], 32U);
                memcpy(pair + 32U, nodes[i + 1U < count ? i + 1U : i], 32U);
                assert(lxp_hash_domain(LXP_DOMAIN_STATE_NODE, pair, sizeof(pair),
                    nodes[i / 2U]) == LXP_OK);
            }
            count = count / 2U + count % 2U;
            index /= 2U;
        }
        memcpy(root, nodes[0], sizeof(root));
        emit_witness(proof, root, module == 10U ? "reserved_width_spot" : "reserved_width_web", &first);
        free(proof);
    }
    printf("]}\n");
}
