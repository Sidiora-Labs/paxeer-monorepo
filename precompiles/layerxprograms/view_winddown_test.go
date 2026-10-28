package layerxprograms

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type windDownViewFixture struct {
	ctx         sdk.Context
	key         *sdk.KVStoreKey
	p           *PrecompileExecutor
	private     ed25519.PrivateKey
	public      [32]byte
	did         []byte
	actor       [32]byte
	sequence    uint64
	program     [32]byte
	asset       [32]byte
	destination [32]byte
}

func newWindDownViewFixture(t *testing.T) *windDownViewFixture {
	t.Helper()
	db := dbm.NewMemDB()
	key := sdk.NewKVStoreKey("wind-down-view")
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "wind-down-view", Height: 1, Time: time.UnixMilli(10000)}, false)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	f := &windDownViewFixture{ctx: ctx, key: key, p: &PrecompileExecutor{host: host},
		private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, 32)),
		program: [32]byte{0x71}, asset: [32]byte{0xaa}, destination: [32]byte{0x91}}
	copy(f.public[:], f.private.Public().(ed25519.PublicKey))
	f.did = []byte("did:layerx:" + hex.EncodeToString(f.public[:]))
	f.actor, err = lxphost.DIDID(f.did)
	require.NoError(t, err)
	registration := append([]byte{0x71, 1, 0, 2}, f.actor[:]...)
	f.execute(t, 0x00070001, append(registration, f.public[:]...))
	program := append([]byte{1}, f.actor[:]...)
	codeHash := sha256.Sum256([]byte("wind-down-view-program"))
	program = append(program, codeHash[:]...)
	program = binary.BigEndian.AppendUint16(program, 2)
	program = binary.BigEndian.AppendUint32(program, 1)
	f.put("program\x00", f.program[:], program)
	f.put("program-owner\x00", f.program[:], append([]byte{1}, f.actor[:]...))
	f.account(t, f.destination, f.asset, 0, 1)
	return f
}

func (f *windDownViewFixture) put(prefix string, suffix, value []byte) {
	key := append([]byte{1, 0, 9}, []byte(prefix)...)
	key = append(key, suffix...)
	f.ctx.KVStore(f.key).Set(key, value)
}

