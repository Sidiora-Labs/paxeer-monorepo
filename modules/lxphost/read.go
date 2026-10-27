//go:build cgo

package lxphost

/*
#include <stdlib.h>
#include <string.h>
#include "layerx/lxp_hash.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/programs.h"
#include "layerx/lx_asset.h"
#include "layerx/lx_budget.h"
#include "layerx/lxp_governance.h"
#include "layerx/lxp_authority.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lx_stream.h"

typedef struct lxphost_read_state {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_identity_store identities;
    uint64_t parameters;
} lxphost_read_state;

static lxphost_read_state *lxphost_read_new(uint64_t sequence, uint64_t epoch,
                                            int32_t *status)
{
    lxphost_read_state *state = calloc(1U, sizeof(*state));
    if (state == NULL) {
        *status = LXP_ERR_ARENA_EXHAUSTED;
        return NULL;
    }
    *status = lxp_state_store_init(&state->store, sequence);
    if (*status != LXP_OK) {
        free(state);
        return NULL;
    }
    state->parameters = 1U;
    *status = lx_account_registry_init(&state->accounts);
    if (*status == LXP_OK)
        *status = lxp_state_store_bind_accounts(&state->store, &state->accounts);
    if (*status == LXP_OK)
        *status = lxp_state_store_require_account_root(&state->store);
    if (*status == LXP_OK)
        *status = lxp_kernel_create(&state->kernel, &state->store, &state->journal,
                                &state->parameters, epoch);
    if (*status != LXP_OK) {
        (void)lxp_state_store_destroy(&state->store);
        lx_account_registry_release(&state->accounts);
        free(state);
        return NULL;
    }
    return state;
}

static int32_t lxphost_read_free(lxphost_read_state *state)
{
    int32_t status = lxp_state_store_destroy(&state->store);
    lx_account_registry_release(&state->accounts);
    free(state);
    return status;
}

static int32_t lxphost_read_kv(lxphost_read_state *state, uint16_t module,
                               const uint8_t *key, size_t key_length,
                               const uint8_t *value, size_t value_length)
{
    lxp_module_kv_entry *entry;
    if (module == 0U || key == NULL || key_length == 0U ||
        key_length > LXP_MODULE_MAX_KEY_BYTES ||
        (value == NULL && value_length != 0U) ||
        value_length > LXP_MODULE_MAX_VALUE_BYTES)
        return LXP_FATAL_INVARIANT;
    if (state->kernel.module_kv_count >= LXP_KERNEL_MAX_MODULE_KV)
        return LXP_ERR_ARENA_EXHAUSTED;
    entry = &state->kernel.module_kv[state->kernel.module_kv_count++];
    entry->module_id = module;
    entry->key_length = (uint16_t)key_length;
    entry->value_length = (uint32_t)value_length;
    memcpy(entry->key, key, key_length);
    if (value_length != 0U) memcpy(entry->value, value, value_length);
    return LXP_OK;
}

static int32_t lxphost_read_replay_load(lxphost_read_state *state,
                                        const uint8_t hash[32],
                                        const uint8_t *bytes, size_t length)
{
    lxp_idempotency_key_state *entry;
    int32_t status;
    if (state->store.idempotency_count >= LXP_STATE_MAX_IDEMPOTENCY)
        return LXP_ERR_ARENA_EXHAUSTED;
    status = lxp_idempotency_reserve(&state->store);
    if (status != LXP_OK) return status;
    entry = &state->store.idempotency[state->store.idempotency_count];
    status = lxp_idempotency_snapshot_decode(entry, bytes, length);
    if (status != LXP_OK) return status;
    memcpy(entry->key_hash, hash, 32U);
    ++state->store.idempotency_count;
    return LXP_OK;
}

static int32_t lxphost_read_asset(lxphost_read_state *state,
                                  const uint8_t id[32], lx_asset_record *out)
{
    lx_asset_record *record = NULL;
    lx_asset_registry *assets = calloc(1U, sizeof(*assets));
    int32_t status;
    if (assets == NULL) return LXP_ERR_ARENA_EXHAUSTED;
    status = lx_asset_committed_records(&state->kernel, assets->assets,
                                         LX_ASSET_REGISTRY_CAPACITY,
                                         &assets->count);
    if (status == LXP_OK) status = lx_asset_lookup(assets, id, &record);
    if (status == LXP_OK) *out = *record;
    free(assets);
    return status;
}

static int32_t lxphost_read_total(lxphost_read_state *state,
                                  const uint8_t id[32], lxp_u128 *out)
{
    lx_asset_registry *assets = calloc(1U, sizeof(*assets));
    int32_t status;
    if (assets == NULL) return LXP_ERR_ARENA_EXHAUSTED;
    status = lx_asset_committed_records(&state->kernel, assets->assets,
                                         LX_ASSET_REGISTRY_CAPACITY,
                                         &assets->count);
    if (status == LXP_OK)
        status = lx_asset_total_units(assets, state->store.accounts, id, out);
    free(assets);
    return status;
}

static int32_t lxphost_read_account(lxphost_read_state *state,
                                    const uint8_t *name, size_t name_length,
                                    const uint8_t id[32], lx_account *out)
{
    lx_account *account = NULL;
    int32_t status = lx_account_lookup(state->store.accounts, name,
                                        name_length, id, &account);
    if (status == LXP_OK) *out = *account;
    return status;
}

static int32_t lxphost_read_owner(lxphost_read_state *state,
                                  const uint8_t id[32], uint8_t owner[32])
{
    size_t slot = 0U;
    int32_t status = lx_account_registry_index_lookup(state->store.accounts,
                                                      id, &slot);
    if (status != LXP_OK) return status;
    return lx_account_owner_did(&state->store.accounts->accounts[slot], owner);
}

static int32_t lxphost_read_receipt(lxphost_read_state *state,
                                    const uint8_t *did, size_t did_length,
                                    const uint8_t key[32], lxp_receipt *receipt,
                                    const uint8_t **encoded, size_t *length)
{
    int32_t status = lxp_idempotency_canonical_lookup(&state->store, did,
                                                      did_length, key,
                                                      encoded, length);
    if (status == LXP_OK) return LXP_ERR_UNKNOWN_FIELD;
    if (status != LXP_ERR_IDEMPOTENT_REPLAY) return status;
    return lxp_receipt_decode(*encoded, *length, true, receipt);
}

static int32_t lxphost_read_identity_id(lxphost_read_state *state,
                                        const uint8_t id[32], lxp_identity *out)
{
    int32_t status = lxp_governance_identities_restore(&state->kernel,
                                                       &state->identities);
    if (status != LXP_OK) return status;
    for (size_t i = 0U; i < state->identities.count; ++i) {
        const lxp_identity *identity = &state->identities.identities[i];
        if (memcmp(identity->did_id, id, 32U) != 0) continue;
        if (identity->status == LXP_IDENTITY_FROZEN) return LXP_ERR_IDENTITY_FROZEN;
        if (identity->status != LXP_IDENTITY_ACTIVE &&
            identity->status != LXP_IDENTITY_RECOVERING) return LXP_ERR_UNKNOWN_DID;
        *out = *identity;
        return LXP_OK;
    }
    return LXP_ERR_UNKNOWN_DID;
}

static int32_t lxphost_read_identity(lxphost_read_state *state,
                                     const uint8_t *did, size_t length,
                                     lxp_identity *out)
{
    uint8_t id[32];
    int32_t status = lxp_did_id_derive(did, length, id);
    if (status != LXP_OK) return status;
    return lxphost_read_identity_id(state, id, out);
}

static int32_t lxphost_read_live(lxphost_read_state *state,
                                 const lxp_authority_grant *grant,
                                 uint64_t timestamp, uint64_t sequence)
{
    lxp_identity identity;
    int32_t status = lxphost_read_identity_id(state, grant->grantor, &identity);
    if (status != LXP_OK) return status;
    return lxp_authority_is_live(grant, identity.revocation_sequence,
                                  timestamp, sequence);
}

static int32_t lxphost_read_authority_envelope(lxphost_read_state *state,
                                               lxp_authority_envelope *out)
{
    int32_t status = lxp_kernel_register_module(&state->kernel,
                                                 lxp_governance_module_iface());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&state->kernel,
                                             programs_module_registration());
    if (status == LXP_OK)
        status = lxp_kernel_register_module(&state->kernel,
                                             lx_asset_module_iface());
    if (status != LXP_OK) return status;
    return lxp_authority_envelope_declare(&state->kernel, state->kernel.epoch, out);
}

static int32_t lxphost_read_resolve(lxphost_read_state *state,
                                    const uint8_t actor[32],
                                    const lxp_authority_grant *grant,
                                    const lxp_authority_envelope *envelope,
                                    lxp_authority_resolved *out)
{
    // This API has no activity type: resolve eligibility within the stored
    // scope, against the same registered envelope used by execution.
    for (size_t i = 0U; i < state->kernel.module_count; ++i) {
        const lxp_module_registration *module = &state->kernel.modules[i];
        if (!module->enabled || state->kernel.epoch < module->enabled_epoch ||
            state->kernel.epoch >= module->disabled_epoch) continue;
        for (size_t j = 0U; j < module->activity_type_count; ++j) {
            uint32_t type = module->activity_types[j];
            if (lxp_authority_check_scope(&grant->scope, type,
                    envelope->module_mask, envelope->activity_ordinal_min,
                    envelope->activity_ordinal_max) != LXP_OK) continue;
            int32_t status = lxp_authority_resolve(grant, actor, type,
                envelope->module_mask, envelope->activity_ordinal_min,
                envelope->activity_ordinal_max, true, out);
            // The Go caller copies the scope from grant; do not retain its pointer.
            out->scope = NULL;
            return status;
        }
    }
    return LXP_ERR_AUTH_SCOPE;
}

static int32_t lxphost_read_check_activity(lxphost_read_state *state,
                                           const uint8_t *did, size_t length,
                                           const uint8_t id[32],
                                           const uint8_t payload_hash[32],
                                           uint64_t timestamp, uint64_t sequence,
                                           lxp_authority_grant *grant,
                                           lxp_authority_resolved *out)
{
    lxp_identity identity;
    lxp_authority_envelope envelope;
    int32_t status = lxphost_read_identity(state, did, length, &identity);
    if (status == LXP_OK) status = lxp_authority_grant_load(&state->kernel, id, grant);
    if (status == LXP_OK) status = lxphost_read_live(state, grant, timestamp, sequence);
    if (status != LXP_OK) return status;
    if (!lxp_ct_is_zero(grant->scope.purpose_hash, 32U) &&
        lxp_ct_memcmp(grant->scope.purpose_hash, payload_hash, 32U) != 0)
        return LXP_ERR_AUTH_SCOPE;
    status = lxphost_read_authority_envelope(state, &envelope);
    if (status != LXP_OK) return status;
    return lxphost_read_resolve(state, identity.did_id, grant, &envelope, out);
}

static int32_t lxphost_read_identity_state(lxphost_read_state *state,
                                           const uint8_t *did, size_t length,
                                           lxp_identity *identity,
                                           lxp_authority_grant *grant,
                                           lxp_authority_resolved *out)
{
    lxp_authority_envelope envelope;
    int32_t status = lxphost_read_identity(state, did, length, identity);
    if (status == LXP_OK) status = lxphost_read_authority_envelope(state, &envelope);
    if (status == LXP_OK)
        status = lxp_authority_owner_grant(identity, identity->primary_key,
                                            &envelope, 0U, UINT64_MAX - 1U, grant);
    if (status != LXP_OK) return status;
    return lxphost_read_resolve(state, identity->did_id, grant, &envelope, out);
}

static int32_t lxphost_read_record(lxphost_read_state *state, uint16_t module,
                                   const uint8_t *key, size_t key_length,
                                   const uint8_t **encoded, size_t *length)
{
    for (size_t i = 0U; i < state->kernel.module_kv_count; ++i) {
        const lxp_module_kv_entry *entry = &state->kernel.module_kv[i];
        if (entry->module_id != module || entry->key_length != key_length ||
            memcmp(entry->key, key, key_length) != 0) continue;
        *encoded = entry->value;
        *length = entry->value_length;
        return LXP_OK;
    }
    return LXP_ERR_UNKNOWN_FIELD;
}

static int32_t lxphost_read_stream_accrued(lxphost_read_state *state,
                                           const uint8_t id[32], uint64_t at,
                                           lxp_u128 *amount)
{
    uint8_t key[LX_STREAM_KEY_BYTES];
    const uint8_t *encoded;
    size_t length;
    lx_stream_record record;
    int32_t status = lx_stream_state_key(id, key);
    if (status == LXP_OK)
        status = lxphost_read_record(state, LXP_MODULE_STREAM, key,
                                      sizeof(key), &encoded, &length);
    if (status == LXP_OK)
        status = lx_stream_record_decode(encoded, length, &record);
    if (status != LXP_OK) return status;
    if (memcmp(record.stream_id, id, 32U) != 0) return LXP_FATAL_INVARIANT;
    return lx_stream_accrue(&record, at, amount);
}
*/
import "C"

