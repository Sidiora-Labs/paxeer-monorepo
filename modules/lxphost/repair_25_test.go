//go:build cgo

package lxphost_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair25Open(t *testing.T, directory string, epoch uint64) (*proofHostChain, *lxphost.Host, func()) {
	t.Helper()
	db, err := dbm.NewDB("repair25", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err)
	key := sdk.NewKVStoreKey("lxphost")
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	chain := &proofHostChain{key: key, cms: cms, cfg: lxphost.Config{
		StoreKey: key, NetworkID: 7, ParameterVersion: 1, Epoch: epoch,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	}}
	host := chain.open(t)
	closed := false
	closeStore := func() {
		if !closed {
			host.Close()
			require.NoError(t, cms.Close())
			closed = true
		}
	}
	t.Cleanup(closeStore)
	return chain, host, closeStore
}

func repair25Activity(t *testing.T, host *lxphost.Host, private ed25519.PrivateKey, authority lxphost.KernelAuthority, kind uint32, sequence uint64, payload []byte, reference [32]byte, limit uint64) lxphost.Activity {
	t.Helper()
	activity := proofHostActivity(authority, payload)
	activity.Type, activity.AccountSequence, activity.IdempotencyKey = kind, sequence, reference
	binary.BigEndian.PutUint64(activity.FeeLimit[8:], limit)
	hash, err := host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(private, hash[:])
	return activity
}

func repair25Execute(t *testing.T, chain *proofHostChain, host *lxphost.Host, private ed25519.PrivateKey, authority lxphost.KernelAuthority, kind uint32, identitySequence, globalSequence uint64, payload []byte) lxphost.Receipt {
	t.Helper()
	activity := repair25Activity(t, host, private, authority, kind, identitySequence, payload, sha256.Sum256(payload), 0)
	receipt, err := host.Execute(chain.context(globalSequence), activity, authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt
}

func repair25Asset(t *testing.T, chain *proofHostChain, host *lxphost.Host, private ed25519.PrivateKey, authority lxphost.KernelAuthority, amount int64) ([32]byte, [32]byte) {
	t.Helper()
	salt := [32]byte{0x25}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), authority.Actor[:]...), salt[:]...))
	payload, err := ledgertypes.EncodeRegisterPayload(ledgertypes.RegisterArgs{
		AssetID: asset, Salt: salt, Symbol: "R", Name: "Repair", Decimals: 6,
		SupplyCap: sdk.NewInt(amount), IssuerKind: 1,
	})
	require.NoError(t, err)
	registered := repair25Execute(t, chain, host, private, authority, 0x00010001, 0, 1, payload)
	require.Error(t, (lxphost.StoreGenesis{LastSequence: 1, Records: []kv.Pair{{
		Key: append([]byte{4}, asset[:]...), Value: registered.Encoded,
	}}}).Validate(), "generic activity receipts must never restore retired idempotency state")
	repair25Execute(t, chain, host, private, authority, 0x00010004, 1, 2, ledgertypes.EncodeOpenAccountPayload(asset))
	_, payer, err := lxphost.AssetAccountID(authority.VerifiedKey, asset)
	require.NoError(t, err)
	payload, err = ledgertypes.EncodeSupplyPayload(asset, payer, sdk.NewInt(amount))
	require.NoError(t, err)
	repair25Execute(t, chain, host, private, authority, 0x0001000a, 2, 3, payload)
	return asset, payer
}

