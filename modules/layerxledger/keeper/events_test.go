package keeper

import (
	"bytes"
	"crypto/ed25519"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestDecodeLedgerEvents(t *testing.T) {
	asset, from, to := [32]byte{1}, [32]byte{2}, [32]byte{3}
	grant, key, context, reference := [32]byte{4}, [32]byte{5}, [32]byte{6}, [32]byte{7}
	amount := sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	leg := lxphost.TransferLeg{Asset: asset, From: from, To: to, Amount: amount}
	args := EventArgs{GrantID: grant, IdempotencyKey: key, ContextHash: context}

	t.Run("identifier_events", func(t *testing.T) {
		for _, tc := range []struct {
			activity uint32
			code     uint16
			name     string
			field    string
			id       [32]byte
		}{
			{0x00010001, 1, "AssetRegistered", "assetId", asset},
			{0x00010002, 2, "AssetPaused", "assetId", asset},
			{0x00010003, 3, "AssetUnpaused", "assetId", asset},
			{0x00010004, 4, "AccountOpened", "accountId", to},
			{0x00010007, 7, "PayerGrantIssued", "grantId", grant},
			{0x00010008, 8, "PayerGrantRevoked", "grantId", grant},
		} {
			t.Run(tc.name, func(t *testing.T) {
				receipt := lxphost.Receipt{Events: []lxphost.ReceiptEvent{{Code: tc.code, Body: append([]byte(nil), tc.id[:]...)}}}
				events, err := LedgerEvents(tc.activity, receipt, EventArgs{GrantID: [32]byte{99}})
				require.NoError(t, err)
				require.Equal(t, []Event{{Name: tc.name, Fields: map[string]any{tc.field: tc.id}}}, events)
				receipt.Events[0].Body[0] ^= 0xff
				require.Equal(t, tc.id, events[0].Fields[tc.field], "decoded identifiers must own their bytes")

				for _, body := range [][]byte{nil, make([]byte, 31), make([]byte, 33)} {
					bad := lxphost.Receipt{Events: []lxphost.ReceiptEvent{{Code: tc.code, Body: body}}}
					got, err := LedgerEvents(tc.activity, bad, EventArgs{})
					require.ErrorIs(t, err, types.ErrNonCanonical)
					require.Nil(t, got)
				}
				for _, entries := range [][]lxphost.ReceiptEvent{
					nil,
					{{Code: 99, Body: tc.id[:]}},
					{{Code: tc.code, Body: tc.id[:]}, {Code: tc.code, Body: tc.id[:]}},
				} {
					got, err := LedgerEvents(tc.activity, lxphost.Receipt{Events: entries}, EventArgs{})
					require.ErrorIs(t, err, types.ErrNonCanonical)
					require.Nil(t, got)
				}
			})
		}
	})

	t.Run("transfers", func(t *testing.T) {
		for _, activity := range []uint32{0x00010005, 0x00010006, 0x00010009, 0x0001000a, 0x0001000b} {
			events, err := LedgerEvents(activity, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg}}, args)
			require.NoError(t, err)
			want := []Event{{Name: "Transfer", Fields: map[string]any{
				"from": from, "to": to, "assetId": asset, "amount": amount.BigInt(), "kind": uint8(activity & 0xffff),
			}}}
			if activity == 0x00010006 {
				want = append(want, Event{Name: "PaymentDrawn", Fields: map[string]any{
					"grantId": grant, "from": from, "to": to, "assetId": asset, "amount": amount.BigInt(),
					"idempotencyKey": key, "contextHash": context,
				}})
			}
			require.Equal(t, want, events)
			missing, err := LedgerEvents(activity, lxphost.Receipt{}, args)
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Nil(t, missing)
		}
		second := lxphost.TransferLeg{Asset: [32]byte{8}, From: to, To: from, Amount: sdk.NewInt(17)}
		events, err := LedgerEvents(0x00010005, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg, second}}, EventArgs{})
		require.NoError(t, err)
		require.Len(t, events, 2)
		require.Equal(t, "Transfer", events[1].Name)
		require.Equal(t, map[string]any{
			"from": to, "to": from, "assetId": second.Asset, "amount": big.NewInt(17), "kind": uint8(5),
		}, events[1].Fields)
		events[0].Fields["amount"].(*big.Int).SetInt64(1)
		require.Equal(t, 128, amount.BigInt().BitLen(), "decoding must not alias the receipt amount")
	})

	t.Run("exact_payment", func(t *testing.T) {
		for _, ref := range [][32]byte{reference, {}} {
			exact := EventArgs{PayExact: true, PaymentReference: ref}
			events, err := LedgerEvents(0x00010005, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg}}, exact)
			require.NoError(t, err)
			require.Len(t, events, 2)
			require.Equal(t, "Transfer", events[0].Name)
			require.Equal(t, Event{Name: "PaymentExact", Fields: map[string]any{
				"paymentReference": ref, "from": from, "to": to, "assetId": asset, "amount": amount.BigInt(),
			}}, events[1])
		}
		ordinary, err := LedgerEvents(0x00010005, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg}}, EventArgs{PaymentReference: reference})
		require.NoError(t, err)
		require.Len(t, ordinary, 1)
		for _, activity := range []uint32{0x00010006, 0x00010007, 0x0001000a} {
			events, err := LedgerEvents(activity, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg}}, EventArgs{PayExact: true})
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Nil(t, events)
		}
		for _, tc := range []struct {
			activity uint32
			args     EventArgs
		}{{0x00010005, EventArgs{PayExact: true}}, {0x00010006, args}} {
			events, err := LedgerEvents(tc.activity, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg, leg}}, tc.args)
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Nil(t, events)
		}
	})

	t.Run("invalid_amounts", func(t *testing.T) {
		for _, invalid := range []sdk.Int{{}, sdk.ZeroInt(), sdk.NewInt(-1), sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128))} {
			bad := leg
			bad.Amount = invalid
			events, err := LedgerEvents(0x00010005, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg, bad}}, EventArgs{})
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Nil(t, events, "malformed legs must not return partial events")
		}
	})

	t.Run("unknown_activity_and_failed_receipt", func(t *testing.T) {
		for _, activity := range []uint32{0, 1, 0x00010000, 0x0001000c, 0x00020005, 0xffffffff} {
			events, err := LedgerEvents(activity, lxphost.Receipt{Transfers: []lxphost.TransferLeg{leg}}, args)
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Nil(t, events)
		}
		for _, activity := range []uint32{0x00010001, 0x00010005, 0x00010006} {
			events, err := LedgerEvents(activity, lxphost.Receipt{ResultCode: -403, Transfers: []lxphost.TransferLeg{leg}}, args)
			require.ErrorIs(t, err, types.ErrAssetPaused)
			require.Nil(t, events)
		}
	})

	t.Run("kernel_receipts", func(t *testing.T) {
		k, ctx, _, _ := newExecuteKeeper(t)
		actor := executeActor(t, 101)
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{101}, ed25519.SeedSize))
		var sequence uint64
		execute := func(activityType uint32, payload []byte) lxphost.Receipt {
			t.Helper()
			now := uint64(ctx.BlockTime().UnixMilli())
			activity := lxphost.Activity{
				ProtocolVersion: 3, Type: activityType, ActorDID: actor.DID,
				AuthorityKey: actor.Authority.VerifiedKey, AccountSequence: sequence,
				NotBefore: now, NotAfter: now,
				IdempotencyKey: [32]byte{byte(sequence + 1)}, Payload: payload,
			}
			hash, err := k.host.SigningHash(activity)
			require.NoError(t, err)
			activity.Signature = ed25519.Sign(private, hash[:])
			sequence++
			receipt, err := k.host.Execute(lxphost.WithSequence(ctx, sequence), activity, actor.Authority)
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), receipt.ResultCode)
			return receipt
		}
		payload, assetID := executeAssetPayload(actor, 1)
		for _, tc := range []struct {
			activity uint32
			payload  []byte
			name     string
			field    string
			id       [32]byte
		}{
			{0x00010001, payload, "AssetRegistered", "assetId", assetID},
			{0x00010002, types.EncodePausePayload(assetID), "AssetPaused", "assetId", assetID},
			{0x00010003, types.EncodePausePayload(assetID), "AssetUnpaused", "assetId", assetID},
		} {
			receipt := execute(tc.activity, tc.payload)
			events, err := LedgerEvents(tc.activity, receipt, EventArgs{})
			require.NoError(t, err)
			require.Equal(t, []Event{{Name: tc.name, Fields: map[string]any{tc.field: tc.id}}}, events)
		}
		_, accountID, err := lxphost.AssetAccountID(actor.DIDKey, assetID)
		require.NoError(t, err)
		opened := execute(0x00010004, types.EncodeOpenAccountPayload(assetID))
		events, err := LedgerEvents(0x00010004, opened, EventArgs{})
		require.NoError(t, err)
		require.Equal(t, []Event{{Name: "AccountOpened", Fields: map[string]any{"accountId": accountID}}}, events)
	})
}