import (
	"encoding/binary"
	"math"
	"math/big"
	"runtime"
	"unsafe"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type AssetRecord struct {
	AssetID          [32]byte
	Symbol           string
	Name             string
	Decimals         uint8
	SupplyCap        sdk.Int
	IssuerDID        [32]byte
	Salt             [32]byte
	IssuerKind       uint8
	CustodyKind      uint8
	CustodyReference []byte
	Paused           bool
	TotalUnits       sdk.Int
}

type AccountRecord struct {
	ID                [32]byte
	Name              string
	Kind              uint8
	Balance           sdk.Int
	AssetID           [32]byte
	HasAsset          bool
	NextSequence      uint64
	CreatedAtSequence uint64
	Frozen            bool
	HasOpenReference  bool
	AuthorityKey      [32]byte
	HasAuthorityKey   bool
}

type PayerGrantState struct {
	Grant           []byte
	DrawnTotal      sdk.Int
	DrawnThisPeriod sdk.Int
	WindowStart     uint64
	RevokedAt       uint64
	Revoked         bool
	InvoiceSettled  bool
}

type AuthorityScope struct {
	ModuleMask         uint64
	ActivityOrdinalMin uint16
	ActivityOrdinalMax uint16
	AssetID            [32]byte
	MaximumPerActivity sdk.Int
	MaximumTotal       sdk.Int
	SpentTotal         sdk.Int
	PeriodLength       uint64
	MaximumPerPeriod   sdk.Int
	SpentThisPeriod    sdk.Int
	PeriodStart        uint64
	PurposeHash        [32]byte
	EarliestSequence   uint64
	EarliestTimestamp  uint64
	SignerThreshold    uint8
	SignerCount        uint8
	ApprovalCount      uint8
	Signers            [8][32]byte
	Approvals          [8][32]byte
}

type AuthorityFeeBudget struct {
	Present            bool
	AssetID            [32]byte
	MaximumPerActivity sdk.Int
	MaximumTotal       sdk.Int
	SpentTotal         sdk.Int
	PeriodLength       uint64
	MaximumPerPeriod   sdk.Int
	SpentThisPeriod    sdk.Int
	PeriodStart        uint64
}

type AuthorityGrant struct {
	GrantID                   [32]byte
	Grantor                   [32]byte
	Grantee                   [32]byte
	Kind                      uint8
	Key                       [32]byte
	Scope                     AuthorityScope
	NotBefore                 uint64
	NotAfter                  uint64
	GrantorRevocationSequence uint64
	Revoked                   bool
	RevokedAtSequence         uint64
	GrantorSignature          [64]byte
	FeeBudget                 AuthorityFeeBudget
	AuthenticationOnly        bool
}

type AuthorityState struct {
	Actor         [32]byte
	Principal     [32]byte
	Kind          uint8
	VerifiedKey   [32]byte
	Scope         AuthorityScope
	AuthorityHash [32]byte
	GrantID       [32]byte
}

type IdentityState struct {
	DIDID                     [32]byte
	Status                    uint8
	PrimaryKey                [32]byte
	PendingKey                [32]byte
	HasPendingKey             bool
	RotationAnnouncedAt       uint64
	RotationEffectiveAt       uint64
	RotationLapseAt           uint64
	RotationEffectiveSequence uint64
	SupersededKey             [32]byte
	HasSupersededKey          bool
	NextSequence              uint64
	RevocationSequence        uint64
	RecoveryRoot              [32]byte
	RecoveryThreshold         uint16
	RecoveryPendingKey        [32]byte
	RecoveryApprovals         uint16
	RecoveryEffectiveAt       uint64
	RecoveryLapseAt           uint64
	RecoveryVetoed            bool
	EVMPayoutAddress          [20]byte
	HasEVMPayoutBinding       bool
	Authority                 AuthorityState
}

func readAuthorityScope(scope *C.struct_lxp_authority_scope) AuthorityScope {
	return AuthorityScope{
		ModuleMask:         uint64(scope.module_mask),
		ActivityOrdinalMin: uint16(scope.activity_ordinal_min),
		ActivityOrdinalMax: uint16(scope.activity_ordinal_max),
		AssetID:            *(*[32]byte)(unsafe.Pointer(&scope.asset_id[0])),
		MaximumPerActivity: readAmount(scope.maximum_per_activity),
		MaximumTotal:       readAmount(scope.maximum_total),
		SpentTotal:         readAmount(scope.spent_total),
		PeriodLength:       uint64(scope.period_length),
		MaximumPerPeriod:   readAmount(scope.maximum_per_period),
		SpentThisPeriod:    readAmount(scope.spent_this_period),
		PeriodStart:        uint64(scope.period_start),
		PurposeHash:        *(*[32]byte)(unsafe.Pointer(&scope.purpose_hash[0])),
		EarliestSequence:   uint64(scope.earliest_sequence),
		EarliestTimestamp:  uint64(scope.earliest_timestamp),
		SignerThreshold:    uint8(scope.signer_threshold),
		SignerCount:        uint8(scope.signer_count),
		ApprovalCount:      uint8(scope.approval_count),
		Signers:            *(*[8][32]byte)(unsafe.Pointer(&scope.signers[0][0])),
		Approvals:          *(*[8][32]byte)(unsafe.Pointer(&scope.approvals[0][0])),
	}
}

func readAuthorityState(authority *C.struct_lxp_authority_resolved, scope *C.struct_lxp_authority_scope) AuthorityState {
	return AuthorityState{
		Actor:         *(*[32]byte)(unsafe.Pointer(&authority.actor[0])),
		Principal:     *(*[32]byte)(unsafe.Pointer(&authority.principal[0])),
		Kind:          uint8(authority.kind),
		VerifiedKey:   *(*[32]byte)(unsafe.Pointer(&authority.verified_key[0])),
		Scope:         readAuthorityScope(scope),
		AuthorityHash: *(*[32]byte)(unsafe.Pointer(&authority.authority_hash[0])),
		GrantID:       *(*[32]byte)(unsafe.Pointer(&authority.grant_id[0])),
	}
}

func readAmount(amount C.struct_lxp_u128) sdk.Int {
	value := new(big.Int).SetUint64(uint64(amount.hi))
	value.Lsh(value, 64)
	value.Or(value, new(big.Int).SetUint64(uint64(amount.lo)))
	return sdk.NewIntFromBigInt(value)
}

func (h *Host) withRead(ctx sdk.Context, modules []uint16, read func(*C.lxphost_read_state) error) (err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.node == nil {
		return codeNonCanonical
	}
	branch := &branchCtx{store: ctx.KVStore(h.cfg.StoreKey)}
	last, err := branch.lastSequence()
	if err != nil {
		return err
	}
	if last == math.MaxUint64 {
		return Code(C.LXP_ERR_SEQUENCE_EXHAUSTED)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var status C.int32_t
	state := C.lxphost_read_new(C.uint64_t(last+1), C.uint64_t(h.cfg.Epoch), &status)
	if state == nil {
		return Code(status)
	}
	defer func() {
		if releaseErr := result(C.lxphost_read_free(state)); releaseErr != nil {
			err = releaseErr
		}
	}()
	if err := loadReadState(branch.store, state, modules); err != nil {
		return err
	}
	return read(state)
}

func loadReadState(store sdk.KVStore, state *C.lxphost_read_state, modules []uint16) error {
	for _, module := range modules {
		iterator := sdk.KVStorePrefixIterator(store, moduleKVKey(module, nil))
		var err error
		for ; iterator.Valid(); iterator.Next() {
			key, value := iterator.Key(), iterator.Value()
			if len(key) < 4 {
				err = codeFatalInvariant
				break
			}
			err = result(C.lxphost_read_kv(state, C.uint16_t(module),
				bytesPtr(key[3:]), C.size_t(len(key)-3), bytesPtr(value), C.size_t(len(value))))
			if err != nil {
				break
			}
		}
		closeErr := iterator.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return codeFatalInvariant
		}
	}
	return loadCommittedAccounts(store, unsafe.Pointer(&state.accounts))
}

func (h *Host) ReadAsset(ctx sdk.Context, id [32]byte) (AssetRecord, error) {
	var out AssetRecord
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_ASSET}, func(state *C.lxphost_read_state) error {
		var record C.lx_asset_record
		if err := result(C.lxphost_read_asset(state, bytes32(&id), &record)); err != nil {
			return err
		}
		if record.symbol_length > C.LX_ASSET_SYMBOL_MAX || record.name_length > C.LX_ASSET_NAME_MAX ||
			record.custody_reference_length > C.LX_ASSET_CUSTODY_REFERENCE_MAX {
			return codeNonCanonical
		}
		out = AssetRecord{
			AssetID:          *(*[32]byte)(unsafe.Pointer(&record.asset_id[0])),
			Symbol:           C.GoStringN(&record.symbol[0], C.int(record.symbol_length)),
			Name:             string(C.GoBytes(unsafe.Pointer(&record.name[0]), C.int(record.name_length))),
			Decimals:         uint8(record.decimals),
			SupplyCap:        readAmount(record.supply_cap),
			IssuerDID:        *(*[32]byte)(unsafe.Pointer(&record.issuer_did32[0])),
			Salt:             *(*[32]byte)(unsafe.Pointer(&record.salt[0])),
			IssuerKind:       uint8(record.issuer_kind),
			CustodyKind:      uint8(record.custody_kind),
			CustodyReference: C.GoBytes(unsafe.Pointer(&record.custody_reference[0]), C.int(record.custody_reference_length)),
			Paused:           bool(record.paused),
			TotalUnits:       readAmount(record.total_units),
		}
		return nil
	})
	if err != nil {
		return AssetRecord{}, err
	}
	return out, nil
}