func repair25Send(t *testing.T, host *lxphost.Host, private ed25519.PrivateKey, authority lxphost.KernelAuthority, asset, from, to, reference [32]byte, identitySequence, accountSequence uint64) lxphost.Activity {
	t.Helper()
	args := ledgertypes.SendArgs{
		From: from, To: to, Asset: asset, Amount: sdk.NewInt(3), Sequence: accountSequence,
		IdempotencyKey: reference, ExpiresAt: uint64(proofHostTime + 1000),
		PublicKey: authority.VerifiedKey, NetworkID: 7, ProtocolVersion: 3,
	}
	payload, err := ledgertypes.EncodeSendPayload(args)
	require.NoError(t, err)
	material := append(bytes.Clone(payload[4:116]), reference[:]...)
	args.ContextHash, err = codec.DomainHash(codec.DomainContextHash, material)
	require.NoError(t, err)
	payload, err = ledgertypes.EncodeSendPayload(args)
	require.NoError(t, err)
	message := append(bytes.Clone(payload[:2]), payload[4:230]...)
	message = append(message, payload[326:364]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(payload[262:326], ed25519.Sign(private, digest[:]))
	return repair25Activity(t, host, private, authority, 0x00010005, identitySequence, payload, reference, 0)
}

func TestRepair_25_payment_reference_genesis(t *testing.T) {
	for _, bound := range []bool{false, true} {
		name := "signed"
		if bound {
			name = "bound"
		}
		t.Run(name, func(t *testing.T) {
			chain, host, closeSource := repair25Open(t, t.TempDir(), 0)
			private, payerAuthority := proofHostAuthority(t, 121)
			payeePrivate, payeeAuthority := proofHostAuthority(t, 122)
			asset, payer := repair25Asset(t, chain, host, private, payerAuthority, 25)
			repair25Execute(t, chain, host, payeePrivate, payeeAuthority, 0x00010004, 0, 4, ledgertypes.EncodeOpenAccountPayload(asset))
			_, payee, err := lxphost.AssetAccountID(payeeAuthority.VerifiedKey, asset)
			require.NoError(t, err)
			reference := [32]byte{0x31}
			payment := repair25Send(t, host, private, payerAuthority, asset, payer, payee, reference, 3, 0)
			if bound {
				clear(payment.Payload[262:326])
				payment.Signature = nil
			}
			paid, err := host.Execute(chain.context(5), payment, payerAuthority)
			require.NoError(t, err)
			require.Zero(t, paid.ResultCode)
			require.Equal(t, []lxphost.TransferLeg{{Asset: asset, From: payer, To: payee, Amount: sdk.NewInt(3)}}, paid.Transfers)
			chain.cms.Commit(true)
			module := lxphost.NewStoreAppModule(chain.key)
			document := module.ExportGenesis(chain.context(6), nil)
			require.NoError(t, (lxphost.StoreAppModuleBasic{}).ValidateGenesis(nil, nil, document))
			var genesis lxphost.StoreGenesis
			require.NoError(t, json.Unmarshal(document, &genesis))
			key := append([]byte{4}, reference[:]...)
			var references []kv.Pair
			for _, row := range genesis.Records {
				if row.Key[0] == 4 {
					references = append(references, row)
				}
			}
			require.Equal(t, []kv.Pair{{Key: key, Value: paid.Encoded}}, references)
			closeSource()

			directory := t.TempDir()
			restored, importedHost, closeImported := repair25Open(t, directory, 0)
			lxphost.NewStoreAppModule(restored.key).InitGenesis(restored.context(6), nil, document)
			require.Equal(t, document, lxphost.NewStoreAppModule(restored.key).ExportGenesis(restored.context(6), nil))
			restored.cms.Commit(true)
			commitID := restored.cms.LastCommitID()
			closeImported()
			restored, importedHost, _ = repair25Open(t, directory, 0)
			require.Equal(t, commitID, restored.cms.LastCommitID())
			require.Equal(t, paid.Encoded, restored.context(6).KVStore(restored.key).Get(key))
			before := restored.rows()
			duplicate := repair25Send(t, importedHost, private, payerAuthority, asset, payer, payee, reference, 4, 1)
			refused, err := importedHost.Execute(restored.context(6), duplicate, payerAuthority)
			require.ErrorIs(t, err, lxphost.Code(-302))
			require.Empty(t, refused.Encoded)
			require.Equal(t, before, restored.rows())
			fresh := repair25Send(t, importedHost, private, payerAuthority, asset, payer, payee, [32]byte{0x32}, 4, 1)
			accepted, err := importedHost.Execute(restored.context(6), fresh, payerAuthority)
			require.NoError(t, err)
			require.Zero(t, accepted.ResultCode)
			require.Equal(t, paid.Transfers, accepted.Transfers)
			for account, want := range map[[32]byte]int64{payer: 19, payee: 6} {
				balance, err := importedHost.ReadBalance(restored.context(7), account, asset)
				require.NoError(t, err)
				require.Equal(t, sdk.NewInt(want), balance)
			}

			for name, row := range map[string]kv.Pair{
				"short key":          {Key: key[:32], Value: paid.Encoded},
				"legacy snapshot":    {Key: key, Value: []byte{1, 2, 3}},
				"truncated receipt":  {Key: key, Value: paid.Encoded[:len(paid.Encoded)-1]},
				"reference mismatch": {Key: append([]byte{4}, bytes.Repeat([]byte{0x33}, 32)...), Value: paid.Encoded},
			} {
				t.Run(name, func(t *testing.T) {
					invalid := lxphost.StoreGenesis{LastSequence: 5, Records: []kv.Pair{row}}
					before := restored.rows()
					require.Error(t, invalid.Validate())
					require.Error(t, lxphost.InitStoreGenesis(restored.context(7), restored.key, invalid))
					require.Equal(t, before, restored.rows())
				})
			}
			require.Error(t, (lxphost.StoreGenesis{LastSequence: 4, Records: references}).Validate())
			corrupt, _ := restored.context(7).CacheContext()
			corrupt.KVStore(restored.key).Set(key, paid.Encoded[:len(paid.Encoded)-1])
			exported, err := lxphost.ExportStoreGenesis(corrupt, restored.key)
			require.Error(t, err, "corrupt current payment records must fail export")
			require.Nil(t, exported)
			registration, err := codec.DecodeReceipt(paid.Encoded)
			require.NoError(t, err)
			require.Equal(t, uint16(3), registration.ProtocolVersion)
			require.Equal(t, uint8(5), registration.Operation)
		})
	}
}

func repair25ModuleKey(key []byte) []byte { return append([]byte{1, 0, 9}, key...) }

func repair25Account(t *testing.T, chain *proofHostChain, name string, asset, authority [32]byte, amount uint64) [32]byte {
	t.Helper()
	id, err := codec.DeriveAccountID([]byte(name))
	require.NoError(t, err)
	kind, ok := codec.AccountKind([]byte(name))
	require.True(t, ok)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	record := field(nil, id[:])
	record = field(record, []byte(name))
	record = append(record, kind)
	record = binary.BigEndian.AppendUint64(record, 0)
	record = binary.BigEndian.AppendUint64(record, amount)
	record = field(record, asset[:])
	record = append(record, 1)
	record = binary.BigEndian.AppendUint64(record, 0)
	record = binary.BigEndian.AppendUint64(record, 1)
	record = append(record, 0, 0)
	record = field(record, authority[:])
	present := byte(0)
	if authority != [32]byte{} {
		present = 1
	}
	record = append(record, present)
	row := kv.Pair{Key: append([]byte{6}, id[:]...), Value: record}
	require.NoError(t, lxphost.InitStoreGenesis(chain.context(4), chain.key, lxphost.StoreGenesis{Records: []kv.Pair{row}}))
	return id
}

func repair25Schedules(chain *proofHostChain, asset, authority [32]byte) {
	store := chain.context(4).KVStore(chain.key)
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, asset[:]...)
	for _, v := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, v)
	}
	fee = append(fee, authority[:]...)
	fee = append(fee, make([]byte, 16)...)
	store.Set(repair25ModuleKey([]byte("progfee/active/v1")), fee)
	store.Set(repair25ModuleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, v := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8, 1} {
		metering = binary.BigEndian.AppendUint64(metering, v)
	}
	metering = append(metering, 1)
	metering = append(metering, authority[:]...)
	store.Set(repair25ModuleKey([]byte("progmet/active/v1")), metering)
	store.Set(repair25ModuleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), metering)
	profile := binary.BigEndian.AppendUint32([]byte("LXMB1"), 1)
	profile = binary.BigEndian.AppendUint32(profile, 4)
	profile = binary.BigEndian.AppendUint32(profile, 1)
	profile = binary.BigEndian.AppendUint64(profile, 1)
	for _, v := range []uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096} {
		profile = binary.BigEndian.AppendUint64(profile, v)
	}
	profile = append(profile, authority[:]...)
	profile = binary.BigEndian.AppendUint64(profile, 1)
	profile = binary.BigEndian.AppendUint64(profile, 0)
	history := binary.BigEndian.AppendUint32([]byte("progmig/history/v1/"), 4)
	history = binary.BigEndian.AppendUint32(history, 1)
	history = binary.BigEndian.AppendUint64(history, 1)
	store.Set(repair25ModuleKey(history), profile)
	store.Set(repair25ModuleKey([]byte("progmig/active/v1")), profile)
}

