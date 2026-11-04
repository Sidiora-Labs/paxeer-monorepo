#include "kernel.h"
/* Reuse the canonical kernel fixture and signed-activity producer. No substitute
 * keeper, ledger or state-proof implementation is involved. */
#define main repair244_upstream_budget_test_main
#include "../../../../../../tests/modules/test_budget_create.c"
#undef main
#include "layerx/lx_escrow.h"

static lx_account *repair_source;
static lx_account *repair_escrow;
static lx_asset_registry repair_assets;
static lx_escrow_runtime repair_escrow_runtime;

int repair244_init(unsigned native) {
    uint8_t public_key[32], signature[64], asset_bytes[512], arena_bytes[4096];
    char source_name[512], asset_hex[65];
    const char digits[] = "0123456789abcdef";
    size_t length;
    lxp_arena arena;
    lxp_module_ctx ctx;
    if (env_init() != 0) return -1;
    if (sign_raw(owner_seed, NULL, 0U, signature, public_key) != 0) return -2;
    env.asset.symbol_length = 3U; memcpy(env.asset.symbol,"TST",4U);
    env.asset.name_length = 5U; memcpy(env.asset.name,"Asset",5U);
    env.asset.issuer_kind = 2U; env.asset.issuer_did32[0] = 1U;
    env.asset.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    env.asset.custody_reference[0] = 1U; env.asset.custody_reference_length = 1U;
    env.asset.total_units.lo = 3000U;
    for (size_t i=0; i<32; ++i) {
        asset_hex[2*i]=digits[env.asset.asset_id[i]>>4];
        asset_hex[2*i+1]=digits[env.asset.asset_id[i]&15];
    }
    asset_hex[64]=0;
    snprintf(source_name,sizeof(source_name),"agent:%s:asset:%s",owner_did,asset_hex);
    if (open_account(source_name,1000U,public_key,&repair_source) != 0 ||
        open_account("agent:did:key:owner:escrow:e1",0U,NULL,&repair_escrow) != 0 ||
        open_account("agent:did:key:owner:budget:8200000000000000000000000000000000000000000000000000000000000000",0U,NULL,&env.budget_account) != 0) return -3;
    /* Native payment resolution requires exactly one owned account per asset.
     * Match the canonical native-source fixture: the main account holds a
     * different asset while the asset account supplies the budget's funds. */
    if (native) {
        uint8_t main_asset[32];
        memcpy(main_asset, env.asset.asset_id, 32U);
        main_asset[0] ^= 0x80U;
        if (lxp_ledger_bootstrap_balance(env.owner, main_asset,
                (lxp_u128){0U,1000U}, 0U) != LXP_OK) return -7;
    }
    if (lxp_kernel_register_module(&env.kernel,lx_asset_module_iface()) != LXP_OK ||
        lxp_kernel_register_module(&env.kernel,lx_escrow_module_iface()) != LXP_OK ||
        lx_asset_registry_init(&repair_assets,0U) != LXP_OK ||
        lx_asset_register(&repair_assets,&env.asset,0U,(lxp_u128){0U,3000U}) != LXP_OK) return -4;
    repair_escrow_runtime.accounts=&env.accounts; repair_escrow_runtime.assets=&repair_assets;
    if (lxp_kernel_bind_module_runtime(&env.kernel,LXP_MODULE_ESCROW,&repair_escrow_runtime) != LXP_OK) return -5;
    env.state.next_sequence=1U;
    if (lxp_state_journal_open(&env.state,1U,&env.journal) != LXP_OK ||
        lx_asset_record_encode(&env.asset,asset_bytes,sizeof(asset_bytes),&length) != LXP_OK ||
        lxp_arena_init(&arena,arena_bytes,sizeof(arena_bytes)) != LXP_OK ||
        lxp_module_ctx_init(&ctx,&env.kernel,LXP_MODULE_ASSET,0U,0U,1U,100000U,&arena,true) != LXP_OK ||
        lxp_ctx_kv_put(&ctx,env.asset.asset_id,32U,asset_bytes,length) != LXP_OK ||
        lxp_module_ctx_commit(&ctx) != LXP_OK || lxp_state_journal_commit(&env.journal) != LXP_OK) return -6;
    env.global_sequence=1U;
    return 0;
}

void repair244_close(void) {
    lxp_state_store_destroy(&env.state);
    lx_account_registry_release(&env.accounts);
}

