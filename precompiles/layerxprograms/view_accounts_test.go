package layerxprograms

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type accountViewFixture struct {
	ctx       sdk.Context
	key       *sdk.KVStoreKey
	executor  *PrecompileExecutor
	program   [32]byte
	asset     [32]byte
	principal [32]byte
}

func newAccountViewFixture(t *testing.T) *accountViewFixture {
	t.Helper()
	key := sdk.NewKVStoreKey("programs-account-views")
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 100, Time: time.UnixMilli(1700000000123)}, false)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	f := &accountViewFixture{ctx: ctx, key: key, executor: &PrecompileExecutor{host: host}}
	copy(f.program[:], bytes.Repeat([]byte{0x71}, 32))
	copy(f.asset[:], bytes.Repeat([]byte{0xaa}, 32))
	copy(f.principal[:], bytes.Repeat([]byte{0x31}, 32))
	return f
}

func accountViewModuleKey(prefix string, id [32]byte) []byte {
	return append(append([]byte{1, 0, 9}, []byte(prefix)...), id[:]...)
}

func (f *accountViewFixture) register(t *testing.T, idHex string, seed []byte, sequence uint64) [32]byte {
	t.Helper()
	idBytes, err := hex.DecodeString(idHex)
	require.NoError(t, err)
	require.Len(t, idBytes, 32)
	var id [32]byte
	copy(id[:], idBytes)
	seedDigest := sha256.Sum256(seed)
	event := append([]byte("LXPA1"), f.program[:]...)
	event = append(event, id[:]...)
	event = append(event, f.asset[:]...)
	event = binary.BigEndian.AppendUint16(event, uint16(len(seed)))
	event = append(event, seedDigest[:]...)
	event = binary.BigEndian.AppendUint64(event, sequence)
	digest := sha256.Sum256(event)
	binding := append([]byte{2}, f.program[:]...)
	binding = append(binding, id[:]...)
	binding = append(binding, f.asset[:]...)
	binding = binary.BigEndian.AppendUint16(binding, uint16(len(seed)))
	binding = binary.BigEndian.AppendUint64(binding, sequence)
	binding = append(binding, digest[:]...)
	binding = append(binding, seed...)
	kv := f.ctx.KVStore(f.key)
	kv.Set(append(accountViewModuleKey("program-account\x00p", f.program), seedDigest[:]...), binding)
	kv.Set(accountViewModuleKey("program-account\x00r", id), binding)
	kv.Set(accountViewModuleKey("program-owner\x00", f.program), append([]byte{1}, f.principal[:]...))
	kv.Set(accountViewModuleKey("program-account-profile\x00", f.program), append([]byte{2}, f.principal[:]...))
	f.setBalance(id, new(big.Int), false)
	return id
}

func (f *accountViewFixture) setBalance(id [32]byte, balance *big.Int, frozen bool) {
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	account := field(nil, id[:])
	account = field(account, []byte("module:programs:value:"+hex.EncodeToString(id[:])))
	account = append(account, 13)
	amount := make([]byte, 16)
	balance.FillBytes(amount)
	account = append(account, amount...)
	account = field(account, f.asset[:])
	account = append(account, 1)
	account = binary.BigEndian.AppendUint64(account, 7)
	account = binary.BigEndian.AppendUint64(account, 42)
	if frozen {
		account = append(account, 1)
	} else {
		account = append(account, 0)
	}
	account = append(account, 0)
	account = field(account, make([]byte, 32))
	account = append(account, 0)
	f.ctx.KVStore(f.key).Set(append([]byte{6}, id[:]...), account)
}

