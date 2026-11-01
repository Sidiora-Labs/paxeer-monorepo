package layerxprograms

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func webRequestViewContext(t *testing.T) (sdk.Context, *sdk.KVStoreKey, *PrecompileExecutor) {
	t.Helper()
	key := sdk.NewKVStoreKey("programs_web_request_host")
	db := dbm.NewMemDB()
	stores := store.NewCommitMultiStore(db)
	stores.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, stores.LoadLatestVersion())
	ctx := sdk.NewContext(stores, tmproto.Header{Height: 100, Time: time.UnixMilli(1700000000123)}, false)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	return ctx, key, &PrecompileExecutor{host: host}
}

func webRequestViewKey(programID [32]byte, requestID uint64) []byte {
	key := append([]byte{1, 0, 9}, []byte("web/pending")...)
	key = append(key, programID[:]...)
	return binary.BigEndian.AppendUint64(key, requestID)
}

func webRequestViewRecord(payload []byte, sequence uint64, fulfilled byte) []byte {
	record := append([]byte{1, 1}, crypto.Keccak256(payload)...)
	record = append(record, bytes.Repeat([]byte{0xa5}, 32)...)
	record = append(record, bytes.Repeat([]byte{0x5a}, 32)...)
	record = binary.BigEndian.AppendUint64(record, 1)
	record = binary.BigEndian.AppendUint64(record, 7)
	record = binary.BigEndian.AppendUint64(record, sequence)
	return append(record, fulfilled)
}

func webRequestViewDump(ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	values := map[string][]byte{}
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func TestProgramsViewWebRequestRecord(t *testing.T) {
	ctx, key, executor := webRequestViewContext(t)
	method := programsABI.Methods["getWebRequest"]
	programID := [32]byte{0xa1, 0, 0xff}
	otherProgram := [32]byte{0xa2, 0, 0xff}
	cases := []struct {
		program [32]byte
		request uint64
		record  []byte
	}{
		{programID, 0, webRequestViewRecord([]byte("zero"), 41, 0)},
		{programID, 257, webRequestViewRecord([]byte("request\x00payload\xff"), 0x0102030405060708, 0)},
		{programID, math.MaxUint64, webRequestViewRecord([]byte("fulfilled"), 43, 1)},
		{otherProgram, 257, webRequestViewRecord([]byte("other program"), 44, 0)},
	}
	for _, tc := range cases {
		require.Len(t, tc.record, 123)
		ctx.KVStore(key).Set(webRequestViewKey(tc.program, tc.request), tc.record)
	}
	before := webRequestViewDump(ctx, key)
	for _, tc := range cases {
		t.Run(fmt.Sprintf("handler/%x/%d", tc.program[0], tc.request), func(t *testing.T) {
			input, err := programsABI.Pack(method.Name, tc.program, tc.request)
			require.NoError(t, err)
			args, err := method.Inputs.Unpack(input[4:])
			require.NoError(t, err)
			stored, err := executor.host.ReadWebRequest(ctx, tc.program, tc.request)
			require.NoError(t, err)
			require.Equal(t, tc.record, stored)
			output, err := getWebRequestHandler(ctx, executor, callContext{method: &method}, args)
			require.NoError(t, err)
			values, err := method.Outputs.Unpack(output)
			require.NoError(t, err)
			require.Equal(t, []interface{}{tc.record}, values)
			require.Len(t, output, 192)
			require.Equal(t, uint64(32), binary.BigEndian.Uint64(output[24:32]))
			require.Equal(t, uint64(123), binary.BigEndian.Uint64(output[56:64]))
			require.Equal(t, tc.record, output[64:187])
			require.Equal(t, make([]byte, 5), output[187:])
			output[64] ^= 0xff
			again, err := executor.host.ReadWebRequest(ctx, tc.program, tc.request)
			require.NoError(t, err)
			require.Equal(t, tc.record, again)
			require.Equal(t, before, webRequestViewDump(ctx, key))
		})
	}
	for _, readOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("executor/readOnly=%t", readOnly), func(t *testing.T) {
			args := []interface{}{programID, uint64(257)}
			output, err := executor.Execute(ctx, &method, common.Address{}, common.Address{}, args, big.NewInt(0), readOnly, nil, nil)
			require.NoError(t, err)
			values, err := method.Outputs.Unpack(output)
			require.NoError(t, err)
			require.Equal(t, []interface{}{cases[1].record}, values)
			require.Equal(t, before, webRequestViewDump(ctx, key))
		})
	}
}

func TestProgramsViewWebRequestAbsent(t *testing.T) {
	ctx, key, executor := webRequestViewContext(t)
	method := programsABI.Methods["getWebRequest"]
	programID := [32]byte{0xa1}
	otherProgram := [32]byte{0xa2}
	ctx.KVStore(key).Set(webRequestViewKey(programID, 257), webRequestViewRecord([]byte("recorded"), 41, 0))
	before := webRequestViewDump(ctx, key)
	for _, tc := range []struct {
		program [32]byte
		request uint64
	}{
		{programID, 0}, {programID, 256}, {programID, 258}, {programID, math.MaxUint64}, {otherProgram, 257},
	} {
		t.Run(fmt.Sprintf("handler/%x/%d", tc.program[0], tc.request), func(t *testing.T) {
			output, err := getWebRequestHandler(ctx, executor, callContext{method: &method}, []interface{}{tc.program, tc.request})
			require.Nil(t, output)
			require.ErrorIs(t, err, lxphost.Code(-7))
			require.EqualError(t, err, "LXP_NOT_FOUND: getWebRequest: LXP_ERR_UNKNOWN_FIELD")
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, unpackErr)
			require.Equal(t, "LXP_NOT_FOUND: getWebRequest: LXP_ERR_UNKNOWN_FIELD", reason)
			require.Equal(t, before, webRequestViewDump(ctx, key))
		})
	}
	for _, readOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("executor/readOnly=%t", readOnly), func(t *testing.T) {
			output, err := executor.Execute(ctx, &method, common.Address{}, common.Address{},
				[]interface{}{programID, uint64(258)}, big.NewInt(0), readOnly, nil, nil)
			require.Nil(t, output)
			require.ErrorIs(t, err, lxphost.Code(-7))
			reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
			require.NoError(t, unpackErr)
			require.Equal(t, "LXP_NOT_FOUND: getWebRequest: LXP_ERR_UNKNOWN_FIELD", reason)
			require.Equal(t, before, webRequestViewDump(ctx, key))
		})
	}
}