int repair244_info(unsigned which, uint8_t id[32], uint8_t asset[32], uint8_t authority[32], uint64_t *balance) {
    lx_account *a = which==0 ? env.owner : which==1 ? env.budget_account : which==2 ? env.recipient : which==3 ? repair_source : which==4 ? repair_escrow : env.foreign;
    memcpy(id,a->id,32U); memcpy(asset,a->asset_id,32U); memcpy(authority,a->authority_key,32U);
    *balance=a->balance.lo;
    return a->balance.hi==0U ? 0 : -1;
}
int repair244_root(uint8_t root[32]) { return lxp_state_root(&env.kernel,root); }
int repair244_proof(unsigned module,const uint8_t *key,size_t keylen,uint8_t *out,size_t capacity,size_t *length) {
    lxp_state_witness *w=malloc(sizeof(*w));
    if (w==NULL) return -1;
    lxp_result rc=lxp_state_proof_build(&env.kernel,(uint16_t)module,(lxp_byte_span){key,keylen},w);
    if (rc==LXP_OK) rc=lxp_state_proof_encode(w,out,capacity,length);
    free(w); return rc;
}
int repair244_budget(unsigned native,const uint8_t *payload,size_t length) {
    budget_call call={0}; lxp_effect_buffer effects; lxp_result result;
    uint32_t kind = length==50U || length==58U ? LX_BUDGET_FUND : LX_BUDGET_CREATE;
    if (lxp_kernel_module_for_activity(&env.kernel,kind,0U,&env.registration) != LXP_OK) return -10;
    call.protocol_version=native ? 3U : LXP_PROTOCOL_VERSION_OCCUPANCY;
    call.activity_type=kind; call.did=owner_did; call.seed=owner_seed;
    call.payload=payload; call.payload_length=length; call.timestamp=500U;
    call.sequence=env.owner->next_sequence;
    if (submit(&call,&effects,&result) != 0) return -11;
    return result;
}
int repair244_escrow(const uint8_t *payload,size_t length) {
    lxp_module_ctx ctx; lxp_arena arena; lxp_activity activity={0};
    lxp_authority_resolved authority={0}; lxp_effect_buffer effects;
    uint8_t arena_bytes[65536]; void *decoded=NULL;
    const lxp_module_iface *iface=lx_escrow_module_iface();
    if (lxp_arena_init(&arena,arena_bytes,sizeof(arena_bytes)) != LXP_OK ||
        lxp_module_ctx_init(&ctx,&env.kernel,LXP_MODULE_ESCROW,500U,0U,++env.global_sequence,1000000U,&arena,true) != LXP_OK ||
        lxp_effect_buffer_init(&effects) != LXP_OK || lxp_module_ctx_bind_effects(&ctx,&effects) != LXP_OK) return -1;
    activity.activity_type=LX_ESCROW_OPEN; activity.payload=(lxp_byte_span){payload,length};
    authority.kind=LXP_AUTHORITY_OWNER; memcpy(authority.principal,env.owner->id,32U); memcpy(authority.actor,env.owner->id,32U);
    lxp_result rc=iface->decode(&ctx,1U,payload,length,&decoded);
    if (rc==LXP_OK) rc=iface->validate(&ctx,&activity,&authority,decoded);
    if (rc==LXP_OK) rc=iface->execute(&ctx,&activity,&authority,decoded,&effects);
    if (rc==LXP_OK) rc=lxp_module_ctx_commit(&ctx);
    return rc;
}
int repair244_transfer(const uint8_t from[32],const uint8_t asset[32],const uint8_t to[32],const uint8_t amount[16]) {
    lxp_transfer_set set={0}; lxp_transfer_source_authority source={0}; lxp_receipt receipt={0};
    size_t from_slot,to_slot;
    if (lx_account_registry_index_lookup(&env.accounts,from,&from_slot) != LXP_OK ||
        lx_account_registry_index_lookup(&env.accounts,to,&to_slot) != LXP_OK) return -1;
    set.legs[0].from=&env.accounts.accounts[from_slot];
    set.legs[0].to=&env.accounts.accounts[to_slot];
    set.leg_count=1U; memcpy(set.legs[0].asset_id,asset,32U);
    lxp_result rc=lxp_u128_from_be(amount,&set.legs[0].amount);
    if (rc!=LXP_OK) return rc;
    set.legs[0].reason=LXP_REASON_BUDGET_FUND;
    set.context.assets=&env.asset_state; set.context.asset_count=1U;
    set.context.sequence_account=set.legs[0].from;
    set.context.actor_sequence=set.legs[0].from->next_sequence;
    set.context.debit_authority_kind=LXP_AUTH_OWNER;
    memcpy(set.context.authorized_from,from,32U);
    memcpy(source.authorized_from,from,32U); source.debit_authority_kind=LXP_AUTH_OWNER;
    set.context.source_authorities=&source; set.context.source_authority_count=1U;
    return lxp_kernel_canonical_ledger_apply(&env.kernel,&set,&receipt);
}
