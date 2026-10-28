//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"runtime"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func authorityAmount(value uint64) [16]byte {
	var amount [16]byte
	binary.BigEndian.PutUint64(amount[8:], value)
	return amount
}

func TestHostAuthorityDelegatedScope(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 101)
	scope := AuthorityScope{
		ModuleMask: 1 << 9, ActivityOrdinalMin: 4, ActivityOrdinalMax: 4,
		AssetID: [32]byte{11}, MaximumPerActivity: sdk.NewInt(12),
		MaximumTotal: sdk.NewInt(13), SpentTotal: sdk.NewInt(14),
		PeriodLength: 15, MaximumPerPeriod: sdk.NewInt(16),
		SpentThisPeriod: sdk.NewInt(17), PeriodStart: 18,
		PurposeHash: [32]byte{19}, EarliestSequence: 20, EarliestTimestamp: 21,
		SignerThreshold: 2, SignerCount: 3, ApprovalCount: 2,
	}
	scope.MaximumTotal = sdk.NewIntFromBigInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 120), big.NewInt(13)))
	for i := range scope.Signers {
		for j := range scope.Signers[i] {
			scope.Signers[i][j] = byte(i*32 + j)
			scope.Approvals[i][j] = byte(255 - i*32 - j)
		}
	}
	authority := actor.authority()
	authority.Kind, authority.Bound = 3, true
	authority.GrantID, authority.AuthorityHash = [32]byte{22}, [32]byte{23}
	authority.Scope = &scope
	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.registry(t, host, 0, bindingBlockTime, 0x71, []byte("manifest")), authority)
	require.Equal(t, Code(0), receipt.ResultCode)
	resolved := &host.node.authority
	require.Equal(t, uint8(3), uint8(resolved.kind))
	require.Equal(t, resolved, host.node.execution.authority)
	require.Equal(t, &host.node.authority_scope, resolved.scope)
	require.True(t, authority_bound(host.node))
	for i := range authority.Actor {
		require.Equal(t, authority.Actor[i], byte(resolved.actor[i]))
		require.Equal(t, authority.Principal[i], byte(resolved.principal[i]))
		require.Equal(t, authority.VerifiedKey[i], byte(resolved.verified_key[i]))
		require.Equal(t, authority.GrantID[i], byte(resolved.grant_id[i]))
		require.Equal(t, authority.AuthorityHash[i], byte(resolved.authority_hash[i]))
		require.Equal(t, scope.AssetID[i], byte(resolved.scope.asset_id[i]))
		require.Equal(t, scope.PurposeHash[i], byte(resolved.scope.purpose_hash[i]))
	}
	copied := resolved.scope
	require.Equal(t, scope.ModuleMask, uint64(copied.module_mask))
	require.Equal(t, scope.ActivityOrdinalMin, uint16(copied.activity_ordinal_min))
	require.Equal(t, scope.ActivityOrdinalMax, uint16(copied.activity_ordinal_max))
	require.Equal(t, uint64(12), uint64(copied.maximum_per_activity.lo))
	require.Zero(t, uint64(copied.maximum_per_activity.hi))
	require.Equal(t, uint64(13), uint64(copied.maximum_total.lo))
	require.Equal(t, uint64(1)<<56, uint64(copied.maximum_total.hi))
	require.Equal(t, uint64(14), uint64(copied.spent_total.lo))
	require.Zero(t, uint64(copied.spent_total.hi))
	require.Equal(t, scope.PeriodLength, uint64(copied.period_length))
	require.Equal(t, uint64(16), uint64(copied.maximum_per_period.lo))
	require.Zero(t, uint64(copied.maximum_per_period.hi))
	require.Equal(t, uint64(17), uint64(copied.spent_this_period.lo))
	require.Zero(t, uint64(copied.spent_this_period.hi))
	require.Equal(t, scope.PeriodStart, uint64(copied.period_start))
	require.Equal(t, scope.EarliestSequence, uint64(copied.earliest_sequence))
	require.Equal(t, scope.EarliestTimestamp, uint64(copied.earliest_timestamp))
	require.Equal(t, scope.SignerThreshold, uint8(copied.signer_threshold))
	require.Equal(t, scope.SignerCount, uint8(copied.signer_count))
	require.Equal(t, scope.ApprovalCount, uint8(copied.approval_count))
	for i := range scope.Signers {
		for j := range scope.Signers[i] {
			require.Equal(t, scope.Signers[i][j], byte(copied.signers[i][j]))
			require.Equal(t, scope.Approvals[i][j], byte(copied.approvals[i][j]))
		}
	}
	scope.ModuleMask = 0
	scope.Signers[0][0] = 255
	authority.Scope = nil
	runtime.GC()
	require.Equal(t, uint64(1<<9), uint64(copied.module_mask))
	require.Zero(t, byte(copied.signers[0][0]))
	require.NoError(t, lxphost_authority_write(host.node, actor.authority()))
	require.Equal(t, uint8(1), uint8(host.node.authority.kind))
	require.Nil(t, host.node.authority.scope)
	require.Zero(t, uint64(host.node.authority_scope.module_mask))
	require.False(t, authority_bound(host.node))
}

