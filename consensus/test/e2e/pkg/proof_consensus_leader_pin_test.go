// Proves: Manifest.LeaderPin/LeaderPinHeight and Manifest.Save -> TOML file and LoadManifest -> LoadTestnet and Testnet.Validate.
//
// The consumer uses the exported surface used by the e2e runner: ordered node
// references, their signing public keys, and the activation height. Every read
// decodes the producer's file anew; there is no persistent component to restart.
// This proof repairs consensus/test/e2e/pkg/testnet.go Testnet.Validate accepting
// an activation height without leaders, and consensus/test/e2e/pkg/manifest.go
// Manifest.Save leaving the transport file open and ignoring close errors.
package e2e_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
)

func TestProofConsensusLeaderPin(t *testing.T) {
	producer := e2e.Manifest{
		InitialHeight:   10,
		LeaderPin:       []string{"validator02", "validator01"},
		LeaderPinHeight: 20,
		Nodes: map[string]*e2e.ManifestNode{
			"validator01": {Mode: string(e2e.ModeValidator)},
			"validator02": {Mode: string(e2e.ModeValidator)},
		},
	}
	file := filepath.Join(t.TempDir(), "leaders.toml")
	require.NoError(t, producer.Save(file))

	var previous *e2e.Testnet
	for range 2 {
		decoded, err := e2e.LoadManifest(file)
		require.NoError(t, err)
		require.Equal(t, producer.LeaderPin, decoded.LeaderPin)
		require.Equal(t, producer.LeaderPinHeight, decoded.LeaderPinHeight)

		consumer, err := e2e.LoadTestnet(file)
		require.NoError(t, err)
		require.NoError(t, consumer.Validate())
		require.Equal(t, producer.InitialHeight, consumer.InitialHeight)
		require.Equal(t, producer.LeaderPinHeight, consumer.LeaderPinHeight)
		require.Len(t, consumer.LeaderPin, len(producer.LeaderPin))
		for i, name := range producer.LeaderPin {
			leader := consumer.LeaderPin[i]
			require.Equal(t, name, leader.Name)
			require.Same(t, consumer.LookupNode(name), leader)
			require.Equal(t, e2e.ModeValidator, leader.Mode)
			require.Positive(t, consumer.Validators[leader])
			require.Len(t, leader.PrivvalKey.Public().Bytes(), 32)
			if previous != nil {
				require.NotSame(t, previous.LeaderPin[i], leader)
				require.Equal(t, previous.LeaderPin[i].PrivvalKey.Public(), leader.PrivvalKey.Public())
			}
		}
		require.NotEqual(t, consumer.LeaderPin[0].PrivvalKey.Public(), consumer.LeaderPin[1].PrivvalKey.Public())
		previous = consumer
	}

	// Replace the producer result at the same path to prove reads are not cached.
	producer.LeaderPin = []string{"validator01", "validator02"}
	producer.LeaderPinHeight = 30
	require.NoError(t, producer.Save(file))
	consumer, err := e2e.LoadTestnet(file)
	require.NoError(t, err)
	require.Equal(t, int64(30), consumer.LeaderPinHeight)
	for i, name := range producer.LeaderPin {
		require.Same(t, consumer.LookupNode(name), consumer.LeaderPin[i])
		require.Equal(t, previous.LookupNode(name).PrivvalKey.Public(), consumer.LeaderPin[i].PrivvalKey.Public())
	}

	// Truncate the real producer's bytes inside a pin name, at the file crossing.
	encoded, err := os.ReadFile(file)
	require.NoError(t, err)
	cut := bytes.Index(encoded, []byte(`"validator01"`))
	require.GreaterOrEqual(t, cut, 0)
	require.NoError(t, os.WriteFile(file, encoded[:cut+len(`"validator01`)], 0600))
	_, err = e2e.LoadManifest(file)
	var parseErr toml.ParseError
	require.ErrorAs(t, err, &parseErr)
	consumer, err = e2e.LoadTestnet(file)
	require.ErrorAs(t, err, &parseErr)
	require.Nil(t, consumer)
}

func TestProofConsensusLeaderPinRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		leaders []string
		height  int64
		mode    e2e.Mode
	}{
		{name: "unknown identifier", leaders: []string{"missing"}, height: 20, mode: e2e.ModeValidator},
		{name: "duplicate identifier", leaders: []string{"validator01", "validator01"}, height: 20, mode: e2e.ModeValidator},
		{name: "non-validator", leaders: []string{"validator01"}, height: 20, mode: e2e.ModeFull},
		{name: "missing activation", leaders: []string{"validator01"}, mode: e2e.ModeValidator},
		{name: "activation below initial height", leaders: []string{"validator01"}, height: 9, mode: e2e.ModeValidator},
		{name: "negative activation", leaders: []string{"validator01"}, height: -1, mode: e2e.ModeValidator},
		{name: "activation without leaders", height: 20, mode: e2e.ModeValidator},
	} {
		t.Run(tc.name, func(t *testing.T) {
			producer := e2e.Manifest{
				InitialHeight:   10,
				LeaderPin:       tc.leaders,
				LeaderPinHeight: tc.height,
				Nodes: map[string]*e2e.ManifestNode{
					"validator01": {Mode: string(tc.mode)},
				},
			}
			file := filepath.Join(t.TempDir(), "refused.toml")
			require.NoError(t, producer.Save(file))
			decoded, err := e2e.LoadManifest(file)
			require.NoError(t, err)
			require.Len(t, decoded.LeaderPin, len(producer.LeaderPin))
			for i, name := range producer.LeaderPin {
				require.Equal(t, name, decoded.LeaderPin[i])
			}
			require.Equal(t, producer.LeaderPinHeight, decoded.LeaderPinHeight)
			consumer, err := e2e.LoadTestnet(file)
			require.Error(t, err)
			if tc.name == "unknown identifier" {
				require.Nil(t, consumer)
			} else {
				require.NotNil(t, consumer)
				require.Error(t, consumer.Validate())
			}
		})
	}
}

func TestProofConsensusLeaderPinDisabled(t *testing.T) {
	producer := e2e.Manifest{
		Nodes: map[string]*e2e.ManifestNode{"validator01": {}},
	}
	file := filepath.Join(t.TempDir(), "disabled.toml")
	require.NoError(t, producer.Save(file))
	for range 2 {
		consumer, err := e2e.LoadTestnet(file)
		require.NoError(t, err)
		require.Empty(t, consumer.LeaderPin)
		require.Zero(t, consumer.LeaderPinHeight)
		require.NoError(t, consumer.Validate())
	}
}
