package layerxprograms_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestEncodeSandboxPayloads(t *testing.T) {
	id := func(value byte) (out [32]byte) {
		for i := range out {
			out[i] = value
		}
		return out
	}
	decode := func(text string) []byte {
		data, err := hex.DecodeString(text)
		require.NoError(t, err)
		return data
	}
	leaseID, digest := id(0x11), id(0x22)
	escrow, asset, destination := id(0x33), id(0x44), id(0x55)
	tenant, host, transferProgram := id(0x66), id(0x77), id(0x88)
	schedule := [7]uint64{0x0102030405060708, 0, 1, 0x1122334455667788, 0x99aabbccddeeff00, math.MaxUint64, 7}
	scheduleHex := "0102030405060708" + "0000000000000000" + "0000000000000001" +
		"1122334455667788" + "99aabbccddeeff00" + "ffffffffffffffff" + "0000000000000007"
	callHex := strings.Repeat("99", 32) + "0001" + "0004" + "00000000" + "0000" +
		"00000000" + "00000040" + strings.Repeat("0000000000000001", 7) + "6d61696e"
	call := decode(callHex)
	sequence := uint64(0x0102030405060708)
	version := uint32(0x01020304)

	t.Run("execute layout", func(t *testing.T) {
		before := bytes.Clone(call)
		payload, err := layerxprograms.EncodeSandboxExecutePayload(leaseID, sequence, digest, escrow, asset, destination, version, schedule, call)
		require.NoError(t, err)
		want := decode("01010000" + strings.Repeat("11", 32) + "0102030405060708" +
			strings.Repeat("22", 32) + strings.Repeat("33", 32) + strings.Repeat("44", 32) +
			strings.Repeat("55", 32) + "01020304" + scheduleHex + fmt.Sprintf("%08x", len(call)) + callHex)
		require.Len(t, payload, 236+len(call))
		require.Equal(t, want, payload)
		require.Equal(t, before, call)
		payload[236] ^= 0xff
		require.Equal(t, before, call)
	})

	t.Run("execute canonical fields", func(t *testing.T) {
		for _, name := range []string{"leaseID", "digest", "escrow", "asset", "destination", "version", "nil call", "empty call"} {
			t.Run(name, func(t *testing.T) {
				lease, expected, account, token, fee := leaseID, digest, escrow, asset, destination
				v, body := version, call
				switch name {
				case "leaseID":
					lease = [32]byte{}
				case "digest":
					expected = [32]byte{}
				case "escrow":
					account = [32]byte{}
				case "asset":
					token = [32]byte{}
				case "destination":
					fee = [32]byte{}
				case "version":
					v = 0
				case "nil call":
					body = nil
				case "empty call":
					body = []byte{}
				}
				payload, err := layerxprograms.EncodeSandboxExecutePayload(lease, sequence, expected, account, token, fee, v, schedule, body)
				require.EqualError(t, err, "LXP_NON_CANONICAL")
				require.Nil(t, payload)
			})
		}
		payload, err := layerxprograms.EncodeSandboxExecutePayload(leaseID, 0, digest, escrow, asset, destination, 1, [7]uint64{}, call)
		require.NoError(t, err)
		require.Equal(t, make([]byte, 8), payload[36:44])
	})

	amount := new(big.Int).SetBytes(decode("0102030405060708090a0b0c0d0e0f10"))
	leaseRequest := []byte{1, 0, 2, 0xff, 0}
	legs := []layerxprograms.TransferLeg{
		{From: tenant, Asset: asset, To: escrow, Amount: new(big.Int).Set(amount)},
		{From: tenant, Asset: asset, To: destination, Amount: big.NewInt(1)},
	}
	t.Run("fund layout and nested transfer", func(t *testing.T) {
		before := bytes.Clone(leaseRequest)
		amountBefore := new(big.Int).Set(amount)
		payload, err := layerxprograms.EncodeSandboxFundPayload(leaseID, tenant, host, escrow, asset, amount, 0x8877665544332211, version, schedule, leaseRequest, transferProgram, legs)
		require.NoError(t, err)
		transfer, err := layerxprograms.EncodeTransferPayload(transferProgram, legs)
		require.NoError(t, err)
		transferHex := strings.Repeat("88", 32) + "0002" +
			strings.Repeat("66", 32) + strings.Repeat("44", 32) + strings.Repeat("33", 32) +
			"0102030405060708090a0b0c0d0e0f10" +
			strings.Repeat("66", 32) + strings.Repeat("44", 32) + strings.Repeat("55", 32) +
			"00000000000000000000000000000001"
		require.Equal(t, decode(transferHex), transfer)
		want := decode("01020000" + strings.Repeat("11", 32) + strings.Repeat("66", 32) +
			strings.Repeat("77", 32) + strings.Repeat("33", 32) + strings.Repeat("44", 32) +
			"0102030405060708090a0b0c0d0e0f10" + "8877665544332211" + "01020304" +
			scheduleHex + "00000005" + "010002ff00" + fmt.Sprintf("%08x", len(transfer)) + transferHex)
		require.Len(t, payload, 256+len(leaseRequest)+len(transfer))
		require.Equal(t, want, payload)
		require.Equal(t, before, leaseRequest)
		require.Zero(t, amount.Cmp(amountBefore))
		require.Zero(t, legs[0].Amount.Cmp(amountBefore))
		payload[252] ^= 0xff
		require.Equal(t, before, leaseRequest)
	})

	t.Run("fund refuses empty lease request", func(t *testing.T) {
		for _, request := range [][]byte{nil, {}} {
			payload, err := layerxprograms.EncodeSandboxFundPayload(leaseID, tenant, host, escrow, asset, amount, 1, version, schedule, request, transferProgram, legs)
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Nil(t, payload)
		}
	})

	t.Run("fund amount bounds", func(t *testing.T) {
		for _, invalid := range []*big.Int{nil, big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 128)} {
			payload, err := layerxprograms.EncodeSandboxFundPayload(leaseID, tenant, host, escrow, asset, invalid, 1, version, schedule, leaseRequest, transferProgram, legs)
			require.EqualError(t, err, "LXP_OVERFLOW")
			require.Nil(t, payload)
		}
		maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
		payload, err := layerxprograms.EncodeSandboxFundPayload(leaseID, tenant, host, escrow, asset, maximum, 1, version, schedule, leaseRequest, transferProgram, legs)
		require.NoError(t, err)
		require.Equal(t, bytes.Repeat([]byte{0xff}, 16), payload[164:180])
	})

	t.Run("fund propagates transfer refusal", func(t *testing.T) {
		_, transferErr := layerxprograms.EncodeTransferPayload(transferProgram, nil)
		require.Error(t, transferErr)
		payload, err := layerxprograms.EncodeSandboxFundPayload(leaseID, tenant, host, escrow, asset, amount, 1, version, schedule, leaseRequest, transferProgram, nil)
		require.EqualError(t, err, transferErr.Error())
		require.Nil(t, payload)
	})

	t.Run("activate layout", func(t *testing.T) {
		payload, err := layerxprograms.EncodeSandboxActivatePayload(leaseID, digest, sequence)
		require.NoError(t, err)
		require.Len(t, payload, 76)
		require.Equal(t, decode("01030000"+strings.Repeat("11", 32)+strings.Repeat("22", 32)+"0102030405060708"), payload)
		payload, err = layerxprograms.EncodeSandboxActivatePayload([32]byte{}, [32]byte{}, math.MaxUint64)
		require.NoError(t, err)
		require.Equal(t, decode("01030000"+strings.Repeat("00", 64)+"ffffffffffffffff"), payload)
	})
}