func (h *Host) ReadBalance(ctx sdk.Context, accountID, assetID [32]byte) (sdk.Int, error) {
	out := sdk.ZeroInt()
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_ASSET}, func(state *C.lxphost_read_state) error {
		var amount C.struct_lxp_u128
		if err := result(C.lx_asset_balance_get(state.store.accounts, bytes32(&accountID), bytes32(&assetID), &amount)); err != nil {
			return err
		}
		out = readAmount(amount)
		return nil
	})
	if err != nil {
		return sdk.ZeroInt(), err
	}
	return out, nil
}

func (h *Host) ReadTotalUnits(ctx sdk.Context, assetID [32]byte) (sdk.Int, error) {
	out := sdk.ZeroInt()
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_ASSET}, func(state *C.lxphost_read_state) error {
		var amount C.struct_lxp_u128
		if err := result(C.lxphost_read_total(state, bytes32(&assetID), &amount)); err != nil {
			return err
		}
		out = readAmount(amount)
		return nil
	})
	if err != nil {
		return sdk.ZeroInt(), err
	}
	return out, nil
}

func (h *Host) ReadAccountOwner(ctx sdk.Context, accountID [32]byte) ([32]byte, error) {
	var owner [32]byte
	err := h.withRead(ctx, nil, func(state *C.lxphost_read_state) error {
		return result(C.lxphost_read_owner(state, bytes32(&accountID), bytes32(&owner)))
	})
	if err != nil {
		return [32]byte{}, err
	}
	return owner, nil
}

