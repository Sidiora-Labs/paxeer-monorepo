package simulation_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ibc "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core"
	clienttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	connectiontypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/03-connection/types"
	channeltypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/04-channel/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	host "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/24-host"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/simulation"
	solotypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/06-solomachine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

func TestRepairM_1221_CreationAllocationDiffs(t *testing.T) {
	app := simapp.Setup(false)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	ctx := app.NewContext(false, tmproto.Header{ChainID: "repair-1221", Height: 10, Time: time.Unix(1700000000, 0).UTC()})
	k := app.IBCKeeper
	registry := sdk.StoreDecoderRegistry{}
	ibc.NewAppModule(k).RegisterStoreDecoder(registry)
	decode := registry[host.StoreKey]
	require.NotNil(t, decode)
	store := ctx.KVStore(app.GetKey(host.StoreKey))
	pubKey, err := codectypes.NewAnyWithValue(ed25519.GenPrivKeyFromSecret([]byte("repair-1221")).PubKey())
	require.NoError(t, err)
	consensus := &solotypes.ConsensusState{PublicKey: pubKey, Timestamp: uint64(ctx.BlockTime().Unix())}
	client := solotypes.NewClientState(1, consensus, false)
	require.NoError(t, client.Validate())
	require.NoError(t, consensus.ValidateBasic())
	portID := "repair1221"
	portCap := k.PortKeeper.BindPort(ctx, portID)

	for _, sequence := range []uint64{0, 255, 1<<63 + 255, math.MaxUint64 - 1} {
		t.Run(fmt.Sprint(sequence), func(t *testing.T) {
			k.ClientKeeper.SetNextClientSequence(ctx, sequence)
			k.ConnectionKeeper.SetNextConnectionSequence(ctx, sequence)
			k.ChannelKeeper.SetNextChannelSequence(ctx, sequence)
			checkDiff := func(key, label string, create func()) {
				t.Helper()
				before := kv.Pair{Key: []byte(key), Value: bytes.Clone(store.Get([]byte(key)))}
				create()
				after := kv.Pair{Key: []byte(key), Value: bytes.Clone(store.Get([]byte(key)))}
				require.Len(t, before.Value, 8)
				require.Len(t, after.Value, 8)
				require.Equal(t, sequence, binary.BigEndian.Uint64(before.Value))
				require.Equal(t, sequence+1, binary.BigEndian.Uint64(after.Value))
				require.Equal(t, fmt.Sprintf("%s A: %d\n%s B: %d", label, sequence, label, sequence+1), decode(before, after))
				require.Equal(t, fmt.Sprintf("%s A: %d\n%s B: %d", label, sequence+1, label, sequence), decode(after, before))
			}
			var clientID, connectionID string
			checkDiff(clienttypes.KeyNextClientSequence, "NextClientSequence", func() {
				clientID, err = k.ClientKeeper.CreateClient(ctx, client, consensus)
				require.NoError(t, err)
				require.Equal(t, clienttypes.FormatClientIdentifier(client.ClientType(), sequence), clientID)
				stored, found := k.ClientKeeper.GetClientState(ctx, clientID)
				require.True(t, found)
				require.Equal(t, client, stored)
			})
			checkDiff(connectiontypes.KeyNextConnectionSequence, "NextConnectionSequence", func() {
				counterparty := connectiontypes.NewCounterparty(clientID, "", commitmenttypes.NewMerklePrefix([]byte(host.StoreKey)))
				connectionID, err = k.ConnectionKeeper.ConnOpenInit(ctx, clientID, counterparty, nil, 0)
				require.NoError(t, err)
				require.Equal(t, connectiontypes.FormatConnectionIdentifier(sequence), connectionID)
				connection, found := k.ConnectionKeeper.GetConnection(ctx, connectionID)
				require.True(t, found)
				require.Equal(t, connectiontypes.INIT, connection.State)
				require.Equal(t, clientID, connection.ClientId)
			})
			checkDiff(channeltypes.KeyNextChannelSequence, "NextChannelSequence", func() {
				counterparty := channeltypes.NewCounterparty(portID, "")
				hops := []string{connectionID}
				channelID, channelCap, err := k.ChannelKeeper.ChanOpenInit(ctx, channeltypes.UNORDERED, hops, portID, portCap, counterparty, "repair-1221")
				require.NoError(t, err)
				require.NotNil(t, channelCap)
				require.Equal(t, channeltypes.FormatChannelIdentifier(sequence), channelID)
				k.ChannelKeeper.WriteOpenInitChannel(ctx, portID, channelID, channeltypes.UNORDERED, hops, counterparty, "repair-1221")
				channel, found := k.ChannelKeeper.GetChannel(ctx, portID, channelID)
				require.True(t, found)
				require.Equal(t, channeltypes.INIT, channel.State)
				require.Equal(t, hops, channel.ConnectionHops)
			})
		})
	}
}

func TestRepairM_1221_RejectUnknownKeys(t *testing.T) {
	decode := simulation.NewDecodeStore(keeper.Keeper{})
	for _, key := range []string{"unknown", "nextSequence", clienttypes.KeyNextClientSequence + "/suffix", connectiontypes.KeyNextConnectionSequence + "/suffix", channeltypes.KeyNextChannelSequence + "/suffix", "x" + clienttypes.KeyNextClientSequence} {
		t.Run(key, func(t *testing.T) {
			pair := kv.Pair{Key: []byte(key), Value: sdk.Uint64ToBigEndian(1)}
			require.PanicsWithValue(t, fmt.Sprintf("invalid %s key prefix: %s", host.ModuleName, key), func() { decode(pair, pair) })
		})
	}
}

func TestRepairM_1221_RejectMalformedAllocationValues(t *testing.T) {
	decode := simulation.NewDecodeStore(keeper.Keeper{})
	for _, key := range []string{clienttypes.KeyNextClientSequence, connectiontypes.KeyNextConnectionSequence, channeltypes.KeyNextChannelSequence} {
		for _, length := range []int{0, 1, 7, 9, 16} {
			t.Run(fmt.Sprintf("%s/length_%d", key, length), func(t *testing.T) {
				valid := kv.Pair{Key: []byte(key), Value: sdk.Uint64ToBigEndian(1)}
				invalid := kv.Pair{Key: []byte(key), Value: make([]byte, length)}
				message := fmt.Sprintf("invalid %s allocation value: %s", host.ModuleName, key)
				require.PanicsWithValue(t, message, func() { decode(invalid, valid) })
				require.PanicsWithValue(t, message, func() { decode(valid, invalid) })
			})
		}
	}
}
