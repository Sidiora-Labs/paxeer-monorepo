package sequencer_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// Loaded into the existing native node integration package using Go's overlay.
// The producer, root RPC registration, execution, WAL and standby are real.
func TestRepairRampNode2231211(t *testing.T) {
	admissionChainID(t, 125)
	h := newRPCE2EHarness(t)
	primary := h.node.App.GetCheckCtx().WithBlockHeight(openHeight)
	require.Contains(t, h.node.App.EvmKeeper.CustomPrecompiles(primary), common.HexToAddress(layerxledger.LedgerAddress))
	backup := h.backup.GetCheckCtx().WithBlockHeight(openHeight)
	require.Contains(t, h.backup.EvmKeeper.CustomPrecompiles(backup), common.HexToAddress(layerxledger.LedgerAddress))
	directory := os.Getenv("RAMP_REPAIR_EXCHANGE")
	require.NotEmpty(t, directory)
	public := h.node.ReceiptKey.PublicKey()
	key, err := h.node.ReceiptKey.KeyID(openHeight)
	require.NoError(t, err)
	data, err := json.Marshal(map[string]interface{}{
		"endpoint":   h.url,
		"secret":     hex.EncodeToString(crypto.FromECDSA(h.sender.EvmPrivateKey)),
		"operator":   hex.EncodeToString(h.sender.EvmAddress[:]),
		"key_id":     hex.EncodeToString(key[:]),
		"public_key": hex.EncodeToString(public[:]),
		"height":     openHeight,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "node.json"), data, 0600))
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(directory, "stop")); err == nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "Rust consumer must finish against the running node")
		time.Sleep(10 * time.Millisecond)
	}
}
