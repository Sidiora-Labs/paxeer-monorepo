package keeper

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/require"
)

func TestDecodeAuthEvents(t *testing.T) {
	var sessionReceipt lxphost.Receipt
	for _, method := range []string{"session", "capability", "revoke"} {
		t.Run(method, func(t *testing.T) {
			f := newAuthViewFixture(t)
			actor := authExecuteActor(t, f)
			payload, grantID := authExecutePayload(t, f, method)
			result, err := f.keeper.execute(f.ctx, actor, payload, [32]byte{0x75})
			require.NoError(t, err)
			receipt := result.Receipt
			events, err := AuthEvents(receipt)
			require.NoError(t, err)
			var expected []Event
			var codes []uint16
			if method == "revoke" {
				record := append([]byte(nil), grantID[:]...)
				record = append(record, 1)
				record = binary.BigEndian.AppendUint64(record, 44)
				expected = append(expected, Event{Name: "GrantRevoked", GrantID: grantID, Body: record})
				codes = append(codes, 0x7106)
			} else {
				sub, first, continuation := uint8(8), uint16(0x7108), uint16(0x7128)
				if method == "session" {
					summary, err := f.keeper.SessionKey(f.ctx, grantID)
					require.NoError(t, err)
					expected = append(expected, Event{Name: "SessionGranted", GrantID: grantID, Body: summary})
					codes = append(codes, 0x7145)
					sub, first, continuation = 5, 0x7105, 0x7125
					sessionReceipt = receipt
				} else {
					expected = append(expected, Event{Name: "CapabilityGranted", GrantID: grantID, Body: grantID[:]})
					codes = append(codes, 0x7148)
				}
				body, err := f.keeper.GetGrant(f.ctx, grantID)
				require.NoError(t, err)
				require.Greater(t, len(body), 256)
				for offset := 0; offset < len(body); offset += 256 {
					end := offset + 256
					if end > len(body) {
						end = len(body)
					}
					expected = append(expected, Event{Name: "GrantBody", GrantID: grantID, Sub: sub, Chunk: uint8(offset / 256), Body: body[offset:end]})
					if offset == 0 {
						codes = append(codes, first)
					} else {
						codes = append(codes, continuation)
					}
				}
			}
			state := make([]byte, 223)
			copy(state, "LXGI1")
			copy(state[5:37], f.didID[:])
			copy(state[37:69], f.public[:])
			revocation := uint64(41)
			if method == "revoke" {
				revocation = 44
			}
			binary.BigEndian.PutUint64(state[69:77], revocation)
			binary.BigEndian.PutUint64(state[215:223], 44)
			expected = append(expected, Event{Name: "IdentityStateUpdated", DIDID: f.didID, Body: state})
			codes = append(codes, 0x7110)
			require.Equal(t, expected, events)
			require.Len(t, receipt.Events, len(codes))
			for i, code := range codes {
				require.Equal(t, code, receipt.Events[i].Code)
			}
			for i := range events {
				before := bytes.Clone(receipt.Events[i].Body)
				events[i].Body[0] ^= 0xff
				require.Equal(t, before, receipt.Events[i].Body)
			}
		})
	}

	t.Run("empty", func(t *testing.T) {
		events, err := AuthEvents(lxphost.Receipt{})
		require.NoError(t, err)
		require.Empty(t, events)
	})

	require.GreaterOrEqual(t, len(sessionReceipt.Events), 4)
	header := sessionReceipt.Events[0]
	first := sessionReceipt.Events[1]
	continuation := sessionReceipt.Events[2]
	state := sessionReceipt.Events[len(sessionReceipt.Events)-1]
	for _, tc := range []struct {
		name string
		raw  []lxphost.ReceiptEvent
	}{
		{"unknown", []lxphost.ReceiptEvent{{Code: 0xffff}}},
		{"unknown after valid", []lxphost.ReceiptEvent{header, first, continuation, state, {Code: 0x7107}}},
		{"short summary", []lxphost.ReceiptEvent{{Code: 0x7145, Body: header.Body[:208]}}},
		{"long summary", []lxphost.ReceiptEvent{{Code: 0x7145, Body: append(bytes.Clone(header.Body), 0)}}},
		{"summary prefix", []lxphost.ReceiptEvent{{Code: 0x7145, Body: make([]byte, 209)}}},
		{"short capability id", []lxphost.ReceiptEvent{{Code: 0x7148, Body: make([]byte, 31)}}},
		{"long capability id", []lxphost.ReceiptEvent{{Code: 0x7148, Body: make([]byte, 33)}}},
		{"short revocation", []lxphost.ReceiptEvent{{Code: 0x7106, Body: make([]byte, 40)}}},
		{"long revocation", []lxphost.ReceiptEvent{{Code: 0x7106, Body: make([]byte, 42)}}},
		{"short state", []lxphost.ReceiptEvent{{Code: 0x7110, Body: state.Body[:222]}}},
		{"long state", []lxphost.ReceiptEvent{{Code: 0x7110, Body: append(bytes.Clone(state.Body), 0)}}},
		{"state prefix", []lxphost.ReceiptEvent{{Code: 0x7110, Body: make([]byte, 223)}}},
		{"orphan first chunk", []lxphost.ReceiptEvent{first}},
		{"orphan continuation", []lxphost.ReceiptEvent{continuation}},
		{"missing first chunk", []lxphost.ReceiptEvent{header, continuation}},
		{"duplicate first chunk", []lxphost.ReceiptEvent{header, first, first}},
		{"wrong grant kind", []lxphost.ReceiptEvent{header, {Code: 0x7108, Body: first.Body}}},
		{"empty chunk", []lxphost.ReceiptEvent{header, {Code: 0x7105}}},
		{"oversized chunk", []lxphost.ReceiptEvent{header, {Code: 0x7105, Body: make([]byte, 257)}}},
		{"continuation after short chunk", []lxphost.ReceiptEvent{header, {Code: 0x7105, Body: first.Body[:255]}, continuation}},
		{"chunk after state", []lxphost.ReceiptEvent{header, first, state, continuation}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, err := AuthEvents(lxphost.Receipt{Events: tc.raw})
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Nil(t, events)
		})
	}

	t.Run("chunk index overflow", func(t *testing.T) {
		raw := []lxphost.ReceiptEvent{header, first}
		for i := 1; i <= 256; i++ {
			raw = append(raw, lxphost.ReceiptEvent{Code: 0x7125, Body: first.Body})
		}
		events, err := AuthEvents(lxphost.Receipt{Events: raw[:257]})
		require.NoError(t, err)
		require.Equal(t, uint8(255), events[256].Chunk)
		events, err = AuthEvents(lxphost.Receipt{Events: raw})
		require.ErrorIs(t, err, types.ErrNonCanonical)
		require.Nil(t, events)
	})
}
