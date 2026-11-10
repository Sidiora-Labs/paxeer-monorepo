// Proves: keeper.Keeper.ExportGenesis -> layerxpay.AppModule JSON / types.GenesisState -> keeper.Keeper.InitGenesis / keeper.Keeper.GetOffer
//
// Real signed kernel service activities produce the records. NewAppModule drives
// the application's actual JSON exporter and importer; both sides close and
// reopen their on-disk stores and hosts. A second export includes a real service
// withdrawal, so observing it requires reading fresh producer state.
//
// Keeper-owned service records are decoded at the JSON validation boundary and
// validated again before import writes.
package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	proofcodec "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const proofPayGenesisTime int64 = 1700000000000

type proofPayGenesisNode struct {
	ctx     sdk.Context
	cms     sdk.CommitMultiStore
	cdc     *codec.ProtoCodec
	payKey  *sdk.KVStoreKey
	hostKey *sdk.KVStoreKey
	evmKey  *sdk.KVStoreKey
	host    *lxphost.Host
	keeper  keeper.Keeper
	module  layerxpay.AppModule
	closed  bool
}

func openProofPayGenesisNode(t *testing.T, dir string) *proofPayGenesisNode {
	t.Helper()
	db, err := dbm.NewGoLevelDB("pay-genesis", dir)
	require.NoError(t, err)
	n := &proofPayGenesisNode{
		cdc:    codec.NewProtoCodec(cdctypes.NewInterfaceRegistry()),
		payKey: sdk.NewKVStoreKey(types.StoreKey), hostKey: sdk.NewKVStoreKey("lxphost"),
		evmKey: sdk.NewKVStoreKey(evmtypes.StoreKey),
	}
	n.cms = store.NewCommitMultiStore(db)
	t.Cleanup(func() { n.close(t) })
	n.cms.MountStoreWithDB(n.payKey, sdk.StoreTypeIAVL, db)
	n.cms.MountStoreWithDB(n.hostKey, sdk.StoreTypeIAVL, db)
	n.cms.MountStoreWithDB(n.evmKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, n.cms.LoadLatestVersion())
	n.ctx = sdk.NewContext(n.cms, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: n.cms.LastCommitID().Version + 1,
		Time: time.UnixMilli(proofPayGenesisTime),
	}, false)
	n.host, err = lxphost.NewHost(lxphost.Config{
		StoreKey: n.hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	n.keeper, err = keeper.NewKeeper(n.cdc, n.payKey, n.hostKey, n.host, testutil.NewEVMBindingReader(n.cdc, n.evmKey), nil, "pay-governance")
	require.NoError(t, err)
	n.module = layerxpay.NewAppModule(n.keeper)
	return n
}

func (n *proofPayGenesisNode) close(t *testing.T) {
	t.Helper()
	if n.closed {
		return
	}
	if n.host != nil {
		n.host.Close()
	}
	require.NoError(t, n.cms.Close())
	n.closed = true
}

func (n *proofPayGenesisNode) snapshot(t *testing.T) map[string]map[string][]byte {
	t.Helper()
	result := make(map[string]map[string][]byte)
	for _, key := range []*sdk.KVStoreKey{n.payKey, n.hostKey, n.evmKey} {
		values := make(map[string][]byte)
		iterator := n.ctx.KVStore(key).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			values[string(iterator.Key())] = bytes.Clone(iterator.Value())
		}
		require.NoError(t, iterator.Close())
		result[key.Name()] = values
	}
	return result
}