func (f *windDownViewFixture) execute(t *testing.T, activityType uint32, payload []byte) lxphost.Receipt {
	t.Helper()
	activity := lxphost.Activity{ProtocolVersion: 3, Type: activityType, ActorDID: f.did,
		AuthorityKey: f.public, AccountSequence: f.sequence, NotBefore: 9000, NotAfter: 11000,
		IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(append([]byte(nil), payload...), f.sequence)), Payload: payload}
	hash, err := f.p.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private, hash[:])
	receipt, err := f.p.host.Execute(lxphost.WithSequence(f.ctx, f.sequence+1), activity,
		lxphost.KernelAuthority{Actor: f.actor, Principal: f.actor, VerifiedKey: f.public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
	f.sequence++
	return receipt
}

func (f *windDownViewFixture) account(t *testing.T, id, asset [32]byte, balance, created uint64) {
	t.Helper()
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	account := field(nil, id[:])
	account = field(account, []byte("module:programs:value:"+hex.EncodeToString(id[:])))
	account = append(account, 13)
	account = binary.BigEndian.AppendUint64(account, 0)
	account = binary.BigEndian.AppendUint64(account, balance)
	account = field(account, asset[:])
	account = append(account, 1)
	account = binary.BigEndian.AppendUint64(account, 0)
	account = binary.BigEndian.AppendUint64(account, created)
	account = append(account, 0, 0)
	account = field(account, make([]byte, 32))
	account = append(account, 0)
	f.ctx.KVStore(f.key).Set(append([]byte{6}, id[:]...), account)
}

func (f *windDownViewFixture) bind(t *testing.T, seed []byte, balance uint64) [32]byte {
	t.Helper()
	material := append([]byte("LayerX/programs/program-account/v1\x00"), f.program[:]...)
	material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
	id := sha256.Sum256(append(material, seed...))
	seedHash := sha256.Sum256(seed)
	event := append([]byte("LXPA1"), f.program[:]...)
	event = append(event, id[:]...)
	event = append(event, f.asset[:]...)
	event = binary.BigEndian.AppendUint16(event, uint16(len(seed)))
	event = append(event, seedHash[:]...)
	event = binary.BigEndian.AppendUint64(event, 1)
	digest := sha256.Sum256(event)
	binding := append([]byte{2}, f.program[:]...)
	binding = append(binding, id[:]...)
	binding = append(binding, f.asset[:]...)
	binding = binary.BigEndian.AppendUint16(binding, uint16(len(seed)))
	binding = binary.BigEndian.AppendUint64(binding, 1)
	binding = append(binding, digest[:]...)
	binding = append(binding, seed...)
	primary := append(append([]byte(nil), f.program[:]...), seedHash[:]...)
	f.put("program-account\x00p", primary, binding)
	f.put("program-account\x00r", id[:], binding)
	f.account(t, id, f.asset, balance, 1)
	return id
}

func (f *windDownViewFixture) setRoute(t *testing.T, account, destination [32]byte, seed []byte) {
	t.Helper()
	// winddown.c stores version, asset, destination, seed length, then seed.
	record := append([]byte{1}, f.asset[:]...)
	record = append(record, destination[:]...)
	record = binary.BigEndian.AppendUint16(record, uint16(len(seed)))
	record = append(record, seed...)
	require.Len(t, record, 67+len(seed))
	key := append(append([]byte(nil), f.program[:]...), account[:]...)
	f.put("wind-down\x00r", key, record)
}

func (f *windDownViewFixture) dump(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	iterator := f.ctx.KVStore(f.key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		out[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return out
}

func TestProgramsViewWindDownRecord(t *testing.T) {
	f := newWindDownViewFixture(t)
	method := programsABI.Methods["getWindDown"]
	read := func(expected []interface{}) {
		t.Helper()
		before := f.dump(t)
		data, err := getWindDownHandler(f.ctx, f.p, callContext{method: &method}, []interface{}{f.program})
		require.NoError(t, err)
		require.Len(t, data, 6*32)
		actual, err := method.Outputs.Unpack(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		require.Equal(t, before, f.dump(t))
	}
	read([]interface{}{uint8(1), [32]byte{}, uint64(0), uint64(0), uint16(0), uint16(0)})
	for i, seed := range [][]byte{[]byte("vault\x00reserve"), []byte("empty")} {
		id := f.bind(t, seed, uint64(250*(1-i)))
		f.setRoute(t, id, f.destination, seed)
	}
	const deadline = uint64(0x0102030405060708)
	sequence := uint64(0x1112131415161718)
	record := append([]byte{1, 2}, f.program[:]...)
	record = binary.BigEndian.AppendUint64(record, deadline)
	record = binary.BigEndian.AppendUint64(record, sequence)
	record = binary.BigEndian.AppendUint16(record, 2)
	record = binary.BigEndian.AppendUint16(record, 1)
	f.put("wind-down\x00s", f.program[:], record)
	read([]interface{}{uint8(2), f.program, deadline, sequence, uint16(2), uint16(1)})
	recordKey := append([]byte{1, 0, 9}, []byte("wind-down\x00s")...)
	recordKey = append(recordKey, f.program[:]...)
	record = f.ctx.KVStore(f.key).Get(recordKey)
	require.Len(t, record, 54)
	require.Equal(t, byte(1), record[0])
	require.Equal(t, byte(2), record[1])
	require.Equal(t, f.program[:], record[2:34])
	require.Equal(t, deadline, binary.BigEndian.Uint64(record[34:42]))
	require.Equal(t, sequence, binary.BigEndian.Uint64(record[42:50]))
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(record[50:52]))
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(record[52:54]))
	sequence++
	record = bytes.Clone(record)
	record[1] = 3
	binary.BigEndian.PutUint64(record[42:50], sequence)
	f.put("wind-down\x00s", f.program[:], record)
	read([]interface{}{uint8(3), f.program, deadline, sequence, uint16(2), uint16(1)})
	t.Run("executor dispatch", func(t *testing.T) {
		for _, readOnly := range []bool{false, true} {
			before := f.dump(t)
			data, err := f.p.Execute(f.ctx, &method, common.Address{}, common.Address{},
				[]interface{}{f.program}, nil, readOnly, nil, nil)
			require.NoError(t, err)
			actual, err := method.Outputs.Unpack(data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{uint8(3), f.program, deadline, sequence, uint16(2), uint16(1)}, actual)
			require.Equal(t, before, f.dump(t))
		}
	})

	data, err := getWindDownHandler(f.ctx, f.p, callContext{method: &method}, []interface{}{[32]byte{0xfe}})
	require.Nil(t, data)
	require.ErrorContains(t, err, "LXP_NOT_FOUND:")
	for _, args := range [][]interface{}{nil, {f.program, f.program}, {"invalid"}, {[32]byte{}}} {
		data, err := getWindDownHandler(f.ctx, f.p, callContext{method: &method}, args)
		require.Nil(t, data)
		require.Error(t, err)
	}
	f.put("wind-down\x00s", f.program[:], []byte{1})
	require.PanicsWithValue(t, lxphost.Code(-1001), func() {
		_, _ = getWindDownHandler(f.ctx, f.p, callContext{method: &method}, []interface{}{f.program})
	})
}

func TestProgramsViewExitRoute(t *testing.T) {
	for _, seed := range [][]byte{nil, []byte("vault\x00reserve"), bytes.Repeat([]byte{0xff}, 128)} {
		t.Run(hex.EncodeToString(seed), func(t *testing.T) {
			f := newWindDownViewFixture(t)
			id := f.bind(t, seed, 250)
			method := programsABI.Methods["getExitRoute"]
			args := []interface{}{f.program, id}
			before := f.dump(t)
			data, err := getExitRouteHandler(f.ctx, f.p, callContext{method: &method}, args)
			require.Nil(t, data)
			require.ErrorIs(t, err, lxphost.Code(-7))
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, unpackErr)
			require.Contains(t, reason, "LXP_NOT_FOUND: getExitRoute:")
			require.Equal(t, before, f.dump(t))
			f.setRoute(t, id, f.destination, seed)
			for _, destination := range [][32]byte{f.destination, {0x92}} {
				if destination != f.destination {
					f.account(t, destination, f.asset, 0, 1)
					f.setRoute(t, id, destination, seed)
				}
				before = f.dump(t)
				for i := 0; i < 2; i++ {
					data, err = getExitRouteHandler(f.ctx, f.p, callContext{method: &method}, args)
					require.NoError(t, err)
					actual, err := method.Outputs.Unpack(data)
					require.NoError(t, err)
					require.Equal(t, []interface{}{f.asset, destination, append([]byte{}, seed...)}, actual)
					if len(seed) > 0 {
						actual[2].([]byte)[0] ^= 0xff
					}
				}
				require.Equal(t, before, f.dump(t))
			}
			for _, missing := range [][]interface{}{{f.program, [32]byte{0xfe}}, {[32]byte{0xfe}, id}} {
				data, err = getExitRouteHandler(f.ctx, f.p, callContext{method: &method}, missing)
				require.Nil(t, data)
				require.ErrorContains(t, err, "LXP_NOT_FOUND:")
			}
			for _, invalid := range [][]interface{}{nil, {f.program}, {f.program, id, id}, {f.program, "invalid"}, {[32]byte{}, id}, {f.program, [32]byte{}}} {
				data, err = getExitRouteHandler(f.ctx, f.p, callContext{method: &method}, invalid)
				require.Nil(t, data)
				require.Error(t, err)
			}
			t.Run("executor dispatch", func(t *testing.T) {
				for _, readOnly := range []bool{false, true} {
					before := f.dump(t)
					data, err := f.p.Execute(f.ctx, &method, common.Address{}, common.Address{}, args, nil, readOnly, nil, nil)
					require.NoError(t, err)
					actual, err := method.Outputs.Unpack(data)
					require.NoError(t, err)
					require.Equal(t, []interface{}{f.asset, [32]byte{0x92}, append([]byte{}, seed...)}, actual)
					require.Equal(t, before, f.dump(t))
					data, err = f.p.Execute(f.ctx, &method, common.Address{}, common.Address{},
						[]interface{}{f.program, [32]byte{0xfe}}, nil, readOnly, nil, nil)
					require.Nil(t, data)
					require.ErrorContains(t, err, "LXP_NOT_FOUND:")
				}
			})
			f.p.host.Close()
			data, err = getExitRouteHandler(f.ctx, f.p, callContext{method: &method}, args)
			require.Nil(t, data)
			require.ErrorIs(t, err, lxphost.Code(-3))
			require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		})
	}
}