func (h *Host) ReadAccount(ctx sdk.Context, name string, accountID [32]byte) (AccountRecord, error) {
	var out AccountRecord
	err := h.withRead(ctx, nil, func(state *C.lxphost_read_state) error {
		var account C.lx_account
		nameBytes := []byte(name)
		if err := result(C.lxphost_read_account(state, bytesPtr(nameBytes), C.size_t(len(nameBytes)), bytes32(&accountID), &account)); err != nil {
			return err
		}
		if account.name_length > C.LX_ACCOUNT_NAME_MAX {
			return codeNonCanonical
		}
		out = AccountRecord{
			ID:                *(*[32]byte)(unsafe.Pointer(&account.id[0])),
			Name:              string(C.GoBytes(unsafe.Pointer(&account.name[0]), C.int(account.name_length))),
			Kind:              uint8(account.kind),
			Balance:           readAmount(account.balance),
			AssetID:           *(*[32]byte)(unsafe.Pointer(&account.asset_id[0])),
			HasAsset:          bool(account.has_asset),
			NextSequence:      uint64(account.next_sequence),
			CreatedAtSequence: uint64(account.created_at_sequence),
			Frozen:            bool(account.frozen),
			HasOpenReference:  bool(account.has_open_reference),
			AuthorityKey:      *(*[32]byte)(unsafe.Pointer(&account.authority_key[0])),
			HasAuthorityKey:   bool(account.has_authority_key),
		}
		return nil
	})
	if err != nil {
		return AccountRecord{}, err
	}
	return out, nil
}