func TestRetireStorageRequestBound(t *testing.T) {
	for count := 1; count <= 3; count++ {
		for _, size := range []int{370 + count*149, 370 + count*149 + 1, 32768} {
			t.Run(fmt.Sprintf("accept count=%d bytes=%d", count, size), func(t *testing.T) {
				request := bytes.Repeat([]byte{0xa5}, size)
				request[0], request[1] = 1, byte(count)
				before := bytes.Clone(request)
				require.NoError(t, layerxprograms.CheckRetireStorageRequest(request))
				require.Equal(t, before, request)
			})
		}
	}
	for _, tc := range []struct {
		count byte
		size  int
	}{
		{1, 0}, {1, 1}, {1, 369}, {1, 370}, {1, 518},
		{2, 667}, {3, 816}, {1, 32769}, {2, 32769}, {3, 32769},
		{0, 519}, {4, 966}, {255, 32768},
	} {
		t.Run(fmt.Sprintf("refuse count=%d bytes=%d", tc.count, tc.size), func(t *testing.T) {
			request := make([]byte, tc.size)
			if len(request) >= 2 {
				request[0], request[1] = 1, tc.count
			}
			before := bytes.Clone(request)
			require.EqualError(t, layerxprograms.CheckRetireStorageRequest(request), "LXP_LENGTH_LIMIT")
			require.Equal(t, before, request)
		})
	}
	require.EqualError(t, layerxprograms.CheckRetireStorageRequest(nil), "LXP_LENGTH_LIMIT")
}