func (f *accountViewFixture) snapshot() map[string][]byte {
	values := make(map[string][]byte)
	iterator := f.ctx.KVStore(f.key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func (f *accountViewFixture) view(t *testing.T, name string, handler func(sdk.Context, *PrecompileExecutor, callContext, []interface{}) ([]byte, error), args ...interface{}) ([]interface{}, error) {
	t.Helper()
	method := programsABI.Methods[name]
	packed, err := method.Inputs.Pack(args...)
	require.NoError(t, err)
	decoded, err := method.Inputs.Unpack(packed)
	require.NoError(t, err)
	output, err := handler(f.ctx, f.executor, callContext{method: &method}, decoded)
	if err != nil {
		require.Nil(t, output)
		return nil, err
	}
	return method.Outputs.Unpack(output)
}

func TestProgramsViewProgramAccountLookups(t *testing.T) {
	f := newAccountViewFixture(t)
	maxSeed := make([]byte, 128)
	for i := range maxSeed {
		maxSeed[i] = byte(i)
	}
	for i, tc := range []struct {
		name string
		seed []byte
		id   string
	}{
		{"binary seed", []byte("vault\x00reserve"), "09eb48e764b8ae13a39eea521e522a2ea6b2d11d56f4aefcd20749c7827ba9cc"},
		{"empty seed", []byte{}, "cfadced9229eddc2ba2683d27b285a404aa45963ff945e8f4c654758833e0170"},
		{"maximum seed", maxSeed, "03b0fee1b382429b918938acae35286dc97babfc745eff94194b93e32c31368d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sequence := uint64(1<<63) + uint64(i)
			id := f.register(t, tc.id, tc.seed, sequence)
			before := f.snapshot()
			bySeed, err := f.view(t, "getProgramAccount", getProgramAccountHandler, f.program, f.asset, tc.seed)
			require.NoError(t, err)
			require.Equal(t, []interface{}{id, f.asset, sequence}, bySeed)
			byID, err := f.view(t, "getProgramAccountById", getProgramAccountByIdHandler, bySeed[0])
			require.NoError(t, err)
			require.Equal(t, []interface{}{f.program, f.asset, tc.seed}, byID)
			profile, err := f.view(t, "getAccountProfile", getAccountProfileHandler, byID[0])
			require.NoError(t, err)
			require.Equal(t, []interface{}{uint8(2), f.principal}, profile)
			_, err = f.view(t, "getProgramAccount", getProgramAccountHandler, f.program, [32]byte{9}, tc.seed)
			require.ErrorIs(t, err, lxphost.Code(-402))
			require.Equal(t, before, f.snapshot())
		})
	}
	before := f.snapshot()
	_, err := f.view(t, "getProgramAccount", getProgramAccountHandler, f.program, f.asset, []byte("missing"))
	require.ErrorContains(t, err, "LXP_NOT_FOUND:")
	_, err = f.view(t, "getProgramAccountById", getProgramAccountByIdHandler, [32]byte{9})
	require.ErrorContains(t, err, "LXP_NOT_FOUND:")
	_, err = f.view(t, "getAccountProfile", getAccountProfileHandler, [32]byte{9})
	require.ErrorContains(t, err, "LXP_NOT_FOUND:")
	_, err = f.view(t, "getProgramAccount", getProgramAccountHandler, f.program, f.asset, make([]byte, 129))
	require.ErrorIs(t, err, lxphost.Code(-5))
	method := programsABI.Methods["getProgramAccount"]
	_, err = getProgramAccountHandler(f.ctx, f.executor, callContext{method: &method}, []interface{}{f.program})
	require.ErrorContains(t, err, "LXP_PRECOMPILE_ARG_COUNT:")
	require.Equal(t, before, f.snapshot())
}

func TestProgramsViewValueAccountBalance(t *testing.T) {
	f := newAccountViewFixture(t)
	t.Run("never funded", func(t *testing.T) {
		before := f.snapshot()
		actual, err := f.view(t, "getValueAccount", getValueAccountHandler, f.program, [32]byte{9})
		require.NoError(t, err)
		require.Equal(t, []interface{}{new(big.Int).SetBytes(make([]byte, 16)), false}, actual)
		require.Equal(t, before, f.snapshot())
	})
	id := f.register(t, "09eb48e764b8ae13a39eea521e522a2ea6b2d11d56f4aefcd20749c7827ba9cc", []byte("vault\x00reserve"), 42)
	for _, tc := range []struct {
		name    string
		balance *big.Int
		frozen  bool
	}{
		{"existing zero balance", new(big.Int).SetBytes(make([]byte, 16)), false},
		{"funded with ten units", big.NewInt(10), false},
		{"more than uint64", new(big.Int).Add(new(big.Int).Lsh(big.NewInt(3), 64), big.NewInt(250)), false},
		{"maximum uint128 frozen", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.setBalance(id, tc.balance, tc.frozen)
			before := f.snapshot()
			actual, err := f.view(t, "getValueAccount", getValueAccountHandler, f.program, id)
			require.NoError(t, err)
			require.Equal(t, []interface{}{tc.balance, true}, actual)
			require.Equal(t, before, f.snapshot())
		})
	}
	before := f.snapshot()
	actual, err := f.view(t, "getValueAccount", getValueAccountHandler, f.program, [32]byte{9})
	require.NoError(t, err)
	require.Equal(t, []interface{}{new(big.Int).SetBytes(make([]byte, 16)), false}, actual)
	_, err = f.view(t, "getValueAccount", getValueAccountHandler, [32]byte{9}, id)
	require.ErrorIs(t, err, lxphost.Code(-213))
	require.Equal(t, before, f.snapshot())
	f.executor.host.Close()
	_, err = f.view(t, "getValueAccount", getValueAccountHandler, f.program, id)
	require.ErrorIs(t, err, lxphost.Code(-3))
}