func TestHostAuthorityRefusesUnknownKind(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 102)
	authority := actor.authority()
	authority.Kind, authority.Bound = 9, true
	before := chain.dump()
	receipt, err := host.HostExecute(chain.context(1, bindingBlockTime, 1),
		actor.register(t, host, 0, bindingBlockTime), authority)
	require.Equal(t, codeNonCanonical, err)
	require.Equal(t, "LXP_ERR_NON_CANONICAL", err.Error())
	require.Empty(t, receipt.Encoded)
	require.Equal(t, before, chain.dump())
	require.Zero(t, uint64(host.node.execution.global_sequence))
	require.False(t, authority_bound(host.node))
	for _, kind := range []uint8{1, 2, 3, 4, 5, 6, 7, 8} {
		authority.Kind = kind
		require.NoError(t, lxphost_authority_write(host.node, authority))
		require.Equal(t, kind, uint8(host.node.authority.kind))
	}
	authority.Kind = 255
	require.Equal(t, codeNonCanonical, lxphost_authority_write(host.node, authority))
	require.False(t, authority_bound(host.node))
}

func authoritySeedMain(t *testing.T, chain *testChain, actor testActor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := MainAccountID(actor.public)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	row := field(nil, id[:])
	row = field(row, []byte(name))
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = field(row, asset[:])
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 1)
	row = append(row, 0, 0)
	row = field(row, actor.public[:])
	row = append(row, 1)
	chain.context(1, bindingBlockTime, 1).KVStore(chain.key).Set(accountKey(id), row)
	require.Equal(t, id, parseAccountRow(t, accountKey(id), row).id)
	return id
}

