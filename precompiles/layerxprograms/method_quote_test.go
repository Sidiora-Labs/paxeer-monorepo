package layerxprograms

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func quoteModuleKey(key []byte) []byte {
	return append([]byte{1, 0, 9}, key...)
}

func quoteFixture(t *testing.T) (sdk.Context, *sdk.KVStoreKey, *PrecompileExecutor) {
	t.Helper()
	key := sdk.NewKVStoreKey("programs_quote")
	cms := store.NewCommitMultiStore(dbm.NewMemDB())
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "programs-quote", Height: 1, Time: time.UnixMilli(1700000000000)}, false)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	return ctx, key, &PrecompileExecutor{host: host}
}

func quoteFeeRecord(version uint32, prices [7]uint64, asset [32]byte) []byte {
	record := binary.BigEndian.AppendUint32([]byte("LXFR1"), version)
	for _, price := range prices {
		record = binary.BigEndian.AppendUint64(record, price)
	}
	record = append(record, asset[:]...)
	for _, word := range []uint64{1000, 8, 1, 8, 1, math.MaxUint64, 1, 0, 1} {
		record = binary.BigEndian.AppendUint64(record, word)
	}
	record = append(record, bytes.Repeat([]byte{0xa5}, 32)...)
	return append(record, make([]byte, 16)...)
}

func quoteResult(t *testing.T, ctx sdk.Context, p *PrecompileExecutor, budget [7]uint64) (uint32, *big.Int, [32]byte) {
	t.Helper()
	input, err := programsABI.Pack("quoteCallFee", budget)
	require.NoError(t, err)
	method, err := programsABI.MethodById(input[:4])
	require.NoError(t, err)
	args, err := method.Inputs.Unpack(input[4:])
	require.NoError(t, err)
	encoded, err := p.Execute(ctx, method, common.Address{}, common.Address{}, args, nil, true, nil, nil)
	require.NoError(t, err)
	values, err := programsABI.Methods["quoteCallFee"].Outputs.Unpack(encoded)
	require.NoError(t, err)
	require.Len(t, values, 3)
	return values[0].(uint32), values[1].(*big.Int), values[2].([32]byte)
}

func quoteExecute(ctx sdk.Context, p *PrecompileExecutor, args []interface{}) ([]byte, error) {
	method := programsABI.Methods["quoteCallFee"]
	return p.Execute(ctx, &method, common.Address{}, common.Address{}, args, nil, true, nil, nil)
}