func (h *Host) ReadSessionKey(ctx sdk.Context, grantID [32]byte) ([]byte, error) {
	var summary [209]byte
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_GOVERNANCE}, func(state *C.lxphost_read_state) error {
		return result(C.lxp_governance_session_key_read(&state.kernel, bytes32(&grantID), bytesPtr(summary[:])))
	})
	if err != nil {
		return nil, err
	}
	return summary[:], nil
}

func (h *Host) ReadGrant(ctx sdk.Context, grantID [32]byte) (AuthorityGrant, error) {
	var out AuthorityGrant
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_GOVERNANCE}, func(state *C.lxphost_read_state) error {
		var grant C.lxp_authority_grant
		if err := result(C.lxp_authority_grant_load(&state.kernel, bytes32(&grantID), &grant)); err != nil {
			return err
		}
		fee := &grant.fee_budget
		out = AuthorityGrant{
			GrantID:                   *(*[32]byte)(unsafe.Pointer(&grant.grant_id[0])),
			Grantor:                   *(*[32]byte)(unsafe.Pointer(&grant.grantor[0])),
			Grantee:                   *(*[32]byte)(unsafe.Pointer(&grant.grantee[0])),
			Kind:                      uint8(grant.kind),
			Key:                       *(*[32]byte)(unsafe.Pointer(&grant.key[0])),
			Scope:                     readAuthorityScope(&grant.scope),
			NotBefore:                 uint64(grant.not_before),
			NotAfter:                  uint64(grant.not_after),
			GrantorRevocationSequence: uint64(grant.grantor_revocation_sequence),
			Revoked:                   bool(grant.revoked),
			RevokedAtSequence:         uint64(grant.revoked_at_sequence),
			GrantorSignature:          *(*[64]byte)(unsafe.Pointer(&grant.grantor_signature[0])),
			FeeBudget: AuthorityFeeBudget{
				Present:            bool(fee.present),
				AssetID:            *(*[32]byte)(unsafe.Pointer(&fee.asset_id[0])),
				MaximumPerActivity: readAmount(fee.maximum_per_activity),
				MaximumTotal:       readAmount(fee.maximum_total),
				SpentTotal:         readAmount(fee.spent_total),
				PeriodLength:       uint64(fee.period_length),
				MaximumPerPeriod:   readAmount(fee.maximum_per_period),
				SpentThisPeriod:    readAmount(fee.spent_this_period),
				PeriodStart:        uint64(fee.period_start),
			},
			AuthenticationOnly: bool(grant.authentication_only),
		}
		return nil
	})
	if err != nil {
		return AuthorityGrant{}, err
	}
	return out, nil
}