func TestProofPayGenesis(t *testing.T) {
	producerDir := t.TempDir()
	producer := openProofPayGenesisNode(t, producerDir)
	producer.keeper.SetParams(producer.ctx, types.DefaultParams())
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{114}, ed25519.SeedSize))
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	did := []byte("did:layerx:" + hex.EncodeToString(public[:]))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	id, asset, terms, spec := [32]byte{1}, [32]byte{2}, [32]byte{3}, [32]byte{4}
	price := proofcodec.U128{Lo: 10}
	payload, err := types.EncodeServiceOffer(id, asset, price, terms, spec,
		uint64(proofPayGenesisTime)+10000, 1000, 1000, 1, uint64(proofPayGenesisTime)+5000)
	require.NoError(t, err)
	var previousJSON []byte
	for round := uint64(0); round < 2; round++ {
		activityType := uint32(0x00050001)
		if round == 1 {
			activityType = 0x00050002
			payload, err = types.EncodeServiceIdentifier(id)
			require.NoError(t, err)
		}
		activity := lxphost.Activity{
			ProtocolVersion: 3, Type: activityType, ActorDID: did, AuthorityKey: public,
			AccountSequence: round, NotBefore: uint64(proofPayGenesisTime) - 1000,
			NotAfter:       uint64(proofPayGenesisTime) + 1000,
			IdempotencyKey: sha256.Sum256(append(bytes.Clone(payload), byte(round))), Payload: payload,
		}
		hash, err := producer.host.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(private, hash[:])
		receipt, err := producer.host.Execute(lxphost.WithSequence(producer.ctx, round+1), activity,
			lxphost.KernelAuthority{Actor: actor, Principal: actor, VerifiedKey: public})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
		want, found := producer.keeper.GetOffer(producer.ctx, id)
		require.True(t, found)
		require.Equal(t, id, want.OfferID)
		require.Equal(t, actor, want.OfferingAgent)
		require.Equal(t, price, want.Price)
		require.Equal(t, round == 1, want.Withdrawn)
		beforeClose := producer.snapshot(t)
		producer.cms.Commit(true)
		producer.close(t)
		producer = openProofPayGenesisNode(t, producerDir)
		require.Equal(t, beforeClose, producer.snapshot(t))
		exported := producer.keeper.ExportGenesis(producer.ctx)
		require.NoError(t, exported.Validate())
		require.NotEmpty(t, exported.KernelRecords)
		wire := producer.module.ExportGenesis(producer.ctx, producer.cdc)
		var decoded types.GenesisState
		require.NoError(t, json.Unmarshal(wire, &decoded))
		require.Equal(t, exported, &decoded)
		require.NotEqual(t, previousJSON, []byte(wire), "withdrawal must appear in the next export")
		previousJSON = bytes.Clone(wire)

		for delivery := 0; delivery < 2; delivery++ {
			t.Run(fmt.Sprintf("round_%d/delivery_%d", round, delivery), func(t *testing.T) {
				dir := t.TempDir()
				consumer := openProofPayGenesisNode(t, dir)
				_, found := consumer.keeper.GetOffer(consumer.ctx, id)
				require.False(t, found)
				require.NoError(t, consumer.module.ValidateGenesis(consumer.cdc, nil, wire))
				require.Empty(t, consumer.module.InitGenesis(consumer.ctx, consumer.cdc, wire))
				for _, record := range decoded.KernelRecords {
					require.Equal(t, record.Value, consumer.ctx.KVStore(consumer.hostKey).Get(record.Key))
					require.False(t, consumer.ctx.KVStore(consumer.payKey).Has(record.Key))
				}
				consumer.cms.Commit(true)
				consumer.close(t)
				consumer = openProofPayGenesisNode(t, dir)
				for read := 0; read < 2; read++ {
					got, found := consumer.keeper.GetOffer(consumer.ctx, id)
					require.True(t, found)
					require.Equal(t, want, got)
					require.Equal(t, exported, consumer.keeper.ExportGenesis(consumer.ctx))
					require.Equal(t, exported.Params, consumer.keeper.GetParams(consumer.ctx))
				}

				before := consumer.snapshot(t)
				malformedJSON := wire[:len(wire)-1]
				require.ErrorContains(t, consumer.module.ValidateGenesis(consumer.cdc, nil, malformedJSON),
					"failed to unmarshal layerxpay genesis state")
				require.Panics(t, func() { consumer.module.InitGenesis(consumer.ctx, consumer.cdc, malformedJSON) })
				require.Equal(t, before, consumer.snapshot(t))

				var invalid types.GenesisState
				require.NoError(t, json.Unmarshal(wire, &invalid))
				invalid.KernelRecords = append(invalid.KernelRecords, invalid.KernelRecords[0])
				invalidJSON, err := json.Marshal(invalid)
				require.NoError(t, err)
				require.ErrorIs(t, consumer.module.ValidateGenesis(consumer.cdc, nil, invalidJSON), types.ErrInvalidGenesis)
				require.ErrorIs(t, consumer.keeper.InitGenesis(consumer.ctx, invalid), types.ErrInvalidGenesis)
				require.Panics(t, func() { consumer.module.InitGenesis(consumer.ctx, consumer.cdc, invalidJSON) })
				require.Equal(t, before, consumer.snapshot(t))
			})
		}

		t.Run(fmt.Sprintf("round_%d/opaque_record_decode_refusal", round), func(t *testing.T) {
			var malformed types.GenesisState
			require.NoError(t, json.Unmarshal(wire, &malformed))
			key := types.KernelKVKey(types.ServiceModuleID, append([]byte("offer::1"), id[:]...))
			changed := false
			for i := range malformed.KernelRecords {
				if bytes.Equal(malformed.KernelRecords[i].Key, key) {
					malformed.KernelRecords[i].Value = malformed.KernelRecords[i].Value[:1]
					changed = true
				}
			}
			require.True(t, changed)
			badWire, err := json.Marshal(malformed)
			require.NoError(t, err)
			consumer := openProofPayGenesisNode(t, t.TempDir())
			validationErr := consumer.module.ValidateGenesis(consumer.cdc, nil, badWire)
			require.ErrorIs(t, validationErr, types.ErrInvalidGenesis)
			require.ErrorContains(t, validationErr, "invalid offer::1 record body: LXP_NON_CANONICAL")
			require.ErrorIs(t, keeper.ValidateOwnedGenesis(malformed), types.ErrInvalidGenesis)
			require.Empty(t, consumer.module.InitGenesis(consumer.ctx, consumer.cdc, wire))
			before := consumer.snapshot(t)
			require.ErrorIs(t, consumer.keeper.InitGenesis(consumer.ctx, malformed), types.ErrInvalidGenesis)
			require.Equal(t, before, consumer.snapshot(t))
			// AppModule's SDK initialization contract translates keeper errors to panics,
			// but neither this failed init nor a subsequent public read writes any state.
			require.Panics(t, func() { consumer.module.InitGenesis(consumer.ctx, consumer.cdc, badWire) })
			require.Equal(t, before, consumer.snapshot(t))
			got, found, err := consumer.keeper.ServiceViews().GetOffer(consumer.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, want, got)
			require.Equal(t, before, consumer.snapshot(t))
		})
	}
}