func TestProgramsQuoteCallFeeMatchesKernelCeiling(t *testing.T) {
	ctx, key, p := quoteFixture(t)
	prices := [7]uint64{2, 3, 5, 7, 11, 13, 17}
	record := quoteFeeRecord(0x01020304, prices, [32]byte{0x55})
	activeKey := quoteModuleKey([]byte("progfee/active/v1"))
	ctx.KVStore(key).Set(activeKey, record)
	for _, tc := range []struct {
		name   string
		budget [7]uint64
		want   string
	}{
		{"zero", [7]uint64{}, "0"},
		{"all dimensions", [7]uint64{19, 23, 29, 31, 37, 41, 43}, "2140"},
		{"wide product", [7]uint64{math.MaxUint64}, "36893488147419103230"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			version, ceiling, digest := quoteResult(t, ctx, p, tc.budget)
			require.Equal(t, uint32(0x01020304), version)
			require.Equal(t, tc.want, ceiling.String())
			require.Equal(t, sha256.Sum256(record), digest)
			require.Equal(t, record, ctx.KVStore(key).Get(activeKey))
		})
	}
	for i, price := range prices {
		budget := [7]uint64{}
		budget[i] = 1
		_, ceiling, _ := quoteResult(t, ctx, p, budget)
		require.Equal(t, new(big.Int).SetUint64(price), ceiling, "dimension %d", i)
	}
	updated := quoteFeeRecord(0x01020305, [7]uint64{19, 23, 29, 31, 37, 41, 43}, [32]byte{0x56})
	ctx.KVStore(key).Set(activeKey, updated)
	version, ceiling, digest := quoteResult(t, ctx, p, [7]uint64{1, 1, 1, 1, 1, 1, 1})
	require.Equal(t, uint32(0x01020305), version)
	require.Equal(t, "223", ceiling.String())
	require.Equal(t, sha256.Sum256(updated), digest)

	maximum := [7]uint64{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	ctx.KVStore(key).Set(activeKey, quoteFeeRecord(1, maximum, [32]byte{1}))
	_, ceiling, _ = quoteResult(t, ctx, p, [7]uint64{math.MaxUint64})
	require.Equal(t, "340282366920938463426481119284349108225", ceiling.String())
	_, ceiling, _ = quoteResult(t, ctx, p, [7]uint64{math.MaxUint64, 2})
	require.Equal(t, "340282366920938463463374607431768211455", ceiling.String())
	encoded, err := quoteExecute(ctx, p, []interface{}{[7]uint64{math.MaxUint64, 3}})
	require.ErrorContains(t, err, "LXP_OVERFLOW:")
	require.Nil(t, encoded)
	encoded, err = quoteExecute(ctx, p, []interface{}{maximum})
	require.ErrorContains(t, err, "LXP_OVERFLOW:")
	require.Nil(t, encoded)
	for _, args := range [][]interface{}{nil, {uint64(1)}, {[7]uint64{}, [7]uint64{}}} {
		encoded, err := quoteExecute(ctx, p, args)
		require.Error(t, err)
		require.Nil(t, encoded)
	}
	invalidDemand := quoteFeeRecord(1, prices, [32]byte{1})
	binary.BigEndian.PutUint64(invalidDemand[97:105], 0)
	for _, invalid := range [][]byte{[]byte("LXFR1"), append([]byte("LXFR2"), record[5:]...), quoteFeeRecord(0, prices, [32]byte{1}), quoteFeeRecord(1, [7]uint64{}, [32]byte{1}), quoteFeeRecord(1, prices, [32]byte{}), invalidDemand} {
		ctx.KVStore(key).Set(activeKey, invalid)
		encoded, err := quoteExecute(ctx, p, []interface{}{[7]uint64{}})
		require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		require.Nil(t, encoded)
	}
	encoded, err = quoteExecute(ctx, &PrecompileExecutor{}, []interface{}{[7]uint64{}})
	require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT:")
	require.Nil(t, encoded)
	ctx.KVStore(key).Delete(activeKey)
	encoded, err = quoteExecute(ctx, p, []interface{}{[7]uint64{}})
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.Nil(t, encoded)
}