func (h *Host) ReadIsLive(ctx sdk.Context, grantID [32]byte, atSequence uint64) (bool, error) {
	live := false
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_GOVERNANCE}, func(state *C.lxphost_read_state) error {
		var grant C.lxp_authority_grant
		if err := result(C.lxp_authority_grant_load(&state.kernel, bytes32(&grantID), &grant)); err != nil {
			return err
		}
		timestamp := ctx.BlockTime().UnixMilli()
		if timestamp < 0 {
			return codeNonCanonical
		}
		status := C.lxphost_read_live(state, &grant, C.uint64_t(timestamp), C.uint64_t(atSequence))
		switch status {
		case C.LXP_OK:
			live = true
			return nil
		case C.LXP_ERR_NOT_YET_VALID, C.LXP_ERR_AUTH_EXPIRED, C.LXP_ERR_AUTH_REVOKED, C.LXP_ERR_AUTH_NOT_MATURE:
			return nil
		default:
			return result(status)
		}
	})
	if err != nil {
		return false, err
	}
	return live, nil
}

func (h *Host) ReadCapabilityScope(ctx sdk.Context, grantID [32]byte) (AuthorityScope, error) {
	var out AuthorityScope
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_GOVERNANCE}, func(state *C.lxphost_read_state) error {
		var grant C.lxp_authority_grant
		if err := result(C.lxp_authority_grant_load(&state.kernel, bytes32(&grantID), &grant)); err != nil {
			return err
		}
		out = readAuthorityScope(&grant.scope)
		return nil
	})
	if err != nil {
		return AuthorityScope{}, err
	}
	return out, nil
}

// ReadCheckActivity resolves grant eligibility and its purpose-hash restriction.
// It does not authenticate a signature or validate a concrete activity type.
func (h *Host) ReadCheckActivity(ctx sdk.Context, actorDID []byte, grantID [32]byte, payloadHash [32]byte) (AuthorityState, error) {
	var out AuthorityState
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_GOVERNANCE}, func(state *C.lxphost_read_state) error {
		timestamp := ctx.BlockTime().UnixMilli()
		if timestamp < 0 {
			return codeNonCanonical
		}
		branch := &branchCtx{store: ctx.KVStore(h.cfg.StoreKey)}
		sequence, err := branch.lastSequence()
		if err != nil {
			return err
		}
		var grant C.lxp_authority_grant
		var authority C.struct_lxp_authority_resolved
		if err := result(C.lxphost_read_check_activity(state, bytesPtr(actorDID), C.size_t(len(actorDID)),
			bytes32(&grantID), bytes32(&payloadHash), C.uint64_t(timestamp), C.uint64_t(sequence), &grant, &authority)); err != nil {
			return err
		}
		out = readAuthorityState(&authority, &grant.scope)
		return nil
	})
	if err != nil {
		return AuthorityState{}, err
	}
	return out, nil
}