func repair25Wasm(trap bool, marker byte) []byte {
	wasm := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	section := func(id byte, body []byte) { wasm = append(wasm, id, byte(len(body))); wasm = append(wasm, body...) }
	section(1, []byte{3, 0x60, 2, 0x7f, 0x7f, 1, 0x7f, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 0, 0})
	section(3, []byte{3, 0, 1, 2})
	section(5, []byte{1, 0, 1})
	exports := []byte{4}
	for _, export := range []struct {
		name        string
		kind, index byte
	}{{"layerx_call", 0, 0}, {"layerx_reserve", 0, 1}, {"memory", 2, 0}, {"migrate", 0, 2}} {
		exports = append(exports, byte(len(export.name)))
		exports = append(exports, export.name...)
		exports = append(exports, export.kind, export.index)
	}
	section(7, exports)
	instruction := byte(1)
	if trap {
		instruction = 0
	}
	section(10, []byte{3, 4, 0, 0x41, 0, 0x0b, 4, 0, 0x41, 0, 0x0b, 3, 0, instruction, 0x0b})
	section(0, []byte{1, 'r', marker})
	return wasm
}

func TestRepair_25_migration_fee_binding(t *testing.T) {
	directory := t.TempDir()
	chain, host, closeStore := repair25Open(t, directory, 1)
	private, authority := proofHostAuthority(t, 123)
	authority.Kind = lxphost.AuthorityKindOwner
	var err error
	authority.AuthorityHash, err = lxphost.AuthorityHash(authority.Kind, authority.GrantID, authority.VerifiedKey)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, authority.AuthorityHash)
	const funding = int64(1000000000)
	asset, payer := repair25Asset(t, chain, host, private, authority, funding)
	name, main, err := lxphost.MainAccountID(authority.VerifiedKey)
	require.NoError(t, err)
	admissionAsset := [32]byte{0x91}
	require.Equal(t, main, repair25Account(t, chain, name, admissionAsset, authority.VerifiedKey, uint64(funding)))
	treasury := repair25Account(t, chain, "system:fees", asset, [32]byte{}, 0)
	repair25Schedules(chain, asset, authority.Principal)
	program := sha256.Sum256([]byte("repair25-migration"))
	oldWasm := repair25Wasm(false, 1)
	oldHash := sha256.Sum256(oldWasm)
	payload, err := lp.EncodeDeployPayload(program, 1, 1, authority.Principal, oldHash, nil, oldWasm)
	require.NoError(t, err)
	repair25Execute(t, chain, host, private, authority, 0x00090001, 3, 4, payload)
	chain.cms.Commit(true)
	before := chain.rows()
	upgrade := func(wasm []byte, ref byte) lxphost.Activity {
		t.Helper()
		payload, err := lp.EncodeUpgradePayload(program, 1, 1, oldHash, sha256.Sum256(wasm), []byte("migrate"), nil, wasm)
		require.NoError(t, err)
		return repair25Activity(t, host, private, authority, 0x00090002, 4, payload, [32]byte{ref}, 100000000)
	}
	failedContext, _ := chain.context(5).CacheContext()
	failed, err := host.Execute(failedContext, upgrade(repair25Wasm(true, 2), 0x41), authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(-3), failed.ResultCode)
	require.True(t, failed.FeeCharged.IsZero())
	iterator := failedContext.KVStore(chain.key).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		if iterator.Key()[0] != 2 && iterator.Key()[0] != 3 {
			require.Equal(t, before[string(iterator.Key())], iterator.Value(), "failed migration changed %x", iterator.Key())
		}
	}
	require.NoError(t, iterator.Close())
	require.Equal(t, before, chain.rows(), "failed migration must roll back balances, program, blobs and sequences")
	nextWasm := repair25Wasm(false, 3)
	migration := upgrade(nextWasm, 0x42)
	overflowContext, _ := chain.context(5).CacheContext()
	treasuryKey := append([]byte{6}, treasury[:]...)
	treasuryRecord := bytes.Clone(overflowContext.KVStore(chain.key).Get(treasuryKey))
	balanceOffset := 4 + 32 + 4 + len("system:fees") + 1
	copy(treasuryRecord[balanceOffset:balanceOffset+16], bytes.Repeat([]byte{0xff}, 16))
	overflowContext.KVStore(chain.key).Set(treasuryKey, treasuryRecord)
	overflowBefore := map[string][]byte{}
	iterator = overflowContext.KVStore(chain.key).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		overflowBefore[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	require.NoError(t, iterator.Close())
	refused, err := host.Execute(overflowContext, migration, authority)
	require.ErrorIs(t, err, lxphost.Code(-500), "fee credit overflow after migration must abort the transaction")
	require.Empty(t, refused.Encoded)
	overflowAfter := map[string][]byte{}
	iterator = overflowContext.KVStore(chain.key).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		overflowAfter[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	require.NoError(t, iterator.Close())
	require.Equal(t, overflowBefore, overflowAfter, "fee preparation failure must roll back the migration and payer debit")
	require.Equal(t, before, chain.rows())
	cached, _ := chain.context(5).CacheContext()
	rolledBack, err := host.Execute(cached, migration, authority)
	require.NoError(t, err)
	require.Zero(t, rolledBack.ResultCode)
	require.True(t, rolledBack.FeeCharged.IsPositive())
	require.Equal(t, before, chain.rows(), "discarded transaction must roll back fee and migration together")
	paid, err := host.Execute(chain.context(5), migration, authority)
	require.NoError(t, err)
	require.Zero(t, paid.ResultCode)
	require.Equal(t, rolledBack.FeeCharged, paid.FeeCharged)
	require.Equal(t, rolledBack.Encoded, paid.Encoded)
	decoded, err := codec.DecodeReceipt(paid.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(9), decoded.ModuleID)
	accountingKey := repair25ModuleKey(append([]byte("progmig/account/v1/"), decoded.ActivityID[:]...))
	accounting := chain.context(6).KVStore(chain.key).Get(accountingKey)
	require.Len(t, accounting, 613)
	require.Equal(t, payer[:], accounting[189:221])
	require.Equal(t, asset[:], accounting[221:253])
	assertRecovered := func() {
		t.Helper()
		for account, want := range map[[32]byte]sdk.Int{
			payer: sdk.NewInt(funding).Sub(paid.FeeCharged), treasury: paid.FeeCharged,
		} {
			balance, err := host.ReadBalance(chain.context(6), account, asset)
			require.NoError(t, err)
			require.Equal(t, want, balance)
		}
		admissionBalance, err := host.ReadBalance(chain.context(6), main, admissionAsset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(funding), admissionBalance, "fee must debit the schedule's actual payer")
		record, err := host.ReadProgram(chain.context(6), program)
		require.NoError(t, err)
		require.Equal(t, sha256.Sum256(nextWasm), record.CodeHash)
		require.Equal(t, uint32(2), record.Version)
		require.Equal(t, accounting, chain.context(6).KVStore(chain.key).Get(accountingKey))
	}
	assertRecovered()
	chain.cms.Commit(true)
	commitID := chain.cms.LastCommitID()
	committed := chain.rows()
	closeStore()
	chain, host, _ = repair25Open(t, directory, 1)
	require.Equal(t, commitID, chain.cms.LastCommitID())
	require.Equal(t, committed, chain.rows())
	assertRecovered()
	duplicate, err := host.Execute(chain.context(6), migration, authority)
	require.ErrorIs(t, err, lxphost.Code(-301))
	require.Empty(t, duplicate.Encoded)
	require.Equal(t, committed, chain.rows())
}
