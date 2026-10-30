//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostPerpsModuleRegistered(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 111)

	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.activity(t, host, 0x00060001, 0, bindingBlockTime, make([]byte, 622)), actor.authority())
	require.NotEqual(t, Code(-103), receipt.ResultCode)
	require.Equal(t, Code(-734), receipt.ResultCode)
	require.Equal(t, "LXP_ERR_PARAMETER_BOUNDS", receipt.ResultCode.Error())
	require.NotEmpty(t, receipt.Encoded)
}

func TestHostPerpsOracleTransportDecoded(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 115)
	payload := make([]byte, 137)
	payload[0] = 1
	payload[1] = 1
	binary.BigEndian.PutUint64(payload[33:41], 1)
	binary.BigEndian.PutUint64(payload[49:57], 1)
	binary.BigEndian.PutUint64(payload[57:65], uint64(bindingBlockTime))
	binary.BigEndian.PutUint64(payload[65:73], 1)
	copy(payload[73:137], ed25519.Sign(actor.private, payload[1:73]))

	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.activity(t, host, 0x00060003, 0, bindingBlockTime, payload), actor.authority())
	require.NotEqual(t, Code(-3), receipt.ResultCode)
	require.NotEqual(t, Code(-103), receipt.ResultCode)
	require.Equal(t, Code(-7), receipt.ResultCode)
	require.Equal(t, "LXP_ERR_UNKNOWN_FIELD", receipt.ResultCode.Error())
	require.NotEmpty(t, receipt.Encoded)
}

func TestHostSpotModuleRegistered(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 112)
	payload := make([]byte, 160)
	payload[0], payload[32], payload[64] = 1, 2, 3
	binary.BigEndian.PutUint64(payload[104:112], 1)
	binary.BigEndian.PutUint64(payload[120:128], 1)
	copy(payload[128:160], actor.id[:])

	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.activity(t, host, 0x000a0001, 0, bindingBlockTime, payload), actor.authority())
	require.NotEqual(t, Code(-103), receipt.ResultCode)
	require.Equal(t, Code(-402), receipt.ResultCode)
	require.Equal(t, "LXP_ERR_ASSET_MISMATCH", receipt.ResultCode.Error())
	require.NotEmpty(t, receipt.Encoded)
}

func TestHostStreamModuleRegistered(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 113)
	payload := make([]byte, 204)
	payload[1] = 1
	payload[2], payload[34], payload[66], payload[98] = 1, 2, 3, 4
	payload[130] = 1
	binary.BigEndian.PutUint64(payload[139:147], 1)
	binary.BigEndian.PutUint64(payload[147:155], 1000)
	binary.BigEndian.PutUint64(payload[155:163], uint64(bindingBlockTime))
	binary.BigEndian.PutUint64(payload[179:187], 100)
	binary.BigEndian.PutUint64(payload[195:203], 10)
	authority := actor.authority()
	authority.Kind = 1

	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.activity(t, host, 0x00040001, 0, bindingBlockTime, payload), authority)
	require.NotEqual(t, Code(-103), receipt.ResultCode)
	require.Equal(t, Code(-409), receipt.ResultCode)
	require.Equal(t, "LXP_ERR_ACCOUNT_ID_MISMATCH", receipt.ResultCode.Error())
	require.NotEmpty(t, receipt.Encoded)
}

func TestHostServiceModuleRegistered(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 114)
	payload := make([]byte, 179)
	payload[1] = 1
	payload[2], payload[34], payload[82], payload[114] = 1, 2, 3, 4
	binary.BigEndian.PutUint64(payload[74:82], 10)
	binary.BigEndian.PutUint64(payload[146:154], uint64(bindingBlockTime)+10000)
	binary.BigEndian.PutUint64(payload[154:162], 1000)
	binary.BigEndian.PutUint64(payload[162:170], 1000)
	payload[170] = 1
	binary.BigEndian.PutUint64(payload[171:179], uint64(bindingBlockTime)+5000)

	receipt := execute(t, host, chain.context(1, bindingBlockTime, 1),
		actor.activity(t, host, 0x00050001, 0, bindingBlockTime, payload), actor.authority())
	require.NotEqual(t, Code(-103), receipt.ResultCode)
	require.Equal(t, Code(0), receipt.ResultCode)
	require.NotEmpty(t, receipt.Encoded)
	event := append(append([]byte(nil), payload[2:34]...), actor.id[:]...)
	event = binary.BigEndian.AppendUint64(append(event, 1), 1)
	require.Contains(t, receipt.Events, ReceiptEvent{Code: 1, Body: event})
	key := moduleKVKey(5, append([]byte("offer::1"), payload[2:34]...))
	record := append([]byte(nil), chain.get(key)...)
	require.Len(t, record, 252)
	require.Equal(t, payload[2:34], record[1:33])
	require.Equal(t, actor.id[:], record[65:97])

	receipt = execute(t, host, chain.context(1, bindingBlockTime, 2),
		actor.activity(t, host, 0x00050001, 1, bindingBlockTime, payload), actor.authority())
	require.Equal(t, Code(-301), receipt.ResultCode)
	require.Equal(t, "LXP_ERR_SEQUENCE_REUSED", receipt.ResultCode.Error())
	require.Equal(t, record, chain.get(key))
}