func (h *Host) ReadIdentityState(ctx sdk.Context, actorDID []byte) (IdentityState, error) {
	var out IdentityState
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_GOVERNANCE}, func(state *C.lxphost_read_state) error {
		var identity C.struct_lxp_identity
		var grant C.lxp_authority_grant
		var authority C.struct_lxp_authority_resolved
		if err := result(C.lxphost_read_identity_state(state, bytesPtr(actorDID), C.size_t(len(actorDID)),
			&identity, &grant, &authority)); err != nil {
			return err
		}
		id := *(*[32]byte)(unsafe.Pointer(&identity.did_id[0]))
		branch := &branchCtx{store: ctx.KVStore(h.cfg.StoreKey)}
		next, err := branch.identitySequence(id)
		if err != nil {
			return err
		}
		out = IdentityState{
			DIDID:                     id,
			Status:                    uint8(identity.status),
			PrimaryKey:                *(*[32]byte)(unsafe.Pointer(&identity.primary_key[0])),
			PendingKey:                *(*[32]byte)(unsafe.Pointer(&identity.pending_key[0])),
			HasPendingKey:             bool(identity.has_pending_key),
			RotationAnnouncedAt:       uint64(identity.rotation_announced_at),
			RotationEffectiveAt:       uint64(identity.rotation_effective_at),
			RotationLapseAt:           uint64(identity.rotation_lapse_at),
			RotationEffectiveSequence: uint64(identity.rotation_effective_sequence),
			SupersededKey:             *(*[32]byte)(unsafe.Pointer(&identity.superseded_key[0])),
			HasSupersededKey:          bool(identity.has_superseded_key),
			NextSequence:              next,
			RevocationSequence:        uint64(identity.revocation_sequence),
			RecoveryRoot:              *(*[32]byte)(unsafe.Pointer(&identity.recovery_root[0])),
			RecoveryThreshold:         uint16(identity.recovery_threshold),
			RecoveryPendingKey:        *(*[32]byte)(unsafe.Pointer(&identity.recovery_pending_key[0])),
			RecoveryApprovals:         uint16(identity.recovery_approvals),
			RecoveryEffectiveAt:       uint64(identity.recovery_effective_at),
			RecoveryLapseAt:           uint64(identity.recovery_lapse_at),
			RecoveryVetoed:            bool(identity.recovery_vetoed),
			EVMPayoutAddress:          *(*[20]byte)(unsafe.Pointer(&identity.evm_payout_address[0])),
			HasEVMPayoutBinding:       bool(identity.has_evm_payout_binding),
			Authority:                 readAuthorityState(&authority, &grant.scope),
		}
		return nil
	})
	if err != nil {
		return IdentityState{}, err
	}
	return out, nil
}

func (h *Host) ReadPayerGrant(ctx sdk.Context, grantID [32]byte) (PayerGrantState, error) {
	var out PayerGrantState
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_ASSET, C.LXP_MODULE_BUDGET}, func(state *C.lxphost_read_state) error {
		var grant C.lxp_grant_state
		if err := result(C.lx_asset_payer_grant_read(&state.kernel, bytes32(&grantID), &grant)); err != nil {
			return err
		}
		var draws C.lxp_grant_state
		if err := result(C.lx_budget_grant_draws_read(&state.kernel, bytes32(&grantID), &draws)); err != nil {
			return err
		}
		var sum C.struct_lxp_u128
		if err := result(C.lxp_u128_add(grant.drawn_total, draws.drawn_total, &sum)); err != nil {
			return err
		}
		grant.drawn_total = sum
		if draws.window_start > grant.window_start {
			grant.drawn_this_period = draws.drawn_this_period
			grant.window_start = draws.window_start
		} else if draws.window_start == grant.window_start {
			if err := result(C.lxp_u128_add(grant.drawn_this_period, draws.drawn_this_period, &sum)); err != nil {
				return err
			}
			grant.drawn_this_period = sum
		}
		var encoded [1024]byte
		var length C.size_t
		if err := result(C.lxp_payer_grant_encode(&grant.grant, bytesPtr(encoded[:]), C.size_t(len(encoded)), &length)); err != nil {
			return err
		}
		if length > C.size_t(len(encoded)) {
			return codeFatalInvariant
		}
		out = PayerGrantState{
			Grant:           append([]byte(nil), encoded[:int(length)]...),
			DrawnTotal:      readAmount(grant.drawn_total),
			DrawnThisPeriod: readAmount(grant.drawn_this_period),
			WindowStart:     uint64(grant.window_start),
			RevokedAt:       uint64(grant.revoked_at_sequence),
			Revoked:         bool(grant.revoked),
			InvoiceSettled:  bool(grant.invoice_settled),
		}
		return nil
	})
	if err != nil {
		return PayerGrantState{}, err
	}
	return out, nil
}

