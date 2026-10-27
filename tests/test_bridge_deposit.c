#include "layerx/lxp_bridge.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"

#include <string.h>

static lxp_result apply_capability(lxp_kernel *kernel,
                                   const lxp_transfer_set *set,
                                   lxp_receipt *receipt)
{
    lxp_transfer_set_result result;
    lxp_transfer_context context = set->context;
    lxp_result status;
    (void)kernel;
    status = lxp_apply_transfer_set((lxp_transfer_leg *)set->legs,
                                    set->leg_count, &context, &result);
    if (status == LXP_OK)
        (void)memcpy(receipt->transfer_set_root, result.transfer_set_root, 32U);
    return status;
}

static int unchanged(const lx_account *reserve, const lx_account *agent,
                     size_t staged)
{
    return reserve->balance.hi == 0U && reserve->balance.lo == 100U &&
           agent->balance.hi == 0U && agent->balance.lo == 0U &&
           staged == 0U;
}

int main(void)
{
    lx_asset_registry assets;
    lx_asset_record asset;
    lx_account_registry accounts;
    lx_account *reserve;
    lx_account *agent;
    lx_checkpoint_registry *checkpoints = NULL;
    lx_deposit_proof proof;
    lx_deposit_proof altered;
    lx_deposit_proof other;
    lx_asset_transfer_request transfer;
    lxp_bridge_deposit_context bridge;
    lxp_transfer_asset_state asset_state;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx module_ctx;
    lxp_receipt receipt;
    lxp_arena arena;
    uint8_t arena_bytes[4096];
    uint8_t first_nullifier[32];
    uint8_t second_nullifier[32];
    uint8_t leaf_hashes[2][32];
    uint8_t deposit_root[32];
    static const uint8_t paxeer_public_key[32] = {9U};
    uint64_t parameters = 1U;
    lxp_transfer_source_authority authority = {0};
    const char *reserve_name = "system:paxeer-reserve";
    const char *agent_name = "agent:did:key:a:main";

    (void)memset(&asset, 0, sizeof(asset));
    asset.asset_id[0] = 1U;
    (void)memcpy(asset.symbol, "A", 2U);
    asset.symbol_length = 1U;
    asset.decimals = 6U;
    asset.name[0] = (uint8_t)'A';
    asset.name_length = 1U;
    asset.issuer_kind = 2U;
    asset.issuer_did32[0] = 1U;
    asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    asset.custody_reference[0] = 5U;
    asset.custody_reference_length = 32U;
    if (lx_asset_registry_init(&assets, 0U) != LXP_OK ||
        lx_asset_register(&assets, &asset, 0U, (lxp_u128){ 0U, 0U }) != LXP_OK ||
        lx_account_registry_init(&accounts) != LXP_OK ||
        lx_asset_account_open(&assets, &accounts, asset.asset_id,
                              (const uint8_t *)reserve_name,
                              strlen(reserve_name), 1U, LX_ACCOUNT_OPEN_GENESIS,
                              NULL, &reserve) != LXP_OK ||
        lx_asset_account_open(&assets, &accounts, asset.asset_id,
                              (const uint8_t *)agent_name, strlen(agent_name),
                              1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &agent) != LXP_OK ||
        lxp_ledger_bootstrap_balance(reserve, asset.asset_id,
                                     (lxp_u128){ 0U, 100U }, 0U) != LXP_OK ||
        lx_asset_transfer_state(&asset, &asset_state) != LXP_OK ||
        lxp_state_store_init(&state, 0U) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, lx_asset_module_iface()) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL, apply_capability) != LXP_OK ||
        lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK ||
        lxp_module_ctx_init(&module_ctx, &kernel, LXP_MODULE_ASSET, 10U, 0U,
                            1U, 1000U, &arena, true) != LXP_OK) return 1;

    (void)memset(&proof, 0, sizeof(proof));
    proof.deposit_id[0] = 4U;
    proof.custody_reference[0] = 5U;
    (void)memcpy(proof.asset_id, asset.asset_id, 32U);
    proof.amount = (lxp_u128){ 0U, 25U };
    proof.checkpoint_id[0] = 2U;
    proof.network_id = 7U;
    proof.protocol_version = LXP_PROTOCOL_VERSION;
    other = proof;
    other.deposit_id[0] = 6U;
    other.amount.lo = 10U;
    if (lx_paxeer_deposit_leaf_hash(&proof, leaf_hashes[0]) != LXP_OK ||
        lx_paxeer_deposit_leaf_hash(&other, leaf_hashes[1]) != LXP_OK ||
        lxp_merkle_proof_generate(
            (const uint8_t (*)[32])leaf_hashes, 2U, 0U, &arena,
            &proof.inclusion_proof, deposit_root) != LXP_OK)
        return 1;
    if (lx_checkpoint_registry_create(
            paxeer_public_key, 7U, LXP_PROTOCOL_VERSION,
            &checkpoints) != LXP_OK ||
        lxp_deposit_nullifier(&proof, first_nullifier) != LXP_OK ||
        lxp_deposit_nullifier(&proof, second_nullifier) != LXP_OK ||
        memcmp(first_nullifier, second_nullifier, 32U) != 0) return 1;

    (void)memset(&transfer, 0, sizeof(transfer));
    transfer.from = reserve;
    transfer.to = agent;
    transfer.asset = &asset;
    transfer.amount = proof.amount;
    transfer.context.assets = &asset_state;
    transfer.context.asset_count = 1U;
    transfer.context.protocol_system_capability = true;
    (void)memcpy(authority.authorized_from, reserve->id, 32U);
    authority.debit_authority_kind = LXP_AUTH_PROTOCOL_MODULE;
    authority.protocol_system_capability = true;
    transfer.context.debit_authority_kind = LXP_AUTH_PROTOCOL_MODULE;
    transfer.context.source_authorities = &authority;
    transfer.context.source_authority_count = 1U;
    bridge = (lxp_bridge_deposit_context){
        &module_ctx, &assets, &accounts, checkpoints, 7U,
        LXP_PROTOCOL_VERSION
    };

    if (lxp_deposit_proof_verify(NULL, checkpoints, 7U,
                                 LXP_PROTOCOL_VERSION) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        lxp_bridge_deposit_credit(&bridge, &transfer, NULL, &receipt) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.network_id = 8U;
    if (lxp_deposit_proof_verify(&altered, checkpoints, 7U,
                                 LXP_PROTOCOL_VERSION) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.protocol_version = (uint16_t)(LXP_PROTOCOL_VERSION + 1U);
    if (lxp_deposit_proof_verify(&altered, checkpoints, 7U,
                                 LXP_PROTOCOL_VERSION) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.inclusion_proof.leaf_count = 1U;
    altered.inclusion_proof.leaf_index = 0U;
    altered.inclusion_proof.depth = 0U;
    if (lxp_bridge_deposit_credit(&bridge, &transfer, &altered, &receipt) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.inclusion_proof.siblings[0][0] ^= 0xffU;
    if (lxp_bridge_deposit_credit(&bridge, &transfer, &altered, &receipt) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.inclusion_proof.leaf_index = 1U;
    if (lxp_bridge_deposit_credit(&bridge, &transfer, &altered, &receipt) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.checkpoint_id[0] ^= 0xffU;
    if (lxp_bridge_deposit_credit(&bridge, &transfer, &altered, &receipt) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    altered = proof;
    altered.custody_reference[0] ^= 0xffU;
    if (lxp_bridge_deposit_credit(&bridge, &transfer, &altered, &receipt) !=
            LXP_ERR_DEPOSIT_PROOF_NOT_FINAL ||
        !unchanged(reserve, agent, module_ctx.staged_count)) return 1;
    if (lx_checkpoint_registry_destroy(&checkpoints) != LXP_OK ||
        lxp_state_store_destroy(&state) != LXP_OK) return 1;
    return 0;
}
