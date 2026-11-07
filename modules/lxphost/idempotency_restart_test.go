//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestPaymentReferenceStaysRefusedAfterReopen(t *testing.T) {
	for _, bound := range []bool{false, true} {
		name := "signed send"
		if bound {
			name = "payExact binding authority"
		}
		t.Run(name, func(t *testing.T) {
			const blockTime = int64(1700000000000)
			directory := t.TempDir()
			open := func() (*testChain, *Host, func()) {
				t.Helper()
				db, err := dbm.NewDB("payment-restart", dbm.GoLevelDBBackend, directory)
				require.NoError(t, err)
				key := sdk.NewKVStoreKey("lxphost")
				cms := store.NewCommitMultiStore(db)
				cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
				require.NoError(t, cms.LoadLatestVersion())
				chain := &testChain{key: key, cms: cms}
				host := chain.host(t)
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
			chain, host, closeStore := open()
			payer, payee := newTestActor(t, 91), newTestActor(t, 92)
			asset := newTestAsset(t, payer, 0x51)
			setup := []struct {
				actor    testActor
				activity Activity
			}{
				{payer, payer.registerAsset(t, host, 0, blockTime, asset, 0x51, 1000)},
				{payer, payer.openAssetAccount(t, host, 1, blockTime, asset)},
				{payee, payee.openAssetAccount(t, host, 0, blockTime, asset)},
				{payer, payer.mint(t, host, 2, blockTime, asset, 25)},
			}
			for i, item := range setup {
				receipt := execute(t, host, chain.context(1, blockTime, uint64(i+1)), item.activity, item.actor.authority())
				require.Zero(t, receipt.ResultCode)
			}
			_, to, err := AssetAccountID(payee.public, asset.id)
			require.NoError(t, err)
			pay := func(reference IdempotencyKey, sequence, accountSequence uint64) (Receipt, error) {
				t.Helper()
				// Use the same send producer and reference/context binding as payExact.
				args := ledgertypes.SendArgs{
					From: asset.account, To: to, Asset: asset.id, Amount: sdk.NewInt(3),
					Sequence: accountSequence, IdempotencyKey: reference,
					ExpiresAt: uint64(blockTime + 1000), PublicKey: payer.public,
					NetworkID: testNetworkID, ProtocolVersion: stateCommitment,
				}
				payload, err := ledgertypes.EncodeSendPayload(args)
				require.NoError(t, err)
				material := append(append([]byte(nil), payload[4:116]...), reference[:]...)
				args.ContextHash, err = codec.DomainHash(codec.DomainContextHash, material)
				require.NoError(t, err)
				payload, err = ledgertypes.EncodeSendPayload(args)
				require.NoError(t, err)
				message := append([]byte(nil), payload[:2]...)
				message = append(message, payload[4:230]...)
				message = append(message, payload[326:364]...)
				digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
				require.NoError(t, err)
				copy(payload[262:326], ed25519.Sign(payer.private, digest[:]))
				identitySequence, err := host.ReadIdentitySequence(chain.context(2, blockTime, sequence), payer.did)
				require.NoError(t, err)
				activity := payer.activity(t, host, 0x00010005, identitySequence, blockTime, payload)
				activity.IdempotencyKey = reference
				hash, err := host.SigningHash(activity)
				require.NoError(t, err)
				activity.Signature = ed25519.Sign(payer.private, hash[:])
				authority := payer.authority()
				if bound {
					// payExact relies on verified binding authority for both signatures.
					clear(payload[262:326])
					activity.Signature = nil
					authority.Bound = true
				}
				return host.Execute(chain.context(2, blockTime, sequence), activity, authority)
			}
			reference := IdempotencyKey{0x71}
			paid, err := pay(reference, 5, 0)
			require.NoError(t, err)
			require.Zero(t, paid.ResultCode)
			require.NotEmpty(t, paid.Encoded)
			require.Equal(t, []TransferLeg{{Asset: asset.id, From: asset.account, To: to, Amount: sdk.NewInt(3)}}, paid.Transfers)
			beforeCommit := chain.dump()
			_, err = pay(reference, 6, 1)
			require.ErrorIs(t, err, Code(-302))
			require.EqualError(t, err, "LXP_ERR_IDEMPOTENT_REPLAY")
			require.Equal(t, beforeCommit, chain.dump(), "duplicate must not mutate the uncommitted branch")
			chain.commit()
			committed := chain.cms.LastCommitID()
			require.Positive(t, committed.Version)
			require.NotEmpty(t, committed.Hash)
			require.Equal(t, paid.Encoded, chain.get(idempotencyKey(reference)))
			closeStore()

			chain, host, closeStore = open()
			require.Equal(t, committed, chain.cms.LastCommitID(), "reopen must load the committed store version")
			require.Equal(t, paid.Encoded, chain.get(idempotencyKey(reference)), "reference must survive closing the disk store")
			before := chain.dump()
			duplicate, err := pay(reference, 6, 1)
			require.ErrorIs(t, err, Code(-302))
			require.EqualError(t, err, "LXP_ERR_IDEMPOTENT_REPLAY")
			require.Empty(t, duplicate.Encoded)
			require.Equal(t, before, chain.dump(), "duplicate must not mutate committed payment state")
			fresh := IdempotencyKey{0x72}
			receipt, err := pay(fresh, 6, 1)
			require.NoError(t, err)
			require.Zero(t, receipt.ResultCode)
			require.Equal(t, uint64(6), receipt.GlobalSequence)
			require.Equal(t, paid.Transfers, receipt.Transfers, "fresh reference must execute the same payment after reopen")
			chain.commit()
			require.Equal(t, receipt.Encoded, chain.get(idempotencyKey(fresh)))
			require.Equal(t, binary.BigEndian.AppendUint64(nil, 6), chain.get([]byte{lastSequenceKey}))
			for account, want := range map[[32]byte]int64{asset.account: 19, to: 6} {
				balance, err := host.ReadBalance(chain.context(3, blockTime, 7), account, asset.id)
				require.NoError(t, err)
				require.Equal(t, sdk.NewInt(want), balance)
			}
		})
	}
}