func (h *Host) ReadIdempotentReceipt(ctx sdk.Context, actorDID []byte, key [32]byte) (Receipt, error) {
	var out Receipt
	err := h.withRead(ctx, nil, func(state *C.lxphost_read_state) error {
		iterator := sdk.KVStorePrefixIterator(ctx.KVStore(h.cfg.StoreKey), []byte{idempotencyPrefix})
		var err error
		for ; iterator.Valid(); iterator.Next() {
			key, value := iterator.Key(), iterator.Value()
			if len(key) != 33 {
				err = codeFatalInvariant
				break
			}
			err = result(C.lxphost_read_replay_load(state, bytesPtr(key[1:]),
				bytesPtr(value), C.size_t(len(value))))
			if err != nil {
				break
			}
		}
		closeErr := iterator.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return codeFatalInvariant
		}
		var receipt C.struct_lxp_receipt
		var encoded *C.uint8_t
		var length C.size_t
		if err := result(C.lxphost_read_receipt(state, bytesPtr(actorDID), C.size_t(len(actorDID)), bytes32(&key),
			&receipt, &encoded, &length)); err != nil {
			return err
		}
		if length > C.LXP_STATE_MAX_RECEIPT_BYTES {
			return codeFatalInvariant
		}
		out = Receipt{
			ResultCode:         Code(receipt.result_code),
			GlobalSequence:     uint64(receipt.global_sequence),
			Timestamp:          uint64(receipt.timestamp),
			PreviousStateRoot:  *(*[32]byte)(unsafe.Pointer(&receipt.previous_state_root[0])),
			ResultingStateRoot: *(*[32]byte)(unsafe.Pointer(&receipt.resulting_state_root[0])),
			Encoded:            C.GoBytes(unsafe.Pointer(encoded), C.int(length)),
		}
		return nil
	})
	if err != nil {
		return Receipt{}, err
	}
	return out, nil
}

func (h *Host) ReadLastSequence(ctx sdk.Context, actorDID []byte) (uint64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.node == nil || len(actorDID) == 0 || len(actorDID) > C.LXP_MAX_DID_LENGTH {
		return 0, codeNonCanonical
	}
	if _, err := DIDID(actorDID); err != nil {
		return 0, err
	}
	branch := &branchCtx{store: ctx.KVStore(h.cfg.StoreKey)}
	return branch.lastSequence()
}

func (h *Host) ReadIdentitySequence(ctx sdk.Context, actorDID []byte) (uint64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.node == nil || len(actorDID) == 0 || len(actorDID) > C.LXP_MAX_DID_LENGTH {
		return 0, codeNonCanonical
	}
	id, err := DIDID(actorDID)
	if err != nil {
		return 0, err
	}
	branch := &branchCtx{store: ctx.KVStore(h.cfg.StoreKey)}
	return branch.identitySequence(id)
}

func (h *Host) ReadStreamAccrued(ctx sdk.Context, streamID [32]byte, atBlock uint64) (sdk.Int, error) {
	out := sdk.ZeroInt()
	err := h.withRead(ctx, []uint16{C.LXP_MODULE_STREAM}, func(state *C.lxphost_read_state) error {
		var amount C.struct_lxp_u128
		if err := result(C.lxphost_read_stream_accrued(state, bytes32(&streamID), C.uint64_t(atBlock), &amount)); err != nil {
			return err
		}
		out = readAmount(amount)
		return nil
	})
	if err != nil {
		return sdk.ZeroInt(), err
	}
	return out, nil
}

func (h *Host) readRecord(ctx sdk.Context, module uint16, key []byte) ([]byte, error) {
	var out []byte
	err := h.withRead(ctx, []uint16{module}, func(state *C.lxphost_read_state) error {
		var encoded *C.uint8_t
		var length C.size_t
		if err := result(C.lxphost_read_record(state, C.uint16_t(module), bytesPtr(key), C.size_t(len(key)), &encoded, &length)); err != nil {
			return err
		}
		if length > C.LXP_MODULE_MAX_VALUE_BYTES {
			return codeFatalInvariant
		}
		out = C.GoBytes(unsafe.Pointer(encoded), C.int(length))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Host) ReadFeeSchedule(ctx sdk.Context) ([]byte, error) {
	return h.readRecord(ctx, uint16(C.LXP_MODULE_PROGRAMS), []byte("progfee/active/v1"))
}

func (h *Host) ReadPendingFeeSchedule(ctx sdk.Context) ([]byte, error) {
	return h.readRecord(ctx, uint16(C.LXP_MODULE_PROGRAMS), []byte("progfee/pending/v1"))
}

func (h *Host) ReadMeteringSchedule(ctx sdk.Context) ([]byte, error) {
	return h.readRecord(ctx, uint16(C.LXP_MODULE_PROGRAMS), []byte("progmet/active/v1"))
}

func (h *Host) ReadWebRequest(ctx sdk.Context, programID [32]byte, requestID uint64) ([]byte, error) {
	key := append([]byte("web/pending"), programID[:]...)
	key = binary.BigEndian.AppendUint64(key, requestID)
	return h.readRecord(ctx, uint16(C.LXP_MODULE_PROGRAMS), key)
}