func TestProgramsQuoteBoundsCallFee(t *testing.T) {
	ctx, key, p := quoteFixture(t)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x63}, 32))
	public := [32]byte(private.Public().(ed25519.PublicKey))
	did := []byte("did:layerx:" + hex.EncodeToString(public[:]))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	authority := lxphost.KernelAuthority{Actor: actor, Principal: actor, VerifiedKey: public, AuthorityHash: sha256.Sum256(public[:])}
	activity := func(kind uint32, sequence uint64, payload []byte, limit *big.Int) lxphost.Activity {
		value := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: did, AuthorityKey: public,
			AccountSequence: sequence, NotBefore: 1699999999000, NotAfter: 1700000001000,
			IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint32(nil, kind), sequence)), Payload: payload}
		if limit != nil {
			limit.FillBytes(value.FeeLimit[:])
		}
		hash, err := p.host.SigningHash(value)
		require.NoError(t, err)
		value.Signature = ed25519.Sign(private, hash[:])
		return value
	}
	execute := func(sequence uint64, value lxphost.Activity) lxphost.Receipt {
		receipt, err := p.host.Execute(lxphost.WithSequence(ctx, sequence), value, authority)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		return receipt
	}
	meter := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		meter = binary.BigEndian.AppendUint64(meter, coefficient)
	}
	meter = append(binary.BigEndian.AppendUint64(meter, 1), 1)
	meter = append(meter, bytes.Repeat([]byte{0xa5}, 32)...)
	ctx.KVStore(key).Set(quoteModuleKey([]byte("progmet/active/v1")), meter)
	ctx.KVStore(key).Set(quoteModuleKey(binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)), meter)
	register := append(append([]byte{0x71, 1, 0, 2}, actor[:]...), public[:]...)
	execute(41, activity(0x00070001, 0, register, nil))
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor[:]...), bytes.Repeat([]byte{0x55}, 32)...))
	registerAsset := append(append([]byte{0, 1}, asset[:]...), bytes.Repeat([]byte{0x55}, 32)...)
	registerAsset = append(registerAsset, 1, 'T', 1, 'T', 6)
	registerAsset = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(registerAsset, 0), 1000000000)
	registerAsset = append(registerAsset, 1, 0)
	execute(42, activity(0x00010001, 1, registerAsset, nil))
	execute(43, activity(0x00010004, 2, append([]byte{0, 1}, asset[:]...), nil))
	_, assetAccount, err := lxphost.AssetAccountID(public, asset)
	require.NoError(t, err)
	mint := append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	mint = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mint, 0), 500000000)
	execute(44, activity(0x0001000a, 3, mint, nil))
	prices := [7]uint64{1, 1, 1, 1, 1, 1, 1}
	record := quoteFeeRecord(1, prices, asset)
	ctx.KVStore(key).Set(quoteModuleKey([]byte("progfee/active/v1")), record)
	ctx.KVStore(key).Set(quoteModuleKey(binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), record)
	treasury, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	account := append(field(field(nil, treasury[:]), []byte("system:fees")), 10)
	account = append(account, make([]byte, 16)...)
	account = append(field(account, asset[:]), 1)
	account = append(account, make([]byte, 16)...)
	account = append(account, 0, 0)
	account = append(field(account, make([]byte, 32)), 0)
	ctx.KVStore(key).Set(append([]byte{6}, treasury[:]...), account)
	feeAccount, err := p.host.ReadAccount(ctx, "system:fees", treasury)
	require.NoError(t, err)
	require.False(t, feeAccount.Frozen)
	require.Equal(t, asset, feeAccount.AssetID)
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	program := [32]byte(bytes.Repeat([]byte{0x71}, 32))
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = append(iface, 0, 1, 0, 1, 0, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
	deploy := append(append([]byte(nil), program[:]...), 0, 1, 1, 0)
	deploy = append(append(deploy, actor[:]...), codeHash[:]...)
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(wasm)))
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(iface)))
	deploy = append(append(deploy, iface...), wasm...)
	execute(45, activity(0x00090001, 4, deploy, nil))
	ctx.KVStore(key).Set(quoteModuleKey(append([]byte("program-owner\x00"), program[:]...)), append([]byte{1}, actor[:]...))
	mainName, mainID, err := lxphost.MainAccountID(public)
	require.NoError(t, err)
	mainAsset := [32]byte{0x91}
	mainAccount := append(field(field(nil, mainID[:]), []byte(mainName)), 1)
	mainAccount = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mainAccount, 0), 500000000)
	mainAccount = append(field(mainAccount, mainAsset[:]), 1)
	mainAccount = binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(mainAccount, 5), 41)
	mainAccount = append(mainAccount, 0, 0)
	mainAccount = append(field(mainAccount, public[:]), 1)
	ctx.KVStore(key).Set(append([]byte{6}, mainID[:]...), mainAccount)
	budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
	_, ceiling, _ := quoteResult(t, ctx, p, budget)
	payer, err := p.host.ReadAccount(ctx, mainName, mainID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, payer.Balance.BigInt().Cmp(ceiling), 0)
	payload, err := EncodeCallPayload(program, 1, "layerx_call", nil, []byte{0, 0}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), 16, budget)
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		limit *big.Int
		want  lxphost.Code
	}{
		{"quoted ceiling", ceiling, 0},
		{"below execution ceiling", new(big.Int).Sub(new(big.Int).Sub(new(big.Int).Set(ceiling), new(big.Int).SetUint64(budget[6])), big.NewInt(1)), -207},
	} {
		t.Run(tc.name, func(t *testing.T) {
			branch, _ := ctx.CacheContext()
			receipt, err := p.host.Execute(lxphost.WithSequence(branch, 46), activity(0x00090003, 5, payload, tc.limit), authority)
			require.NoError(t, err)
			require.Equal(t, tc.want, receipt.ResultCode)
			require.NotEmpty(t, receipt.Encoded)
		})
	}
}