func authorityFundedMain(t *testing.T, chain *testChain, host *Host, actor testActor) (testAsset, [32]byte) {
	t.Helper()
	asset := newTestAsset(t, actor, 0x73)
	main := authoritySeedMain(t, chain, actor, asset.id)
	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.registerAsset(t, host, 0, bindingBlockTime, asset, 0x73, 1000), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	receipt = execute(t, host, chain.context(1, bindingBlockTime, 2),
		actor.openAssetAccount(t, host, 1, bindingBlockTime, asset), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	receipt = execute(t, host, chain.context(1, bindingBlockTime, 3),
		actor.mint(t, host, 2, bindingBlockTime, asset, 200), actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	amount := authorityAmount(200)
	activity := actor.activity(t, host, 0x00010005, 3, bindingBlockTime, nil)
	material := append(append(append(append([]byte(nil), asset.account[:]...), main[:]...), asset.id[:]...), amount[:]...)
	material = append(material, activity.IdempotencyKey[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, material)
	require.NoError(t, err)
	common := append(append(append(append([]byte(nil), asset.account[:]...), main[:]...), asset.id[:]...), amount[:]...)
	common = binary.BigEndian.AppendUint64(common, accountRows(t, chain)[asset.account].next)
	common = append(common, activity.IdempotencyKey[:]...)
	common = binary.BigEndian.AppendUint64(common, uint64(bindingBlockTime)+10000)
	common = append(common, contextHash[:]...)
	common = append(common, 0)
	suffix := binary.BigEndian.AppendUint32(nil, testNetworkID)
	suffix = binary.BigEndian.AppendUint16(suffix, stateCommitment)
	message := append([]byte{0x53, 1}, common...)
	message = append(message, 1)
	message = append(message, asset.account[:]...)
	message = append(message, contextHash[:]...)
	message = append(message, suffix...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	signature := ed25519.Sign(actor.private, digest[:])
	require.True(t, ed25519.Verify(actor.public[:], digest[:], signature))
	payload := append([]byte{0x53, 1, 0, 10}, common...)
	payload = append(payload, 1)
	payload = append(payload, asset.account[:]...)
	payload = append(payload, actor.public[:]...)
	payload = append(payload, signature...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	activity = actor.activity(t, host, 0x00010005, 3, bindingBlockTime, payload)
	require.Equal(t, sha256.Sum256(append(append([]byte(nil), actor.did...), 3, 5)), activity.IdempotencyKey)
	receipt = execute(t, host, chain.context(1, bindingBlockTime, 4), activity, actor.authority())
	require.Equal(t, Code(0), receipt.ResultCode)
	require.Equal(t, uint64(200), accountRows(t, chain)[main].balance)
	return asset, main
}

func TestHostBudgetAuthority(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 103)
	asset, main := authorityFundedMain(t, chain, host, actor)
	budget := [32]byte{0x74}
	name := "agent:" + string(actor.did) + ":budget:" + hex.EncodeToString(budget[:])
	account, err := kernelAccountID([]byte(name))
	require.NoError(t, err)
	payload := make([]byte, 211)
	payload[1] = 1
	copy(payload[2:34], budget[:])
	copy(payload[34:66], account[:])
	copy(payload[66:98], asset.id[:])
	limit, amount := authorityAmount(100), authorityAmount(50)
	copy(payload[130:146], limit[:])
	copy(payload[162:178], amount[:])
	binary.BigEndian.PutUint64(payload[178:186], 1000)
	binary.BigEndian.PutUint64(payload[186:194], uint64(bindingBlockTime))
	binary.BigEndian.PutUint64(payload[194:202], uint64(bindingBlockTime)+10000)
	payload[210] = 1
	authority := actor.authority()
	authority.Kind, authority.Principal = 1, main
	receipt := execute(t, host, chain.context(1, bindingBlockTime, 5),
		actor.activity(t, host, 0x00030001, 4, bindingBlockTime, payload), authority)
	require.Equal(t, Code(0), receipt.ResultCode)
	require.NotEmpty(t, receipt.Encoded)
	record := chain.get(moduleKVKey(3, append([]byte("budget:"), budget[:]...)))
	require.NotEmpty(t, record)
	require.Equal(t, budget[:], record[2:34])
	require.Equal(t, main[:], record[34:66])
	rows := accountRows(t, chain)
	require.Equal(t, uint64(150), rows[main].balance)
	require.Equal(t, uint64(50), rows[account].balance)
	require.Contains(t, receipt.Events, ReceiptEvent{Code: 1, Body: append(append(append([]byte(nil), budget[:]...), asset.id[:]...), amount[:]...)})
}

func TestHostEscrowDelegatedCapture(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	owner := newTestActor(t, 104)
	beneficiary := newTestActor(t, 105)
	delegate := newTestActor(t, 106)
	asset, main := authorityFundedMain(t, chain, host, owner)
	beneficiaryMain := authoritySeedMain(t, chain, beneficiary, asset.id)
	hold := [32]byte{0x75}
	name := "agent:" + string(owner.did) + ":escrow:" + hex.EncodeToString(hold[:])
	custody, err := kernelAccountID([]byte(name))
	require.NoError(t, err)
	payload := make([]byte, 288)
	copy(payload[:32], hold[:])
	copy(payload[32:64], main[:])
	copy(payload[64:96], custody[:])
	copy(payload[96:128], beneficiaryMain[:])
	copy(payload[160:192], asset.id[:])
	amount := authorityAmount(100)
	copy(payload[192:208], amount[:])
	binary.BigEndian.PutUint64(payload[208:216], uint64(bindingBlockTime)+10000)
	authority := owner.authority()
	authority.Kind, authority.Principal = 1, main
	opened := execute(t, host, chain.context(1, bindingBlockTime, 5),
		owner.activity(t, host, 0x00020001, 4, bindingBlockTime, payload), authority)
	require.Equal(t, Code(0), opened.ResultCode)
	require.Equal(t, uint64(100), accountRows(t, chain)[custody].balance)
	capture := make([]byte, 80)
	copy(capture[:32], hold[:])
	copy(capture[32:48], amount[:])
	capture[48] = 0x76
	holdKey := moduleKVKey(2, append([]byte("hold:"), hold[:]...))
	beforeHold := append([]byte(nil), chain.get(holdKey)...)
	require.NotEmpty(t, beforeHold)
	beforeAccounts := rawAccountRows(chain)
	refused := execute(t, host, chain.context(1, bindingBlockTime, 6),
		owner.activity(t, host, 0x00020002, 5, bindingBlockTime, capture), authority)
	require.Equal(t, Code(-708), refused.ResultCode)
	require.Equal(t, "LXP_ERR_UNAUTHORIZED_CAPTURE", refused.ResultCode.Error())
	require.Equal(t, beforeHold, chain.get(holdKey))
	require.Equal(t, beforeAccounts, rawAccountRows(chain))
	nonHolder := delegate.authority()
	nonHolder.Kind = 1
	refused = execute(t, host, chain.context(1, bindingBlockTime, 7),
		delegate.activity(t, host, 0x00020002, 0, bindingBlockTime, capture), nonHolder)
	require.Equal(t, Code(-708), refused.ResultCode)
	require.Equal(t, "LXP_ERR_UNAUTHORIZED_CAPTURE", refused.ResultCode.Error())
	require.Equal(t, beforeHold, chain.get(holdKey))
	require.Equal(t, beforeAccounts, rawAccountRows(chain))
	delegated := delegate.authority()
	delegated.Kind, delegated.Principal = 3, main
	delegated.GrantID, delegated.AuthorityHash = [32]byte{0x77}, [32]byte{0x78}
	delegated.Scope = &AuthorityScope{ModuleMask: 1 << 2, ActivityOrdinalMin: 2, ActivityOrdinalMax: 2, AssetID: asset.id}
	captured := execute(t, host, chain.context(1, bindingBlockTime, 8),
		delegate.activity(t, host, 0x00020002, 1, bindingBlockTime, capture), delegated)
	require.Equal(t, Code(0), captured.ResultCode)
	require.Equal(t, uint8(3), uint8(host.node.execution.authority.kind))
	require.Equal(t, uint64(1<<2), uint64(host.node.execution.authority.scope.module_mask))
	require.Equal(t, uint16(2), uint16(host.node.execution.authority.scope.activity_ordinal_min))
	require.Equal(t, uint16(2), uint16(host.node.execution.authority.scope.activity_ordinal_max))
	rows := accountRows(t, chain)
	require.Zero(t, rows[custody].balance)
	require.Equal(t, uint64(100), rows[main].balance)
	require.Equal(t, uint64(100), rows[beneficiaryMain].balance)
	record := chain.get(holdKey)
	require.Equal(t, byte(3), record[224])
	require.Equal(t, amount[:], record[208:224])
	require.Equal(t, make([]byte, 16), record[192:208])
	require.NotEmpty(t, captured.Encoded)
	require.Contains(t, captured.Events, ReceiptEvent{Code: 2, Body: append(append(append(append([]byte(nil), hold[:]...), 0, 2, 3), amount[:]...), make([]byte, 16)...)})
}
